package api

import (
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	// Login codes are issued by the public endpoint; letting an authenticated
	// caller mint one here would hand a signed-in session a way to produce
	// login codes for its own address on demand.
	if !models.ValidOTPPurpose(req.Purpose) || req.Purpose == models.OTPPurposeLogin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的验证用途"})
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
		msg := "新密码至少 8 位"
		if len(strings.TrimSpace(req.Code)) != 6 {
			msg = "需要 6 位邮箱验证码"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if status, msg := s.consumeOTP(u.Email, req.Code, models.OTPPurposePassword); msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if len([]rune(name)) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "昵称最多 32 个字符"})
		return
	}
	if err := s.DB.Model(u).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name})
}
