package models

import (
	"time"

	"github.com/google/uuid"
)

type ProfileKind string

const (
	// ProfilePlatform is the shared pool we host (MinIO). UserID is nil.
	ProfilePlatform ProfileKind = "platform"
	// ProfileUserR2 / ProfileUserS3 are bring-your-own-storage buckets. The
	// user pays for them, so they don't consume platform quota.
	ProfileUserR2 ProfileKind = "user_r2"
	ProfileUserS3 ProfileKind = "user_s3"
)

type ProfileStatus string

const (
	ProfileActive  ProfileStatus = "active"
	ProfileInvalid ProfileStatus = "invalid" // last probe failed
	ProfileTesting ProfileStatus = "testing"
)

// StorageProfile is one S3-compatible destination. Exactly one platform
// profile is seeded at boot; users may add their own R2/S3 buckets on top.
//
// Credentials are stored AES-256-GCM encrypted (see internal/crypto) and are
// never returned to the client — only AccessKeyMask is.
type StorageProfile struct {
	ID     uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	UserID *uuid.UUID  `gorm:"type:uuid;index" json:"user_id,omitempty"` // nil = platform-owned
	Kind   ProfileKind `gorm:"size:16;not null;index" json:"kind"`
	Name   string      `gorm:"size:64;not null" json:"name"`

	Endpoint  string `gorm:"size:255" json:"endpoint"`
	Region    string `gorm:"size:32;not null;default:'auto'" json:"region"`
	Bucket    string `gorm:"size:128;not null" json:"bucket"`
	KeyPrefix string `gorm:"size:128" json:"key_prefix"`
	// PathStyle is true for MinIO and most self-hosted S3; false for R2.
	PathStyle bool `gorm:"not null;default:false" json:"path_style"`

	AccessKeyEnc  []byte `gorm:"type:bytea" json:"-"`
	SecretKeyEnc  []byte `gorm:"type:bytea" json:"-"`
	AccessKeyMask string `gorm:"size:32" json:"access_key_mask"`

	// PublicBaseURL is the CDN origin this bucket is served from, e.g.
	// "https://cdn.openimg.io". Object URLs are assembled at read time.
	PublicBaseURL string `gorm:"size:255;not null" json:"public_base_url"`

	// IsDefault marks the profile new uploads land on. Scoped per user; the
	// platform profile is the fallback when a user has none.
	IsDefault bool `gorm:"not null;default:false" json:"is_default"`
	// BackupOfID, when set, makes this profile the replication target of
	// another one. Uploads to the source are async-copied here.
	BackupOfID *uuid.UUID `gorm:"type:uuid;index" json:"backup_of_id,omitempty"`

	Status        ProfileStatus `gorm:"size:16;not null;default:'active'" json:"status"`
	LastError     string        `gorm:"size:500" json:"last_error,omitempty"`
	LastCheckedAt *time.Time    `json:"last_checked_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsPlatform reports whether uploads here consume platform quota.
func (p StorageProfile) IsPlatform() bool { return p.Kind == ProfilePlatform }

// APIToken authenticates non-browser uploaders (PicGo, Typora, curl). Only
// the SHA-256 of the token is stored; the plaintext is shown exactly once.
type APIToken struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	Name       string     `gorm:"size:64;not null" json:"name"`
	TokenHash  string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	Prefix     string     `gorm:"size:12;not null" json:"prefix"` // display only, e.g. "oimg_a1b2c3"
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Revoked    bool       `gorm:"not null;default:false;index" json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
}
