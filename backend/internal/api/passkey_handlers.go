package api

import (
	"github.com/gentpan/openimg/backend/internal/i18n"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"gorm.io/gorm"
)

// ---- Enroll ----

type enrollBeginReq struct {
	Code string `json:"code" binding:"required,len=6"`
}

// Enrolment is gated on an emailed code because a passkey is a permanent
// second key to the account: whoever adds one keeps access even after the
// password changes and every session is revoked. The gate sits on begin rather
// than finish so the browser never even prompts for a biometric unless the
// request was authorised.
func (s *Server) handlePasskeyEnrollBegin(c *gin.Context) {
	if s.Passkey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey not configured"})
		return
	}
	user := auth.MustUser(c)
	var req enrollBeginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "passkey.otp_required")})
		return
	}
	if status, key := s.consumeOTP(user.Email, req.Code, models.OTPPurposePasskey); key != "" {
		c.JSON(status, gin.H{"error": i18n.T(c, key)})
		return
	}
	options, flowID, err := s.Passkey.BeginEnroll(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"flow": flowID, "options": options})
}

type enrollFinishReq struct {
	Flow       string                               `json:"flow" binding:"required"`
	Name       string                               `json:"name"`
	Credential *protocol.CredentialCreationResponse `json:"credential" binding:"required"`
}

func (s *Server) handlePasskeyEnrollFinish(c *gin.Context) {
	if s.Passkey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey not configured"})
		return
	}
	user := auth.MustUser(c)
	var req enrollFinishReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parsed, err := req.Credential.Parse()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parse credential: " + err.Error()})
		return
	}
	pkc, err := s.Passkey.FinishEnroll(user, req.Flow, req.Name, parsed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pkc)
}

// ---- Login ----

type loginBeginReq struct {
	Email string `json:"email"` // optional — empty triggers discoverable login
}

func (s *Server) handlePasskeyLoginBegin(c *gin.Context) {
	if s.Passkey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey not configured"})
		return
	}
	var req loginBeginReq
	_ = c.ShouldBindJSON(&req) // empty body = discoverable
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))

	// Email-less path: ask the browser to surface any matching passkey on
	// device. The user picks one in the platform UI; the credential's userHandle
	// reveals which account to log in. No email guessing required.
	if emailAddr == "" {
		options, flowID, err := s.Passkey.BeginDiscoverableLogin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"flow": flowID, "options": options, "discoverable": true})
		return
	}

	var u models.User
	if err := s.DB.Where("email = ?", emailAddr).First(&u).Error; err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusUnauthorized, gin.H{"error": i18n.T(c, "passkey.none_registered")})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	options, flowID, err := s.Passkey.BeginLogin(&u)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"flow": flowID, "options": options})
}

type loginFinishReq struct {
	Email      string                                `json:"email"` // optional for discoverable
	Flow       string                                `json:"flow" binding:"required"`
	Credential *protocol.CredentialAssertionResponse `json:"credential" binding:"required"`
	// Set by a native client. WebAuthn is finished the same way either
	// way; what differs is what the caller can hold onto afterwards — a Mac
	// app has no cookie jar, so it gets a handoff code instead of a session.
	Native bool `json:"native"`
}

func (s *Server) handlePasskeyLoginFinish(c *gin.Context) {
	if s.Passkey == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey not configured"})
		return
	}
	var req loginFinishReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parsed, err := req.Credential.Parse()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parse credential: " + err.Error()})
		return
	}

	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	var u *models.User

	if emailAddr == "" {
		// Discoverable: the credential reveals the user.
		resolved, err := s.Passkey.FinishDiscoverableLogin(req.Flow, parsed)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		u = resolved
	} else {
		var got models.User
		if err := s.DB.Where("email = ?", emailAddr).First(&got).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": i18n.T(c, "passkey.login_failed")})
			return
		}
		if err := s.Passkey.FinishLogin(&got, req.Flow, parsed); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		u = &got
	}

	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		return
	}
	// A native client takes the same one-time code the OAuth flow hands back,
	// and redeems it at /auth/native/exchange for a personal access token.
	// Reusing that path rather than minting a token here keeps every native
	// credential coming out of one place, with one expiry and one naming rule.
	//
	// No cookie is issued in this branch: the request came from an app, and a
	// session it cannot present is a session that only widens what a stolen
	// response is worth.
	if req.Native {
		c.JSON(http.StatusOK, gin.H{"code": s.nativeCodes.issue(u.ID)})
		return
	}
	if err := s.Auth.IssueSession(c, u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session: " + err.Error()})
		return
	}
	var group *models.UserGroup
	if u.GroupID != nil {
		var g models.UserGroup
		if err := s.DB.First(&g, "id = ?", *u.GroupID).Error; err == nil {
			group = &g
		}
	}
	c.JSON(http.StatusOK, toPublic(u, group, s.AvatarFor(u)))
}

// ---- List + delete user's passkeys ----

type pkPublic struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

func (s *Server) handlePasskeyList(c *gin.Context) {
	user := auth.MustUser(c)
	creds := []models.PasskeyCredential{}
	s.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Find(&creds)
	out := make([]pkPublic, 0, len(creds))
	for _, c := range creds {
		p := pkPublic{ID: c.ID.String(), Name: c.Name, CreatedAt: c.CreatedAt.Format(time.RFC3339)}
		if c.LastUsedAt != nil {
			s := c.LastUsedAt.Format(time.RFC3339)
			p.LastUsedAt = &s
		}
		out = append(out, p)
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": out})
}

func (s *Server) handlePasskeyDelete(c *gin.Context) {
	user := auth.MustUser(c)
	id := c.Param("id")
	res := s.DB.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.PasskeyCredential{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
