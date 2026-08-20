// Package checkin implements the daily space grant that replaces payment in
// this project: users earn storage by showing up rather than by paying.
//
// Idempotence comes from the (user_id, date) unique index on CheckinRecord —
// a double-submit hits a constraint violation and is reported as "already
// checked in today" rather than granting twice. The user's streak counter and
// the quota ledger are updated in the same transaction as the record, so a
// crash can't leave a grant without its receipt.
package checkin

import (
	"log"

	"errors"
	"fmt"
	"github.com/gentpan/openimg/backend/internal/aigen"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrAlreadyCheckedIn 是同一天里的第二次签到。"一天"按用户时区算,见 TodayIn。
var ErrAlreadyCheckedIn = errors.New("今天已经签到过了")

// Milestone lengths. A "month" is 30 days rather than a calendar month so the
// reward does not depend on which month a streak happens to start in —
// February would otherwise be worth the same as August for two days less work.
const (
	DaysPerWeek  = 7
	DaysPerMonth = 30
)

// Result describes what a successful check-in granted.
type Result struct {
	Granted int64 `json:"granted_bytes"`
	// AICredits 是本次签到附带送出的 AI 生成次数。0 表示这个用户组没开
	// 这项赠送,或者余额已经顶到上限。
	AICredits  int    `json:"ai_credits"`
	BonusPart  int64  `json:"bonus_bytes"` // week + month milestones, if any fired
	WeekBonus  int64  `json:"week_bonus_bytes"`
	MonthBonus int64  `json:"month_bonus_bytes"`
	Streak     int    `json:"streak"`
	Date       string `json:"date"`
	QuotaAfter int64  `json:"quota_bytes"`
	Capped     bool   `json:"capped"` // group cap swallowed part or all of the grant
}

// Today 是"今天"的日期键,按给定时区算。
//
// 原来一律用 UTC,理由写的是"这样跨时区旅行也不能一天签两次"。但那个性质对
// 非 UTC 用户本来就不成立:东八区的日界落在本地早上 8 点,7 点签一次、9 点
// 还能再签一次,一个自然日里就是两次。UTC 既没挡住它想挡的,又让所有东八区
// 用户的"一天"从早上 8 点开始——本地时间才是用户实际过的那一天。
//
// 防重复现在靠 Do() 里那道 `>=`:签过 8-20 之后,任何时区都换不出一个 ≤8-20
// 的"今天"。
func TodayIn(loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}

// Today 保留 UTC 语义,给拿不到用户的调用点用。
func Today() string { return TodayIn(time.UTC) }

// randomGrant picks a uniformly random amount in [min, max]. Degenerate
// ranges (min > max, or either unset) collapse to whichever bound is usable,
// so a half-configured group still grants something rather than erroring.
func randomGrant(min, max int64) int64 {
	if max <= 0 {
		return maxInt64(min, 0)
	}
	if min <= 0 {
		min = 1
	}
	if min >= max {
		return max
	}
	return min + rand.Int64N(max-min+1)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func yesterdayOf(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

// Do performs today's check-in for a user.
func Do(db *gorm.DB, user *models.User, group *models.UserGroup) (*Result, error) {
	if group == nil {
		return nil, fmt.Errorf("checkin: user has no group")
	}
	today := TodayIn(user.Location())

	// 已经签过今天、或者签过一个"更晚"的日期,都不给再签。
	//
	// 原来是等值判断,那在按本地时区算日界之后有个洞:签完之后把时区往西调,
	// "今天"会退回昨天,等值不成立就又能签一次。日期是 ISO 串,字典序就是时
	// 间序,一个 >= 就堵死了。
	if user.LastCheckinDate >= today && user.LastCheckinDate != "" {
		return nil, ErrAlreadyCheckedIn
	}

	// Streak continues only if the previous check-in was literally yesterday;
	// any gap resets it to 1.
	streak := 1
	if user.LastCheckinDate == yesterdayOf(today) {
		streak = user.CheckinStreak + 1
	}

	base := randomGrant(group.CheckinMinSpace, group.CheckinMaxSpace)

	// Milestones, not a permanent raise. The previous rule paid the bonus on
	// every check-in once the streak passed seven days, so day 400 earned the
	// same "streak bonus" as day 7 — there was nothing left to keep coming
	// back for. Now it lands on the day a whole week or a whole month closes.
	var weekBonus, monthBonus int64
	if streak > 0 && streak%DaysPerWeek == 0 {
		weekBonus = group.WeekBonusSpace
	}
	if streak > 0 && streak%DaysPerMonth == 0 {
		monthBonus = group.MonthBonusSpace
	}
	want := base + weekBonus + monthBonus
	if want <= 0 {
		return nil, fmt.Errorf("checkin: 当前用户组未配置签到奖励")
	}

	// The record is written first, inside its own transaction, so the unique
	// index rejects a concurrent double-submit before any space is granted.
	rec := models.CheckinRecord{
		ID:     uuid.New(),
		UserID: user.ID,
		Date:   today,
		Bytes:  want,
		Streak: streak,
	}
	if err := db.Create(&rec).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyCheckedIn
		}
		return nil, err
	}

	reason := fmt.Sprintf("每日签到（连续 %d 天）", streak)
	switch {
	case weekBonus > 0 && monthBonus > 0:
		reason = fmt.Sprintf("每日签到（连续 %d 天 · 满周 + 满月奖励）", streak)
	case monthBonus > 0:
		reason = fmt.Sprintf("每日签到（连续 %d 天 · 满月奖励）", streak)
	case weekBonus > 0:
		reason = fmt.Sprintf("每日签到（连续 %d 天 · 满周奖励）", streak)
	}
	granted, err := quota.GrantCapacity(db, user.ID, want, models.QuotaCheckin, reason, group.MaxTotalSpace)
	capped := false
	if errors.Is(err, quota.ErrCapped) {
		// The user is at their ceiling. The check-in still counts — losing a
		// streak because you're full would be a nasty surprise — but nothing
		// is granted. Record the zero so the ledger matches reality.
		capped = true
		granted = 0
		db.Model(&rec).Update("bytes", 0)
	} else if err != nil {
		// Roll the record back so the day can be retried rather than being
		// silently consumed by a transient DB error.
		db.Delete(&rec)
		return nil, err
	}
	if granted < want {
		capped = true
		if granted != want {
			db.Model(&rec).Update("bytes", granted)
		}
	}

	if err := db.Model(user).Updates(map[string]any{
		"last_checkin_date": today,
		"checkin_streak":    streak,
	}).Error; err != nil {
		return nil, err
	}
	user.LastCheckinDate = today
	user.CheckinStreak = streak

	var fresh models.User
	quotaAfter := user.QuotaBytes + granted
	if db.Select("quota_bytes").First(&fresh, "id = ?", user.ID).Error == nil {
		quotaAfter = fresh.QuotaBytes
	}

	effectiveBonus := weekBonus + monthBonus
	if granted < want && effectiveBonus > granted {
		// Report the bonus actually received, not the nominal one.
		effectiveBonus = granted
	}
	// 签到附带送 AI 次数。送不出去(没开/顶到上限)不算签到失败——空间已经
	// 发了,为一份附赠回滚整次签到不合算。
	aiGranted, aiErr := aigen.GrantCheckin(db, user, *group)
	if aiErr != nil {
		log.Printf("checkin: AI 次数赠送失败 user=%s: %v", user.ID, aiErr)
	}

	return &Result{
		Granted:    granted,
		AICredits:  aiGranted,
		BonusPart:  effectiveBonus,
		WeekBonus:  weekBonus,
		MonthBonus: monthBonus,
		Streak:     streak,
		Date:       today,
		QuotaAfter: quotaAfter,
		Capped:     capped,
	}, nil
}

// History returns the user's recent check-ins, newest first — the data behind
// the calendar on the space page.
func History(db *gorm.DB, userID uuid.UUID, limit int) ([]models.CheckinRecord, error) {
	if limit <= 0 || limit > 366 {
		limit = 60
	}
	out := []models.CheckinRecord{}
	err := db.Where("user_id = ?", userID).Order("date DESC").Limit(limit).Find(&out).Error
	return out, err
}

// isUniqueViolation detects Postgres SQLSTATE 23505. GORM wraps the driver
// error, so match on the code it carries in the message.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key value")
}
