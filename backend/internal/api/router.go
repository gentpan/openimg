package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/aigen"
	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/crypto"
	"github.com/gentpan/openimg/backend/internal/email"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/passkey"
	"github.com/gentpan/openimg/backend/internal/scheduler"
	"github.com/gentpan/openimg/backend/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const refCookieName = "openimg_ref"

type Server struct {
	DB      *gorm.DB
	Queue   *scheduler.Queue
	Auth    *auth.Service
	Storage *storage.Registry
	// FrontendDir, when set, is the built SPA this process also serves.
	FrontendDir string

	// AppleAppID is "TEAMID.bundleid" for the Mac app, e.g.
	// "ABCDE12345.io.openimg.mac". Empty means the associated-domains document
	// is not served at all — see apple_assoc.go for why a placeholder would be
	// worse than nothing.
	AppleAppID string
	// ReactionSalt keeps hashed visitor addresses non-portable.
	ReactionSalt string
	// AIGen 是文生图上游。没配 APIMART_API_KEY 时它是禁用态,相关接口
	// 一律回"未开启",界面据此把入口藏起来——而不是让用户点一下才收到 401。
	AIGen *aigen.Client
	Cipher       *crypto.Cipher

	StorageDir string
	// PublicBaseURL is this site's own origin, used to absolutise object URLs
	// when a storage profile doesn't carry one.
	PublicBaseURL string

	MaxUploadSize        int64
	RequireEmailVerified bool

	CookieDomain string
	CookieSecure bool

	OAuth   OAuthConfig
	Email   email.Sender
	Passkey *passkey.Service

	uploadLimiter *rateLimiter
	reportLimiter *rateLimiter
	oauthCache    oauthCache
	nativeCodes   nativeCodes
	uploadCache   uploadSettingsCache
}

func New(db *gorm.DB, q *scheduler.Queue, a *auth.Service, st *storage.Registry, cipher *crypto.Cipher) *Server {
	return &Server{
		DB: db, Queue: q, Auth: a, Storage: st, Cipher: cipher,
		// Generous enough that a batch upload from the web UI never trips it,
		// tight enough that a script can't hammer the transcoder.
		uploadLimiter: newRateLimiter(60, time.Minute),
		reportLimiter: newRateLimiter(10, time.Minute),
	}
}

// configureClientIP makes ClientIP() return the visitor's address instead of
// a Cloudflare edge node's.
//
// The chain is Cloudflare → Caddy → this process. Caddy, not trusting anyone
// by default, overwrites X-Forwarded-For with its own peer — a Cloudflare
// edge — so every per-IP feature was keying on Cloudflare's addresses: the
// upload rate limiter collapsed into one shared 60/min bucket per edge node,
// and the signup/login/upload IPs the admin panel shows were all Cloudflare's.
//
// CF-Connecting-IP is Cloudflare's authoritative statement of who connected,
// immune to X-Forwarded-For prepend games. Trusting it is safe only because
// the Caddyfile strips that header from any request that did not arrive from
// Cloudflare's published ranges — without that, anyone hitting the origin
// directly could impersonate any address. The two configs are a pair; do not
// change one without the other.
func configureClientIP(r *gin.Engine) {
	r.TrustedPlatform = gin.PlatformCloudflare
	// Requests that bypass Cloudflare (health checks, direct dev access) have
	// no CF header after Caddy's strip; they fall back to X-Forwarded-For, in
	// which only the local Caddy is a trusted hop.
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
}

func (s *Server) Router() *gin.Engine {
	r := gin.Default()
	configureClientIP(r)
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = false
	corsCfg.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Accept-Language", "Authorization", "X-Openimg-Brand"}
	r.Use(cors.New(corsCfg))
	// Records the caller's language for every handler. Before the routes so
	// even a 404 answers in the right language.
	r.Use(i18n.Middleware())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/api/public-stats", s.handlePublicStats)
	// Abuse reports accept anonymous submissions, so they get their own
	// (tighter) limiter to keep the queue from being flooded.
	// Share page: public, and reactions need to know whether the visitor is
	// signed in without requiring it.
	shareGroup := r.Group("", s.Auth.Middleware(false))
	shareGroup.GET("/api/s/:code", s.handleShareInfo)
	shareGroup.POST("/api/s/:code/react", s.reportLimiter.middleware("ratelimit.too_frequent"), s.handleReact)
	shareGroup.POST("/api/s/:code/report", s.reportLimiter.middleware("report.rate_limited"), s.handleShareReport)

	r.POST("/api/report", s.Auth.Middleware(false),
		s.reportLimiter.middleware("report.rate_limited"), s.handleReport)

	// Auth (public + protected)
	r.POST("/auth/register/code", s.handleRegisterCode)
	r.POST("/auth/register", s.handleRegister)
	r.POST("/auth/login", s.handleLogin)
	r.GET("/auth/providers", s.handleListProviders)
	r.POST("/auth/native/exchange", s.handleNativeExchange)
	r.GET("/auth/google/start", s.handleOAuthStart("google", ""))
	r.GET("/auth/github/start", s.handleOAuthStart("github", ""))
	// Callbacks run with optional auth so the link-intent path can find the
	// currently logged-in user via cookie.
	callbackGroup := r.Group("", s.Auth.Middleware(false))
	callbackGroup.GET("/auth/google/callback", s.handleOAuthCallback("google"))
	callbackGroup.GET("/auth/github/callback", s.handleOAuthCallback("github"))
	r.POST("/auth/password/reset/request", s.handleResetRequest)
	r.POST("/auth/password/reset", s.handleResetConfirm)
	r.POST("/auth/email/request-otp", s.handleRequestOTP)
	r.POST("/auth/email/verify", s.handleVerifyOTP)
	r.POST("/auth/passkey/login/begin", s.handlePasskeyLoginBegin)
	r.POST("/auth/passkey/login/finish", s.handlePasskeyLoginFinish)

	authed := r.Group("", s.Auth.Middleware(true))
	authed.POST("/auth/logout", s.handleLogout)
	authed.GET("/auth/google/link-start", s.handleOAuthStart("google", "link"))
	authed.GET("/auth/github/link-start", s.handleOAuthStart("github", "link"))
	authed.GET("/api/referral/me", s.handleReferralMe)
	authed.GET("/api/referral/list", s.handleReferralList)
	r.GET("/api/referral/lookup", s.handleReferralLookup)

	// Upload + image library. These accept a personal access token as well as
	// a session cookie, so PicGo / Typora / curl can post here. Account
	// management routes above deliberately do not.
	machine := r.Group("", s.Auth.TokenOrSession())
	machine.POST("/api/upload", s.uploadLimiter.middleware("upload.rate_limited"), s.handleUpload)
	machine.GET("/api/images", s.handleListImages)
	machine.GET("/api/images/:id", s.handleGetImage)
	machine.DELETE("/api/images/:id", s.handleDeleteImage)
	machine.POST("/api/images/bulk-delete", s.handleBulkDelete)
	machine.POST("/api/images/:id/variant", s.handleMakeVariant)
	// Moved down from the cookie-only group rather than added here: Gin panics
	// on a duplicate method+path, so registering them in both places takes the
	// process down at boot.
	//
	// A native client cannot function without these two. /auth/me answers "who
	// is this token" so the app can show the account it is acting as, and
	// /api/quota carries the tier limits — max file size, allowed formats,
	// remaining space — which is what lets a client reject a file before
	// spending the user's upload on a 413. Neither writes anything, and both
	// only ever return the token owner's own data, which a token that can
	// already bulk-delete 500 images plainly has access to.
	machine.GET("/auth/me", s.handleMe)
	machine.GET("/api/quota", s.handleGetQuota)
	// The same reasoning extends to the rest of the account's own read-only
	// numbers: a native client that cannot show storage composition or the
	// space ledger is missing the half of the product that explains where the
	// quota went. None of these write anything or reveal anything the token
	// holder cannot already derive from the image list.
	machine.GET("/api/quota/transactions", s.handleQuotaTransactions)
	machine.GET("/api/storage/summary", s.handleStorageSummary)

	// AI 文生图。放在机器组:它就是一种上传,只不过图是生成出来的。
	// 额度由 aigen 的余额与每日上限把关,不另设中间件。
	machine.GET("/api/ai/status", s.handleAIStatus)
	machine.GET("/api/ai/generations", s.handleAIGenerations)
	machine.POST("/api/ai/generate", s.handleAIGenerate)
	// 修图与生成同组同闸:它同样是"产出一张新图",只是多了几张源图作输入。
	machine.POST("/api/ai/edit", s.handleAIEdit)
	machine.GET("/api/checkin/history", s.handleCheckinHistory)
	// Check-in is the one write here, and it is safe to expose: the only thing
	// it can do is grant the token's own owner more space. Daily check-in is
	// also how space is earned on this site, so a client without it sends the
	// user back to the browser every day.
	machine.POST("/api/checkin", s.handleCheckin)
	// Conversion preferences. A write, and the only one here that changes an
	// account-level setting rather than the user's own images — but what it can
	// change is whether uploads get re-encoded and which derivative is kept.
	// A leaked token could make future uploads cost more storage; it cannot
	// reach credentials, delete the account, or mint more tokens, all of which
	// stay cookie-only. Worth it so the client can offer the setting where the
	// user is actually about to upload.
	machine.PATCH("/api/preferences", s.handleUpdatePreferences)
	// Nickname and avatar. Moved down from the cookie-only group (again: Gin
	// panics on a duplicate registration, so this is a move, not an addition).
	//
	// The line this group draws is between changing how an account *presents*
	// and changing who *controls* it. A leaked token that can rename you or
	// swap your picture is vandalism; one that can change your password, mint
	// more tokens, read your S3 credentials or delete the account is a
	// takeover. Those all stay below, and they stay cookie-only.
	//
	// The alternative was a native settings page whose account section is
	// entirely read-only, which is what shipped first and what prompted this.
	machine.PATCH("/api/account/profile", s.handleUpdateNickname)
	// Password and passkey enrolment. These look like the things a token must
	// never reach, and the route group is the wrong place to look: both are
	// gated on a six-digit code mailed to the account's own address
	// (consumeOTP with OTPPurposePassword / OTPPurposePasskey). The second
	// factor is the mailbox, not the cookie — so a token lifted out of a PicGo
	// config still cannot change a password, and moving them here costs the
	// property nothing while letting the native app do what the website does.
	//
	// /api/account/otp is what sends that code, and only ever to the address
	// already on the account, so it moves with them.
	machine.POST("/api/account/otp", s.handleAccountOTP)
	machine.POST("/api/account/password", s.handleChangePassword)
	machine.GET("/auth/passkey/list", s.handlePasskeyList)
	machine.POST("/auth/passkey/enroll/begin", s.handlePasskeyEnrollBegin)
	machine.POST("/auth/passkey/enroll/finish", s.handlePasskeyEnrollFinish)
	// Deleting a passkey and unlinking a provider carry no code, and are here
	// on a weaker but still sound argument: both take a login method away
	// rather than granting one. The worst a leaked token achieves is leaving
	// you with fewer ways in, and handleOAuthUnlink already refuses to remove
	// the last one. Neither can be used to become you.
	machine.DELETE("/auth/passkey/:id", s.handlePasskeyDelete)
	machine.POST("/auth/google/unlink", s.handleOAuthUnlink("google"))
	machine.POST("/auth/github/unlink", s.handleOAuthUnlink("github"))
	//
	// Still cookie-only, and staying that way: minting tokens (a token that
	// mints tokens is a token that never expires) and deleting the account
	// (the one action no second factor makes acceptable to automate). OAuth
	// *linking* stays too, for a different reason — it is a full-page redirect
	// carrying an intent cookie, and the app's web session is ephemeral, so
	// there is nothing for the callback to attach the link to.
	machine.POST("/api/account/avatar", s.handleUploadAvatar)
	machine.DELETE("/api/account/avatar", s.handleDeleteAvatar)

	// API tokens (cookie-only: a token must not be able to mint more tokens)
	authed.POST("/auth/native/code", s.handleNativeCode)
	authed.DELETE("/api/account", s.handleDeleteAccount)
	authed.GET("/api/tokens", s.handleListTokens)
	authed.POST("/api/tokens", s.handleCreateToken)
	authed.DELETE("/api/tokens/:id", s.handleDeleteToken)

	// Space economy

	// Storage profiles (bring your own bucket)
	//
	// 令牌可达,但改动类操作一律要邮箱验证码作二次因子——与改密码、注册
	// Passkey 同一条界线。理由是这组接口比"能传能删"更进一步:能新建一个
	// 指向别处的桶并设为默认,此后连用户在网页上传的图也会落进那里,是一
	// 条持久的外泄通道。有了验证码,单靠泄露的令牌做不成这件事。
	//
	// 读取(列表)与探测不要码:前者只返回打码后的 access key,后者只是拿
	// 用户自己已存的凭据去戳自己的桶。
	machine.GET("/api/storage/profiles", s.handleListProfiles)
	machine.POST("/api/storage/profiles", s.handleCreateProfile)
	machine.PATCH("/api/storage/profiles/:id", s.handleUpdateProfile)
	machine.DELETE("/api/storage/profiles/:id", s.handleDeleteProfile)
	machine.POST("/api/storage/profiles/:id/test", s.handleTestProfile)
	machine.POST("/api/storage/profiles/:id/default", s.handleSetDefaultProfile)
	machine.POST("/api/storage/profiles/:id/backup", s.handleSetBackupProfile)

	// Admin API
	adminAPI := r.Group("/admin/api", s.Auth.Middleware(true), auth.AdminOnly())
	{
		adminAPI.GET("/stats", s.adminStats)
		adminAPI.GET("/dashboard", s.adminDashboard)
		adminAPI.GET("/images", s.adminListImages)
		adminAPI.GET("/users", s.adminListUsers)
		adminAPI.PATCH("/users/:id", s.adminUpdateUser)
		adminAPI.POST("/users/:id/reconcile", s.adminReconcileQuota)
		adminAPI.GET("/groups", s.adminListGroups)
		adminAPI.PATCH("/groups/:id", s.adminUpdateGroup)
		adminAPI.GET("/quota/transactions", s.adminListTransactions)
		adminAPI.POST("/quota/adjust", s.adminAdjustQuota)
		adminAPI.GET("/reports", s.adminListReports)
		adminAPI.POST("/reports/:id/resolve", s.adminResolveReport)
		adminAPI.POST("/images/:id/block", s.adminBlockImage)
		adminAPI.POST("/users/:id/ban", s.adminBanUser)
		adminAPI.GET("/oauth", s.adminOAuthStatus)
		adminAPI.POST("/oauth", s.adminOAuthSave)
		adminAPI.GET("/upload-settings", s.adminUploadSettings)
		adminAPI.POST("/upload-settings", s.adminSaveUploadSettings)
		adminAPI.GET("/storage/platform", s.adminPlatformStorage)
		adminAPI.PATCH("/storage/platform", s.adminUpdatePlatformStorage)
	}

	// Local-disk fallback for development. In production these objects come
	// from the CDN, which must send the same header — see deploy notes.
	storageGroup := r.Group("/storage", func(c *gin.Context) {
		// Original-mode uploads are stored byte-for-byte, so a polyglot file
		// can still be a valid HTML document. nosniff makes the browser honour
		// the stored image/* type instead of guessing, which is what stops it
		// executing on our own origin.
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
		c.Next()
	})
	storageGroup.Static("/", s.StorageDir)

	// Short links live at the root, so this must be registered last: Gin gives
	// static routes priority over a wildcard sibling, and everything above has
	// already claimed its path.
	//
	// When FRONTEND_DIR is set the same handler also serves the SPA for any
	// path that is not a code, which is what lets one process own the whole
	// domain and makes the short link a genuine 302 instead of a page that
	// loads React and then navigates.
	r.GET("/:code", s.handleShortLink)
	// Before NoRoute: the SPA fallback would answer this path with index.html,
	// which Apple rejects.
	s.registerAppleAssociation(r)

	r.NoRoute(s.serveAppOrNotFound)
	return r
}

// serveAppOrNotFound hands the request to the built frontend when this process
// is serving it, and otherwise reports a plain 404 for the reverse proxy to
// deal with.
func (s *Server) serveAppOrNotFound(c *gin.Context) {
	if s.FrontendDir == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	// A real file if the path names one, the SPA otherwise.
	//
	// Returning index.html for everything looks fine to anything that only
	// checks status codes — /assets/index-abc.js answers 200 — but the browser
	// asked for JavaScript and received HTML, so nothing boots and the page is
	// blank. curl cannot see that; only the Content-Type gives it away.
	if full, ok := s.frontendFile(c.Request.URL.Path); ok {
		// Vite fingerprints these filenames, so the content behind one can
		// never change. index.html must not be cached: it is what points at
		// the current fingerprints.
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.File(full)
		return
	}

	// Any unmatched path is a client-side route: the SPA reads the URL itself.
	c.Header("Cache-Control", "no-cache")
	c.File(filepath.Join(s.FrontendDir, "index.html"))
}

// frontendFile resolves a request path to a file inside FrontendDir, or
// reports that there isn't one.
//
// The join is checked rather than trusted: a path of "/../../etc/passwd"
// would otherwise walk straight out of the directory, and this handler serves
// every unmatched request on the domain.
func (s *Server) frontendFile(urlPath string) (string, bool) {
	if urlPath == "" || urlPath == "/" {
		return "", false
	}
	root, err := filepath.Abs(s.FrontendDir)
	if err != nil {
		return "", false
	}
	full := filepath.Join(root, filepath.FromSlash(path.Clean("/"+urlPath)))
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", false
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		return "", false
	}
	return full, true
}

// groupFor resolves a user's tier, falling back to the "free" group so a user
// whose group row was deleted still gets sane limits rather than zeroes —
// which would silently forbid every upload.
func (s *Server) groupFor(u *models.User) models.UserGroup {
	var g models.UserGroup
	if u != nil && u.GroupID != nil {
		if err := s.DB.First(&g, "id = ?", *u.GroupID).Error; err == nil {
			return g
		}
	}
	if err := s.DB.Where("name = ?", "free").First(&g).Error; err == nil {
		return g
	}
	return models.UserGroup{}
}
