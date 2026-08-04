package models

import (
	"time"

	"github.com/google/uuid"
)

type ReportStatus string

const (
	ReportOpen     ReportStatus = "open"
	ReportResolved ReportStatus = "resolved"
)

// Report is an abuse complaint about one image. Reporters may be anonymous —
// requiring an account to report is a good way to never hear about the problem
// until your host does.
type Report struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ImageID uuid.UUID `gorm:"type:uuid;index;not null" json:"image_id"`

	ReporterID *uuid.UUID `gorm:"type:uuid;index" json:"reporter_id,omitempty"`
	ReporterIP string     `gorm:"size:64;index" json:"-"`
	Reason     string     `gorm:"size:500;not null" json:"reason"`
	Contact    string     `gorm:"size:255" json:"contact,omitempty"`

	Status ReportStatus `gorm:"size:16;not null;default:'open';index" json:"status"`
	// Action records what the moderator did: dismiss | block | block_and_ban.
	Action     string     `gorm:"size:24" json:"action,omitempty"`
	Note       string     `gorm:"size:500" json:"note,omitempty"`
	ResolvedBy *uuid.UUID `gorm:"type:uuid" json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
