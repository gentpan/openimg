package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"io"
	"net/http"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// allowedThumbWidths mirrors imageproc.ThumbWidths. Restricting to a fixed set
// keeps the object namespace predictable — arbitrary widths would let one
// image sprout unbounded derivatives.
var allowedThumbWidths = map[int]string{
	200:  models.VariantW200,
	600:  models.VariantW600,
	1200: models.VariantW1200,
}

type makeVariantReq struct {
	Width int `json:"width" binding:"required"`
}

// POST /api/images/:id/variant — render one size on demand.
//
// Nothing is generated at upload any more, so this is where derivative storage
// is actually spent. It's charged to the owner's quota like any other bytes,
// and it's idempotent: asking twice returns the existing URL rather than
// re-encoding and double-charging.
func (s *Server) handleMakeVariant(c *gin.Context) {
	u := auth.MustUser(c)

	var req makeVariantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name, ok := allowedThumbWidths[req.Width]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "variant.size_unsupported")})
		return
	}

	var img models.Image
	q := s.DB.Where("id = ? AND deleted_at IS NULL", c.Param("id"))
	if !u.IsAdmin() {
		q = q.Where("user_id = ?", u.ID)
	}
	if err := q.First(&img).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "image.not_found")})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	backend, profile, err := s.Storage.ForStored(ctx, img.ProfileID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": i18n.T(c, "storage.unavailable", err.Error())})
		return
	}

	key := models.VariantKey(img.ObjectKey, name)
	// Already there — either this user generated it before, or a deduplicated
	// sibling did. Either way, don't pay for it twice.
	if img.HasVariant(name) {
		c.JSON(http.StatusOK, gin.H{
			"ok": true, "existing": true,
			"variant": name, "url": storage.VariantURLFor(profile, name, key, s.PublicBaseURL),
		})
		return
	}

	rc, err := backend.Get(ctx, img.ObjectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": i18n.T(c, "variant.read_failed", err.Error())})
		return
	}
	raw, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	out, err := imageproc.MakeThumbnail(raw, req.Width)
	if err != nil {
		if errors.Is(err, imageproc.ErrNotWorthIt) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": i18n.T(c, "variant.generate_failed", err.Error())})
		return
	}

	onPlatform := profile == nil || profile.IsPlatform()
	if onPlatform {
		id := img.ID
		if err := quota.Consume(s.DB, img.UserID, out.Size(), fmt.Sprintf("生成 %s 尺寸", name), &id); err != nil {
			if errors.Is(err, quota.ErrInsufficient) {
				c.JSON(http.StatusInsufficientStorage, gin.H{
					"error": i18n.T(c, "variant.no_space", humanBytes(out.Size())),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := backend.Put(ctx, key, bytes.NewReader(out.Data), out.Size(), out.MIME); err != nil {
		if onPlatform {
			id := img.ID
			_ = quota.Release(s.DB, img.UserID, out.Size(), "生成尺寸失败回退", &id)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": i18n.T(c, "storage.write_failed", err.Error())})
		return
	}

	variants := img.Variants
	if variants == "" {
		variants = name
	} else {
		variants += "," + name
	}
	// Every row sharing this object gains the variant. 按 object_key 而非
	// sha 圈定:同字节可能因处理设置不同各存一份,变体 key 从各自
	// object_key 推导,标到别的格式的行上会得到不存在的链接,还会把对方
	// 的 Variants 快照整体覆盖掉。
	if err := s.DB.Model(&models.Image{}).
		Where("profile_id = ? AND object_key = ? AND deleted_at IS NULL", img.ProfileID, img.ObjectKey).
		Updates(map[string]any{
			"variants":    variants,
			"size_stored": gorm.Expr("size_stored + ?", out.Size()),
			"size_thumbs": gorm.Expr("size_thumbs + ?", out.Size()),
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true, "variant": name,
		"url":    storage.VariantURLFor(profile, name, key, s.PublicBaseURL),
		"bytes":  out.Size(),
		"width":  out.Width,
		"height": out.Height,
	})
}
