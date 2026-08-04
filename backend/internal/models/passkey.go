package models

import (
	"time"

	"github.com/google/uuid"
)

// PasskeyCredential is one WebAuthn credential bound to a user. A user may
// register multiple passkeys (different devices). credential_id is the raw
// (binary) credential id from the authenticator.
type PasskeyCredential struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	CredentialID    []byte    `gorm:"type:bytea;uniqueIndex;not null" json:"-"`
	PublicKey       []byte    `gorm:"type:bytea;not null" json:"-"`
	AttestationType string    `gorm:"size:32" json:"-"`
	AAGUID          []byte    `gorm:"type:bytea" json:"-"`
	SignCount       uint32    `gorm:"not null;default:0" json:"-"`
	CloneWarning    bool      `gorm:"not null;default:false" json:"-"`
	// Flags is the single-byte representation of the authenticator's
	// CredentialFlags (UP/UV/BE/BS bits). Stored so we can return matching
	// flags during login validation; the BE bit in particular flips when a
	// user enables iCloud Keychain / Google Password Manager sync, and the
	// library will reject login if this doesn't match. We self-heal by
	// updating the stored byte from each successful login.
	Flags      uint8      `gorm:"not null;default:0" json:"-"`
	Transports string     `gorm:"size:255" json:"transports,omitempty"`
	Name       string     `gorm:"size:128" json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
