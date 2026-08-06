package api

import (
	"github.com/gentpan/openimg/backend/internal/i18n"
	"net/http"
	"strings"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// Sensitive account changes are gated on a code mailed to the account's own
// address. A live session is not enough on its own: a borrowed laptop or a
// stolen cookie should not be able to change the password, add a passkey
// (which is a permanent second key to the account), or erase the library.
//
// The code is always sent to the address on the account, never to one supplied
// in the request — otherwise the gate would just be asking the attacker where
// to mail the key.

type actionOTPReq struct {
	Purpose string `json:"purpose" binding:"required"`
}

// POST /api/account/otp — mail a code for one sensitive action.
func (s *Server) handleAccountOTP(c *gin.Context) {
	u := auth.MustUser(c)
	var req actionOTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "auth.bad_params")})
		return
	}
	// Login codes are issued by the public endpoint; letting an authenticated
	// caller mint one here would hand a signed-in session a way to produce
	// login codes for its own address on demand.
	if !models.ValidOTPPurpose(req.Purpose) || req.Purpose == models.OTPPurposeLogin {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "otp.purpose_unknown")})
		return
	}
	if status, msg := s.issueOTP(c, strings.ToLower(u.Email), req.Purpose); msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"sent":      true,
		"email":     maskEmail(u.Email),
		"purpose":   req.Purpose,
		"ttl_secs":  int(otpTTL.Seconds()),
		"resend_in": int(otpRateLimitWin.Seconds()),
	})
}

// maskEmail shows enough of the address for the user to recognise which
// mailbox to open, without printing it in full on a screen someone may be
// looking over.
func maskEmail(e string) string {
	at := strings.LastIndex(e, "@")
	if at <= 0 {
		return e
	}
	local, domain := e[:at], e[at:]
	if len(local) <= 2 {
		return local[:1] + "***" + domain
	}
	return local[:2] + strings.Repeat("*", 3) + domain
}

type changePasswordReq struct {
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
	Code        string `json:"code" binding:"required,len=6"`
}

// POST /api/account/password — set a new password, gated on an emailed code.
//
// The current password is deliberately not required. Proving control of the
// mailbox is the stronger check of the two, and demanding both would lock out
// exactly the users who need this most: the ones who signed up with a magic
// link or an OAuth provider and have no password to type.
func (s *Server) handleChangePassword(c *gin.Context) {
	u := auth.MustUser(c)
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := i18n.T(c, "auth.password_short")
		if len(strings.TrimSpace(req.Code)) != 6 {
			msg = i18n.T(c, "auth.otp_needed")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if status, key := s.consumeOTP(u.Email, req.Code, models.OTPPurposePassword); key != "" {
		c.JSON(status, gin.H{"error": i18n.T(c, key)})
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": i18n.T(c, "auth.password_hash")})
		return
	}
	if err := s.DB.Model(u).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type nicknameReq struct {
	// Nickname is a display name, nothing more. It is not unique and is never
	// used to look an account up — the email is the identity. So there is no
	// reason to reserve it, reject duplicates, or restrict the character set
	// beyond a length bound.
	Name string `json:"name"`
}

// PATCH /api/account/profile — update the display nickname.
func (s *Server) handleUpdateNickname(c *gin.Context) {
	u := auth.MustUser(c)
	var req nicknameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "auth.bad_params")})
		return
	}
	name := strings.TrimSpace(req.Name)
	if len([]rune(name)) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "auth.name_too_long")})
		return
	}
	if err := s.DB.Model(u).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name})
}
