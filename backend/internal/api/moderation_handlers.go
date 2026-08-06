package api

import (
	"github.com/gentpan/openimg/backend/internal/i18n"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type reportReq struct {
	ImageID  string `json:"image_id" binding:"required"`
	Category string `json:"category"`
	Reason   string `json:"reason" binding:"required,max=500"`
	Contact  string `json:"contact" binding:"max=255"`
}

// POST /api/report — anyone who can see an image can report it, including
// anonymous visitors. A free host without a working abuse channel becomes
// someone else's problem very quickly.
func (s *Server) handleReport(c *gin.Context) {
	var req reportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	imageID, err := uuid.Parse(req.ImageID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "report.bad_image_id")})
		return
	}
	var img models.Image
	if err := s.DB.First(&img, "id = ?", imageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "image.not_found")})
		return
	}

	category := strings.TrimSpace(req.Category)
	if !models.ValidReportCategory(category) {
		category = "other"
	}
	rep := models.Report{
		ID:         uuid.New(),
		ImageID:    imageID,
		Category:   category,
		Reason:     strings.TrimSpace(req.Reason),
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
	s.notifyAdminsOfReport(&rep, &img)
	c.JSON(http.StatusCreated, gin.H{"ok": true, "message": i18n.T(c, "report.received")})
}

// GET /admin/api/reports — the moderation queue.
func (s *Server) adminListReports(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	type row struct {
		ID        string `json:"id"`
		ImageID   string `json:"image_id"`
		Category  string `json:"category"`
		Reason    string `json:"reason"`
		Contact   string `json:"contact"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`

		ObjectKey   string `json:"object_key"`
		ProfileID   string `json:"profile_id"`
		OwnerEmail  string `json:"owner_email"`
		OwnerID     string `json:"owner_id"`
		ImageStatus string `json:"image_status"`
		// For the detail panel: enough to judge a report without leaving it.
		OrigName   string `json:"orig_name"`
		ShortCode  string `json:"short_code"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		SizeStored int64  `json:"size_stored"`
		Anonymous  bool   `json:"anonymous"`
		ReportsOn  int64  `json:"reports_on_image"`
	}
	var rows []row
	q := `
		SELECT r.id, r.image_id, r.category, r.reason, r.contact, r.status, r.created_at,
		       (r.reporter_id IS NULL) AS anonymous,
		       i.object_key, i.profile_id, i.status AS image_status,
		       i.orig_name, i.short_code, i.width, i.height, i.size_stored,
		       u.email AS owner_email, u.id AS owner_id,
		       (SELECT COUNT(*) FROM reports r2 WHERE r2.image_id = r.image_id) AS reports_on
		FROM reports r
		JOIN images i ON i.id = r.image_id
		JOIN users u ON u.id = i.user_id
	`
	if st := c.Query("status"); st != "" {
		q += " WHERE r.status = ?"
		s.DB.Raw(q+" ORDER BY r.created_at DESC LIMIT ?", st, limit).Scan(&rows)
	} else {
		s.DB.Raw(q+" WHERE r.status = 'open' ORDER BY r.created_at DESC LIMIT ?", limit).Scan(&rows)
	}
	c.JSON(http.StatusOK, gin.H{"reports": rows})
}

type resolveReportReq struct {
	// Action is one of: dismiss | block | block_and_ban
	Action string `json:"action" binding:"required"`
	Note   string `json:"note"`
}

// POST /admin/api/reports/:id/resolve
func (s *Server) adminResolveReport(c *gin.Context) {
	admin := auth.MustUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "admin.report.bad_id")})
		return
	}
	var req resolveReportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var rep models.Report
	if err := s.DB.First(&rep, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "admin.report.not_found")})
		return
	}
	var img models.Image
	if err := s.DB.First(&img, "id = ?", rep.ImageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "image.not_found")})
		return
	}

	switch req.Action {
	case "dismiss":
		// Nothing to do to the image.
	case "block", "block_and_ban":
		// Blocking hides the image but keeps the object: if the decision is
		// wrong it can be undone, and if it's evidence it still exists.
		s.DB.Model(&img).Update("status", models.ImageBlocked)
		if req.Action == "block_and_ban" {
			s.DB.Model(&models.User{}).Where("id = ?", img.UserID).
				Update("status", models.UserSuspended)
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "admin.report.bad_action")})
		return
	}

	now := time.Now()
	s.DB.Model(&rep).Updates(map[string]any{
		"status":      models.ReportResolved,
		"action":      req.Action,
		"note":        req.Note,
		"resolved_by": admin.ID,
		"resolved_at": &now,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /admin/api/images/:id/block — toggle an image's visibility directly,
// without a report behind it.
func (s *Server) adminBlockImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "admin.image.bad_id")})
		return
	}
	var img models.Image
	if err := s.DB.First(&img, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "image.not_found")})
		return
	}
	next := models.ImageBlocked
	if img.Status == models.ImageBlocked {
		next = models.ImageActive
	}
	s.DB.Model(&img).Update("status", next)
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": next})
}

type banUserReq struct {
	// PurgeImages also soft-deletes every image the user owns and queues the
	// objects for removal.
	PurgeImages bool   `json:"purge_images"`
	Reason      string `json:"reason"`
}

// POST /admin/api/users/:id/ban — suspend an account and, optionally, remove
// everything it uploaded. This is the button you need at 3am; it exists so
// nobody has to write SQL under pressure.
func (s *Server) adminBanUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "admin.user.bad_id")})
		return
	}
	var req banUserReq
	_ = c.ShouldBindJSON(&req) // body is optional

	var target models.User
	if err := s.DB.First(&target, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "admin.user.not_found")})
		return
	}
	if target.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "admin.user.is_admin")})
		return
	}

	s.DB.Model(&target).Update("status", models.UserSuspended)
	// Kill every live session so the ban takes effect immediately rather than
	// whenever their JWT happens to expire.
	s.DB.Where("user_id = ?", target.ID).Delete(&models.Session{})
	s.DB.Model(&models.APIToken{}).Where("user_id = ?", target.ID).Update("revoked", true)

	purged := 0
	if req.PurgeImages {
		var images []models.Image
		s.DB.Where("user_id = ? AND deleted_at IS NULL", target.ID).Find(&images)
		now := time.Now()
		for _, img := range images {
			s.DB.Model(&models.Image{}).Where("id = ?", img.ID).Updates(map[string]any{
				"deleted_at": now,
				"status":     models.ImageDeleted,
			})
			if s.Queue != nil {
				s.Queue.Submit(scheduler.JobPurge, img.ID)
			}
			purged++
		}
		// Reset their occupancy rather than issuing one refund per image —
		// the ledger entry should read as the single administrative act it is.
		if _, err := quota.Reconcile(s.DB, target.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": i18n.T(c, "admin.reconcile_failed", err.Error())})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "purged": purged})
}
