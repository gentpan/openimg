package api

import (
	"errors"
	"github.com/google/uuid"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// GET /:code — the share page behind a short link.
//
// Renders a page rather than redirecting to the file. The short link is what
// gets pasted into a forum post or a chat, where the useful landing is a page
// that shows the picture *and* hands over every link format — a bare redirect
// leaves the visitor on a raw image with no way back and nothing to copy.
//
// Embedding still uses the direct URL, which the page displays; that split is
// deliberate and matches how every other host does it.
func (s *Server) handleShortLink(c *gin.Context) {
	code := c.Param("code")
	if !models.IsValidShortCode(code) {
		s.serveAppOrNotFound(c)
		return
	}

	var img models.Image
	err := s.DB.Where("short_code = ? AND deleted_at IS NULL", code).First(&img).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Not a code we issued — hand it to the SPA, which will show its own
		// not-found rather than a bare 404 from the API.
		s.serveAppOrNotFound(c)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Counted here rather than on the image URL: the CDN serves those without
	// ever reaching us, so a share-page visit is the only view we can observe.
	// Best-effort — a failed counter must not break the page.
	if img.Status != models.ImageBlocked {
		go func(id uuid.UUID) {
			if err := s.DB.Model(&models.Image{}).Where("id = ?", id).
				UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
				log.Printf("shortlink: view count for %s failed: %v", id, err)
			}
		}(img.ID)
	}

	s.serveShareApp(c, &img)
}

// serveShareApp returns the SPA with Open Graph tags for this image.
//
// The tags have to be in the HTML the crawler receives: a link preview is
// fetched by a bot that does not run JavaScript, so anything React sets after
// mount is invisible to it — and a share page whose whole job is to be pasted
// somewhere is exactly where previews matter.
func (s *Server) serveShareApp(c *gin.Context, img *models.Image) {
	if s.FrontendDir == "" {
		// Development: Vite serves the SPA and the React route renders it.
		// Previews are a production concern, so there is nothing to inject.
		s.serveAppOrNotFound(c)
		return
	}
	raw, err := os.ReadFile(filepath.Join(s.FrontendDir, "index.html"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "frontend not available"})
		return
	}

	var profile *models.StorageProfile
	var p models.StorageProfile
	if s.DB.First(&p, "id = ?", img.ProfileID).Error == nil {
		profile = &p
	}
	imageURL := storage.URLFor(profile, img.ObjectKey, s.PublicBaseURL)
	pageURL := storage.JoinURL(s.PublicBaseURL, img.ShortCode)

	// html.EscapeString, not string concatenation: the filename is entirely
	// user-controlled and lands inside an attribute.
	esc := html.EscapeString
	tags := strings.Join([]string{
		`<meta property="og:type" content="website">`,
		`<meta property="og:title" content="` + esc(img.OrigName) + `">`,
		`<meta property="og:url" content="` + esc(pageURL) + `">`,
		`<meta property="og:image" content="` + esc(imageURL) + `">`,
		`<meta property="og:image:width" content="` + strconv.Itoa(img.Width) + `">`,
		`<meta property="og:image:height" content="` + strconv.Itoa(img.Height) + `">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta name="twitter:title" content="` + esc(img.OrigName) + `">`,
		`<meta name="twitter:image" content="` + esc(imageURL) + `">`,
	}, "\n    ")

	// Injected at </head> so these override the site-wide defaults earlier in
	// the document — last tag wins for the crawlers that accept duplicates.
	out := strings.Replace(string(raw), "</head>", "    "+tags+"\n  </head>", 1)

	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(out))
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
