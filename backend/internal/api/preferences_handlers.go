package api

import (
	"net/http"
	"time"

	"github.com/gentpan/openimg/backend/internal/i18n"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

type preferencesReq struct {
	UploadMode    *string `json:"upload_mode"`
	VariantFormat *string `json:"variant_format"`
	MaxImageWidth *int    `json:"max_image_width"`
	ThumbWidth    *int    `json:"thumb_width"`
	ThumbFormat   *string `json:"thumb_format"`
	// Timezone 决定这个账号的"一天"从几点开始,签到与日限都按它算。
	// IANA 名,客户端启动时报一次。
	Timezone *string `json:"timezone"`

	// 关掉之后不再收沉寂提醒。指针类型:不传表示"这次不动这一项"。
	EmailNotify *bool `json:"email_notify"`
}

var (
	allowedModes    = map[string]bool{"optimized": true, "original": true}
	allowedVariants = map[string]bool{"none": true, "webp": true, "avif": true}
)

// allowedWidths are the presets the UI offers. Restricting to a list keeps a
// stray value like 37 from silently destroying someone's uploads.
var allowedWidths = map[int]bool{0: true, 1280: true, 1920: true, 2560: true, 3840: true}

// Thumbnail policy presets. Same reasoning: the variant name and object key
// embed the width, so an arbitrary number would mint an unbounded family of
// key shapes.
var (
	thumbWidthPresets  = map[int]bool{200: true, 400: true, 600: true, 800: true, 1000: true}
	thumbFormatPresets = map[string]bool{"webp": true, "avif": true, "jpg": true}
)

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
	if req.Timezone != nil {
		// 用 LoadLocation 校验:它只认时区库里真实存在的名字,顺带挡掉任何
		// 想借这个字段塞路径的写法。空串是合法的,表示回到 UTC。
		if *req.Timezone != "" {
			if _, err := time.LoadLocation(*req.Timezone); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "prefs.bad_timezone")})
				return
			}
		}
		updates["timezone"] = *req.Timezone
	}
	if req.EmailNotify != nil {
		updates["email_notify"] = *req.EmailNotify
	}
	if req.UploadMode != nil {
		if !allowedModes[*req.UploadMode] {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "prefs.bad_upload_mode")})
			return
		}
		updates["upload_mode"] = *req.UploadMode
	}
	if req.VariantFormat != nil {
		// 单选:转换语义下主图只能编码成一种格式;况且能解 AVIF 的浏览器
		// 都能解 WebP,同字节存两种格式没有意义。
		if !allowedVariants[*req.VariantFormat] {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "prefs.bad_variant_format")})
			return
		}
		updates["variant_format"] = *req.VariantFormat
	}
	if req.MaxImageWidth != nil {
		if !allowedWidths[*req.MaxImageWidth] {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "prefs.bad_width_preset")})
			return
		}
		updates["max_image_width"] = *req.MaxImageWidth
	}
	if req.ThumbWidth != nil {
		if !thumbWidthPresets[*req.ThumbWidth] {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "prefs.bad_thumb_width")})
			return
		}
		updates["thumb_width"] = *req.ThumbWidth
	}
	if req.ThumbFormat != nil {
		if !thumbFormatPresets[*req.ThumbFormat] {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "prefs.bad_thumb_format")})
			return
		}
		updates["thumb_format"] = *req.ThumbFormat
	}
	if len(updates) > 0 {
		if err := s.DB.Model(u).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
