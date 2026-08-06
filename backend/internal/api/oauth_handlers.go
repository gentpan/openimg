package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/referral"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

const (
	stateCookie  = "openimg_oauth_state"
	intentCookie = "openimg_oauth_intent"
	stateMaxAge  = 600 // 10 min
	googleScopes = "openid email profile"
	githubScopes = "read:user user:email"
)

type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string
	BaseURL            string // e.g. https://openimg.io
}

func (s *Server) googleOAuth() *oauth2.Config {
	id, secret := s.oauth().credsFor("google")
	if id == "" {
		return nil
	}
	return &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		RedirectURL:  s.OAuth.BaseURL + "/auth/google/callback",
		Scopes:       strings.Fields(googleScopes),
		Endpoint:     google.Endpoint,
	}
}

func (s *Server) githubOAuth() *oauth2.Config {
	id, secret := s.oauth().credsFor("github")
	if id == "" {
		return nil
	}
	return &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		RedirectURL:  s.OAuth.BaseURL + "/auth/github/callback",
		Scopes:       strings.Fields(githubScopes),
		Endpoint:     github.Endpoint,
	}
}

// /auth/{provider}/start: redirect to provider with random state. If `intent`
// is "link", the callback will link the provider to the currently logged-in
// user instead of upserting/creating a new user. The link route is registered
// behind auth middleware, so we trust the cookie.
func (s *Server) handleOAuthStart(provider, intent string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var conf *oauth2.Config
		switch provider {
		case "google":
			conf = s.googleOAuth()
		case "github":
			conf = s.githubOAuth()
		}
		if conf == nil {
			c.String(http.StatusServiceUnavailable, "%s OAuth not configured", provider)
			return
		}
		state := randHex(24)
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(stateCookie+"_"+provider, state, stateMaxAge, "/", s.CookieDomain, s.CookieSecure, true)
		// Set or clear the intent cookie so the callback knows what to do.
		// A native client asks for it by query rather than by route, so the
		// same /start endpoint serves the website and the Mac app.
		effective := intent
		if effective == "" && c.Query("native") != "" {
			effective = nativeIntentName
		}
		if effective != "" {
			c.SetCookie(intentCookie, effective, stateMaxAge, "/", s.CookieDomain, s.CookieSecure, true)
		} else {
			c.SetCookie(intentCookie, "", -1, "/", s.CookieDomain, s.CookieSecure, true)
		}
		c.Redirect(http.StatusFound, conf.AuthCodeURL(state, oauth2.AccessTypeOnline))
	}
}

// /auth/{provider}/callback: verify state, exchange code, fetch profile, find/create user, issue session.
func (s *Server) handleOAuthCallback(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var conf *oauth2.Config
		switch provider {
		case "google":
			conf = s.googleOAuth()
		case "github":
			conf = s.githubOAuth()
		}
		if conf == nil {
			c.String(http.StatusServiceUnavailable, "%s OAuth not configured", provider)
			return
		}

		// Verify state
		want, _ := c.Cookie(stateCookie + "_" + provider)
		got := c.Query("state")
		if want == "" || got == "" || want != got {
			c.String(http.StatusBadRequest, "OAuth state mismatch")
			return
		}
		c.SetCookie(stateCookie+"_"+provider, "", -1, "/", s.CookieDomain, s.CookieSecure, true)

		if errMsg := c.Query("error"); errMsg != "" {
			c.Redirect(http.StatusFound, "/login?error="+errMsg)
			return
		}
		code := c.Query("code")
		if code == "" {
			c.String(http.StatusBadRequest, "OAuth code missing")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		token, err := conf.Exchange(ctx, code)
		if err != nil {
			c.String(http.StatusBadGateway, "OAuth exchange failed: %v", err)
			return
		}

		// Fetch profile
		var profile *oauthProfile
		switch provider {
		case "google":
			profile, err = fetchGoogleProfile(ctx, conf.Client(ctx, token))
		case "github":
			profile, err = fetchGithubProfile(ctx, conf.Client(ctx, token))
		}
		if err != nil {
			c.String(http.StatusBadGateway, "fetch profile failed: %v", err)
			return
		}
		if profile.Email == "" {
			c.String(http.StatusBadRequest, "OAuth profile has no verified email — please add a verified email to your "+provider+" account and try again")
			return
		}

		// If link intent + currently authenticated, attach this provider to
		// the logged-in user instead of upserting/creating a new one.
		intent, _ := c.Cookie(intentCookie)
		c.SetCookie(intentCookie, "", -1, "/", s.CookieDomain, s.CookieSecure, true)

		if intent == "link" {
			currentUser, ok := auth.UserFrom(c)
			if !ok {
				c.Redirect(http.StatusFound, "/login?error=link_requires_login")
				return
			}
			if err := s.linkProviderToUser(c.Request.Context(), currentUser, provider, profile); err != nil {
				c.Redirect(http.StatusFound, "/settings?error="+url.QueryEscape(err.Error()))
				return
			}
			c.Redirect(http.StatusFound, "/settings?linked="+provider)
			return
		}

		user, err := s.upsertOAuthUser(provider, profile, c.ClientIP())
		if err != nil {
			c.String(http.StatusInternalServerError, "user upsert: %v", err)
			return
		}
		// If a referral cookie was placed before the redirect, attach it now.
		// AttachAndReward is idempotent and skips if user already has one.
		if refCode, _ := c.Cookie(refCookieName); refCode != "" {
			_ = referral.AttachAndReward(s.DB, user, refCode)
			c.SetCookie(refCookieName, "", -1, "/", s.CookieDomain, s.CookieSecure, false)
		}
		// A native client gets a one-time code instead of a session: the sheet
		// it runs in has its own cookie jar and closes the moment this
		// redirect fires, so a cookie would be written nowhere useful.
		if intent == nativeIntentName {
			c.SetCookie(intentCookie, "", -1, "/", s.CookieDomain, s.CookieSecure, true)
			c.Redirect(http.StatusFound,
				NativeScheme+"://auth?code="+url.QueryEscape(s.nativeCodes.issue(user.ID)))
			return
		}
		if err := s.Auth.IssueSession(c, user); err != nil {
			c.String(http.StatusInternalServerError, "session: %v", err)
			return
		}
		c.Redirect(http.StatusFound, "/")
	}
}

// linkProviderToUser attaches the given OAuth profile to the currently
// authenticated user. Refuses if the OAuth identity is already linked to a
// different account (avoids account takeover by signing in with a hijacked
// google account whose email matches an existing user).
func (s *Server) linkProviderToUser(ctx context.Context, user *models.User, provider string, p *oauthProfile) error {
	subjectCol := map[string]string{"google": "google_sub", "github": "github_id"}[provider]
	if subjectCol == "" || p.Subject == "" {
		return errors.New(i18n.TCtx(ctx, "oauth.no_user_id", provider))
	}
	// 1) check this OAuth subject isn't already linked to a different account
	var other models.User
	if err := s.DB.Where(subjectCol+" = ? AND id <> ?", p.Subject, user.ID).First(&other).Error; err == nil {
		return errors.New(i18n.TCtx(ctx, "oauth.linked_elsewhere", provider, other.Email))
	}
	// 2) link
	updates := map[string]any{subjectCol: p.Subject}
	if user.AvatarURL == "" && p.AvatarURL != "" {
		updates["avatar_url"] = p.AvatarURL
	}
	return s.DB.Model(user).Updates(updates).Error
}

type oauthProfile struct {
	Provider  string
	Subject   string // provider-stable user id
	Email     string
	Verified  bool
	Name      string
	AvatarURL string
}

func fetchGoogleProfile(ctx context.Context, client *http.Client) (*oauthProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google %d: %s", resp.StatusCode, body)
	}
	var u struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &oauthProfile{
		Provider:  "google",
		Subject:   u.Sub,
		Email:     strings.ToLower(u.Email),
		Verified:  u.EmailVerified,
		Name:      u.Name,
		AvatarURL: u.Picture,
	}, nil
}

func fetchGithubProfile(ctx context.Context, client *http.Client) (*oauthProfile, error) {
	// 1) /user — basic profile (may have null email if user hides it)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github user %d: %s", resp.StatusCode, body)
	}
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	out := &oauthProfile{
		Provider:  "github",
		Subject:   fmt.Sprintf("%d", u.ID),
		Email:     strings.ToLower(u.Email),
		Name:      pickStr(u.Name, u.Login),
		AvatarURL: u.AvatarURL,
	}
	if out.Email != "" {
		out.Verified = true
	} else {
		// 2) /user/emails — fetch primary verified email
		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
		req2.Header.Set("Accept", "application/vnd.github+json")
		resp2, err := client.Do(req2)
		if err == nil {
			defer resp2.Body.Close()
			b2, _ := io.ReadAll(resp2.Body)
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if json.Unmarshal(b2, &emails) == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						out.Email = strings.ToLower(e.Email)
						out.Verified = true
						break
					}
				}
				if out.Email == "" {
					for _, e := range emails {
						if e.Verified {
							out.Email = strings.ToLower(e.Email)
							out.Verified = true
							break
						}
					}
				}
			}
		}
	}
	return out, nil
}

// upsertOAuthUser: link by provider subject first; else by email; else create new.
func (s *Server) upsertOAuthUser(provider string, p *oauthProfile, signupIP string) (*models.User, error) {
	// 1. by provider subject
	var u models.User
	subjectCol := map[string]string{"google": "google_sub", "github": "github_id"}[provider]
	if subjectCol != "" && p.Subject != "" {
		if err := s.DB.Where(subjectCol+" = ?", p.Subject).First(&u).Error; err == nil {
			return &u, nil
		}
	}
	// 2. by verified email
	if p.Email != "" && p.Verified {
		if err := s.DB.Where("email = ?", p.Email).First(&u).Error; err == nil {
			// Link this provider to existing account
			updates := map[string]any{}
			if subjectCol != "" {
				updates[subjectCol] = p.Subject
			}
			if !u.EmailVerified {
				updates["email_verified"] = true
			}
			if u.AvatarURL == "" && p.AvatarURL != "" {
				updates["avatar_url"] = p.AvatarURL
			}
			if u.Name == "" && p.Name != "" {
				updates["name"] = p.Name
			}
			if len(updates) > 0 {
				s.DB.Model(&u).Updates(updates)
				s.DB.First(&u, "id = ?", u.ID)
			}
			return &u, nil
		}
	}
	// 3. create new
	if p.Email == "" {
		return nil, fmt.Errorf("provider returned no email; cannot create account")
	}
	var freeGroup models.UserGroup
	s.DB.Where("name = ?", "free").First(&freeGroup)
	newU := models.NewUser(p.Email, pickStr(p.Name, p.Email), "")
	newU.SignupIP = signupIP
	newU.EmailVerified = p.Verified
	newU.AvatarURL = p.AvatarURL
	if subjectCol == "google_sub" {
		newU.GoogleSub = p.Subject
	} else if subjectCol == "github_id" {
		newU.GithubID = p.Subject
	}
	if freeGroup.ID != uuid.Nil {
		newU.GroupID = &freeGroup.ID
	}
	if err := s.DB.Create(&newU).Error; err != nil {
		return nil, err
	}
	_ = referral.EnsureCode(s.DB, &newU)
	if _, err := quota.SignupGrant(s.DB, newU.ID, &freeGroup); err != nil {
		log.Printf("oauth: signup grant failed for %s: %v", newU.ID, err)
	}
	s.DB.First(&newU, "id = ?", newU.ID)
	return &newU, nil
}

// /auth/{provider}/unlink — disconnect this provider from the current user.
// Refused if it's the only remaining login method.
func (s *Server) handleOAuthUnlink(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth.MustUser(c)
		col := map[string]string{"google": "google_sub", "github": "github_id"}[provider]
		if col == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown provider"})
			return
		}
		// Count remaining login methods AFTER hypothetical unlink.
		methods := 0
		if user.PasswordHash != "" {
			methods++
		}
		if user.GoogleSub != "" && provider != "google" {
			methods++
		}
		if user.GithubID != "" && provider != "github" {
			methods++
		}
		if methods == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "oauth.unlink_last")})
			return
		}
		if err := s.DB.Model(user).Update(col, "").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// /auth/providers — frontend asks "which auth methods are available right now"
func (s *Server) handleListProviders(c *gin.Context) {
	cfg := s.oauth()
	out := []string{}
	if cfg.GoogleClientID != "" {
		out = append(out, "google")
	}
	if cfg.GithubClientID != "" {
		out = append(out, "github")
	}
	if s.Email != nil && s.Email.Configured() {
		out = append(out, "email_otp")
	}
	if s.Passkey != nil {
		out = append(out, "passkey")
	}
	c.JSON(http.StatusOK, gin.H{"providers": out})
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// reuse pickStr from router.go
var _ = gorm.ErrRecordNotFound // appease linter when conditions not used
