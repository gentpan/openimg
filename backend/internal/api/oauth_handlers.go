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
	// verifierCookie 存 PKCE 的 code_verifier。它必须活在浏览器这一侧、
	// 跟着这一次授权走:服务端全局存一份的话,同一个用户开两个标签页就会
	// 互相覆盖,而按 state 索引到服务端内存里则活不过一次重启。
	verifierCookie = "openimg_oauth_verifier"
	stateMaxAge    = 600 // 10 min
	googleScopes   = "openid email profile"
	githubScopes   = "read:user user:email"
	// picbiScopes 只要额度这一项。授权范围越窄,同意页上那句话越说得清楚。
	picbiScopes = "credits"
)

type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string
	// pic.bi 只从环境变量读,不进后台的 site_settings。
	//
	// 理由是它和 Google/GitHub 不是一类东西:那两个是登录方式,少配一个只是
	// 少一个按钮;pic.bi 这一条连着的是钱,后台能改的密钥意味着拿到管理员
	// 权限就能把扣费指向自己的服务。
	PicbiBaseURL      string // e.g. https://pic.bi
	PicbiClientID     string
	PicbiClientSecret string
	BaseURL           string // e.g. https://openimg.io
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

// picbiOAuth 是 pic.bi 那一侧的 OAuth 客户端配置。
//
// AuthStyleInParams 是明写的:pic.bi 的 /oauth/token 从请求体里读
// client_secret。让 x/oauth2 去"探测"授权风格会先发一次 Basic 认证的请求、
// 失败后再重试,而那次失败在对端看起来就是一次密钥错误的告警。
func (s *Server) picbiOAuth() *oauth2.Config {
	base := strings.TrimRight(strings.TrimSpace(s.OAuth.PicbiBaseURL), "/")
	if base == "" || s.OAuth.PicbiClientID == "" {
		return nil
	}
	return &oauth2.Config{
		ClientID:     s.OAuth.PicbiClientID,
		ClientSecret: s.OAuth.PicbiClientSecret,
		RedirectURL:  strings.TrimRight(s.OAuth.BaseURL, "/") + "/auth/picbi/callback",
		Scopes:       strings.Fields(picbiScopes),
		Endpoint: oauth2.Endpoint{
			AuthURL:   base + "/oauth/authorize",
			TokenURL:  base + "/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// oauthConfigFor 把三个 provider 收在一处。分散在 start 与 callback 里各写
// 一个 switch 的结果是加第三个 provider 时漏掉其中一个。
func (s *Server) oauthConfigFor(provider string) *oauth2.Config {
	switch provider {
	case "google":
		return s.googleOAuth()
	case "github":
		return s.githubOAuth()
	case "picbi":
		return s.picbiOAuth()
	}
	return nil
}

// /auth/{provider}/start: redirect to provider with random state. If `intent`
// is "link", the callback will link the provider to the currently logged-in
// user instead of upserting/creating a new user. The link route is registered
// behind auth middleware, so we trust the cookie.
func (s *Server) handleOAuthStart(provider, intent string) gin.HandlerFunc {
	return func(c *gin.Context) {
		conf := s.oauthConfigFor(provider)
		if conf == nil {
			c.String(http.StatusServiceUnavailable, "%s OAuth not configured", provider)
			return
		}
		state := randHex(24)
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(stateCookie+"_"+provider, state, stateMaxAge, "/", s.CookieDomain, s.CookieSecure, true)
		// PKCE 只对 pic.bi 开:那边把它定成了必选。Google/GitHub 这两条路
		// 现在能用,顺手给它们也加上意味着一次没有必要的行为变更。
		opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOnline}
		if provider == "picbi" {
			verifier := oauth2.GenerateVerifier()
			c.SetCookie(verifierCookie+"_"+provider, verifier, stateMaxAge, "/", s.CookieDomain, s.CookieSecure, true)
			opts = append(opts, oauth2.S256ChallengeOption(verifier))
		}
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
		c.Redirect(http.StatusFound, conf.AuthCodeURL(state, opts...))
	}
}

// /auth/{provider}/callback: verify state, exchange code, fetch profile, find/create user, issue session.
func (s *Server) handleOAuthCallback(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		conf := s.oauthConfigFor(provider)
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
		exchangeOpts := []oauth2.AuthCodeOption{}
		if provider == "picbi" {
			verifier, _ := c.Cookie(verifierCookie + "_" + provider)
			c.SetCookie(verifierCookie+"_"+provider, "", -1, "/", s.CookieDomain, s.CookieSecure, true)
			if verifier == "" {
				// 没有 verifier 就换不了 token,而且这通常意味着这次回调
				// 不是本浏览器发起的那一次。
				c.String(http.StatusBadRequest, "OAuth verifier missing")
				return
			}
			exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(verifier))
		}
		token, err := conf.Exchange(ctx, code, exchangeOpts...)
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
		case "picbi":
			profile, err = fetchPicbiProfile(ctx, conf.Client(ctx, token), s.OAuth.PicbiBaseURL)
		}
		if err != nil {
			c.String(http.StatusBadGateway, "fetch profile failed: %v", err)
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

		// 下面是"用这个身份登录或注册"。
		//
		// pic.bi 永远不走这条路,而且这道拦截必须在这里(而不是只靠"没注册
		// /auth/picbi/start"):upsertOAuthUser 会按已验证邮箱去合并账号,
		// 于是在 pic.bi 上注册一个同邮箱的账号就能接管别人的 openimg 账号。
		// 少注册一条路由只是让人不容易走到,这一句才是把路堵死。
		if provider == "picbi" {
			c.Redirect(http.StatusFound, "/settings?error=picbi_link_only")
			return
		}

		// 邮箱校验挪到这里,不能留在取完 profile 之后。
		//
		// 放在前面时它拦在 link 分支之前:一个 provider 不给邮箱(GitHub 把
		// 邮箱设为私密就是这样),绑定这条路会永远失败——而绑定根本不需要
		// 邮箱,需要的只是那个 subject。只有"创建新账号"这条路非要邮箱不可。
		if profile.Email == "" {
			c.String(http.StatusBadRequest, "OAuth profile has no verified email — please add a verified email to your "+provider+" account and try again")
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
	subjectCol := subjectColumn(provider)
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

// subjectColumn 是 provider → users 表上那一列。
//
// 只有一份:三处(绑定、解绑、upsert)各写一遍 map 字面量的话,加
// provider 时漏掉其中一处不会报错,只会静默地不生效。
func subjectColumn(provider string) string {
	switch provider {
	case "google":
		return "google_sub"
	case "github":
		return "github_id"
	case "picbi":
		return "picbi_id"
	}
	return ""
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

// fetchPicbiProfile 读 pic.bi 的 /oauth/userinfo。
//
// 只取 sub 是硬要求,邮箱与头像是可有可无的装饰:关联靠的是 sub,而
// openimg 这边的账号本来就有自己的邮箱。返回的邮箱在这条路上不会被用来
// 合并账号——那条路已经在 callback 里堵死了。
func fetchPicbiProfile(ctx context.Context, client *http.Client, base string) (*oauthProfile, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(base), "/") + "/oauth/userinfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("picbi userinfo %d: %s", resp.StatusCode, body)
	}
	var u struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		AvatarURL     string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	if strings.TrimSpace(u.Sub) == "" {
		return nil, errors.New("picbi userinfo 没有返回 sub")
	}
	return &oauthProfile{
		Provider:  "picbi",
		Subject:   strings.TrimSpace(u.Sub),
		Email:     strings.ToLower(strings.TrimSpace(u.Email)),
		Verified:  u.EmailVerified,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
	}, nil
}

// upsertOAuthUser: link by provider subject first; else by email; else create new.
func (s *Server) upsertOAuthUser(provider string, p *oauthProfile, signupIP string) (*models.User, error) {
	// 1. by provider subject
	var u models.User
	// picbi 永远走不到这里(callback 已经把它挡在 link 分支里),这一行只是
	// 按 provider 取列名。
	subjectCol := subjectColumn(provider)
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
		col := subjectColumn(provider)
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
		if user.PicbiID != "" && provider != "picbi" {
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
