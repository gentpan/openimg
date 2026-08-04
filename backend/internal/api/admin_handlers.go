package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) adminStats(c *gin.Context) {
	type counts struct {
		TotalUsers    int64 `json:"total_users"`
		ActiveToday   int64 `json:"active_today"`
		TotalImages   int64 `json:"total_images"`
		ImagesToday   int64 `json:"images_today"`
		BlockedImages int64 `json:"blocked_images"`
		PendingBackup int64 `json:"pending_backup"`
		QueueDepth    int   `json:"queue_depth"`
		StoredBytes   int64 `json:"stored_bytes"`
		GrantedBytes  int64 `json:"granted_bytes"`
		UniqueObjects int64 `json:"unique_objects"`
	}
	var out counts
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	s.DB.Model(&models.User{}).Count(&out.TotalUsers)
	s.DB.Model(&models.CheckinRecord{}).Where("date = ?", now.Format("2006-01-02")).Count(&out.ActiveToday)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL").Count(&out.TotalImages)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL AND created_at >= ?", dayStart).Count(&out.ImagesToday)
	s.DB.Model(&models.Image{}).Where("status = ?", models.ImageBlocked).Count(&out.BlockedImages)
	s.DB.Model(&models.Image{}).Where("backup_state IN ?", []models.BackupState{models.BackupPending, models.BackupFailed}).Count(&out.PendingBackup)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL").Select("COALESCE(SUM(size_stored), 0)").Row().Scan(&out.StoredBytes)
	s.DB.Model(&models.User{}).Select("COALESCE(SUM(quota_bytes), 0)").Row().Scan(&out.GrantedBytes)
	// Distinct SHA-256 count is what actually occupies the pool — the gap
	// against TotalImages is what dedup saved us.
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL").
		Distinct("sha256").Count(&out.UniqueObjects)
	if s.Queue != nil {
		out.QueueDepth = s.Queue.Depth()
	}

	c.JSON(http.StatusOK, out)
}

// adminListImages is the moderation queue's data source.
func (s *Server) adminListImages(c *gin.Context) {
	limit := 60
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	q := s.DB.Order("created_at DESC").Limit(limit)
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	} else {
		q = q.Where("deleted_at IS NULL")
	}
	if uid := c.Query("user_id"); uid != "" {
		q = q.Where("user_id = ?", uid)
	}
	images := []models.Image{}
	if err := q.Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"images": s.decorate(images)})
}

type adminUser struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	GroupID     *string `json:"group_id,omitempty"`
	GroupName   string  `json:"group_name,omitempty"`
	CreatedAt   string  `json:"created_at"`
	LastLoginAt string  `json:"last_login_at,omitempty"`
	SignupIP    string  `json:"signup_ip,omitempty"`
	LastLoginIP string  `json:"last_login_ip,omitempty"`
	ImageCount  int64   `json:"image_count"`
	QuotaBytes  int64   `json:"quota_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
}

func (s *Server) adminListUsers(c *gin.Context) {
	var users []models.User
	s.DB.Order("created_at DESC").Find(&users)

	groupsByID := map[string]string{}
	var groups []models.UserGroup
	s.DB.Find(&groups)
	for _, g := range groups {
		groupsByID[g.ID.String()] = g.Name
	}

	// One grouped count instead of a query per user — this list grows with
	// registrations and the N+1 showed up immediately on the old dashboard.
	counts := map[string]int64{}
	rows := []struct {
		UserID string
		N      int64
	}{}
	s.DB.Model(&models.Image{}).
		Select("user_id, COUNT(*) AS n").
		Where("deleted_at IS NULL").
		Group("user_id").Scan(&rows)
	for _, r := range rows {
		counts[r.UserID] = r.N
	}

	out := make([]adminUser, 0, len(users))
	for _, u := range users {
		au := adminUser{
			ID:          u.ID.String(),
			Email:       u.Email,
			Name:        u.Name,
			Role:        string(u.Role),
			Status:      string(u.Status),
			CreatedAt:   u.CreatedAt.Format(time.RFC3339),
			SignupIP:    u.SignupIP,
			LastLoginIP: u.LastLoginIP,
			ImageCount:  counts[u.ID.String()],
			QuotaBytes:  u.QuotaBytes,
			UsedBytes:   u.UsedBytes,
		}
		if u.GroupID != nil {
			gid := u.GroupID.String()
			au.GroupID = &gid
			au.GroupName = groupsByID[gid]
		}
		if u.LastLoginAt != nil {
			au.LastLoginAt = u.LastLoginAt.Format(time.RFC3339)
		}
		out = append(out, au)
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

type updateUserReq struct {
	Role    *string `json:"role"`
	Status  *string `json:"status"`
	GroupID *string `json:"group_id"`
	Name    *string `json:"name"`
}

func (s *Server) adminUpdateUser(c *gin.Context) {
	id := c.Param("id")
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var u models.User
	if err := s.DB.First(&u, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	updates := map[string]any{}
	if req.Role != nil {
		switch strings.ToLower(*req.Role) {
		case "admin":
			updates["role"] = models.RoleAdmin
		case "user":
			updates["role"] = models.RoleUser
		}
	}
	if req.Status != nil {
		switch strings.ToLower(*req.Status) {
		case "active":
			updates["status"] = models.UserActive
		case "suspended":
			updates["status"] = models.UserSuspended
		}
	}
	if req.GroupID != nil {
		if *req.GroupID == "" {
			updates["group_id"] = nil
		} else {
			updates["group_id"] = *req.GroupID
		}
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if len(updates) > 0 {
		s.DB.Model(&u).Updates(updates)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// adminAdjustQuota grants or revokes capacity by hand.
type adjustQuotaReq struct {
	UserID string `json:"user_id" binding:"required"`
	Bytes  int64  `json:"bytes" binding:"required"`
	Reason string `json:"reason"`
}

func (s *Server) adminAdjustQuota(c *gin.Context) {
	var req adjustQuotaReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad user_id"})
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "管理员手动调整"
	}
	newQuota, err := quota.AdminAdjust(s.DB, uid, req.Bytes, reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"quota_bytes": newQuota})
}

// adminReconcileQuota repairs a user's drifted UsedBytes from the images table.
func (s *Server) adminReconcileQuota(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad user id"})
		return
	}
	delta, err := quota.Reconcile(s.DB, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "delta": delta})
}

func (s *Server) adminListTransactions(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	type row struct {
		ID         string `json:"id"`
		UserEmail  string `json:"user_email"`
		Type       string `json:"type"`
		Bytes      int64  `json:"bytes"`
		QuotaAfter int64  `json:"quota_after"`
		UsedAfter  int64  `json:"used_after"`
		Reason     string `json:"reason"`
		CreatedAt  string `json:"created_at"`
	}
	var rows []row
	s.DB.Raw(`
		SELECT t.id, u.email AS user_email, t.type, t.bytes,
		       t.quota_after, t.used_after, t.reason, t.created_at
		FROM quota_transactions t JOIN users u ON u.id = t.user_id
		ORDER BY t.created_at DESC LIMIT ?
	`, limit).Scan(&rows)
	c.JSON(http.StatusOK, gin.H{"transactions": rows})
}

func (s *Server) adminListGroups(c *gin.Context) {
	groups := []models.UserGroup{}
	s.DB.Order("name").Find(&groups)
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

type updateGroupReq struct {
	Description      *string `json:"description"`
	MaxFileSize      *int64  `json:"max_file_size"`
	DailyUploadCount *int    `json:"daily_upload_count"`
	AllowedFormats   *string `json:"allowed_formats"`
	MaxWidth         *int    `json:"max_width"`
	MaxHeight        *int    `json:"max_height"`
	SignupSpace      *int64  `json:"signup_space"`
	CheckinMinSpace  *int64  `json:"checkin_min_space"`
	CheckinMaxSpace  *int64  `json:"checkin_max_space"`
	StreakBonusSpace *int64  `json:"streak_bonus_space"`
	StreakBonusDays  *int    `json:"streak_bonus_days"`
	ReferralSpace    *int64  `json:"referral_space"`
	MaxTotalSpace    *int64  `json:"max_total_space"`
	AllowBYOS        *bool   `json:"allow_byos"`
	MaxProfiles      *int    `json:"max_profiles"`
}

func (s *Server) adminUpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var req updateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var g models.UserGroup
	if err := s.DB.First(&g, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	updates := map[string]any{}
	setStr := func(col string, v *string) {
		if v != nil {
			updates[col] = *v
		}
	}
	setInt := func(col string, v *int) {
		if v != nil {
			updates[col] = *v
		}
	}
	setI64 := func(col string, v *int64) {
		if v != nil {
			updates[col] = *v
		}
	}
	setStr("description", req.Description)
	setStr("allowed_formats", req.AllowedFormats)
	setI64("max_file_size", req.MaxFileSize)
	setInt("daily_upload_count", req.DailyUploadCount)
	setInt("max_width", req.MaxWidth)
	setInt("max_height", req.MaxHeight)
	setI64("signup_space", req.SignupSpace)
	setI64("checkin_min_space", req.CheckinMinSpace)
	setI64("checkin_max_space", req.CheckinMaxSpace)
	setI64("streak_bonus_space", req.StreakBonusSpace)
	setInt("streak_bonus_days", req.StreakBonusDays)
	setI64("referral_space", req.ReferralSpace)
	setI64("max_total_space", req.MaxTotalSpace)
	setInt("max_profiles", req.MaxProfiles)
	if req.AllowBYOS != nil {
		updates["allow_byos"] = *req.AllowBYOS
	}
	if len(updates) > 0 {
		s.DB.Model(&g).Updates(updates)
	}
	s.DB.First(&g, "id = ?", id)
	c.JSON(http.StatusOK, g)
}
