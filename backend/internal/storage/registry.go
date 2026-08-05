package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gentpan/openimg/backend/internal/crypto"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNoPlatformProfile = errors.New("storage: no platform profile configured")
	ErrProfileNotFound   = errors.New("storage: profile not found")
)

// Registry resolves a models.StorageProfile into a live Backend, caching the
// underlying client. Building an S3 client per upload would be wasteful —
// each one carries its own HTTP transport and connection pool — so entries are
// keyed by profile ID and invalidated when the row's UpdatedAt moves.
type Registry struct {
	db     *gorm.DB
	cipher *crypto.Cipher

	// devFallback is used when no platform profile exists yet, so a fresh
	// checkout runs without MinIO. Nil in production.
	devFallback Backend

	mu    sync.RWMutex
	cache map[uuid.UUID]*cacheEntry
}

type cacheEntry struct {
	backend   Backend
	updatedAt time.Time
}

func NewRegistry(db *gorm.DB, cipher *crypto.Cipher, devFallback Backend) *Registry {
	return &Registry{
		db:          db,
		cipher:      cipher,
		devFallback: devFallback,
		cache:       map[uuid.UUID]*cacheEntry{},
	}
}

// For returns the Backend behind a profile, building and caching it on first
// use.
func (r *Registry) For(ctx context.Context, p *models.StorageProfile) (Backend, error) {
	if p == nil {
		return nil, ErrProfileNotFound
	}
	r.mu.RLock()
	entry, ok := r.cache[p.ID]
	r.mu.RUnlock()
	if ok && entry.updatedAt.Equal(p.UpdatedAt) {
		return entry.backend, nil
	}

	backend, err := r.build(ctx, p)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[p.ID] = &cacheEntry{backend: backend, updatedAt: p.UpdatedAt}
	r.mu.Unlock()
	return backend, nil
}

// ForStored resolves the backend holding an already-stored object. A nil ID
// means the object predates any storage profile — it lives on the local-disk
// fallback — so route it to the platform resolver rather than failing the
// lookup. Background jobs (transcode, backup, purge) all go through here;
// using ForID directly made every one of them fail in that mode.
func (r *Registry) ForStored(ctx context.Context, id uuid.UUID) (Backend, *models.StorageProfile, error) {
	if id == uuid.Nil {
		return r.Platform(ctx)
	}
	return r.ForID(ctx, id)
}

// ForID loads a profile by ID and resolves it.
func (r *Registry) ForID(ctx context.Context, id uuid.UUID) (Backend, *models.StorageProfile, error) {
	var p models.StorageProfile
	if err := r.db.First(&p, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrProfileNotFound
		}
		return nil, nil, err
	}
	b, err := r.For(ctx, &p)
	return b, &p, err
}

func (r *Registry) build(ctx context.Context, p *models.StorageProfile) (Backend, error) {
	access, err := r.cipher.Decrypt(p.AccessKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("profile %s: decrypt access key: %w", p.Name, err)
	}
	secret, err := r.cipher.Decrypt(p.SecretKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("profile %s: decrypt secret key: %w", p.Name, err)
	}
	return NewS3(ctx, S3Config{
		Endpoint:      p.Endpoint,
		Region:        p.Region,
		Bucket:        p.Bucket,
		AccessKey:     access,
		SecretKey:     secret,
		PublicURLBase: p.PublicBaseURL,
		KeyPrefix:     p.KeyPrefix,
		UsePathStyle:  p.PathStyle,
		Kind:          string(p.Kind),
	})
}

// Invalidate drops a cached client, forcing a rebuild on next use. Call after
// updating a profile's credentials.
func (r *Registry) Invalidate(id uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, id)
	r.mu.Unlock()
}

// Platform returns the shared pool profile. Falls back to a local filesystem
// backend when none is configured, so development doesn't require MinIO.
func (r *Registry) Platform(ctx context.Context) (Backend, *models.StorageProfile, error) {
	var p models.StorageProfile
	err := r.db.Where("kind = ? AND backup_of_id IS NULL", models.ProfilePlatform).
		Order("is_default DESC, created_at ASC").First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if r.devFallback != nil {
			return r.devFallback, nil, nil
		}
		return nil, nil, ErrNoPlatformProfile
	}
	if err != nil {
		return nil, nil, err
	}
	b, err := r.For(ctx, &p)
	return b, &p, err
}

// Resolve picks the destination for a user's next upload: their chosen default
// profile if it's still valid, otherwise the platform pool. Returning the
// profile alongside the backend matters because the caller needs to know
// whether the upload consumes platform quota.
func (r *Registry) Resolve(ctx context.Context, u *models.User) (Backend, *models.StorageProfile, error) {
	if u != nil && u.DefaultProfileID != nil {
		var p models.StorageProfile
		err := r.db.First(&p, "id = ?", *u.DefaultProfileID).Error
		switch {
		case err == nil && p.Status == models.ProfileActive && (p.UserID == nil || *p.UserID == u.ID):
			b, berr := r.For(ctx, &p)
			if berr == nil {
				return b, &p, nil
			}
			log.Printf("storage: user %s default profile %s unusable, falling back to platform: %v", u.ID, p.ID, berr)
		case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
			return nil, nil, err
		}
	}
	return r.Platform(ctx)
}

// BackupFor returns the replication target of a profile, or (nil, nil, nil)
// when none is configured.
func (r *Registry) BackupFor(ctx context.Context, profileID uuid.UUID) (Backend, *models.StorageProfile, error) {
	var p models.StorageProfile
	err := r.db.Where("backup_of_id = ? AND status = ?", profileID, models.ProfileActive).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	b, err := r.For(ctx, &p)
	return b, &p, err
}

// EnsurePlatformProfile bootstraps the shared pool from S3_* env vars on first
// boot. Once the row exists it is the source of truth and env changes are
// ignored — otherwise an admin edit in the UI would be silently reverted on
// every restart.
func (r *Registry) EnsurePlatformProfile(ctx context.Context, cfg S3Config) error {
	var count int64
	if err := r.db.Model(&models.StorageProfile{}).
		Where("kind = ?", models.ProfilePlatform).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if cfg.Bucket == "" {
		log.Printf("storage: no platform profile and no S3_BUCKET — using local filesystem backend")
		return nil
	}
	if !r.cipher.Enabled() {
		return fmt.Errorf("storage: STORAGE_MASTER_KEY must be set to store the platform bucket credentials")
	}
	accessEnc, err := r.cipher.Encrypt(cfg.AccessKey)
	if err != nil {
		return err
	}
	secretEnc, err := r.cipher.Encrypt(cfg.SecretKey)
	if err != nil {
		return err
	}
	p := models.StorageProfile{
		ID:            uuid.New(),
		UserID:        nil,
		Kind:          models.ProfilePlatform,
		Name:          "平台存储池",
		Endpoint:      cfg.Endpoint,
		Region:        cfg.Region,
		Bucket:        cfg.Bucket,
		KeyPrefix:     cfg.KeyPrefix,
		PathStyle:     cfg.UsePathStyle,
		AccessKeyEnc:  accessEnc,
		SecretKeyEnc:  secretEnc,
		AccessKeyMask: crypto.Mask(cfg.AccessKey),
		PublicBaseURL: cfg.PublicURLBase,
		ThumbBaseURL:  cfg.ThumbURLBase,
		IsDefault:     true,
		Status:        models.ProfileActive,
	}
	if err := r.db.Create(&p).Error; err != nil {
		return err
	}
	log.Printf("storage: bootstrapped platform profile bucket=%s base=%s", p.Bucket, p.PublicBaseURL)
	return nil
}

// URLFor assembles an object's public address without building a client — the
// hot path for listing images, where only the profile's base URL is needed.
//
// fallbackBase is the site's own origin, used when no storage profile exists
// (development, serving from the local disk) or when a profile was saved
// without an absolute base. The result is always absolute: a relative link is
// worthless the moment a user pastes it into a forum or a README.
// ThumbURLFor is URLFor for a display-size variant: it prefers the profile's
// thumbnail origin and falls back to the main one when none is configured.
func ThumbURLFor(p *models.StorageProfile, key, fallbackBase string) string {
	if p != nil && p.ThumbBaseURL != "" {
		return JoinURL(strings.TrimRight(p.ThumbBaseURL, "/"), p.KeyPrefix+key)
	}
	return URLFor(p, key, fallbackBase)
}

func URLFor(p *models.StorageProfile, key, fallbackBase string) string {
	base := ""
	full := key
	if p != nil {
		base = p.PublicBaseURL
		full = p.KeyPrefix + key
	} else {
		full = "storage/" + key
	}
	if !isAbsoluteURL(base) {
		// Join whatever the profile had (often empty, sometimes a bare path)
		// underneath the site origin.
		base = JoinURL(strings.TrimRight(fallbackBase, "/"), strings.Trim(base, "/"))
	}
	return JoinURL(base, full)
}

func isAbsoluteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
