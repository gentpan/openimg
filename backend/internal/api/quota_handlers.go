package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/checkin"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gin-gonic/gin"
)

// GET /api/quota — the space page's header: balance, tier limits, and what
// today's check-in would be worth.
func (s *Server) handleGetQuota(c *gin.Context) {
	u := auth.MustUser(c)
	g := s.groupFor(u)

	var uploadsToday int64
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	s.DB.Model(&models.Image{}).
		Where("user_id = ? AND created_at >= ? AND deleted_at IS NULL", u.ID, dayStart).
		Count(&uploadsToday)

	var imageCount int64
	s.DB.Model(&models.Image{}).Where("user_id = ? AND deleted_at IS NULL", u.ID).Count(&imageCount)

	today := checkin.Today()
	nextStreak := 1
	if u.LastCheckinDate == today {
		nextStreak = u.CheckinStreak
	} else if u.CheckinStreak > 0 {
		// Only continues if the last check-in was yesterday; otherwise the
		// preview should show a reset streak rather than an optimistic one.
		if u.LastCheckinDate == time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02") {
			nextStreak = u.CheckinStreak + 1
		}
	}
	// The next grant is a range, not a number — the amount is rolled at
	// check-in time. Only the streak bonus is deterministic.
	nextMin, nextMax := g.CheckinMinSpace, g.CheckinMaxSpace
	if g.StreakBonusDays > 0 && nextStreak >= g.StreakBonusDays {
		nextMin += g.StreakBonusSpace
		nextMax += g.StreakBonusSpace
	}

	c.JSON(http.StatusOK, gin.H{
		"quota_bytes":     u.QuotaBytes,
		"used_bytes":      u.UsedBytes,
		"available_bytes": quota.Available(u),
		"image_count":     imageCount,
		"uploads_today":   uploadsToday,
		"tier": gin.H{
			"name":               g.Name,
			"description":        g.Description,
			"max_file_size":      g.MaxFileSize,
			"daily_upload_count": g.DailyUploadCount,
			"allowed_formats":    g.FormatList(),
			"max_total_space":    g.MaxTotalSpace,
			"allow_byos":         g.AllowBYOS,
			"max_profiles":       g.MaxProfiles,
		},
		"checkin": gin.H{
			"checked_in_today":  u.LastCheckinDate == today,
			"streak":            u.CheckinStreak,
			"last_date":         u.LastCheckinDate,
			"next_min_bytes":    nextMin,
			"next_max_bytes":    nextMax,
			"streak_bonus_days": g.StreakBonusDays,
			"week_bonus":        g.WeekBonusSpace,
			"month_bonus":       g.MonthBonusSpace,
			"days_per_week":     checkin.DaysPerWeek,
			"days_per_month":    checkin.DaysPerMonth,
			"min_bytes":         g.CheckinMinSpace,
			"max_bytes":         g.CheckinMaxSpace,
			"streak_bonus":      g.StreakBonusSpace,
		},
	})
}

// POST /api/checkin — claim today's space.
func (s *Server) handleCheckin(c *gin.Context) {
	u := auth.MustUser(c)
	g := s.groupFor(u)
	res, err := checkin.Do(s.DB, u, &g)
	if errors.Is(err, checkin.ErrAlreadyCheckedIn) {
		c.JSON(http.StatusConflict, gin.H{
			"error":  err.Error(),
			"streak": u.CheckinStreak,
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if res.Capped && res.Granted == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"granted": 0,
			"streak":  res.Streak,
			"capped":  true,
			"message": "签到成功，但空间已达上限，本次未发放",
		})
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /api/checkin/history — calendar data.
func (s *Server) handleCheckinHistory(c *gin.Context) {
	u := auth.MustUser(c)
	limit := 60
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	rows, err := checkin.History(s.DB, u.ID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"records": rows})
}

// GET /api/quota/transactions — the space ledger.
func (s *Server) handleQuotaTransactions(c *gin.Context) {
	u := auth.MustUser(c)
	limit := 20
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

	// The total comes back with the page so the client can render "page 3 of
	// 12" without a second round trip, and so it can tell "empty page" apart
	// from "past the end".
	var total int64
	base := s.DB.Model(&models.QuotaTransaction{}).Where("user_id = ?", u.ID)
	base.Count(&total)

	txs := []models.QuotaTransaction{}
	base.Order("created_at DESC").Limit(limit).Offset(offset).Find(&txs)
	c.JSON(http.StatusOK, gin.H{
		"transactions": txs, "total": total, "limit": limit, "offset": offset,
	})
}
