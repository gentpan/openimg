package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Native apps cannot finish an OAuth sign-in the way the website does.
//
// The browser flow ends by writing a session cookie and redirecting to "/",
// which works because the browser *is* the client. A Mac app opening that flow
// in ASWebAuthenticationSession gets neither: the sheet has its own cookie jar,
// and the session it holds dies with it. The app also cannot live on a session
// even if it could read one — seven days, no refresh endpoint.
//
// So the callback redirects to a private URL scheme carrying a single-use code,
// and the app trades that code for a personal access token. The code is what
// crosses the process boundary, not the credential: it is worthless after one
// use, expires in a minute, and grants nothing on its own.
const (
	NativeScheme     = "openimg"
	nativeCodeTTL    = 60 * time.Second
	nativeIntentName = "native"
)

type nativeCodes struct {
	mu sync.Mutex
	m  map[string]nativeCode
}

type nativeCode struct {
	userID uuid.UUID
	expiry time.Time
}

func (n *nativeCodes) issue(userID uuid.UUID) string {
	code := randHex(24)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.m == nil {
		n.m = map[string]nativeCode{}
	}
	// Sweep on write. The map only grows on sign-in, so there is no traffic
	// pattern where this is expensive and nothing to schedule or shut down.
	now := time.Now()
	for k, v := range n.m {
		if now.After(v.expiry) {
			delete(n.m, k)
		}
	}
	n.m[code] = nativeCode{userID: userID, expiry: now.Add(nativeCodeTTL)}
	return code
}

// redeem returns the user the code was issued for, and invalidates it whether
// or not it had expired — a code that failed once must not be retryable.
func (n *nativeCodes) redeem(code string) (uuid.UUID, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	v, ok := n.m[code]
	if !ok {
		return uuid.Nil, false
	}
	delete(n.m, code)
	if time.Now().After(v.expiry) {
		return uuid.Nil, false
	}
	return v.userID, true
}

// POST /auth/native/code — mints a handoff code for an already-signed-in
// session. Cookie-only, like everything else that can create credentials.
//
// This exists so the web sign-in page can hand a native client back a code no
// matter which method the user chose. The OAuth callback has its own shortcut,
// but passkey cannot: WebAuthn happens entirely in the page, and a native app
// cannot drive it without an associated-domains entitlement, which needs a
// Team ID that a locally signed build does not have. Running it in the web
// sheet and handing back a code works today and needs no entitlement.
func (s *Server) handleNativeCode(c *gin.Context) {
	u := auth.MustUser(c)
	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被停用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": s.nativeCodes.issue(u.ID)})
}

type nativeExchangeReq struct {
	Code string `json:"code" binding:"required"`
	// Device names the token on the server so the user can recognise it in the
	// token list and revoke the right one.
	Device string `json:"device"`
}

// POST /auth/native/exchange — one-time code to personal access token.
func (s *Server) handleNativeExchange(c *gin.Context) {
	var req nativeExchangeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	userID, ok := s.nativeCodes.redeem(strings.TrimSpace(req.Code))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录码无效或已过期，请重新登录"})
		return
	}

	var u models.User
	if err := s.DB.First(&u, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不存在"})
		return
	}
	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被停用"})
		return
	}

	name := nativeTokenName(req.Device)
	// Retire the previous token from the same device rather than stacking a new
	// one beside it: the per-user cap is small, and a list full of identically
	// named entries is one the user cannot act on.
	s.DB.Where("user_id = ? AND name = ? AND revoked = ?", u.ID, name, false).
		Delete(&models.APIToken{})

	plain, hash, prefix := auth.NewAPIToken()
	exp := time.Now().AddDate(1, 0, 0)
	tok := models.APIToken{
		ID: uuid.New(), UserID: u.ID, Name: name,
		TokenHash: hash, Prefix: prefix, ExpiresAt: &exp,
	}
	if err := s.DB.Create(&tok).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var group *models.UserGroup
	if u.GroupID != nil {
		var g models.UserGroup
		if s.DB.First(&g, "id = ?", *u.GroupID).Error == nil {
			group = &g
		}
	}
	c.JSON(http.StatusOK, gin.H{"plain": plain, "user": toPublic(&u, group)})
}

func nativeTokenName(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		device = "Mac"
	}
	if len([]rune(device)) > 40 {
		device = string([]rune(device)[:40])
	}
	return "Openimg for Mac · " + device
}
