package api

import (
	"context"
	"errors"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/crypto"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// profileOut is the safe projection of a StorageProfile. Credentials never
// leave the server — only the masked access key does.
type profileOut struct {
	ID            string  `json:"id"`
	Kind          string  `json:"kind"`
	Name          string  `json:"name"`
	Endpoint      string  `json:"endpoint"`
	Region        string  `json:"region"`
	Bucket        string  `json:"bucket"`
	KeyPrefix     string  `json:"key_prefix"`
	PathStyle     bool    `json:"path_style"`
	AccessKeyMask string  `json:"access_key_mask"`
	PublicBaseURL string  `json:"public_base_url"`
	IsDefault     bool    `json:"is_default"`
	IsPlatform    bool    `json:"is_platform"`
	BackupOfID    *string `json:"backup_of_id,omitempty"`
	Status        string  `json:"status"`
	LastError     string  `json:"last_error,omitempty"`
	LastCheckedAt string  `json:"last_checked_at,omitempty"`
	ImageCount    int64   `json:"image_count"`
	StoredBytes   int64   `json:"stored_bytes"`
}

func toProfileOut(p models.StorageProfile, isUserDefault bool) profileOut {
	out := profileOut{
		ID:            p.ID.String(),
		Kind:          string(p.Kind),
		Name:          p.Name,
		Endpoint:      p.Endpoint,
		Region:        p.Region,
		Bucket:        p.Bucket,
		KeyPrefix:     p.KeyPrefix,
		PathStyle:     p.PathStyle,
		AccessKeyMask: p.AccessKeyMask,
		PublicBaseURL: p.PublicBaseURL,
		IsDefault:     isUserDefault,
		IsPlatform:    p.IsPlatform(),
		Status:        string(p.Status),
		LastError:     p.LastError,
	}
	if p.BackupOfID != nil {
		s := p.BackupOfID.String()
		out.BackupOfID = &s
	}
	if p.LastCheckedAt != nil {
		out.LastCheckedAt = p.LastCheckedAt.Format(time.RFC3339)
	}
	return out
}

// GET /api/storage/profiles — the user's own buckets plus the platform pool
// they can fall back to.
func (s *Server) handleListProfiles(c *gin.Context) {
	u := auth.MustUser(c)
	var profiles []models.StorageProfile
	s.DB.Where("user_id = ? OR kind = ?", u.ID, models.ProfilePlatform).
		Order("kind ASC, created_at ASC").Find(&profiles)

	usage := map[string][2]int64{}
	rows := []struct {
		ProfileID string
		N         int64
		Bytes     int64
	}{}
	s.DB.Model(&models.Image{}).
		Select("profile_id, COUNT(*) AS n, COALESCE(SUM(size_stored),0) AS bytes").
		Where("user_id = ? AND deleted_at IS NULL", u.ID).
		Group("profile_id").Scan(&rows)
	for _, r := range rows {
		usage[r.ProfileID] = [2]int64{r.N, r.Bytes}
	}

	out := make([]profileOut, 0, len(profiles))
	for _, p := range profiles {
		isDefault := u.DefaultProfileID != nil && *u.DefaultProfileID == p.ID
		// With no explicit choice, uploads land on the platform pool.
		if u.DefaultProfileID == nil && p.IsPlatform() {
			isDefault = true
		}
		item := toProfileOut(p, isDefault)
		if v, ok := usage[p.ID.String()]; ok {
			item.ImageCount, item.StoredBytes = v[0], v[1]
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"profiles": out})
}

type profileReq struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	KeyPrefix string `json:"key_prefix"`
	// PathStyle is a pointer so "not sent" is distinguishable from "false" —
	// unset means infer it from the endpoint.
	PathStyle     *bool  `json:"path_style"`
	AccessKey     string `json:"access_key"`
	SecretKey     string `json:"secret_key"` // empty on edit = keep existing
	PublicBaseURL string `json:"public_base_url"`
	TestOnly      bool   `json:"test_only"`
}

// inferKind labels a bucket from its endpoint. R2, AWS and MinIO all speak the
// same S3 API — this is only used for display and for picking a sensible
// addressing default, never to change behaviour the user can't see.
func inferKind(endpoint string) models.ProfileKind {
	e := strings.ToLower(endpoint)
	if strings.Contains(e, "r2.cloudflarestorage.com") {
		return models.ProfileUserR2
	}
	return models.ProfileUserS3
}

// inferPathStyle picks the addressing mode. Cloud providers use virtual-hosted
// buckets (bucket.host); self-hosted servers like MinIO almost always need
// path-style (host/bucket). Getting this wrong is the single most common
// "connection failed" cause, so we guess rather than asking.
func inferPathStyle(endpoint string) bool {
	e := strings.ToLower(endpoint)
	for _, hosted := range []string{
		"r2.cloudflarestorage.com",
		"amazonaws.com",
		"backblazeb2.com",
		"digitaloceanspaces.com",
		"aliyuncs.com",
		"myqcloud.com",
	} {
		if strings.Contains(e, hosted) {
			return false
		}
	}
	return true
}

// POST /api/storage/profiles — add a bucket. The probe must pass before
// anything is persisted: a bucket that can't be written to is worse than no
// bucket, because uploads would fail after the user thinks they're set up.
func (s *Server) handleCreateProfile(c *gin.Context) {
	u := auth.MustUser(c)
	g := s.groupFor(u)
	if !g.AllowBYOS {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "profile.byos_denied")})
		return
	}
	if !s.Cipher.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": i18n.T(c, "profile.no_master_key"),
		})
		return
	}

	var req profileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var count int64
	s.DB.Model(&models.StorageProfile{}).Where("user_id = ?", u.ID).Count(&count)
	if g.MaxProfiles > 0 && count >= int64(g.MaxProfiles) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": i18n.T(c, "profile.limit_reached"), "max": g.MaxProfiles,
		})
		return
	}

	cfg, kind, err := s.buildProfileConfig(c.Request.Context(), req, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	if err := probeConfig(ctx, cfg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": i18n.T(c, "profile.test_failed", err.Error())})
		return
	}
	if req.TestOnly {
		c.JSON(http.StatusOK, gin.H{"ok": true, "tested": true})
		return
	}

	accessEnc, err := s.Cipher.Encrypt(cfg.AccessKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	secretEnc, err := s.Cipher.Encrypt(cfg.SecretKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now()
	p := models.StorageProfile{
		ID:            uuid.New(),
		UserID:        &u.ID,
		Kind:          kind,
		Name:          pickStr(strings.TrimSpace(req.Name), cfg.Bucket),
		Endpoint:      cfg.Endpoint,
		Region:        cfg.Region,
		Bucket:        cfg.Bucket,
		KeyPrefix:     cfg.KeyPrefix,
		PathStyle:     cfg.UsePathStyle,
		AccessKeyEnc:  accessEnc,
		SecretKeyEnc:  secretEnc,
		AccessKeyMask: crypto.Mask(cfg.AccessKey),
		PublicBaseURL: cfg.PublicURLBase,
		Status:        models.ProfileActive,
		LastCheckedAt: &now,
	}
	if err := s.DB.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toProfileOut(p, false))
}

// PATCH /api/storage/profiles/:id
func (s *Server) handleUpdateProfile(c *gin.Context) {
	u := auth.MustUser(c)
	p, err := s.ownedProfile(c, u)
	if err != nil {
		return // ownedProfile already answered
	}

	var req profileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfg, kind, err := s.buildProfileConfig(c.Request.Context(), req, p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	if err := probeConfig(ctx, cfg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": i18n.T(c, "profile.test_failed", err.Error())})
		return
	}
	if req.TestOnly {
		c.JSON(http.StatusOK, gin.H{"ok": true, "tested": true})
		return
	}

	accessEnc, _ := s.Cipher.Encrypt(cfg.AccessKey)
	secretEnc, _ := s.Cipher.Encrypt(cfg.SecretKey)
	now := time.Now()
	updates := map[string]any{
		"name":            pickStr(strings.TrimSpace(req.Name), p.Name),
		"kind":            kind,
		"endpoint":        cfg.Endpoint,
		"region":          cfg.Region,
		"bucket":          cfg.Bucket,
		"key_prefix":      cfg.KeyPrefix,
		"path_style":      cfg.UsePathStyle,
		"access_key_enc":  accessEnc,
		"secret_key_enc":  secretEnc,
		"access_key_mask": crypto.Mask(cfg.AccessKey),
		"public_base_url": cfg.PublicURLBase,
		"status":          models.ProfileActive,
		"last_error":      "",
		"last_checked_at": &now,
	}
	if err := s.DB.Model(p).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Credentials changed — drop the cached client so the next upload rebuilds it.
	s.Storage.Invalidate(p.ID)
	s.DB.First(p, "id = ?", p.ID)
	c.JSON(http.StatusOK, toProfileOut(*p, u.DefaultProfileID != nil && *u.DefaultProfileID == p.ID))
}

// POST /api/storage/profiles/:id/test — re-run the probe on a saved profile.
func (s *Server) handleTestProfile(c *gin.Context) {
	u := auth.MustUser(c)
	p, err := s.ownedProfile(c, u)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	backend, _, err := s.Storage.ForID(ctx, p.ID)
	if err == nil {
		err = backend.Probe(ctx)
	}
	now := time.Now()
	if err != nil {
		s.DB.Model(p).Updates(map[string]any{
			"status":          models.ProfileInvalid,
			"last_error":      truncate(err.Error(), 480),
			"last_checked_at": &now,
		})
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
		return
	}
	s.DB.Model(p).Updates(map[string]any{
		"status":          models.ProfileActive,
		"last_error":      "",
		"last_checked_at": &now,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/storage/profiles/:id/default — route new uploads here. The
// platform pool is a legal choice and is represented by clearing the field.
func (s *Server) handleSetDefaultProfile(c *gin.Context) {
	u := auth.MustUser(c)
	p, err := s.ownedProfile(c, u)
	if err != nil {
		return
	}
	if p.Status != models.ProfileActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "profile.not_ready")})
		return
	}
	if p.BackupOfID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "profile.backup_as_default")})
		return
	}
	var val any = p.ID
	if p.IsPlatform() {
		val = nil // nil means "platform pool", which is the fallback anyway
	}
	if err := s.DB.Model(u).Update("default_profile_id", val).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setBackupReq struct {
	// BackupOfID is the profile this one mirrors. Empty string detaches it.
	BackupOfID string `json:"backup_of_id"`
}

// POST /api/storage/profiles/:id/backup — mark this profile as another's
// replication target.
func (s *Server) handleSetBackupProfile(c *gin.Context) {
	u := auth.MustUser(c)
	p, err := s.ownedProfile(c, u)
	if err != nil {
		return
	}
	if p.IsPlatform() {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "profile.platform_backup")})
		return
	}
	var req setBackupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.BackupOfID) == "" {
		s.DB.Model(p).Update("backup_of_id", nil)
		c.JSON(http.StatusOK, gin.H{"ok": true, "backup_of_id": nil})
		return
	}
	srcID, err := uuid.Parse(req.BackupOfID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "profile.bad_backup_of_id")})
		return
	}
	if srcID == p.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "profile.backup_self")})
		return
	}
	// The source must be visible to this user, and must not itself be a backup
	// — chained replication would silently double every upload's cost.
	var src models.StorageProfile
	if err := s.DB.Where("id = ? AND (user_id = ? OR kind = ?)", srcID, u.ID, models.ProfilePlatform).
		First(&src).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "profile.source_not_found")})
		return
	}
	if src.BackupOfID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "profile.source_is_backup")})
		return
	}
	var existing int64
	s.DB.Model(&models.StorageProfile{}).
		Where("backup_of_id = ? AND id <> ?", srcID, p.ID).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": i18n.T(c, "profile.backup_exists")})
		return
	}
	if err := s.DB.Model(p).Update("backup_of_id", srcID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "backup_of_id": srcID})
}

// DELETE /api/storage/profiles/:id — refuses while images still point at it,
// because the rows would become unservable.
func (s *Server) handleDeleteProfile(c *gin.Context) {
	u := auth.MustUser(c)
	p, err := s.ownedProfile(c, u)
	if err != nil {
		return
	}
	if p.IsPlatform() {
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "profile.platform_locked")})
		return
	}
	var n int64
	s.DB.Model(&models.Image{}).Where("profile_id = ? AND deleted_at IS NULL", p.ID).Count(&n)
	if n > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": i18n.T(c, "profile.has_images"),
			"count": n,
		})
		return
	}
	if u.DefaultProfileID != nil && *u.DefaultProfileID == p.ID {
		s.DB.Model(u).Update("default_profile_id", nil)
	}
	if err := s.DB.Delete(p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.Storage.Invalidate(p.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ownedProfile loads the :id profile and verifies the caller owns it. It
// writes the error response itself and returns a non-nil error when the
// caller should stop.
func (s *Server) ownedProfile(c *gin.Context, u *models.User) (*models.StorageProfile, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "profile.bad_id")})
		return nil, err
	}
	var p models.StorageProfile
	if err := s.DB.First(&p, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "profile.not_found")})
		return nil, err
	}
	// Admins may touch platform profiles; nobody may touch another user's.
	if p.UserID == nil {
		if !u.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(c, "profile.platform_readonly")})
			return nil, gorm.ErrRecordNotFound
		}
	} else if *p.UserID != u.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "profile.not_found")})
		return nil, gorm.ErrRecordNotFound
	}
	return &p, nil
}

// buildProfileConfig normalises and validates a submitted config. `existing`
// is non-nil on edit, which is what lets an empty secret mean "keep the one
// on file" — we never send secrets to the client, so requiring a retype on
// every edit would be hostile.
func (s *Server) buildProfileConfig(ctx context.Context, req profileReq, existing *models.StorageProfile) (storage.S3Config, models.ProfileKind, error) {
	endpoint := strings.TrimSpace(req.Endpoint)
	kind := inferKind(endpoint)
	pathStyle := inferPathStyle(endpoint)
	if req.PathStyle != nil {
		pathStyle = *req.PathStyle
	}

	cfg := storage.S3Config{
		Endpoint:      endpoint,
		Region:        pickStr(strings.TrimSpace(req.Region), "auto"),
		Bucket:        strings.TrimSpace(req.Bucket),
		AccessKey:     strings.TrimSpace(req.AccessKey),
		SecretKey:     req.SecretKey, // never trimmed — could contain meaningful whitespace
		PublicURLBase: strings.TrimRight(strings.TrimSpace(req.PublicBaseURL), "/"),
		KeyPrefix:     strings.TrimSpace(req.KeyPrefix),
		UsePathStyle:  pathStyle,
		Kind:          string(kind),
	}
	if cfg.Bucket == "" {
		return cfg, kind, errors.New(i18n.TCtx(ctx, "profile.bucket_required"))
	}
	if err := storage.ValidateUserEndpoint(cfg.Endpoint); err != nil {
		return cfg, kind, err
	}
	if err := storage.ValidatePublicBaseURL(cfg.PublicURLBase); err != nil {
		return cfg, kind, err
	}
	if cfg.KeyPrefix != "" && !strings.HasSuffix(cfg.KeyPrefix, "/") {
		cfg.KeyPrefix += "/"
	}

	if existing != nil {
		if cfg.AccessKey == "" {
			prev, err := s.Cipher.Decrypt(existing.AccessKeyEnc)
			if err != nil {
				return cfg, kind, err
			}
			cfg.AccessKey = prev
		}
		if cfg.SecretKey == "" {
			prev, err := s.Cipher.Decrypt(existing.SecretKeyEnc)
			if err != nil {
				return cfg, kind, err
			}
			cfg.SecretKey = prev
		}
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return cfg, kind, errors.New(i18n.TCtx(ctx, "profile.keys_required"))
	}
	return cfg, kind, nil
}

func probeConfig(ctx context.Context, cfg storage.S3Config) error {
	backend, err := storage.NewS3(ctx, cfg)
	if err != nil {
		return err
	}
	return backend.Probe(ctx)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
