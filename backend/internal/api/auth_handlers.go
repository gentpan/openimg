package api

import (
	"github.com/gentpan/openimg/backend/internal/i18n"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gentpan/openimg/backend/internal/referral"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type registerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	Code     string `json:"code" binding:"required,len=6"`
	Name     string `json:"name" binding:"required,min=1,max=32"`
	Ref      string `json:"ref"`
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type publicUser struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	Group          string `json:"group,omitempty"`
	QuotaBytes     int64  `json:"quota_bytes"`
	UsedBytes      int64  `json:"used_bytes"`
	AvailableBytes int64  `json:"available_bytes"`
	CheckinStreak  int    `json:"checkin_streak"`
	CheckedInToday bool   `json:"checked_in_today"`
	HasPassword    bool   `json:"has_password"`
	EmailVerified  bool   `json:"email_verified"`
	Google         bool   `json:"google_connected"`
	Github         bool   `json:"github_connected"`
	// Picbi 不是登录方式,是"额度从 pic.bi 扣"的关联。前端据此决定要不要
	// 显示 4K 档与那边的余额。
	Picbi         bool   `json:"picbi_connected"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	ReferralCode  string `json:"referral_code,omitempty"`
	UploadMode    string `json:"upload_mode"`
	VariantFormat string `json:"variant_format"`
	MaxImageWidth int    `json:"max_image_width"`
}

func toPublic(u *models.User, group *models.UserGroup) publicUser {
	pu := publicUser{
		ID:             u.ID.String(),
		Email:          u.Email,
		Name:           u.Name,
		Role:           string(u.Role),
		QuotaBytes:     u.QuotaBytes,
		UsedBytes:      u.UsedBytes,
		AvailableBytes: quota.Available(u),
		CheckinStreak:  u.CheckinStreak,
		CheckedInToday: u.LastCheckinDate == time.Now().UTC().Format("2006-01-02"),
		HasPassword:    u.PasswordHash != "",
		EmailVerified:  u.EmailVerified,
		Google:         u.GoogleSub != "",
		Github:         u.GithubID != "",
		Picbi:          u.PicbiID != "",
		AvatarURL:      u.AvatarURL,
		ReferralCode:   u.ReferralCode,
		UploadMode:     u.UploadMode,
		VariantFormat:  u.VariantFormat,
		MaxImageWidth:  u.MaxImageWidth,
	}
	if group != nil {
		pu.Group = group.Name
	}
	return pu
}

func (s *Server) handleRegister(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		msg := i18n.T(c, "auth.credentials_short")
		switch {
		case strings.TrimSpace(req.Name) == "":
			msg = i18n.T(c, "auth.name_required")
		case len([]rune(strings.TrimSpace(req.Name))) > 32:
			msg = i18n.T(c, "auth.name_too_long")
		case len(strings.TrimSpace(req.Code)) != 6:
			msg = i18n.T(c, "auth.otp_required")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)

	var existing models.User
	if err := s.DB.Where("email = ?", email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": i18n.T(c, "auth.email_taken")})
		return
	}

	// The code proves the address exists and belongs to whoever is signing up.
	// Without it the site accepts accounts on addresses their owner has never
	// seen — which is how a free host ends up sending mail nobody asked for and
	// gets its domain listed for it.
	if status, key := s.consumeOTP(email, req.Code, models.OTPPurposeRegister); key != "" {
		c.JSON(status, gin.H{"error": i18n.T(c, key)})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash failed"})
		return
	}

	// Default group = "free"
	var freeGroup models.UserGroup
	s.DB.Where("name = ?", "free").First(&freeGroup)

	u := models.NewUser(email, name, hash)
	u.EmailVerified = true
	u.SignupIP = c.ClientIP()
	if freeGroup.ID != uuid.Nil {
		u.GroupID = &freeGroup.ID
	}
	if err := s.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = referral.EnsureCode(s.DB, &u)
	if _, err := quota.SignupGrant(s.DB, u.ID, &freeGroup); err != nil {
		log.Printf("register: signup grant failed for %s: %v", u.ID, err)
	}
	refCode := req.Ref
	if refCode == "" {
		refCode, _ = c.Cookie(refCookieName)
	}
	_ = referral.AttachAndReward(s.DB, &u, refCode)
	c.SetCookie(refCookieName, "", -1, "/", s.CookieDomain, s.CookieSecure, false)
	s.DB.First(&u, "id = ?", u.ID)
	if err := s.Auth.IssueSession(c, &u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toPublic(&u, &freeGroup))
}

func (s *Server) handleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	var u models.User
	if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if u.Status != models.UserActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account suspended"})
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
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

func (s *Server) handleLogout(c *gin.Context) {
	s.Auth.Logout(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleMe(c *gin.Context) {
	u := auth.MustUser(c)
	var group *models.UserGroup
	if u.GroupID != nil {
		var g models.UserGroup
		if err := s.DB.First(&g, "id = ?", *u.GroupID).Error; err == nil {
			group = &g
		}
	}
	c.JSON(http.StatusOK, toPublic(u, group))
}

type registerCodeReq struct {
	Email string `json:"email" binding:"required,email"`
}

// POST /auth/register/code — mail a code to an address that wants to sign up.
//
// Unlike password recovery, this one says plainly when an address is already
// registered. Signup cannot hide that fact anyway — the account-creation step
// has to reject a duplicate — and pretending otherwise only sends someone to
// wait for a code that was never going to work.
func (s *Server) handleRegisterCode(c *gin.Context) {
	var req registerCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "auth.email_invalid")})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	var existing models.User
	if err := s.DB.Where("email = ?", email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": i18n.T(c, "auth.email_taken")})
		return
	}
	if status, msg := s.issueOTP(c, email, models.OTPPurposeRegister); msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"sent":      true,
		"ttl_secs":  int(otpTTL.Seconds()),
		"resend_in": int(otpRateLimitWin.Seconds()),
	})
}
