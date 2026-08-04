package models

import (
	"crypto/rand"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ImageStatus string

const (
	ImageActive  ImageStatus = "active"  // normal, servable
	ImageBlocked ImageStatus = "blocked" // hidden by moderation, object still present
	ImageDeleted ImageStatus = "deleted" // soft-deleted, object removal pending
)

type BackupState string

const (
	BackupNone    BackupState = "none"    // no backup profile configured
	BackupPending BackupState = "pending" // queued for replication
	BackupDone    BackupState = "done"
	BackupFailed  BackupState = "failed"
)

// Variant names stored in Image.Variants (CSV). The object key of every
// variant is derivable from SHA256 + variant name, so we only record which
// ones exist — see VariantKey.
const (
	VariantWebP  = "webp"
	VariantAVIF  = "avif"
	VariantW200  = "w200"
	VariantW600  = "w600"
	VariantW1200 = "w1200"

	// VariantOriginalPrefix marks the untouched upload kept alongside the
	// optimised primary. The source extension is baked into the name
	// ("orig-heic") because unlike every other variant the original's format is
	// whatever the user sent, and the key has to be derivable from the CSV
	// alone — that CSV is what the purge job walks when it deletes objects.
	VariantOriginalPrefix = "orig-"
)

// Image is one uploaded picture. Content is addressed by SHA256 so the same
// bytes uploaded by different users land on a single stored object per
// profile; the row is per-user (that's the reference).
//
// ObjectKey never includes the host — the public URL is assembled at read
// time from the owning StorageProfile's PublicBaseURL. That indirection is
// what lets a user swap CDN domains without rewriting history.
type Image struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	ProfileID uuid.UUID `gorm:"type:uuid;index;not null" json:"profile_id"`

	// SHA256 of the *original* uploaded bytes, before any re-encoding.
	// Indexed together with ProfileID for dedup lookups.
	SHA256    string `gorm:"size:64;index:idx_images_profile_sha,priority:2;not null" json:"sha256"`
	ObjectKey string `gorm:"size:512;index;not null" json:"object_key"`
	// ShortCode is the 4–6 character path of the image's short link. Unique,
	// unlike ObjectKey — deduplicated uploads share an object but each row is
	// a separate image with its own link, so one user deleting theirs must not
	// break the other's.
	ShortCode string `gorm:"size:8;uniqueIndex" json:"short_code,omitempty"`

	OrigName string `gorm:"size:255" json:"orig_name"`
	MIME     string `gorm:"size:64;not null" json:"mime"`
	Ext      string `gorm:"size:8;not null" json:"ext"`
	Width    int    `gorm:"not null;default:0" json:"width"`
	Height   int    `gorm:"not null;default:0" json:"height"`

	// SizeOrig is what the user handed us; SizeStored is what we actually
	// occupy after optimisation, summed across every variant. Quota is
	// charged on SizeStored.
	SizeOrig   int64 `gorm:"not null;default:0" json:"size_orig"`
	SizeStored int64 `gorm:"not null;default:0" json:"size_stored"`
	// Breakdown of SizeStored. Kept as columns rather than derived on read
	// because the objects live in S3 — recomputing would mean a HEAD per
	// variant per image. The three always sum to SizeStored.
	SizePrimary  int64 `gorm:"not null;default:0" json:"size_primary"`
	SizeVariants int64 `gorm:"not null;default:0" json:"size_variants"` // full-size webp/avif
	SizeThumbs   int64 `gorm:"not null;default:0" json:"size_thumbs"`

	// Variants is a CSV of generated derivative names (see Variant* consts).
	Variants string `gorm:"size:128" json:"variants"`

	BackupState BackupState `gorm:"size:16;not null;default:'none';index" json:"backup_state"`
	BackupError string      `gorm:"size:255" json:"backup_error,omitempty"`

	Status    ImageStatus `gorm:"size:16;not null;default:'active';index" json:"status"`
	ViewCount int64       `gorm:"not null;default:0" json:"view_count"`

	// Upload provenance, kept for abuse handling.
	UploadIP string `gorm:"size:64;index" json:"-"`

	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// KeyIDLength is the random part of a new object key.
//
// 12 characters of a 62-symbol alphabet is 62^12 ≈ 3.2e21 possibilities. At a
// billion images the chance of any collision is around 1e-4 — and the upload
// path re-rolls on the rare hit anyway, so the number only has to make that
// retry effectively never happen.
const KeyIDLength = 12

// keyAlphabet is digits and letters only. No "-" or "_": they survive a URL
// intact but not a double-click (which stops at "-" in some browsers), a
// line-wrapped email, or a Markdown emphasis run.
const keyAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NewKeyID returns a random, URL-safe identifier.
//
// Sampling is by rejection rather than modulo: 256 is not a multiple of 62, so
// `b % 62` would make the first 8 symbols ~25% more likely than the rest and
// quietly shrink the keyspace.
func NewKeyID() string {
	const max = byte(255 - (256 % len(keyAlphabet))) // 247
	out := make([]byte, 0, KeyIDLength)
	buf := make([]byte, KeyIDLength)
	for len(out) < KeyIDLength {
		if _, err := rand.Read(buf); err != nil {
			// crypto/rand does not fail in practice; if it ever did, a
			// predictable key would be worse than a crash.
			panic("models: crypto/rand unavailable: " + err.Error())
		}
		for _, b := range buf {
			if b > max {
				continue
			}
			out = append(out, keyAlphabet[int(b)%len(keyAlphabet)])
			if len(out) == KeyIDLength {
				break
			}
		}
	}
	return string(out)
}

// ReservedShortCodes are paths the short-link space must never claim.
//
// The code alphabet and length overlap the app's own routes — "login",
// "admin", "space" and "refer" are all five characters of [0-9a-zA-Z], and
// "upload" is six. A code equal to any of these would shadow a real page.
// Generation skips them and lookup rejects them, so the two can never disagree.
var ReservedShortCodes = map[string]bool{
	// Client-side routes.
	"login": true, "admin": true, "space": true, "refer": true, "upload": true,
	"docs": true, "help": true, "about": true, "terms": true, "abuse": true,
	// Server prefixes and conventional well-known names, in case the short
	// link handler is ever mounted where these could collide.
	"api": true, "auth": true, "s": true, "img": true, "cdn": true, "www": true,
	"static": true, "assets": true, "public": true, "health": true, "robots": true,
	"report": true, "signup": true, "logout": true, "user": true, "users": true,
}

// IsValidShortCode reports whether s could be a short code at all. Checked
// before any database lookup so an arbitrary path never costs a query.
func IsValidShortCode(s string) bool {
	if len(s) < 4 || len(s) > 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return !ReservedShortCodes[strings.ToLower(s)]
}

// NewShortCode returns a candidate code of the given length. Callers must
// still confirm it is unused — the code space is small enough that collisions
// are ordinary, not exceptional.
func NewShortCode(length int) string {
	for {
		out := make([]byte, length)
		id := NewKeyID()
		copy(out, id[:length])
		if IsValidShortCode(string(out)) {
			return string(out)
		}
	}
}

// ObjectKeyFor builds a key for a newly stored image:
//
//	2026/08/04/aB3dEf7hJ9kL.jpg
//
// The date is the upload day in UTC. It exists to keep any single prefix from
// collecting millions of objects — the same job the old two-level SHA sharding
// did — while producing a link a human can read and place in time.
//
// Keys are still write-once: nothing ever overwrites an existing key, which is
// what keeps the one-year immutable cache header honest.
func ObjectKeyFor(t time.Time, ext string) string {
	return t.UTC().Format("2006/01/02") + "/" + NewKeyID() + "." + ext
}

func OriginalVariant(ext string) string { return VariantOriginalPrefix + ext }

// IsOriginalVariant reports whether a variant name refers to a kept original.
func IsOriginalVariant(name string) bool { return strings.HasPrefix(name, VariantOriginalPrefix) }

// BaseKey strips the extension off an object key, giving the stem every
// derivative hangs off.
func BaseKey(objectKey string) string {
	return strings.TrimSuffix(objectKey, path.Ext(objectKey))
}

// VariantKey returns the object key of a derivative, derived from the image's
// own stored key rather than recomputed from its hash.
//
// That indirection is what makes the key format changeable at all: images
// written under the old content-addressed scheme (img/ab/cd/<sha>.jpg) yield
// byte-identical derivative keys through this function, so old and new layouts
// coexist with no migration and no rewritten objects.
//
// Width-prefixed variants (w200/w600/w1200) are always WebP; webp/avif are
// full-size re-encodes.
func VariantKey(objectKey, variant string) string {
	base := BaseKey(objectKey)
	if IsOriginalVariant(variant) {
		// …/aB3dEf7hJ9kL.orig.heic
		return base + ".orig." + strings.TrimPrefix(variant, VariantOriginalPrefix)
	}
	switch variant {
	case VariantWebP:
		return base + ".webp"
	case VariantAVIF:
		return base + ".avif"
	default:
		// w200 → …/aB3dEf7hJ9kL_w200.webp
		return base + "_" + variant + ".webp"
	}
}

func (i Image) VariantList() []string {
	if i.Variants == "" {
		return nil
	}
	out := []string{}
	for _, v := range strings.Split(i.Variants, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (i Image) HasVariant(name string) bool {
	for _, v := range i.VariantList() {
		if v == name {
			return true
		}
	}
	return false
}
