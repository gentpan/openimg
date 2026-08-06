package api

import (
	"context"
	"fmt"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// bulkDeleteMax bounds one request. Past this the client pages, which keeps a
// single transaction from holding thousands of rows and lets the UI report
// progress instead of hanging on one opaque call.
const bulkDeleteMax = 500

type bulkDeleteReq struct {
	IDs []string `json:"ids"`
	// All ignores IDs and takes everything the user owns, optionally narrowed
	// by the same search string the gallery is showing. Sent by "清空图库", so
	// what gets deleted matches what the user was looking at.
	All   bool   `json:"all"`
	Query string `json:"q"`
	// Code is the emailed confirmation, required only for All. Deleting a
	// selection is recoverable in the sense that the user picked each item;
	// "delete everything I have ever uploaded" is not, so it gets the same
	// gate as a credential change.
	Code string `json:"code"`
}

// POST /api/images/bulk-delete
//
// Same semantics as deleting one by one, but the quota refund is a single
// ledger entry rather than one per image: the ledger is replayed by summing,
// so one row for "released N bytes" reconciles identically while keeping a
// 500-image delete from writing 500 receipts.
func (s *Server) handleBulkDelete(c *gin.Context) {
	u := auth.MustUser(c)
	var req bulkDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "auth.bad_params")})
		return
	}

	if req.All {
		if len(req.Code) != 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "gallery.purge_otp_required")})
			return
		}
		if status, key := s.consumeOTP(u.Email, req.Code, models.OTPPurposePurge); key != "" {
			c.JSON(status, gin.H{"error": i18n.T(c, key)})
			return
		}
	}

	q := s.DB.Where("user_id = ? AND deleted_at IS NULL", u.ID)
	switch {
	case req.All:
		if t := strings.TrimSpace(req.Query); t != "" {
			q = q.Where("orig_name ILIKE ?", "%"+t+"%")
		}
	case len(req.IDs) > 0:
		ids := make([]uuid.UUID, 0, len(req.IDs))
		for _, raw := range req.IDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "gallery.bad_image_id")})
				return
			}
			ids = append(ids, id)
		}
		q = q.Where("id IN ?", ids)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "gallery.no_selection")})
		return
	}

	var images []models.Image
	if err := q.Order("created_at ASC").Limit(bulkDeleteMax).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(images) == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": 0, "remaining": 0})
		return
	}

	// Which profiles are the platform pool — only those consumed quota.
	platform := map[uuid.UUID]bool{}
	var profiles []models.StorageProfile
	s.DB.Find(&profiles)
	for _, p := range profiles {
		platform[p.ID] = p.IsPlatform()
	}

	ids := make([]uuid.UUID, 0, len(images))
	var refund int64
	for _, img := range images {
		ids = append(ids, img.ID)
		if platform[img.ProfileID] {
			refund += img.SizeStored
		}
	}

	if err := s.DB.Model(&models.Image{}).Where("id IN ?", ids).Updates(map[string]any{
		"deleted_at": time.Now(),
		"status":     models.ImageDeleted,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if refund > 0 {
		if err := quota.Release(s.DB, u.ID, refund, fmt.Sprintf("批量删除 %d 张图片", len(ids)), nil); err != nil {
			// The rows are already gone; failing the response here would tell
			// the user nothing happened when it did. Log for the reconcile
			// sweep instead — it recomputes usage from the surviving rows.
			log.Printf("bulk delete: quota release failed for user %s: %v", u.ID, err)
		}
	}

	// Detached so a full queue can't stall the response, and blocking so no
	// purge is dropped — a dropped purge leaks the object permanently.
	if s.Queue != nil {
		go func(ids []uuid.UUID) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			for _, id := range ids {
				if err := s.Queue.SubmitWait(ctx, scheduler.JobPurge, id); err != nil {
					log.Printf("bulk delete: purge enqueue aborted at %s: %v", id, err)
					return
				}
			}
		}(ids)
	}

	// remaining lets the client loop until a "delete everything" really is.
	var remaining int64
	if req.All {
		rq := s.DB.Model(&models.Image{}).Where("user_id = ? AND deleted_at IS NULL", u.ID)
		if t := strings.TrimSpace(req.Query); t != "" {
			rq = rq.Where("orig_name ILIKE ?", "%"+t+"%")
		}
		rq.Count(&remaining)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": len(ids), "remaining": remaining})
}
