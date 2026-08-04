package api

import (
	"net/http"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

type preferencesReq struct {
	UploadMode    *string `json:"upload_mode"`
	VariantFormat *string `json:"variant_format"`
	MaxImageWidth *int    `json:"max_image_width"`
}

var (
	allowedModes    = map[string]bool{"optimized": true, "original": true}
	allowedVariants = map[string]bool{"none": true, "webp": true, "avif": true}
)

// allowedWidths are the presets the UI offers. Restricting to a list keeps a
// stray value like 37 from silently destroying someone's uploads.
var allowedWidths = map[int]bool{0: true, 1280: true, 1920: true, 2560: true, 3840: true}

// PATCH /api/preferences — per-user conversion settings.
//
// These only affect *future* uploads. Re-deriving every existing image when a
// user flips a switch would be a surprising amount of work to trigger from a
// checkbox, and would change quota usage retroactively.
func (s *Server) handleUpdatePreferences(c *gin.Context) {
	u := auth.MustUser(c)
	var req preferencesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{}
	if req.UploadMode != nil {
		if !allowedModes[*req.UploadMode] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "upload_mode 必须是 optimized 或 original"})
			return
		}
		updates["upload_mode"] = *req.UploadMode
	}
	if req.VariantFormat != nil {
		// One derivative at most: any browser that decodes AVIF also decodes
		// WebP, so keeping both is paying twice for the same fallback.
		if !allowedVariants[*req.VariantFormat] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "variant_format 必须是 none / webp / avif"})
			return
		}
		updates["variant_format"] = *req.VariantFormat
	}
	if req.MaxImageWidth != nil {
		if !allowedWidths[*req.MaxImageWidth] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的宽度预设"})
			return
		}
		updates["max_image_width"] = *req.MaxImageWidth
	}
	if len(updates) > 0 {
		if err := s.DB.Model(u).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
