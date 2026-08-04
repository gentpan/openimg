package models

import (
	"time"

	"github.com/google/uuid"
)

// EmailOTP is a single-use one-time-password sent to an email address. We
// store only a SHA-256 hash of the code so a DB leak doesn't yield active
// codes. Rate-limited at insertion time by the handler.
type EmailOTP struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Email string    `gorm:"size:255;index;not null" json:"email"`
	// Purpose scopes the code to one action. Without it a code mailed out for
	// "change your password" would also satisfy the login endpoint, so anyone
	// who talked a user into reading out a code could take the account — the
	// classic OTP-phishing shape. Codes are only ever accepted by the purpose
	// they were issued for.
	Purpose   string     `gorm:"size:24;not null;default:'login';index" json:"purpose"`
	CodeHash  string     `gorm:"size:64;not null" json:"-"`
	IP        string     `gorm:"size:64" json:"-"`
	Attempts  int        `gorm:"not null;default:0" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"-"`
	UsedAt    *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
}

// OTP purposes. The wording in the email names the action, so a user who is
// being socially engineered has a chance to notice the mismatch.
const (
	OTPPurposeLogin    = "login"
	OTPPurposePassword = "password"
	OTPPurposePasskey  = "passkey"
	OTPPurposePurge    = "purge"
	OTPPurposeReset    = "reset"
	OTPPurposeRegister = "register"
)

// OTPPurposeLabel is the human name used in the subject line and the UI.
func OTPPurposeLabel(p string) string {
	switch p {
	case OTPPurposePassword:
		return "修改密码"
	case OTPPurposePasskey:
		return "添加 Passkey"
	case OTPPurposePurge:
		return "清空图库"
	case OTPPurposeReset:
		return "重置密码"
	case OTPPurposeRegister:
		return "注册"
	default:
		return "登录"
	}
}

func ValidOTPPurpose(p string) bool {
	switch p {
	case OTPPurposeLogin, OTPPurposePassword, OTPPurposePasskey, OTPPurposePurge, OTPPurposeReset,
		OTPPurposeRegister:
		return true
	}
	return false
}
