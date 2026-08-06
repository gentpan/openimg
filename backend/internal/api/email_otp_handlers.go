package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/email"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/referral"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	otpLength       = 6
	otpTTL          = 5 * time.Minute
	otpMaxAttempts  = 5
	otpRateLimitWin = 60 * time.Second // 1 OTP per email per minute
)

type requestOTPReq struct {
	Email string `json:"email" binding:"required,email"`
}

type verifyOTPReq struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
	Name  string `json:"name"` // optional, used only when creating a new user
	Ref   string `json:"ref"`  // optional referral code, used only on create
}

// issueOTP creates a code for one purpose and mails it. Returns a client-safe
// error message, or "" on success.
//
// The rate limit is per (email, purpose): a user who just logged in must still
// be able to ask for a password-change code without waiting out the window,
// while a flood of codes for the same action is still blocked.
// otpHash binds a code to both the address and the action. Binding to the
// purpose is what stops a code mailed for one action from satisfying another;
// the purpose filter on the lookup is the second, independent barrier.
func otpHash(emailAddr, purpose, code string) string {
	sum := sha256.Sum256([]byte(emailAddr + ":" + purpose + ":" + code))
	return hex.EncodeToString(sum[:])
}

func (s *Server) issueOTP(c *gin.Context, emailAddr, purpose string) (int, string) {
	if !s.Email.Configured() {
		return http.StatusServiceUnavailable, "otp.not_configured"
	}
	var recent models.EmailOTP
	if err := s.DB.Where("email = ? AND purpose = ? AND created_at > ?",
		emailAddr, purpose, time.Now().Add(-otpRateLimitWin)).
		Order("created_at DESC").First(&recent).Error; err == nil {
		return http.StatusTooManyRequests, "otp.rate_limited"
	}

	code := generateOTP(otpLength)
	otp := models.EmailOTP{
		ID:        uuid.New(),
		Email:     emailAddr,
		Purpose:   purpose,
		CodeHash:  otpHash(emailAddr, purpose, code),
		IP:        c.ClientIP(),
		ExpiresAt: time.Now().Add(otpTTL),
	}
	if err := s.DB.Create(&otp).Error; err != nil {
		return http.StatusInternalServerError, "store otp: " + err.Error()
	}

	// Send in the background so a slow provider doesn't hold the request open.
	//
	// The request context must NOT be used here: it is cancelled the moment the
	// 202 is written, which killed every send mid-flight. WithoutCancel keeps
	// any request-scoped values while detaching the cancellation, and the
	// explicit timeout stops a hung provider from leaking the goroutine.
	labelKey := models.OTPPurposeLabel(purpose)
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 30*time.Second)
	body := email.OTPEmailHTML(code, int(otpTTL.Minutes()))
	go func() {
		defer cancel()
		if err := s.Email.Send(sendCtx, emailAddr,
			i18n.TCtx(sendCtx, "email.otp.subject", i18n.TCtx(sendCtx, labelKey), code), body); err != nil {
			// The OTP row is intentionally kept: the user can retry the send,
			// and rolling it back would invalidate a code that may have gone
			// out anyway.
			log.Printf("[otp:%s] send to %s failed (provider=%s): %v", purpose, emailAddr, s.Email.Name(), err)
		}
	}()
	return 0, ""
}

// consumeOTP checks a code against one purpose and burns it. Returns a
// catalogue key, or "" when the code was valid.
//
// A key rather than a message because this runs without a *gin.Context, so it
// has no language to render into; the six callers all have one and translate
// at the point they answer.
func (s *Server) consumeOTP(emailAddr, code, purpose string) (int, string) {
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	code = strings.TrimSpace(code)

	var otp models.EmailOTP
	err := s.DB.Where("email = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?",
		emailAddr, purpose, time.Now()).
		Order("created_at DESC").First(&otp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusUnauthorized, "otp.invalid_or_expired"
	} else if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	if otp.Attempts >= otpMaxAttempts {
		s.DB.Model(&otp).Update("used_at", time.Now())
		return http.StatusUnauthorized, "otp.too_many_attempts"
	}
	if otpHash(emailAddr, purpose, code) != otp.CodeHash {
		s.DB.Model(&otp).Update("attempts", otp.Attempts+1)
		return http.StatusUnauthorized, "otp.incorrect"
	}
	s.DB.Model(&otp).Update("used_at", time.Now())
	return 0, ""
}

func (s *Server) handleRequestOTP(c *gin.Context) {
	var req requestOTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	if status, msg := s.issueOTP(c, emailAddr, models.OTPPurposeLogin); msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"sent":      true,
		"ttl_secs":  int(otpTTL.Seconds()),
		"resend_in": int(otpRateLimitWin.Seconds()),
	})
}

func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req verifyOTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)

	if status, key := s.consumeOTP(emailAddr, code, models.OTPPurposeLogin); key != "" {
		c.JSON(status, gin.H{"error": i18n.T(c, key)})
		return
	}

	// Find or create user
	var u models.User
	if err := s.DB.Where("email = ?", emailAddr).First(&u).Error; err == gorm.ErrRecordNotFound {
		var freeGroup models.UserGroup
		s.DB.Where("name = ?", "free").First(&freeGroup)
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = emailAddr
		}
		u = models.NewUser(emailAddr, name, "")
		u.EmailVerified = true
		u.SignupIP = c.ClientIP()
		if freeGroup.ID != uuid.Nil {
			u.GroupID = &freeGroup.ID
		}
		if err := s.DB.Create(&u).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create user: " + err.Error()})
			return
		}
		_ = referral.EnsureCode(s.DB, &u)
		if _, err := quota.SignupGrant(s.DB, u.ID, &freeGroup); err != nil {
			log.Printf("otp signup: signup grant failed for %s: %v", u.ID, err)
		}
		refCode := req.Ref
		if refCode == "" {
			refCode, _ = c.Cookie(refCookieName)
		}
		_ = referral.AttachAndReward(s.DB, &u, refCode)
		c.SetCookie(refCookieName, "", -1, "/", s.CookieDomain, s.CookieSecure, false)
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else if !u.EmailVerified {
		s.DB.Model(&u).Update("email_verified", true)
	}

	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		return
	}

	if err := s.Auth.IssueSession(c, &u); err != nil {
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
	c.JSON(http.StatusOK, toPublic(&u, group))
}

// generateOTP returns a numeric code of the requested length, e.g. "418265".
func generateOTP(n int) string {
	const digits = "0123456789"
	out := make([]byte, n)
	max := big.NewInt(int64(len(digits)))
	for i := range out {
		idx, _ := rand.Int(rand.Reader, max)
		out[i] = digits[idx.Int64()]
	}
	return string(out)
}
