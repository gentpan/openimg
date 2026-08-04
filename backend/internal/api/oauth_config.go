package api

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"gorm.io/gorm"
)

// OAuth credentials can come from two places: the environment file (set at
// deploy time) or the admin UI (set at runtime, stored in site_settings with
// the secrets encrypted). The DB wins when present, so an operator can finish
// the Google/GitHub setup without a redeploy — which matters because you can't
// know the client ID until you've filled in the redirect URI, and you can't
// know the redirect URI until the site is already up.
type oauthSettings struct {
	GoogleClientID     string `json:"google_client_id"`
	GoogleClientSecret string `json:"-"`
	GithubClientID     string `json:"github_client_id"`
	GithubClientSecret string `json:"-"`
}

// storedOAuth is the on-disk shape: secrets are AES-GCM ciphertext.
type storedOAuth struct {
	GoogleClientID  string `json:"google_client_id"`
	GoogleSecretEnc []byte `json:"google_secret_enc"`
	GithubClientID  string `json:"github_client_id"`
	GithubSecretEnc []byte `json:"github_secret_enc"`
}

const oauthSettingKey = "oauth"

// oauthCache avoids hitting the DB on every /auth/providers poll — the login
// page asks for it on each render.
type oauthCache struct {
	mu        sync.RWMutex
	value     oauthSettings
	loadedAt  time.Time
	populated bool
}

const oauthCacheTTL = 30 * time.Second

// oauth resolves the effective credentials: DB first, env as fallback.
func (s *Server) oauth() oauthSettings {
	s.oauthCache.mu.RLock()
	if s.oauthCache.populated && time.Since(s.oauthCache.loadedAt) < oauthCacheTTL {
		v := s.oauthCache.value
		s.oauthCache.mu.RUnlock()
		return v
	}
	s.oauthCache.mu.RUnlock()

	// Start from the environment, then overlay anything the admin has saved.
	out := oauthSettings{
		GoogleClientID:     s.OAuth.GoogleClientID,
		GoogleClientSecret: s.OAuth.GoogleClientSecret,
		GithubClientID:     s.OAuth.GithubClientID,
		GithubClientSecret: s.OAuth.GithubClientSecret,
	}
	if stored, err := s.loadStoredOAuth(); err == nil && stored != nil {
		if stored.GoogleClientID != "" {
			out.GoogleClientID = stored.GoogleClientID
			if sec, err := s.Cipher.Decrypt(stored.GoogleSecretEnc); err == nil && sec != "" {
				out.GoogleClientSecret = sec
			}
		}
		if stored.GithubClientID != "" {
			out.GithubClientID = stored.GithubClientID
			if sec, err := s.Cipher.Decrypt(stored.GithubSecretEnc); err == nil && sec != "" {
				out.GithubClientSecret = sec
			}
		}
	}

	s.oauthCache.mu.Lock()
	s.oauthCache.value = out
	s.oauthCache.loadedAt = time.Now()
	s.oauthCache.populated = true
	s.oauthCache.mu.Unlock()
	return out
}

func (s *Server) invalidateOAuthCache() {
	s.oauthCache.mu.Lock()
	s.oauthCache.populated = false
	s.oauthCache.mu.Unlock()
}

func (s *Server) loadStoredOAuth() (*storedOAuth, error) {
	var row models.SiteSetting
	if err := s.DB.Where("key = ?", oauthSettingKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if row.Value == "" {
		return nil, nil
	}
	var out storedOAuth
	if err := json.Unmarshal([]byte(row.Value), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Server) saveStoredOAuth(v storedOAuth) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := s.DB.Save(&models.SiteSetting{Key: oauthSettingKey, Value: string(body)}).Error; err != nil {
		return err
	}
	s.invalidateOAuthCache()
	return nil
}

// redirectURI is what the operator must paste into the provider's console.
// Deriving it here (rather than asking them to type it) removes the most
// common setup failure: a trailing slash or an http/https mismatch.
func (s *Server) redirectURI(provider string) string {
	return strings.TrimRight(s.OAuth.BaseURL, "/") + "/auth/" + provider + "/callback"
}

// maskSecret shows whether a secret is set without revealing it.
func maskSecret(enc []byte, envValue string) string {
	if len(enc) > 0 {
		return "已配置（后台）"
	}
	if envValue != "" {
		return "已配置（环境变量）"
	}
	return ""
}

// oauthCredsFor returns the id/secret pair for one provider.
func (o oauthSettings) credsFor(provider string) (id, secret string) {
	switch provider {
	case "google":
		return o.GoogleClientID, o.GoogleClientSecret
	case "github":
		return o.GithubClientID, o.GithubClientSecret
	}
	return "", ""
}
