package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	CookieName    = "openimg_session"
	contextUser   = "openimg_user"
	contextSID    = "openimg_sid"
	tokenLifetime = 7 * 24 * time.Hour
	bcryptCost    = 11

	// APITokenPrefix identifies personal access tokens on sight, both for the
	// user pasting one into PicGo and for secret scanners.
	APITokenPrefix = "oimg_"
)

type Service struct {
	DB        *gorm.DB
	JWTSecret []byte
	Secure    bool // set true once HTTPS is on; for now permissive
	Domain    string
}

type claims struct {
	UID string `json:"uid"`
	SID string `json:"sid"`
	jwt.RegisteredClaims
}

func New(db *gorm.DB, jwtSecret string, secure bool, domain string) *Service {
	return &Service{DB: db, JWTSecret: []byte(jwtSecret), Secure: secure, Domain: domain}
}

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	return string(h), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func (s *Service) IssueSession(c *gin.Context, user *models.User) error {
	rawToken := newRandomToken(48)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	sess := models.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		ExpiresAt: time.Now().Add(tokenLifetime),
	}
	if err := s.DB.Create(&sess).Error; err != nil {
		return err
	}
	now := time.Now()
	s.DB.Model(user).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": c.ClientIP(),
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UID: user.ID.String(),
		SID: sess.ID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(sess.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	})
	signed, err := tok.SignedString(s.JWTSecret)
	if err != nil {
		return err
	}
	// raw token never leaves the server in plaintext besides the cookie itself;
	// we put the JWT in the cookie and store its sid in DB for revocation.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, signed+"."+rawToken, int(tokenLifetime.Seconds()), "/", s.Domain, s.Secure, true)
	return nil
}

func (s *Service) Logout(c *gin.Context) {
	if u, ok := UserFrom(c); ok {
		if sid, ok2 := c.Get(contextSID); ok2 {
			if sidUUID, err := uuid.Parse(sid.(string)); err == nil {
				s.DB.Where("id = ? AND user_id = ?", sidUUID, u.ID).Delete(&models.Session{})
			}
		}
	}
	c.SetCookie(CookieName, "", -1, "/", s.Domain, s.Secure, true)
}

// Middleware validates the session cookie. If `required` and there's no valid
// user, returns 401.
func (s *Service) Middleware(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(CookieName)
		if err != nil || raw == "" {
			if required {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
				return
			}
			c.Next()
			return
		}
		parts := strings.SplitN(raw, ".", 2)
		// JWT itself has dots — split by LAST dot to get rawToken
		i := strings.LastIndex(raw, ".")
		var jwtPart, rawToken string
		if i > 0 {
			jwtPart = raw[:i]
			rawToken = raw[i+1:]
		} else {
			jwtPart = parts[0]
		}

		tk, err := jwt.ParseWithClaims(jwtPart, &claims{}, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("bad signing method")
			}
			return s.JWTSecret, nil
		})
		if err != nil || !tk.Valid {
			s.clearCookieIfRequired(c, required, "invalid token")
			return
		}
		cl, _ := tk.Claims.(*claims)
		uid, err1 := uuid.Parse(cl.UID)
		sid, err2 := uuid.Parse(cl.SID)
		if err1 != nil || err2 != nil {
			s.clearCookieIfRequired(c, required, "bad claims")
			return
		}

		// verify session exists + raw token matches DB hash
		var sess models.Session
		if err := s.DB.First(&sess, "id = ? AND user_id = ?", sid, uid).Error; err != nil {
			s.clearCookieIfRequired(c, required, "session revoked")
			return
		}
		if time.Now().After(sess.ExpiresAt) {
			s.clearCookieIfRequired(c, required, "session expired")
			return
		}
		hash := sha256.Sum256([]byte(rawToken))
		if hex.EncodeToString(hash[:]) != sess.TokenHash {
			s.clearCookieIfRequired(c, required, "token mismatch")
			return
		}

		var user models.User
		if err := s.DB.First(&user, "id = ?", uid).Error; err != nil {
			s.clearCookieIfRequired(c, required, "user gone")
			return
		}
		if user.Status != models.UserActive {
			s.clearCookieIfRequired(c, required, "user suspended")
			return
		}

		c.Set(contextUser, &user)
		c.Set(contextSID, sid.String())
		c.Next()
	}
}

// TokenOrSession accepts either a session cookie or a personal access token.
// Mounted only on the upload and library routes: a token pasted into a
// third-party tool should be able to post pictures, never to change the
// account's password, storage credentials, or passkeys. Those stay
// cookie-only, so a leaked token can't be escalated into a takeover.
func (s *Service) TokenOrSession() gin.HandlerFunc {
	cookieAuth := s.Middleware(true)
	return func(c *gin.Context) {
		raw := extractBearer(c)
		if raw == "" {
			cookieAuth(c)
			return
		}
		user, tok, err := s.authenticateToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "auth: " + err.Error()})
			return
		}
		// Touch last_used_at without blocking the request on it.
		go func(id uuid.UUID) {
			now := time.Now()
			s.DB.Model(&models.APIToken{}).Where("id = ?", id).Update("last_used_at", now)
		}(tok.ID)

		c.Set(contextUser, user)
		c.Next()
	}
}

// extractBearer pulls a token from either the Authorization header or the
// X-API-Key header. PicGo and Typora can set custom headers but not all of
// them handle the "Bearer " prefix, so both spellings are accepted.
func extractBearer(c *gin.Context) string {
	if h := strings.TrimSpace(c.GetHeader("Authorization")); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
		if strings.HasPrefix(h, APITokenPrefix) {
			return h
		}
	}
	if k := strings.TrimSpace(c.GetHeader("X-API-Key")); k != "" {
		return k
	}
	return ""
}

// authenticateToken resolves a raw token to its owner. Lookup is by hash, so a
// database leak doesn't hand over working credentials.
func (s *Service) authenticateToken(raw string) (*models.User, *models.APIToken, error) {
	if !strings.HasPrefix(raw, APITokenPrefix) {
		return nil, nil, errors.New("invalid token format")
	}
	sum := sha256.Sum256([]byte(raw))
	var tok models.APIToken
	if err := s.DB.First(&tok, "token_hash = ?", hex.EncodeToString(sum[:])).Error; err != nil {
		return nil, nil, errors.New("invalid token")
	}
	if tok.Revoked {
		return nil, nil, errors.New("token revoked")
	}
	if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
		return nil, nil, errors.New("token expired")
	}
	var user models.User
	if err := s.DB.First(&user, "id = ?", tok.UserID).Error; err != nil {
		return nil, nil, errors.New("token owner gone")
	}
	if user.Status != models.UserActive {
		return nil, nil, errors.New("account suspended")
	}
	return &user, &tok, nil
}

// NewAPIToken mints a token, returning the plaintext (shown to the user
// exactly once) and its storable hash.
func NewAPIToken() (plain, hash, prefix string) {
	plain = APITokenPrefix + newRandomToken(24)
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	prefix = plain[:len(APITokenPrefix)+6]
	return plain, hash, prefix
}

func (s *Service) clearCookieIfRequired(c *gin.Context, required bool, reason string) {
	c.SetCookie(CookieName, "", -1, "/", s.Domain, s.Secure, true)
	if required {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "auth: " + reason})
		return
	}
	c.Next()
}

// AdminOnly is a middleware for admin-only routes. Use after Middleware(true).
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := UserFrom(c)
		if !ok || !u.IsAdmin() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
			return
		}
		c.Next()
	}
}

// UserFrom retrieves the authenticated user from gin context.
func UserFrom(c *gin.Context) (*models.User, bool) {
	v, ok := c.Get(contextUser)
	if !ok {
		return nil, false
	}
	u, ok := v.(*models.User)
	return u, ok
}

func MustUser(c *gin.Context) *models.User {
	u, _ := UserFrom(c)
	return u
}

func newRandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("rand: %v", err))
	}
	return hex.EncodeToString(b)
}
