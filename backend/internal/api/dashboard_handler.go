package api

import (
	"net/http"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type dashboardOut struct {
	Stats           dashboardStats     `json:"stats"`
	UploadsByDay    []daySeries        `json:"uploads_by_day"`
	ByFormat        []formatBreakdown  `json:"by_format"`
	ByProfile       []profileBreakdown `json:"by_profile"`
	RecentLogins    []recentLogin      `json:"recent_logins"`
	BackupFailures  []backupFailure    `json:"backup_failures"`
	ActiveSessions  int64              `json:"active_sessions"`
	TopUsersBySpace []topUser          `json:"top_users_by_space"`
}

type dashboardStats struct {
	TotalUsers    int64 `json:"total_users"`
	TotalImages   int64 `json:"total_images"`
	ImagesToday   int64 `json:"images_today"`
	CheckinsToday int64 `json:"checkins_today"`
	BlockedImages int64 `json:"blocked_images"`
	QueueDepth    int   `json:"queue_depth"`
	PendingBackup int64 `json:"pending_backup"`

	// StoredBytes counts what images actually occupy; DedupSavedBytes is the
	// difference between the sum of every reference and the sum of distinct
	// objects — i.e. what content-addressing bought us.
	StoredBytes     int64 `json:"stored_bytes"`
	DedupSavedBytes int64 `json:"dedup_saved_bytes"`
	OriginalBytes   int64 `json:"original_bytes"`
	GrantedBytes    int64 `json:"granted_bytes"`
}

type daySeries struct {
	Date    string `json:"date"`
	Uploads int64  `json:"uploads"`
	Bytes   int64  `json:"bytes"`
}

type formatBreakdown struct {
	Ext   string `json:"ext"`
	Count int64  `json:"count"`
	Bytes int64  `json:"bytes"`
}

type profileBreakdown struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
	Bytes int64  `json:"bytes"`
}

type recentLogin struct {
	Email     string    `json:"email"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	LoginAt   time.Time `json:"login_at"`
}

type backupFailure struct {
	ID        string    `json:"id"`
	OrigName  string    `json:"orig_name"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
}

type topUser struct {
	Email string `json:"email"`
	Used  int64  `json:"used_bytes"`
	Quota int64  `json:"quota_bytes"`
	Count int64  `json:"image_count"`
}

func (s *Server) adminDashboard(c *gin.Context) {
	out := dashboardOut{
		// Initialise every slice so JSON marshals as [] rather than null.
		UploadsByDay:    []daySeries{},
		ByFormat:        []formatBreakdown{},
		ByProfile:       []profileBreakdown{},
		RecentLogins:    []recentLogin{},
		BackupFailures:  []backupFailure{},
		TopUsersBySpace: []topUser{},
	}

	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	st := &out.Stats
	s.DB.Model(&models.User{}).Count(&st.TotalUsers)
	s.DB.Model(&models.CheckinRecord{}).Where("date = ?", now.Format("2006-01-02")).Count(&st.CheckinsToday)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL").Count(&st.TotalImages)
	s.DB.Model(&models.Image{}).Where("deleted_at IS NULL AND created_at >= ?", dayStart).Count(&st.ImagesToday)
	s.DB.Model(&models.Image{}).Where("status = ?", models.ImageBlocked).Count(&st.BlockedImages)
	s.DB.Model(&models.Image{}).
		Where("backup_state IN ?", []models.BackupState{models.BackupPending, models.BackupFailed}).
		Count(&st.PendingBackup)
	s.DB.Model(&models.User{}).Select("COALESCE(SUM(quota_bytes), 0)").Row().Scan(&st.GrantedBytes)

	// Every reference's stored size, vs. one row per distinct object. Both
	// come from the DB — walking 24TB of objects is not an option.
	var referencedBytes int64
	s.DB.Raw(`SELECT COALESCE(SUM(size_stored), 0) FROM images WHERE deleted_at IS NULL`).Row().Scan(&referencedBytes)
	s.DB.Raw(`
		SELECT COALESCE(SUM(size_stored), 0), COALESCE(SUM(size_orig), 0) FROM (
			SELECT DISTINCT ON (profile_id, sha256) size_stored, size_orig
			FROM images WHERE deleted_at IS NULL
			ORDER BY profile_id, sha256, created_at
		) d
	`).Row().Scan(&st.StoredBytes, &st.OriginalBytes)
	st.DedupSavedBytes = referencedBytes - st.StoredBytes

	if s.Queue != nil {
		st.QueueDepth = s.Queue.Depth()
	}

	out.UploadsByDay = s.uploadSeries(14)

	// Format + destination breakdowns over the last 30 days.
	cutoff := now.AddDate(0, 0, -30)
	s.DB.Model(&models.Image{}).
		Select("ext, COUNT(*) AS count, COALESCE(SUM(size_stored), 0) AS bytes").
		Where("deleted_at IS NULL AND created_at >= ?", cutoff).
		Group("ext").Order("count DESC").Scan(&out.ByFormat)

	s.DB.Raw(`
		SELECT p.name, p.kind, COUNT(i.id) AS count, COALESCE(SUM(i.size_stored), 0) AS bytes
		FROM storage_profiles p LEFT JOIN images i
		  ON i.profile_id = p.id AND i.deleted_at IS NULL
		GROUP BY p.id, p.name, p.kind
		ORDER BY bytes DESC
	`).Scan(&out.ByProfile)

	type loginRow struct {
		Email     string
		IP        string
		UserAgent string
		CreatedAt time.Time
	}
	var logins []loginRow
	s.DB.Raw(`
		SELECT u.email, s.ip, s.user_agent, s.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		ORDER BY s.created_at DESC LIMIT 10
	`).Scan(&logins)
	for _, l := range logins {
		out.RecentLogins = append(out.RecentLogins, recentLogin{
			Email: l.Email, IP: l.IP, UserAgent: l.UserAgent, LoginAt: l.CreatedAt,
		})
	}

	s.DB.Model(&models.Session{}).Where("expires_at > ?", now).Count(&out.ActiveSessions)

	var fails []backupFailure
	s.DB.Model(&models.Image{}).
		Select("id, orig_name, backup_error AS error, created_at").
		Where("backup_state = ?", models.BackupFailed).
		Order("created_at DESC").Limit(10).Scan(&fails)
	out.BackupFailures = append(out.BackupFailures, fails...)

	s.DB.Raw(`
		SELECT u.email, u.used_bytes AS used, u.quota_bytes AS quota,
		       COUNT(i.id) AS count
		FROM users u LEFT JOIN images i ON i.user_id = u.id AND i.deleted_at IS NULL
		GROUP BY u.id, u.email, u.used_bytes, u.quota_bytes
		ORDER BY u.used_bytes DESC LIMIT 10
	`).Scan(&out.TopUsersBySpace)

	c.JSON(http.StatusOK, out)
}

// uploadSeries returns one row per day for the last `days` days, ending today
// (UTC). Days with no uploads are present with zeroes so the chart doesn't
// silently compress its x-axis.
func (s *Server) uploadSeries(days int) []daySeries {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	since := dayStart.AddDate(0, 0, -(days - 1))

	type rawRow struct {
		Day     time.Time
		Uploads int64
		Bytes   int64
	}
	var rows []rawRow
	s.DB.Raw(`
		SELECT date_trunc('day', created_at AT TIME ZONE 'UTC') AS day,
		       COUNT(*) AS uploads,
		       COALESCE(SUM(size_stored), 0) AS bytes
		FROM images
		WHERE created_at >= ? AND deleted_at IS NULL
		GROUP BY 1
	`, since).Scan(&rows)

	out := make([]daySeries, days)
	for i := range out {
		out[i] = daySeries{Date: since.AddDate(0, 0, i).Format("2006-01-02")}
	}
	for _, r := range rows {
		i := int(r.Day.UTC().Sub(since).Hours() / 24)
		if i < 0 || i >= days {
			continue
		}
		out[i].Uploads += r.Uploads
		out[i].Bytes += r.Bytes
	}
	return out
}
