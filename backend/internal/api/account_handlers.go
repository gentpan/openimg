package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type deleteAccountReq struct {
	// Confirm must equal the account's own email. Typing it out is the
	// standard friction for an irreversible action — a checkbox is too easy
	// to click through, and a password prompt excludes OAuth-only accounts.
	Confirm string `json:"confirm" binding:"required"`
}

// DELETE /api/account — erase the account and everything it owns.
//
// Ordering matters: images are soft-deleted and queued for object removal
// *before* the user row goes, because the purge job resolves the storage
// profile from the image row and the reference count from siblings. Deleting
// the user first would orphan the objects on disk forever.
func (s *Server) handleDeleteAccount(c *gin.Context) {
	u := auth.MustUser(c)

	var req deleteAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.Confirm), u.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "确认文本与账号邮箱不一致"})
		return
	}
	// The last admin deleting themselves would lock everyone out of the
	// moderation queue.
	if u.IsAdmin() {
		var admins int64
		s.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&admins)
		if admins <= 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "这是最后一个管理员账号，不能删除"})
			return
		}
	}

	var images []models.Image
	s.DB.Where("user_id = ? AND deleted_at IS NULL", u.ID).Find(&images)

	now := time.Now()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Image{}).
			Where("user_id = ? AND deleted_at IS NULL", u.ID).
			Updates(map[string]any{"deleted_at": now, "status": models.ImageDeleted}).Error; err != nil {
			return err
		}
		// Rows that only make sense alongside the user.
		for _, m := range []any{
			&models.Session{}, &models.APIToken{},
			&models.QuotaTransaction{}, &models.CheckinRecord{},
		} {
			if err := tx.Where("user_id = ?", u.ID).Delete(m).Error; err != nil {
				return err
			}
		}
		// Their own buckets: forget the credentials, keep nothing encrypted.
		if err := tx.Where("user_id = ?", u.ID).Delete(&models.StorageProfile{}).Error; err != nil {
			return err
		}
		// Detach anyone they referred so the FK doesn't dangle.
		if err := tx.Model(&models.User{}).Where("referred_by_id = ?", u.ID).
			Update("referred_by_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, "id = ?", u.ID).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败：" + err.Error()})
		return
	}

	// Queue object removal after the transaction commits — the purge job reads
	// the image rows, which must already be soft-deleted for it to act.
	if s.Queue != nil {
		for _, img := range images {
			s.Queue.Submit(scheduler.JobPurge, img.ID)
		}
	}
	s.Auth.Logout(c)
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted_images": len(images)})
}
