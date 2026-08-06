package api

import (
	"database/sql"
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
	c.JSON(http.StatusOK, gin.H{"images": s.decorateWithOwner(images)})
}

// adminImageOut is the moderation view: the same payload the owner sees, plus
// who the owner is. Moderating a picture without knowing whose it is means
// every block is a coin flip on whether the account also needs action — the
// report queue already exposes owner_email for exactly this reason.
//
// Kept separate from decorate() rather than folded into imageOut so the
// address never rides along on a user's own listing, where it is at best
// redundant and at worst a leak the moment that listing is shared.
type adminImageOut struct {
	imageOut
	OwnerID    string `json:"owner_id"`
	OwnerEmail string `json:"owner_email"`
	OwnerName  string `json:"owner_name,omitempty"`
}

func (s *Server) decorateWithOwner(images []models.Image) []adminImageOut {
	base := s.decorate(images)
	out := make([]adminImageOut, 0, len(base))
	if len(base) == 0 {
		return out
	}

	ids := make([]uuid.UUID, 0, len(base))
	seen := map[uuid.UUID]bool{}
	for _, b := range base {
		if !seen[b.UserID] {
			seen[b.UserID] = true
			ids = append(ids, b.UserID)
		}
	}
	// One query for the whole page, not one per row.
	var users []models.User
	s.DB.Select("id, email, name").Where("id IN ?", ids).Find(&users)
	byID := make(map[uuid.UUID]models.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	for _, b := range base {
		item := adminImageOut{imageOut: b, OwnerID: b.UserID.String()}
		if u, ok := byID[b.UserID]; ok {
			item.OwnerEmail = u.Email
			item.OwnerName = u.Name
		} else {
			// Account gone but the image outlived it — say so rather than
			// rendering a blank where an address belongs.
			item.OwnerEmail = "（账号已删除）"
		}
		out = append(out, item)
	}
	return out
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

// maxLedgerQueryRunes bounds the search term. Runes rather than bytes so the
// limit means the same thing whichever language the admin types in; 64 is well
// past any real filename or address and keeps the pattern from bloating the
// query plan.
const maxLedgerQueryRunes = 64

// likePattern turns a search term into a LIKE pattern with the wildcards
// escaped.
//
// Without this, a filename containing _ or % silently becomes a wildcard:
// searching "draft_v2.png" would also match "draftXv2.png", and a lone "%"
// would match every row while looking like it found something. Backslash is
// escaped first, or it would corrupt the escapes added after it.
func likePattern(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(term) + "%"
}

// GET /admin/api/quota/transactions — the ledger, paged and searchable.
func (s *Server) adminListTransactions(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	q := strings.TrimSpace(c.Query("q"))
	// Truncate by runes, not bytes. A Chinese character is three bytes, so a
	// byte-slice at a fixed offset lands mid-character two times out of three
	// and hands Postgres invalid UTF-8, which fails the whole query (22021)
	// rather than searching for something slightly shorter.
	if r := []rune(q); len(r) > maxLedgerQueryRunes {
		q = string(r[:maxLedgerQueryRunes])
	}

	// LEFT JOIN, not INNER: most ledger rows carry an image_id (upload, delete,
	// generate size) but check-ins, referral bonuses and admin adjustments do
	// not, and an inner join would drop exactly those.
	//
	// Deleted images are deliberately not filtered out. Their rows survive —
	// deletion only sets deleted_at, and the purge job removes the stored
	// objects while leaving the row — and a deleted image is precisely what an
	// admin is looking for when they come to this page.
	from := `
		FROM quota_transactions t
		JOIN users u ON u.id = t.user_id
		LEFT JOIN images i ON i.id = t.image_id`
	where := ""
	args := []any{}
	if q != "" {
		// One box, four columns. An admin arrives here with either a person or
		// a file in mind and should not have to say which. reason is included
		// because it is the only place bulk deletions and check-ins name
		// themselves — they have no image to join to.
		where = ` WHERE (u.email ILIKE @q ESCAPE '\' OR u.name ILIKE @q ESCAPE '\'
		           OR i.orig_name ILIKE @q ESCAPE '\' OR t.reason ILIKE @q ESCAPE '\')`
		args = append(args, sql.Named("q", likePattern(q)))
	}

	// Counted over the same FROM and WHERE, so the total always agrees with the
	// rows: a total taken from the unfiltered table would tell the pager to
	// render pages that come back empty.
	//
	// Errors are surfaced rather than swallowed. A discarded Scan error renders
	// as "no records", which is how the placeholder-binding bug below very
	// nearly shipped — a broken query and an empty ledger look identical.
	var total int64
	if err := s.DB.Raw(`SELECT COUNT(*) `+from+where, args...).Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Every placeholder below is named, including the paging ones.
	//
	// Not a style choice. gorm's Raw switches to NamedExpr as soon as the SQL
	// contains an '@', and NamedExpr still resolves '?' positionally against
	// the same Vars slice — without letting the named substitutions advance the
	// index. Mixing the two forms therefore binds the search pattern to LIMIT
	// and the limit to OFFSET. See TestLedgerQueryBindsNamedAndPositional.
	pageArgs := append(append([]any{}, args...),
		sql.Named("lim", limit), sql.Named("off", offset))

	type row struct {
		ID         string  `json:"id"`
		UserEmail  string  `json:"user_email"`
		UserName   string  `json:"user_name"`
		Type       string  `json:"type"`
		Bytes      int64   `json:"bytes"`
		QuotaAfter int64   `json:"quota_after"`
		UsedAfter  int64   `json:"used_after"`
		Reason     string  `json:"reason"`
		ImageName  *string `json:"image_name"`
		CreatedAt  string  `json:"created_at"`
	}
	rows := []row{}
	// created_at alone is not a total order — a bulk delete writes several rows
	// in the same instant — and an unstable sort makes deep paging drop and
	// repeat rows. id breaks the tie.
	if err := s.DB.Raw(`
		SELECT t.id, u.email AS user_email, u.name AS user_name, t.type, t.bytes,
		       t.quota_after, t.used_after, t.reason,
		       i.orig_name AS image_name, t.created_at`+from+where+`
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT @lim OFFSET @off
	`, pageArgs...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": rows, "total": total, "limit": limit, "offset": offset,
	})
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
