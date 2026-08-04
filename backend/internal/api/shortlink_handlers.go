package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Short links: openimg.io/aB3d → 302 → the image on the CDN domain.
//
// A real redirect rather than a client-side one. The whole point of a short
// link is that it can be pasted anywhere — a forum post, a README, an <img
// src> — and all of those need an HTTP redirect, not a page that loads React
// and then navigates.

// newShortCode finds an unused code, starting short and widening only when the
// space at that length is crowded.
//
// Four characters is 14.7M codes; at a hundred thousand images that is under
// 1% occupancy, so a retry is rare. Widening on repeated failure means the
// links stay short for as long as they can rather than being padded from day
// one against a size the site may never reach.
func (s *Server) newShortCode(db *gorm.DB) (string, error) {
	for length := 4; length <= 6; length++ {
		for attempt := 0; attempt < 8; attempt++ {
			code := models.NewShortCode(length)
			var n int64
			if err := db.Model(&models.Image{}).Where("short_code = ?", code).Count(&n).Error; err != nil {
				return "", err
			}
			if n == 0 {
				return code, nil
			}
		}
		log.Printf("shortlink: %d-character space is crowded, widening", length)
	}
	return "", errors.New("生成短链失败")
}

// GET /:code — resolve a short link.
//
// Mounted at the root, so it sees every unmatched path on the domain. It
// rejects anything that is not code-shaped before touching the database, and
// hands everything else to the SPA (or a 404 when the SPA is not served from
// here), which is what keeps /login and friends working.
func (s *Server) handleShortLink(c *gin.Context) {
	code := c.Param("code")
	if !models.IsValidShortCode(code) {
		s.serveAppOrNotFound(c)
		return
	}

	var img models.Image
	err := s.DB.Where("short_code = ? AND deleted_at IS NULL", code).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.serveAppOrNotFound(c)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if img.Status == models.ImageBlocked {
		c.JSON(http.StatusGone, gin.H{"error": "该图片已被屏蔽"})
		return
	}

	var p models.StorageProfile
	profile := (*models.StorageProfile)(nil)
	if s.DB.First(&p, "id = ?", img.ProfileID).Error == nil {
		profile = &p
	}
	target := storage.URLFor(profile, img.ObjectKey, s.PublicBaseURL)

	// Counted here rather than on the image URL: the CDN serves those without
	// ever reaching us, so a short-link hit is the only view we can actually
	// observe. Best-effort — a failed counter must not break the redirect.
	go func(id string) {
		if err := s.DB.Model(&models.Image{}).Where("id = ?", id).
			UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
			log.Printf("shortlink: view count for %s failed: %v", id, err)
		}
	}(img.ID.String())

	// 302, not 301: a permanent redirect would be cached by browsers forever,
	// and the target changes whenever the image domain does.
	c.Redirect(http.StatusFound, target)
}

// backfillShortCodes gives existing images a short link.
//
// Runs once at boot rather than in a migration so a half-finished pass simply
// resumes next start; every row is independent and the unique index is what
// actually guarantees correctness.
func (s *Server) BackfillShortCodes() {
	var pending []models.Image
	if err := s.DB.Where("short_code = '' OR short_code IS NULL").
		Limit(5000).Find(&pending).Error; err != nil {
		log.Printf("shortlink: backfill query failed: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	done := 0
	for _, img := range pending {
		code, err := s.newShortCode(s.DB)
		if err != nil {
			log.Printf("shortlink: backfill stopped at %d/%d: %v", done, len(pending), err)
			break
		}
		if err := s.DB.Model(&models.Image{}).Where("id = ?", img.ID).
			Update("short_code", code).Error; err != nil {
			log.Printf("shortlink: backfill for %s failed: %v", img.ID, err)
			continue
		}
		done++
	}
	log.Printf("shortlink: backfilled %d/%d images", done, len(pending))
}
