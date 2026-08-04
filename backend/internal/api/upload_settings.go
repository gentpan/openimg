package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Upload settings are read on the hot path — once per upload — so they are
// cached rather than re-queried. The cache is only invalidated by the admin
// endpoint below, which means a multi-node deployment would serve a stale
// value until each node restarts. That is acceptable for a flag whose effect
// is "the next upload also keeps its original"; it is not a correctness knob.
type uploadSettingsCache struct {
	mu     sync.RWMutex
	loaded bool
	value  models.UploadSettings
}

func (s *Server) uploadSettings() models.UploadSettings {
	s.uploadCache.mu.RLock()
	if s.uploadCache.loaded {
		v := s.uploadCache.value
		s.uploadCache.mu.RUnlock()
		return v
	}
	s.uploadCache.mu.RUnlock()

	var out models.UploadSettings
	var row models.SiteSetting
	err := s.DB.Where("key = ?", models.SiteSettingUpload).First(&row).Error
	if err == nil && row.Value != "" {
		// A malformed blob leaves out at its zero value, which is the safe
		// default (keep nothing extra) rather than a failed upload.
		_ = json.Unmarshal([]byte(row.Value), &out)
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out // don't cache a DB error as if it were the configured value
	}

	s.uploadCache.mu.Lock()
	s.uploadCache.value, s.uploadCache.loaded = out, true
	s.uploadCache.mu.Unlock()
	return out
}

func (s *Server) adminUploadSettings(c *gin.Context) {
	c.JSON(http.StatusOK, s.uploadSettings())
}

func (s *Server) adminSaveUploadSettings(c *gin.Context) {
	var req models.UploadSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	body, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化失败"})
		return
	}
	if err := s.DB.Save(&models.SiteSetting{Key: models.SiteSettingUpload, Value: string(body)}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	s.uploadCache.mu.Lock()
	s.uploadCache.value, s.uploadCache.loaded = req, true
	s.uploadCache.mu.Unlock()

	c.JSON(http.StatusOK, req)
}
