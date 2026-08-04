// Package referral handles the "invite friends, both get space" flow.
//
// Each user has a short ReferralCode (8 chars, base32-of-randomness). When
// someone signs up with `?ref=CODE` we set the new user's ReferredByID and
// grant both parties a one-time storage bonus, sized by the referrer's group.
package referral

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GenerateCode returns a fresh 8-char uppercase alphanumeric code, retrying
// on the (extremely unlikely) collision.
func GenerateCode(db *gorm.DB) (string, error) {
	for i := 0; i < 6; i++ {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		// base32 without padding, then take 8 chars; replace ambiguous letters.
		raw := strings.TrimRight(base32.StdEncoding.EncodeToString(b[:]), "=")
		code := strings.NewReplacer("0", "Z", "1", "X", "I", "Y", "O", "W").Replace(raw[:8])
		var existing models.User
		if err := db.Where("referral_code = ?", code).First(&existing).Error; err == gorm.ErrRecordNotFound {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate unique referral code")
}

// EnsureCode populates u.ReferralCode if it's empty, persisting it. Cheap to
// call repeatedly — no-op when already set.
func EnsureCode(db *gorm.DB, u *models.User) error {
	if u.ReferralCode != "" {
		return nil
	}
	code, err := GenerateCode(db)
	if err != nil {
		return err
	}
	u.ReferralCode = code
	return db.Model(u).Update("referral_code", code).Error
}

// BackfillCodes assigns codes to any existing users who don't have one. Safe
// to run on every boot — only touches rows where referral_code = ”.
func BackfillCodes(db *gorm.DB) {
	var users []models.User
	if err := db.Where("referral_code = '' OR referral_code IS NULL").Find(&users).Error; err != nil {
		return
	}
	for i := range users {
		_ = EnsureCode(db, &users[i])
	}
}

// LookupReferrer resolves a referral code to its user. Returns (nil, nil) if
// the code is empty or invalid — never an error for "not found", since this
// is called from the public registration path and we don't want to leak
// information about which codes exist.
func LookupReferrer(db *gorm.DB, code string) (*models.User, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, nil
	}
	var u models.User
	if err := db.Where("referral_code = ?", code).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// AttachAndReward links newUser to referrerCode (if valid + not self) and
// grants the signup bonus to both. Idempotent: safe to call even if newUser
// already has ReferredByID set (it'll skip).
func AttachAndReward(db *gorm.DB, newUser *models.User, code string) error {
	if newUser.ReferredByID != nil {
		return nil
	}
	ref, err := LookupReferrer(db, code)
	if err != nil || ref == nil {
		return err
	}
	if ref.ID == newUser.ID {
		return nil // can't refer self
	}
	newUser.ReferredByID = &ref.ID
	if err := db.Model(newUser).Update("referred_by_id", ref.ID).Error; err != nil {
		return err
	}
	// Both sides get the referrer's tier reward, each capped by their own
	// group's MaxTotalSpace.
	bonus := groupOf(db, ref).ReferralSpace
	if bonus <= 0 {
		return nil
	}
	newGroup := groupOf(db, newUser)
	_, _ = quota.GrantCapacity(db, newUser.ID, bonus, models.QuotaReferral,
		"受邀注册奖励（来自 "+MaskEmail(ref.Email)+"）", newGroup.MaxTotalSpace)
	_, _ = quota.GrantCapacity(db, ref.ID, bonus, models.QuotaReferral,
		"邀请 "+MaskEmail(newUser.Email)+" 注册奖励", groupOf(db, ref).MaxTotalSpace)
	return nil
}

// groupOf resolves a user's tier, falling back to "free" and then to a
// zero-value group so callers never have to nil-check.
func groupOf(db *gorm.DB, u *models.User) models.UserGroup {
	var g models.UserGroup
	if u.GroupID != nil {
		if err := db.First(&g, "id = ?", *u.GroupID).Error; err == nil {
			return g
		}
	}
	if err := db.Where("name = ?", "free").First(&g).Error; err == nil {
		return g
	}
	return models.UserGroup{}
}

// SummaryFor returns a snapshot for the "邀请好友" page.
type Summary struct {
	Code       string `json:"code"`
	Count      int    `json:"count"`
	TotalBytes int64  `json:"total_bytes"`
	BonusBytes int64  `json:"bonus_bytes"`
}

func SummaryFor(db *gorm.DB, u *models.User) (Summary, error) {
	out := Summary{Code: u.ReferralCode, BonusBytes: groupOf(db, u).ReferralSpace}
	var count int64
	db.Model(&models.User{}).Where("referred_by_id = ?", u.ID).Count(&count)
	out.Count = int(count)
	var sum struct{ S int64 }
	db.Model(&models.QuotaTransaction{}).
		Select("COALESCE(SUM(bytes),0) AS s").
		Where("user_id = ? AND type = ?", u.ID, models.QuotaReferral).
		Scan(&sum)
	out.TotalBytes = sum.S
	return out, nil
}

// ListInvitees returns the users this user has referred (for the table on
// the referral page). Email is masked for privacy (a***@example.com).
type Invitee struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func MaskEmail(e string) string {
	at := strings.Index(e, "@")
	if at <= 1 {
		return e
	}
	return e[:1] + strings.Repeat("*", min(at-1, 4)) + e[at:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ListInvitees(db *gorm.DB, refID uuid.UUID, limit int) ([]Invitee, error) {
	var rows []models.User
	if err := db.Where("referred_by_id = ?", refID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Invitee, 0, len(rows))
	for _, u := range rows {
		out = append(out, Invitee{
			Name:      u.Name,
			Email:     MaskEmail(u.Email),
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return out, nil
}
