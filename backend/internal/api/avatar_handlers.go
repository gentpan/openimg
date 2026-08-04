package api

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/imageproc"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
)

// avatarMaxUpload bounds what we will read before deciding anything. A profile
// picture ends up at 256px; anything past a few megabytes is a mistake or an
// attempt to make us decode something expensive.
const avatarMaxUpload = 8 << 20 // 8 MiB

// POST /api/account/avatar
//
// Avatars are stored on the platform pool and are deliberately *not* charged
// to the user's quota. One 256px AVIF is a couple of kilobytes, it is account
// furniture rather than something the user is hosting, and billing it would
// mean a user at their limit could not change their profile picture.
func (s *Server) handleUploadAvatar(c *gin.Context) {
	u := auth.MustUser(c)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择一张图片"})
		return
	}
	defer file.Close()
	if header.Size > avatarMaxUpload {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "头像不能超过 8 MB"})
		return
	}

	raw := make([]byte, 0, header.Size)
	buf := bytes.NewBuffer(raw)
	if _, err := buf.ReadFrom(http.MaxBytesReader(c.Writer, file, avatarMaxUpload)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取失败：" + err.Error()})
		return
	}

	// Sniff the real format before handing anything to the decoder — the
	// filename and the declared content type are both attacker-controlled.
	if _, err := imageproc.Detect(buf.Bytes()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不是受支持的图片格式"})
		return
	}

	out, err := imageproc.MakeAvatar(buf.Bytes())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	backend, profile, err := s.Storage.Platform(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "存储不可用"})
		return
	}

	// A fresh key every time rather than one keyed on the user id: the old
	// object stays readable until the new row is committed, so a viewer mid
	// page-load never sees a broken image, and a CDN holding the old URL is
	// not serving a picture the user just replaced.
	key := "avatar/" + models.NewKeyID() + "." + out.Ext
	if err := backend.Put(ctx, key, bytes.NewReader(out.Data), out.Size(), out.MIME); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入存储失败：" + err.Error()})
		return
	}

	previous := u.AvatarKey
	url := storage.URLFor(profile, key, s.PublicBaseURL)
	if err := s.DB.Model(u).Updates(map[string]any{
		"avatar_key": key,
		"avatar_url": url,
	}).Error; err != nil {
		// Roll the object back: a stored file nothing points at is a leak.
		cleanupObjects(ctx, backend, []string{key})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.deleteAvatarObject(previous)
	c.JSON(http.StatusOK, gin.H{"ok": true, "avatar_url": url})
}

// DELETE /api/account/avatar
func (s *Server) handleDeleteAvatar(c *gin.Context) {
	u := auth.MustUser(c)
	previous := u.AvatarKey
	if err := s.DB.Model(u).Updates(map[string]any{"avatar_key": "", "avatar_url": ""}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.deleteAvatarObject(previous)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteAvatarObject removes a superseded avatar in the background.
//
// Failures are logged, not surfaced: the user's avatar has already changed as
// far as they are concerned, and a leaked 5 KB object is not worth turning a
// successful action into an error.
func (s *Server) deleteAvatarObject(key string) {
	if !strings.HasPrefix(key, "avatar/") {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		backend, _, err := s.Storage.Platform(ctx)
		if err != nil {
			log.Printf("avatar: cannot reach storage to drop %s: %v", key, err)
			return
		}
		if err := backend.Delete(ctx, key); err != nil {
			log.Printf("avatar: dropping %s failed: %v", key, err)
		}
	}()
}
