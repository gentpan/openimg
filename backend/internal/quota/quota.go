// Package quota owns every read/write of User.QuotaBytes and User.UsedBytes.
// Always go through these helpers so quota_transactions stays a faithful
// replay log of both counters.
//
// Two ledger families share the table:
//   - capacity  (signup / check-in / referral / admin) moves QuotaBytes
//   - occupancy (upload / delete) moves UsedBytes
//
// Available space is QuotaBytes - UsedBytes. Uploads to a user's own bucket
// bypass this package entirely — they cost us nothing.
package quota

import (
	"errors"
	"fmt"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrInsufficient means the upload would exceed the user's earned space.
	ErrInsufficient = errors.New("insufficient storage quota")
	// ErrCapped means a grant was refused because the group's MaxTotalSpace
	// is already reached.
	ErrCapped = errors.New("storage quota cap reached")
)

// Available is the headroom a user has left, never negative.
func Available(u *models.User) int64 {
	if n := u.QuotaBytes - u.UsedBytes; n > 0 {
		return n
	}
	return 0
}

// GrantCapacity adds headroom and records it. `cap` is the group's
// MaxTotalSpace (0 = uncapped); the grant is clamped so QuotaBytes never
// exceeds it. Returns the number of bytes actually granted, which may be less
// than requested or zero.
func GrantCapacity(db *gorm.DB, userID uuid.UUID, bytes int64, kind models.QuotaTxType, reason string, cap int64) (granted int64, err error) {
	if bytes <= 0 {
		return 0, nil
	}
	if !kind.AffectsCapacity() {
		return 0, fmt.Errorf("quota: %s is not a capacity grant", kind)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var u models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", userID).Error; err != nil {
			return err
		}
		want := bytes
		if cap > 0 {
			if u.QuotaBytes >= cap {
				return ErrCapped
			}
			if u.QuotaBytes+want > cap {
				want = cap - u.QuotaBytes
			}
		}
		if want <= 0 {
			return ErrCapped
		}
		newQuota := u.QuotaBytes + want
		if err := tx.Model(&u).Update("quota_bytes", newQuota).Error; err != nil {
			return err
		}
		granted = want
		return tx.Create(&models.QuotaTransaction{
			ID:         uuid.New(),
			UserID:     userID,
			Type:       kind,
			Bytes:      want,
			QuotaAfter: newQuota,
			UsedAfter:  u.UsedBytes,
			Reason:     reason,
		}).Error
	})
	if errors.Is(err, ErrCapped) {
		return 0, ErrCapped
	}
	return granted, err
}

// Consume reserves space for an upload. Returns ErrInsufficient (without
// writing anything) when the user lacks headroom.
func Consume(db *gorm.DB, userID uuid.UUID, bytes int64, reason string, imageID *uuid.UUID) error {
	if bytes <= 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var u models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", userID).Error; err != nil {
			return err
		}
		newUsed := u.UsedBytes + bytes
		if newUsed > u.QuotaBytes {
			return ErrInsufficient
		}
		if err := tx.Model(&u).Update("used_bytes", newUsed).Error; err != nil {
			return err
		}
		return tx.Create(&models.QuotaTransaction{
			ID:         uuid.New(),
			UserID:     userID,
			Type:       models.QuotaUpload,
			Bytes:      -bytes,
			QuotaAfter: u.QuotaBytes,
			UsedAfter:  newUsed,
			Reason:     reason,
			ImageID:    imageID,
		}).Error
	})
}

// Release gives space back when an image is deleted. UsedBytes is floored at
// zero so a double-delete or a drifted counter can't push it negative.
func Release(db *gorm.DB, userID uuid.UUID, bytes int64, reason string, imageID *uuid.UUID) error {
	if bytes <= 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var u models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", userID).Error; err != nil {
			return err
		}
		newUsed := u.UsedBytes - bytes
		if newUsed < 0 {
			newUsed = 0
		}
		if err := tx.Model(&u).Update("used_bytes", newUsed).Error; err != nil {
			return err
		}
		return tx.Create(&models.QuotaTransaction{
			ID:         uuid.New(),
			UserID:     userID,
			Type:       models.QuotaDelete,
			Bytes:      bytes,
			QuotaAfter: u.QuotaBytes,
			UsedAfter:  newUsed,
			Reason:     reason,
			ImageID:    imageID,
		}).Error
	})
}

// AdminAdjust grants (or, with a negative amount, revokes) capacity by hand.
// Revocation is allowed to push QuotaBytes below UsedBytes — the user simply
// can't upload again until they delete something.
func AdminAdjust(db *gorm.DB, userID uuid.UUID, bytes int64, reason string) (int64, error) {
	if bytes > 0 {
		return GrantCapacity(db, userID, bytes, models.QuotaAdmin, reason, 0)
	}
	if bytes == 0 {
		return 0, nil
	}
	var newQuota int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var u models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", userID).Error; err != nil {
			return err
		}
		newQuota = u.QuotaBytes + bytes // bytes is negative here
		if newQuota < 0 {
			newQuota = 0
		}
		if err := tx.Model(&u).Update("quota_bytes", newQuota).Error; err != nil {
			return err
		}
		return tx.Create(&models.QuotaTransaction{
			ID:         uuid.New(),
			UserID:     userID,
			Type:       models.QuotaAdmin,
			Bytes:      newQuota - u.QuotaBytes,
			QuotaAfter: newQuota,
			UsedAfter:  u.UsedBytes,
			Reason:     reason,
		}).Error
	})
	return newQuota, err
}

// SignupGrant is the welcome allowance. Silently no-ops when the group grants
// nothing, so callers don't need to branch.
func SignupGrant(db *gorm.DB, userID uuid.UUID, group *models.UserGroup) (int64, error) {
	if group == nil || group.SignupSpace <= 0 {
		return 0, nil
	}
	n, err := GrantCapacity(db, userID, group.SignupSpace, models.QuotaSignup, "注册赠送空间", group.MaxTotalSpace)
	if errors.Is(err, ErrCapped) {
		return 0, nil
	}
	return n, err
}

// Reconcile recomputes UsedBytes from the images table and repairs drift.
// Returns the delta applied. Meant for an admin button and a periodic sweep —
// the counters are denormalised, so they can drift after a crash mid-upload.
func Reconcile(db *gorm.DB, userID uuid.UUID) (delta int64, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		var u models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", userID).Error; err != nil {
			return err
		}
		var actual int64
		row := tx.Model(&models.Image{}).
			Joins("JOIN storage_profiles p ON p.id = images.profile_id").
			Where("images.user_id = ? AND images.deleted_at IS NULL AND p.kind = ?", userID, models.ProfilePlatform).
			Select("COALESCE(SUM(images.size_stored), 0)").Row()
		if err := row.Scan(&actual); err != nil {
			return err
		}
		delta = actual - u.UsedBytes
		if delta == 0 {
			return nil
		}
		return tx.Model(&u).Update("used_bytes", actual).Error
	})
	return delta, err
}

// BackfillMissingGrants repairs accounts whose QuotaBytes was set without a
// matching ledger entry — either the old admin-bootstrap path, or a signup
// route that skipped SignupGrant.
//
// It writes the missing receipt rather than re-granting: QuotaBytes is left
// exactly as-is, and a synthetic signup_grant row is inserted so the ledger
// replays to the balance the user already has. Safe to run on every boot; it
// only touches users with a positive balance and no capacity rows at all.
func BackfillMissingGrants(db *gorm.DB) (repaired int, err error) {
	var users []models.User
	err = db.Where(`quota_bytes > 0 AND id NOT IN (
			SELECT DISTINCT user_id FROM quota_transactions WHERE type IN (?, ?, ?, ?)
		)`,
		models.QuotaSignup, models.QuotaCheckin, models.QuotaReferral, models.QuotaAdmin,
	).Find(&users).Error
	if err != nil {
		return 0, err
	}
	for _, u := range users {
		row := models.QuotaTransaction{
			ID:         uuid.New(),
			UserID:     u.ID,
			Type:       models.QuotaSignup,
			Bytes:      u.QuotaBytes,
			QuotaAfter: u.QuotaBytes,
			UsedAfter:  u.UsedBytes,
			Reason:     "初始空间（账本补录）",
		}
		if err := db.Create(&row).Error; err != nil {
			return repaired, err
		}
		repaired++
	}
	return repaired, nil
}
