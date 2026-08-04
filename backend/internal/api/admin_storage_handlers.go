package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// The public base URL is the domain browsers fetch images from. It is stored
// on the platform storage profile rather than read from the environment on
// every request, because the profile is created once at first boot and the env
// var is only consulted then — so after the first start, changing
// S3_PUBLIC_URL_BASE has no effect and this endpoint is the only way to move
// the site to a different image domain.
//
// It is deliberately independent of the site's own origin: serving images from
// a separate domain keeps session cookies off image requests and lets the CDN
// in front of the bucket be configured without touching the app.

type platformStorageOut struct {
	Configured    bool   `json:"configured"`
	Bucket        string `json:"bucket"`
	Endpoint      string `json:"endpoint"`
	KeyPrefix     string `json:"key_prefix"`
	PublicBaseURL string `json:"public_base_url"`
	ThumbBaseURL  string `json:"thumb_base_url"`
	SiteBaseURL   string `json:"site_base_url"`
	SampleURL     string `json:"sample_url"`
	ImageCount    int64  `json:"image_count"`
}

// GET /admin/api/storage/platform
func (s *Server) adminPlatformStorage(c *gin.Context) {
	var p models.StorageProfile
	err := s.DB.Where("kind = ? AND backup_of_id IS NULL", models.ProfilePlatform).
		Order("is_default DESC, created_at ASC").First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, platformStorageOut{Configured: false, SiteBaseURL: s.PublicBaseURL})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var n int64
	s.DB.Model(&models.Image{}).Where("profile_id = ? AND deleted_at IS NULL", p.ID).Count(&n)

	// A real key from a real row, so the admin can see the exact shape of the
	// links this setting produces instead of guessing from a template.
	sample := "img/ab/cd/abcd…ef.jpg"
	var one models.Image
	if s.DB.Where("profile_id = ? AND deleted_at IS NULL", p.ID).First(&one).Error == nil {
		sample = one.ObjectKey
	}

	c.JSON(http.StatusOK, platformStorageOut{
		Configured:    true,
		Bucket:        p.Bucket,
		Endpoint:      p.Endpoint,
		KeyPrefix:     p.KeyPrefix,
		PublicBaseURL: p.PublicBaseURL,
		ThumbBaseURL:  p.ThumbBaseURL,
		SiteBaseURL:   s.PublicBaseURL,
		SampleURL:     storage.URLFor(&p, sample, s.PublicBaseURL),
		ImageCount:    n,
	})
}

type platformStorageReq struct {
	PublicBaseURL string `json:"public_base_url" binding:"required"`
	// Optional. Empty leaves display sizes on the main origin.
	ThumbBaseURL string `json:"thumb_base_url"`
}

// PATCH /admin/api/storage/platform — repoint every image URL at a new domain.
//
// This rewrites nothing: object keys are unchanged and no file moves. URLs are
// assembled at read time from this field, so the switch takes effect on the
// next page load for images already stored. The corollary is that the new
// domain must already serve the same bucket — otherwise every image 404s the
// moment this is saved.
func (s *Server) adminUpdatePlatformStorage(c *gin.Context) {
	var req platformStorageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	base := strings.TrimRight(strings.TrimSpace(req.PublicBaseURL), "/")
	if err := storage.ValidatePublicBaseURL(base); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var p models.StorageProfile
	if err := s.DB.Where("kind = ? AND backup_of_id IS NULL", models.ProfilePlatform).
		Order("is_default DESC, created_at ASC").First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚未配置平台存储"})
		return
	}
	updates := map[string]any{"public_base_url": base}
	thumb := strings.TrimRight(strings.TrimSpace(req.ThumbBaseURL), "/")
	if thumb != "" {
		if err := storage.ValidatePublicBaseURL(thumb); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缩略图域名：" + err.Error()})
			return
		}
	}
	updates["thumb_base_url"] = thumb

	if err := s.DB.Model(&p).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if s.Storage != nil {
		s.Storage.Invalidate(p.ID)
	}

	var n int64
	s.DB.Model(&models.Image{}).Where("profile_id = ? AND deleted_at IS NULL", p.ID).Count(&n)
	c.JSON(http.StatusOK, gin.H{"ok": true, "public_base_url": base, "thumb_base_url": thumb, "affected": n})
}
