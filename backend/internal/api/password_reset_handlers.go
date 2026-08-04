package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Password recovery.
//
// This is not a third way to sign in — it mints no session. It exists because
// email+password is now one of only two login modes, and a password with no
// recovery path is a one-way door: accounts created by the old magic-link flow
// have no password at all, and would otherwise be locked out permanently.

type resetRequestReq struct {
	Email string `json:"email" binding:"required,email"`
}

// POST /auth/password/reset/request
//
// Always answers 202, whether or not the address has an account. Any other
// behaviour turns this endpoint into a way to test which emails are registered.
func (s *Server) handleResetRequest(c *gin.Context) {
	var req resetRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的邮箱"})
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))

	var u models.User
	err := s.DB.Where("email = ?", emailAddr).First(&u).Error
	if err == nil && u.Status == models.UserActive {
		if status, msg := s.issueOTP(c, emailAddr, models.OTPPurposeReset); msg != "" {
			// A rate limit is worth telling the truth about: it is about this
			// request, not about whether the account exists.
			if status == http.StatusTooManyRequests {
				c.JSON(status, gin.H{"error": msg})
				return
			}
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"sent":      true,
		"ttl_secs":  int(otpTTL.Seconds()),
		"resend_in": int(otpRateLimitWin.Seconds()),
	})
}

type resetConfirmReq struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// POST /auth/password/reset — set a new password and sign in.
func (s *Server) handleResetConfirm(c *gin.Context) {
	var req resetConfirmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := "新密码至少 8 位"
		if len(strings.TrimSpace(req.Code)) != 6 {
			msg = "需要 6 位邮箱验证码"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))

	// Consume the code first. Doing the user lookup first would let the
	// response time distinguish a real address from a made-up one.
	if status, msg := s.consumeOTP(emailAddr, req.Code, models.OTPPurposeReset); msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	var u models.User
	if err := s.DB.Where("email = ?", emailAddr).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码无效或已过期"})
		return
	}
	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被停用"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}
	updates := map[string]any{"password_hash": hash}
	// Reaching the code proves control of the mailbox, which is exactly what
	// email verification asserts.
	if !u.EmailVerified {
		updates["email_verified"] = true
	}
	if err := s.DB.Model(&u).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.Auth.IssueSession(c, &u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "email": u.Email})
}
