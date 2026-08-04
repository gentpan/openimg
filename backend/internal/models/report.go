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
	// Category is the reporter's chosen bucket; Reason is their free text.
	// Kept apart so the admin list can be triaged at a glance and filtered,
	// which a single blob of prose cannot support.
	Category string `gorm:"size:24;index" json:"category,omitempty"`
	Reason   string `gorm:"size:500;not null" json:"reason"`
	Contact  string `gorm:"size:255" json:"contact,omitempty"`

	Status ReportStatus `gorm:"size:16;not null;default:'open';index" json:"status"`
	// Action records what the moderator did: dismiss | block | block_and_ban.
	Action     string     `gorm:"size:24" json:"action,omitempty"`
	Note       string     `gorm:"size:500" json:"note,omitempty"`
	ResolvedBy *uuid.UUID `gorm:"type:uuid" json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// ReportCategories are the offered reasons, in display order. The keys are
// stable identifiers; the labels are what a reporter and an admin both see, so
// the two never describe the same report differently.
var ReportCategories = []struct{ Key, Label string }{
	{"porn", "涉黄内容"},
	{"copyright", "侵犯版权"},
	{"violence", "涉恐或血腥"},
	{"privacy", "个人隐私"},
	{"fraud", "诈骗或恶意链接"},
	{"other", "其他违规"},
}

func ReportCategoryLabel(key string) string {
	for _, c := range ReportCategories {
		if c.Key == key {
			return c.Label
		}
	}
	return "其他违规"
}

func ValidReportCategory(key string) bool {
	for _, c := range ReportCategories {
		if c.Key == key {
			return true
		}
	}
	return false
}
