package models

import (
	"time"

	"github.com/google/uuid"
)

// Reaction is one emoji response to an image on its share page.
//
// Anonymous visitors may react — the share page is what gets pasted into
// forums, so most of the people who see it will never have an account, and a
// reaction bar only signed-in users can touch would sit at zero forever.
type Reaction struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ImageID uuid.UUID `gorm:"type:uuid;index:idx_reaction_identity,priority:1;not null" json:"image_id"`

	// Identity is who reacted: the user id for a signed-in visitor, otherwise a
	// hash of the client address. Stored as one column rather than two nullable
	// ones so a single unique index can enforce "one reaction per person per
	// image" across both cases.
	//
	// The address is hashed, not stored: this is a decoration on a public page,
	// and keeping a plain-text log of who looked at what is a liability with no
	// matching benefit.
	Identity string `gorm:"size:64;index:idx_reaction_identity,priority:2;not null" json:"-"`

	Kind      string    `gorm:"size:16;not null" json:"kind"`
	CreatedAt time.Time `json:"created_at"`
}

// ReactionKinds are the offered responses, in display order.
//
// Deliberately all positive. A share page is someone showing you their
// picture; a downvote button there is an invitation to be unpleasant, and the
// report flow already exists for content that is actually a problem.
var ReactionKinds = []string{"like", "clap", "party", "sparkle", "smile"}

func ValidReactionKind(k string) bool {
	for _, v := range ReactionKinds {
		if v == k {
			return true
		}
	}
	return false
}
