package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GET /admin/api/oauth — current state plus the exact redirect URIs to paste
// into Google / GitHub. Secrets are never returned, only whether one is set.
func (s *Server) adminOAuthStatus(c *gin.Context) {
	stored, err := s.loadStoredOAuth()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if stored == nil {
		stored = &storedOAuth{}
	}
	effective := s.oauth()

	// The page must show what is actually in force, not only what is in the
	// database. Configuring through the environment is a supported path, and
	// reporting an empty client id next to "已启用" reads as a bug.
	//
	// `source` names which one won: the stored value overrides the environment,
	// so an operator who edits .env and sees nothing change needs to be told
	// there is a database row shadowing it.
	source := func(storedID, envID string) string {
		switch {
		case storedID != "":
			return "admin"
		case envID != "":
			return "env"
		}
		return ""
	}

	c.JSON(http.StatusOK, gin.H{
		"can_store": s.Cipher.Enabled(),
		"google": gin.H{
			"client_id":    firstNonEmpty(stored.GoogleClientID, effective.GoogleClientID),
			"editable":     stored.GoogleClientID != "" || s.OAuth.GoogleClientID == "",
			"source":       source(stored.GoogleClientID, s.OAuth.GoogleClientID),
			"secret_state": maskSecret(stored.GoogleSecretEnc, s.OAuth.GoogleClientSecret),
			"enabled":      effective.GoogleClientID != "",
			"redirect_uri": s.redirectURI("google"),
			"console_url":  "https://console.cloud.google.com/apis/credentials",
		},
		"github": gin.H{
			"client_id":    firstNonEmpty(stored.GithubClientID, effective.GithubClientID),
			"editable":     stored.GithubClientID != "" || s.OAuth.GithubClientID == "",
			"source":       source(stored.GithubClientID, s.OAuth.GithubClientID),
			"secret_state": maskSecret(stored.GithubSecretEnc, s.OAuth.GithubClientSecret),
			"enabled":      effective.GithubClientID != "",
			"redirect_uri": s.redirectURI("github"),
			"console_url":  "https://github.com/settings/developers",
		},
		"passkey": gin.H{"enabled": s.Passkey != nil},
		"email_otp": gin.H{
			"enabled": s.Email != nil && s.Email.Configured(),
		},
		"base_url": s.OAuth.BaseURL,
	})
}

type oauthSaveReq struct {
	Provider string `json:"provider" binding:"required"` // google | github
	ClientID string `json:"client_id"`
	// Secret empty means "keep the stored one"; "-" clears it.
	ClientSecret string `json:"client_secret"`
}

// POST /admin/api/oauth — save one provider's credentials.
func (s *Server) adminOAuthSave(c *gin.Context) {
	if !s.Cipher.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "服务端未配置 STORAGE_MASTER_KEY，无法加密保存 OAuth 密钥；请改用环境变量配置",
		})
		return
	}
	var req oauthSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != "google" && provider != "github" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 必须是 google 或 github"})
		return
	}

	stored, err := s.loadStoredOAuth()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if stored == nil {
		stored = &storedOAuth{}
	}

	clientID := strings.TrimSpace(req.ClientID)
	secret := strings.TrimSpace(req.ClientSecret)

	// Encrypt only when a new secret was supplied; "-" is the explicit clear.
	var secretEnc []byte
	switch {
	case secret == "-":
		secretEnc = nil
	case secret != "":
		secretEnc, err = s.Cipher.Encrypt(secret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		if provider == "google" {
			secretEnc = stored.GoogleSecretEnc
		} else {
			secretEnc = stored.GithubSecretEnc
		}
	}

	// A client ID with no secret anywhere would produce a login button that
	// always fails — refuse it rather than shipping a broken flow.
	if clientID != "" && len(secretEnc) == 0 {
		envSecret := s.OAuth.GoogleClientSecret
		if provider == "github" {
			envSecret = s.OAuth.GithubClientSecret
		}
		if envSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "填写 Client ID 时必须同时提供 Client Secret"})
			return
		}
	}

	if provider == "google" {
		stored.GoogleClientID = clientID
		stored.GoogleSecretEnc = secretEnc
	} else {
		stored.GithubClientID = clientID
		stored.GithubSecretEnc = secretEnc
	}
	if err := s.saveStoredOAuth(*stored); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": clientID != ""})
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
