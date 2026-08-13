package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type UserStatus string

const (
	UserActive    UserStatus = "active"
	UserSuspended UserStatus = "suspended"
)

type User struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Email         string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	EmailVerified bool      `gorm:"not null;default:false" json:"email_verified"`
	Name          string    `gorm:"size:128" json:"name"`
	AvatarURL     string    `gorm:"size:512" json:"avatar_url,omitempty"`
	// AvatarKey is the object key behind AvatarURL. Stored separately because
	// the URL is a rendered address that changes when the CDN domain changes,
	// while the key is what we need to delete the old file.
	AvatarKey    string     `gorm:"size:255" json:"-"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	Role         UserRole   `gorm:"size:16;not null;default:'user';index" json:"role"`
	Status       UserStatus `gorm:"size:16;not null;default:'active';index" json:"status"`
	GroupID      *uuid.UUID `gorm:"type:uuid;index" json:"group_id,omitempty"`

	// OAuth linkage (one user can be linked to multiple providers)
	GoogleSub string `gorm:"size:64;index" json:"-"`
	GithubID  string `gorm:"size:64;index" json:"-"`

	// Storage quota, in bytes. Both are denormalised counters — the canonical
	// history lives in quota_transactions and can be replayed to rebuild them.
	// QuotaBytes is total capacity earned (signup + check-ins + referrals);
	// UsedBytes is what current images occupy on the platform pool. Uploads to
	// a user's own bucket (BYOS) touch neither.
	QuotaBytes int64 `gorm:"not null;default:0" json:"quota_bytes"`
	UsedBytes  int64 `gorm:"not null;default:0" json:"used_bytes"`

	// Last UTC date the user checked in (yyyy-mm-dd), and how many consecutive
	// days that streak has run. Empty date means never checked in.
	LastCheckinDate string `gorm:"size:10;index" json:"last_checkin_date,omitempty"`
	CheckinStreak   int    `gorm:"not null;default:0" json:"checkin_streak"`

	// DefaultProfileID is the storage destination new uploads land on. Nil
	// means "use the platform pool".
	DefaultProfileID *uuid.UUID `gorm:"type:uuid" json:"default_profile_id,omitempty"`

	// Conversion preferences. Both default on: the derivatives are what make
	// the pool affordable. Turning them off stores only the re-encoded
	// original, which costs less quota per image but serves bigger files.
	// The original is always re-encoded regardless — that's the EXIF-stripping
	// and polyglot-killing step, not an optimisation.
	// UploadMode is "optimized" (re-encode, strip metadata) or "original"
	// (store the bytes untouched, EXIF and all). Original mode still sniffs the
	// real format and still generates thumbnails — the thumbnails are what the
	// site itself renders, and serving full-size originals in a grid would be
	// ruinous for both bandwidth and page weight.
	UploadMode string `gorm:"size:16;not null;default:'optimized'" json:"upload_mode"`
	// VariantFormat is the conversion target: "none" | "webp" | "avif". 优化
	// 模式下主图直接编码成该格式,不保留原格式主图,也没有附加变体;原样
	// 模式下不参与。单选:主图只有一种格式。
	VariantFormat string `gorm:"size:8;not null;default:'webp'" json:"variant_format"`
	// MaxImageWidth downscales anything wider before storing it. 0 keeps the
	// original size. Most uploads are far larger than they'll ever be
	// displayed, so this is the single biggest quota saving available.
	// Downscale only — a narrow image is never blown up.
	MaxImageWidth int `gorm:"not null;default:0" json:"max_image_width"`
	// Thumbnail policy: the width and format of the grid tier generated at
	// upload. Width 600 and WebP are the defaults the site always used; the
	// choice exists because the trade is genuinely the user's — 200px AVIF
	// for someone hoarding quota, 1000px JPEG for someone embedding into
	// software that never learned WebP.
	ThumbWidth  int    `gorm:"not null;default:600" json:"thumb_width"`
	ThumbFormat string `gorm:"size:8;not null;default:'webp'" json:"thumb_format"`

	// Referral. ReferralCode is the user's own unique code shareable in links;
	// ReferredByID is set on signup if they came in through someone's link.
	ReferralCode string     `gorm:"size:16;uniqueIndex" json:"referral_code,omitempty"`
	ReferredByID *uuid.UUID `gorm:"type:uuid;index" json:"referred_by_id,omitempty"`

	// Provenance, kept for abuse handling and visible only in the admin UI.
	SignupIP    string `gorm:"size:64;index" json:"-"`
	LastLoginIP string `gorm:"size:64" json:"-"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

func NewUser(email, name, passwordHash string) User {
	return User{
		ID:           uuid.New(),
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		Role:         RoleUser,
		Status:       UserActive,
	}
}

func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// UserGroup is the tier: how much space a member can earn, how large and how
// many files they may upload, and whether they're allowed to bring their own
// bucket. All byte fields are absolute counts, not MB.
type UserGroup struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"size:255" json:"description"`

	// Upload limits.
	MaxFileSize      int64  `gorm:"not null;default:10485760" json:"max_file_size"` // per file
	DailyUploadCount int    `gorm:"not null;default:50" json:"daily_upload_count"`  // per UTC day
	AllowedFormats   string `gorm:"size:255;not null;default:'jpeg,png,webp,gif,avif'" json:"allowed_formats"`
	MaxWidth         int    `gorm:"not null;default:8000" json:"max_width"` // reject larger, guards decode bombs
	MaxHeight        int    `gorm:"not null;default:8000" json:"max_height"`

	// Space economy (platform pool only). Every grant here is permanent — it
	// raises QuotaBytes and never expires or decays.
	SignupSpace int64 `gorm:"not null;default:1073741824" json:"signup_space"` // granted on registration
	// Daily check-in pays a random amount in [CheckinMinSpace, CheckinMaxSpace].
	// The randomness is the point: a fixed number is a chore, a variable one is
	// worth coming back for.
	CheckinMinSpace  int64 `gorm:"not null;default:1048576" json:"checkin_min_space"`
	CheckinMaxSpace  int64 `gorm:"not null;default:20971520" json:"checkin_max_space"`
	StreakBonusSpace int64 `gorm:"not null;default:31457280" json:"streak_bonus_space"` // deprecated, see WeekBonusSpace
	StreakBonusDays  int   `gorm:"not null;default:7" json:"streak_bonus_days"`         // deprecated
	// Milestone bonuses, paid on the day the streak reaches a whole week or a
	// whole month — not every day thereafter, which is what the old
	// StreakBonus* pair did: once a user passed seven days it kept paying the
	// bonus daily, forever.
	WeekBonusSpace  int64 `gorm:"not null;default:52428800" json:"week_bonus_space"`   // 50 MiB
	MonthBonusSpace int64 `gorm:"not null;default:314572800" json:"month_bonus_space"` // 300 MiB
	ReferralSpace   int64 `gorm:"not null;default:209715200" json:"referral_space"`    // per successful referral
	MaxTotalSpace   int64 `gorm:"not null;default:10737418240" json:"max_total_space"` // 0 = uncapped

	// Bring-your-own-storage.
	AllowBYOS   bool `gorm:"not null;default:true" json:"allow_byos"`
	MaxProfiles int  `gorm:"not null;default:3" json:"max_profiles"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FormatList splits AllowedFormats into normalised extensions.
func (g UserGroup) FormatList() []string {
	out := []string{}
	for _, f := range strings.Split(g.AllowedFormats, ",") {
		if f = strings.ToLower(strings.TrimSpace(f)); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// canonFormat folds the spellings that mean the same format. Existing rows
// (and anything an admin types by hand) say "jpeg", while the detector and the
// encoder both say "jpg" — without this, every JPEG upload is rejected.
func CanonFormat(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	switch ext {
	case "jpeg", "jpe":
		return "jpg"
	case "heif":
		return "heic"
	case "tif":
		return "tiff"
	}
	return ext
}

func (g UserGroup) AllowsFormat(ext string) bool {
	ext = CanonFormat(ext)
	for _, f := range g.FormatList() {
		if CanonFormat(f) == ext {
			return true
		}
	}
	return false
}

// Session is a server-issued JWT id (we keep a row so we can invalidate).
type Session struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string    `gorm:"size:128;uniqueIndex;not null" json:"-"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:255" json:"user_agent"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
