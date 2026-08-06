package models

import "time"

// SiteSetting is a generic key/value store for runtime-editable site config.
// Values are JSON blobs interpreted by their respective consumers.
type SiteSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	SiteSettingUpload  = "upload"  // value is JSON of UploadSettings
)

// UploadSettings are the upload-pipeline knobs an admin can change at runtime.
type UploadSettings struct {
	// KeepOriginal stores the untouched upload alongside the optimised copy.
	// Off by default: it roughly doubles what an upload costs, so it is the
	// operator's call, not a per-user one.
	KeepOriginal bool `json:"keep_original"`
}
