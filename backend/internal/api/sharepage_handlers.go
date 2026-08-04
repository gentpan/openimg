package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// The share page behind a short link.
//
// Public and unauthenticated: the whole point of a short link is that it gets
// pasted somewhere strangers will click it. Only what a viewer needs is
// exposed — no owner id, no storage profile, no object key beyond the URL they
// can already see.

type sharePayload struct {
	Code      string    `json:"code"`
	Name      string    `json:"orig_name"`
	MIME      string    `json:"mime"`
	Ext       string    `json:"ext"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int64     `json:"size"`
	Views     int64     `json:"views"`
	CreatedAt time.Time `json:"created_at"`
	Uploader  string    `json:"uploader"`

	URL        string `json:"url"`
	ShortURL   string `json:"short_url"`
	ThumbURL   string `json:"thumb_url"`
	Markdown   string `json:"markdown"`
	HTML       string `json:"html"`
	HTMLLink   string `json:"html_link"`
	HTMLDirect string `json:"html_direct"`
	BBCode     string `json:"bbcode"`
	BBCodeLink string `json:"bbcode_link"`

	Reactions map[string]int64 `json:"reactions"`
	Mine      string           `json:"mine,omitempty"`
}

// reactionIdentity is the signed-in user, or a hash of the client address.
//
// Salted with the JWT secret so the hashes are not portable: a leaked table
// cannot be checked against a list of candidate addresses without also having
// the server's secret.
func (s *Server) reactionIdentity(c *gin.Context) string {
	if u, ok := auth.UserFrom(c); ok {
		return "u:" + u.ID.String()
	}
	sum := sha256.Sum256([]byte(s.ReactionSalt + "|" + c.ClientIP()))
	return "a:" + hex.EncodeToString(sum[:])[:40]
}

func (s *Server) findByShortCode(code string) (*models.Image, error) {
	if !models.IsValidShortCode(code) {
		return nil, gorm.ErrRecordNotFound
	}
	var img models.Image
	if err := s.DB.Where("short_code = ? AND deleted_at IS NULL", code).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// GET /api/s/:code — everything the share page renders.
func (s *Server) handleShareInfo(c *gin.Context) {
	img, err := s.findByShortCode(c.Param("code"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在或已失效"})
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
	c.JSON(http.StatusOK, s.buildSharePayload(c, img))
}

func (s *Server) buildSharePayload(c *gin.Context, img *models.Image) sharePayload {
	var profile *models.StorageProfile
	var p models.StorageProfile
	if s.DB.First(&p, "id = ?", img.ProfileID).Error == nil {
		profile = &p
	}
	url := storage.URLFor(profile, img.ObjectKey, s.PublicBaseURL)
	short := storage.JoinURL(s.PublicBaseURL, img.ShortCode)

	thumb := url
	for _, v := range []string{models.VariantW600, models.VariantW200} {
		if img.HasVariant(v) {
			thumb = storage.ThumbURLFor(profile, models.VariantKey(img.ObjectKey, v), s.PublicBaseURL)
			break
		}
	}

	// Alt text with a quote in it would break the HTML snippet the page hands
	// out, and a filename is entirely user-controlled.
	alt := strings.ReplaceAll(img.OrigName, `"`, "'")

	var uploader string
	var owner models.User
	if s.DB.Select("name", "email").First(&owner, "id = ?", img.UserID).Error == nil {
		uploader = owner.Name
		if uploader == "" {
			// Never the address itself: this page is public.
			if i := strings.Index(owner.Email, "@"); i > 0 {
				uploader = owner.Email[:i]
			}
		}
	}

	counts := map[string]int64{}
	for _, k := range models.ReactionKinds {
		counts[k] = 0
	}
	var rows []struct {
		Kind string
		N    int64
	}
	s.DB.Model(&models.Reaction{}).Select("kind, COUNT(*) AS n").
		Where("image_id = ?", img.ID).Group("kind").Scan(&rows)
	for _, r := range rows {
		counts[r.Kind] = r.N
	}

	var mine models.Reaction
	mineKind := ""
	if s.DB.Where("image_id = ? AND identity = ?", img.ID, s.reactionIdentity(c)).
		First(&mine).Error == nil {
		mineKind = mine.Kind
	}

	return sharePayload{
		Code: img.ShortCode, Name: img.OrigName, MIME: img.MIME, Ext: img.Ext,
		Width: img.Width, Height: img.Height, Size: img.SizeStored,
		Views: img.ViewCount, CreatedAt: img.CreatedAt, Uploader: uploader,

		URL: url, ShortURL: short, ThumbURL: thumb,
		Markdown:   "![" + alt + "](" + url + ")",
		HTML:       `<img src="` + url + `" alt="` + alt + `">`,
		HTMLLink:   `<a href="` + short + `" target="_blank"><img src="` + url + `" alt="` + alt + `"></a>`,
		HTMLDirect: `<a href="` + url + `" target="_blank"><img src="` + url + `" alt="` + alt + `"></a>`,
		BBCode:     "[img]" + url + "[/img]",
		BBCodeLink: "[url=" + short + "][img]" + url + "[/img][/url]",

		Reactions: counts, Mine: mineKind,
	}
}

type reactReq struct {
	Kind string `json:"kind" binding:"required"`
}

// POST /api/s/:code/react — set, change, or clear this visitor's reaction.
//
// One per person per image: sending the kind already recorded clears it, which
// makes the same button both "react" and "undo" without a second endpoint.
func (s *Server) handleReact(c *gin.Context) {
	var req reactReq
	if err := c.ShouldBindJSON(&req); err != nil || !models.ValidReactionKind(req.Kind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的表情"})
		return
	}
	img, err := s.findByShortCode(c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在或已失效"})
		return
	}
	if img.Status == models.ImageBlocked {
		c.JSON(http.StatusGone, gin.H{"error": "该图片已被屏蔽"})
		return
	}

	identity := s.reactionIdentity(c)
	var existing models.Reaction
	switch err := s.DB.Where("image_id = ? AND identity = ?", img.ID, identity).First(&existing).Error; {
	case err == nil && existing.Kind == req.Kind:
		s.DB.Delete(&existing)
	case err == nil:
		s.DB.Model(&existing).Update("kind", req.Kind)
	case errors.Is(err, gorm.ErrRecordNotFound):
		s.DB.Create(&models.Reaction{
			ID: uuid.New(), ImageID: img.ID, Identity: identity, Kind: req.Kind,
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, s.buildSharePayload(c, img))
}

// POST /api/s/:code/report — report by short code.
//
// The share page never learns the internal image id: it is public, and an id
// is a handle to things a viewer has no business addressing. So reporting goes
// through the code they already have, and the server does the lookup.
func (s *Server) handleShareReport(c *gin.Context) {
	var req struct {
		Category string `json:"category"`
		Reason   string `json:"reason"`
		Contact  string `json:"contact"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = ""
	}
	// The category carries the meaning; free text is genuinely optional, and
	// demanding four characters just teaches people to type "aaaa".
	reason := strings.TrimSpace(req.Reason)
	if !models.ValidReportCategory(strings.TrimSpace(req.Category)) && len([]rune(reason)) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择举报类型，或填写说明"})
		return
	}
	img, err := s.findByShortCode(c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接不存在或已失效"})
		return
	}

	category := strings.TrimSpace(req.Category)
	if !models.ValidReportCategory(category) {
		category = "other"
	}
	rep := models.Report{
		ID:         uuid.New(),
		ImageID:    img.ID,
		Category:   category,
		Reason:     reason,
		Contact:    strings.TrimSpace(req.Contact),
		ReporterIP: c.ClientIP(),
		Status:     models.ReportOpen,
	}
	if u, ok := auth.UserFrom(c); ok {
		rep.ReporterID = &u.ID
	}
	if err := s.DB.Create(&rep).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.notifyAdminsOfReport(&rep, img)
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "举报已收到，我们会尽快处理"})
}
