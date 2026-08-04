package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// imageOut is the wire shape of an image. URLs are assembled here rather than
// stored, so changing a CDN domain re-points every historical image.
type imageOut struct {
	models.Image
	URL       string            `json:"url"`
	Variants  map[string]string `json:"variant_urls"`
	Thumb     string            `json:"thumb_url"`
	ShortURL  string            `json:"short_url,omitempty"`
	Markdown  string            `json:"markdown"`
	HTML      string            `json:"html"`
	BBCode    string            `json:"bbcode"`
	DeleteURL string            `json:"delete_url,omitempty"`
}

// decorate resolves each image's profile once and builds its public URLs.
// Profiles are fetched in a single query — listing 100 images must not mean
// 100 profile lookups.
func (s *Server) decorate(images []models.Image) []imageOut {
	out := make([]imageOut, 0, len(images))
	if len(images) == 0 {
		return out
	}

	ids := make([]uuid.UUID, 0, len(images))
	seen := map[uuid.UUID]bool{}
	for _, img := range images {
		if !seen[img.ProfileID] {
			seen[img.ProfileID] = true
			ids = append(ids, img.ProfileID)
		}
	}
	var profiles []models.StorageProfile
	s.DB.Where("id IN ?", ids).Find(&profiles)
	byID := map[uuid.UUID]*models.StorageProfile{}
	for i := range profiles {
		byID[profiles[i].ID] = &profiles[i]
	}

	for _, img := range images {
		p := byID[img.ProfileID]
		item := imageOut{
			Image:    img,
			URL:      storage.URLFor(p, img.ObjectKey, s.PublicBaseURL),
			Variants: map[string]string{},
		}
		for _, v := range img.VariantList() {
			key := models.VariantKey(img.ObjectKey, v)
			// Display sizes may live on their own origin; full-size
			// derivatives stay with the original they replace.
			if models.IsThumbVariant(v) {
				item.Variants[v] = storage.ThumbURLFor(p, key, s.PublicBaseURL)
			} else {
				item.Variants[v] = storage.URLFor(p, key, s.PublicBaseURL)
			}
		}
		// Prefer the grid tier, then the legacy 200px one that images
		// uploaded before the switch still carry, then the original so a
		// gallery keeps rendering while derivatives are queued.
		if img.ShortCode != "" {
			item.ShortURL = storage.JoinURL(s.PublicBaseURL, img.ShortCode)
		}
		item.Thumb = item.URL
		for _, v := range []string{models.VariantW600, models.VariantW200} {
			if u, ok := item.Variants[v]; ok {
				item.Thumb = u
				break
			}
		}
		alt := img.OrigName
		if alt == "" {
			alt = "image"
		}
		item.Markdown = "![" + alt + "](" + item.URL + ")"
		item.HTML = `<img src="` + item.URL + `" alt="` + alt + `">`
		item.BBCode = "[img]" + item.URL + "[/img]"
		out = append(out, item)
	}
	return out
}

// GET /api/images — the caller's own library.
func (s *Server) handleListImages(c *gin.Context) {
	u := auth.MustUser(c)
	limit := 40
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// Whitelisted sort keys — the value lands in an ORDER BY, so it can never
	// come straight from the query string.
	sortBy := map[string]string{
		"newest":   "created_at DESC",
		"oldest":   "created_at ASC",
		"largest":  "size_stored DESC",
		"smallest": "size_stored ASC",
		"name":     "orig_name ASC",
		"widest":   "width DESC",
	}[c.Query("sort")]
	if sortBy == "" {
		sortBy = "created_at DESC"
	}

	q := s.DB.Where("user_id = ? AND deleted_at IS NULL", u.ID)
	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		q = q.Where("orig_name ILIKE ?", "%"+kw+"%")
	}

	var total int64
	q.Model(&models.Image{}).Count(&total)

	images := []models.Image{}
	if err := q.Order(sortBy).Limit(limit).Offset(offset).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"images": s.decorate(images),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleGetImage(c *gin.Context) {
	u := auth.MustUser(c)
	var img models.Image
	q := s.DB.Where("id = ? AND deleted_at IS NULL", c.Param("id"))
	if !u.IsAdmin() {
		q = q.Where("user_id = ?", u.ID)
	}
	if err := q.First(&img).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, s.decorate([]models.Image{img})[0])
}

// DELETE /api/images/:id — soft-deletes the row, refunds quota, and queues
// object removal.
//
// The objects are only purged when no other row still references the same
// (profile, sha256) pair; dedup means one user's delete must not break another
// user's copy. That check happens in the purge handler, under the same
// transaction that flips the last reference.
func (s *Server) handleDeleteImage(c *gin.Context) {
	u := auth.MustUser(c)
	var img models.Image
	q := s.DB.Where("id = ? AND deleted_at IS NULL", c.Param("id"))
	if !u.IsAdmin() {
		q = q.Where("user_id = ?", u.ID)
	}
	if err := q.First(&img).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	now := time.Now()
	if err := s.DB.Model(&img).Updates(map[string]any{
		"deleted_at": now,
		"status":     models.ImageDeleted,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Refund only for platform-pool images — BYOS uploads never consumed quota.
	var p models.StorageProfile
	if err := s.DB.First(&p, "id = ?", img.ProfileID).Error; err == nil && p.IsPlatform() {
		id := img.ID
		if err := quota.Release(s.DB, img.UserID, img.SizeStored, "删除图片 "+img.OrigName, &id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if s.Queue != nil {
		s.Queue.Submit(scheduler.JobPurge, img.ID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/public-stats — numbers for the landing page. Deliberately coarse:
// no per-user data, and served to anonymous visitors.
func (s *Server) handlePublicStats(c *gin.Context) {
	type out struct {
		TotalImages int64 `json:"total_images"`
		TotalUsers  int64 `json:"total_users"`
		StoredBytes int64 `json:"stored_bytes"`
		ImagesToday int64 `json:"images_today"`
	}
	var o out
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL").Count(&o.TotalImages)
	s.DB.Model(&models.User{}).Count(&o.TotalUsers)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL AND created_at >= ?", dayStart).Count(&o.ImagesToday)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL").
		Select("COALESCE(SUM(size_stored), 0)").Row().Scan(&o.StoredBytes)
	c.JSON(http.StatusOK, o)
}

func pickStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
