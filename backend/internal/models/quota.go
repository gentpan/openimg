package models

import (
	"time"

	"github.com/google/uuid"
)

type QuotaTxType string

const (
	// Capacity grants — these move User.QuotaBytes.
	QuotaSignup   QuotaTxType = "signup_grant"
	QuotaCheckin  QuotaTxType = "checkin"
	QuotaReferral QuotaTxType = "referral"
	QuotaAdmin    QuotaTxType = "admin_grant"
	// Occupancy changes — these move User.UsedBytes.
	QuotaUpload QuotaTxType = "upload"
	QuotaDelete QuotaTxType = "delete_refund"
)

// AffectsCapacity distinguishes the two ledger families: capacity grants add
// headroom, occupancy changes consume it. Keeping both in one table gives the
// user a single readable timeline.
func (t QuotaTxType) AffectsCapacity() bool {
	switch t {
	case QuotaSignup, QuotaCheckin, QuotaReferral, QuotaAdmin:
		return true
	default:
		return false
	}
}

// QuotaTransaction is the append-only ledger behind User.QuotaBytes and
// User.UsedBytes. Both denormalised counters are reconcilable from here.
type QuotaTransaction struct {
	ID     uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID   `gorm:"type:uuid;index;not null" json:"user_id"`
	Type   QuotaTxType `gorm:"size:24;not null;index" json:"type"`
	// Bytes is signed: positive grants capacity or refunds occupancy,
	// negative consumes.
	Bytes int64 `gorm:"not null" json:"bytes"`
	// Snapshot of both counters after this row was applied.
	QuotaAfter int64      `gorm:"not null" json:"quota_after"`
	UsedAfter  int64      `gorm:"not null" json:"used_after"`
	Reason     string     `gorm:"size:255" json:"reason"`
	ImageID    *uuid.UUID `gorm:"type:uuid;index" json:"image_id,omitempty"`
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
}

// CheckinRecord is one day's check-in. The (UserID, Date) unique index is what
// makes check-in idempotent under concurrent requests — a double-submit hits
// a constraint violation rather than granting twice.
type CheckinRecord struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_checkin_user_date,priority:1;not null" json:"user_id"`
	Date   string    `gorm:"size:10;uniqueIndex:idx_checkin_user_date,priority:2;not null" json:"date"` // yyyy-mm-dd UTC
	Bytes  int64     `gorm:"not null" json:"bytes"`
	Streak int       `gorm:"not null;default:1" json:"streak"`

	CreatedAt time.Time `json:"created_at"`
}
