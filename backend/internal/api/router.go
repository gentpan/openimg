package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/crypto"
	"github.com/gentpan/openimg/backend/internal/email"
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
	// ReactionSalt keeps hashed visitor addresses non-portable.
	ReactionSalt string
	Cipher       *crypto.Cipher

	StorageDir string
	TempDir    string
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

func (s *Server) Router() *gin.Engine {
	r := gin.Default()
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = false
	corsCfg.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	r.Use(cors.New(corsCfg))

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
	shareGroup.POST("/api/s/:code/react", s.reportLimiter.middleware("操作过于频繁"), s.handleReact)
	shareGroup.POST("/api/s/:code/report", s.reportLimiter.middleware("举报过于频繁，请稍后再试"), s.handleShareReport)

	r.POST("/api/report", s.Auth.Middleware(false),
		s.reportLimiter.middleware("举报过于频繁，请稍后再试"), s.handleReport)

	// Auth (public + protected)
	r.POST("/auth/register", s.handleRegister)
	r.POST("/auth/login", s.handleLogin)
	r.GET("/auth/providers", s.handleListProviders)
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
	authed.GET("/auth/me", s.handleMe)
	authed.GET("/auth/google/link-start", s.handleOAuthStart("google", "link"))
	authed.GET("/auth/github/link-start", s.handleOAuthStart("github", "link"))
	authed.POST("/auth/google/unlink", s.handleOAuthUnlink("google"))
	authed.POST("/auth/github/unlink", s.handleOAuthUnlink("github"))
	authed.GET("/auth/passkey/list", s.handlePasskeyList)
	authed.POST("/auth/passkey/enroll/begin", s.handlePasskeyEnrollBegin)
	authed.POST("/auth/passkey/enroll/finish", s.handlePasskeyEnrollFinish)
	authed.DELETE("/auth/passkey/:id", s.handlePasskeyDelete)
	authed.GET("/api/referral/me", s.handleReferralMe)
	authed.GET("/api/referral/list", s.handleReferralList)
	r.GET("/api/referral/lookup", s.handleReferralLookup)

	// Upload + image library. These accept a personal access token as well as
	// a session cookie, so PicGo / Typora / curl can post here. Account
	// management routes above deliberately do not.
	machine := r.Group("", s.Auth.TokenOrSession())
	machine.POST("/api/upload", s.uploadLimiter.middleware("上传过于频繁，请稍后再试"), s.handleUpload)
	machine.GET("/api/images", s.handleListImages)
	machine.GET("/api/images/:id", s.handleGetImage)
	machine.DELETE("/api/images/:id", s.handleDeleteImage)
	machine.POST("/api/images/bulk-delete", s.handleBulkDelete)
	machine.POST("/api/images/:id/variant", s.handleMakeVariant)

	// API tokens (cookie-only: a token must not be able to mint more tokens)
	authed.PATCH("/api/preferences", s.handleUpdatePreferences)
	authed.PATCH("/api/account/profile", s.handleUpdateNickname)
	authed.POST("/api/account/otp", s.handleAccountOTP)
	authed.POST("/api/account/password", s.handleChangePassword)
	authed.POST("/api/account/avatar", s.handleUploadAvatar)
	authed.DELETE("/api/account/avatar", s.handleDeleteAvatar)
	authed.DELETE("/api/account", s.handleDeleteAccount)
	authed.GET("/api/tokens", s.handleListTokens)
	authed.POST("/api/tokens", s.handleCreateToken)
	authed.DELETE("/api/tokens/:id", s.handleDeleteToken)

	// Space economy
	authed.GET("/api/quota", s.handleGetQuota)
	authed.GET("/api/quota/transactions", s.handleQuotaTransactions)
	authed.GET("/api/storage/summary", s.handleStorageSummary)
	authed.POST("/api/checkin", s.handleCheckin)
	authed.GET("/api/checkin/history", s.handleCheckinHistory)

	// Storage profiles (bring your own bucket)
	authed.GET("/api/storage/profiles", s.handleListProfiles)
	authed.POST("/api/storage/profiles", s.handleCreateProfile)
	authed.PATCH("/api/storage/profiles/:id", s.handleUpdateNickname)
	authed.DELETE("/api/storage/profiles/:id", s.handleDeleteProfile)
	authed.POST("/api/storage/profiles/:id/test", s.handleTestProfile)
	authed.POST("/api/storage/profiles/:id/default", s.handleSetDefaultProfile)
	authed.POST("/api/storage/profiles/:id/backup", s.handleSetBackupProfile)

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
