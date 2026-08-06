package api

import (
	"github.com/gentpan/openimg/backend/internal/i18n"
	"net/http"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxTokensPerUser keeps the token list manageable and limits the blast radius
// of an account whose tokens keep leaking.
const maxTokensPerUser = 10

type tokenOut struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	Revoked    bool   `json:"revoked"`
	LastUsedAt string `json:"last_used_at,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func toTokenOut(t models.APIToken) tokenOut {
	out := tokenOut{
		ID:        t.ID.String(),
		Name:      t.Name,
		Prefix:    t.Prefix,
		Revoked:   t.Revoked,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	if t.LastUsedAt != nil {
		out.LastUsedAt = t.LastUsedAt.Format(time.RFC3339)
	}
	if t.ExpiresAt != nil {
		out.ExpiresAt = t.ExpiresAt.Format(time.RFC3339)
	}
	return out
}

// GET /api/tokens
func (s *Server) handleListTokens(c *gin.Context) {
	u := auth.MustUser(c)
	var tokens []models.APIToken
	s.DB.Where("user_id = ?", u.ID).Order("created_at DESC").Find(&tokens)
	out := make([]tokenOut, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, toTokenOut(t))
	}
	c.JSON(http.StatusOK, gin.H{"tokens": out})
}

type createTokenReq struct {
	Name string `json:"name" binding:"required,min=1,max=64"`
	// ExpiresInDays of 0 means no expiry.
	ExpiresInDays int `json:"expires_in_days"`
}

// POST /api/tokens — the plaintext token is returned here and nowhere else.
func (s *Server) handleCreateToken(c *gin.Context) {
	u := auth.MustUser(c)
	var req createTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var count int64
	s.DB.Model(&models.APIToken{}).Where("user_id = ? AND revoked = ?", u.ID, false).Count(&count)
	if count >= maxTokensPerUser {
		c.JSON(http.StatusForbidden, gin.H{
			"error": i18n.T(c, "token.limit_reached"), "max": maxTokensPerUser,
		})
		return
	}

	plain, hash, prefix := auth.NewAPIToken()
	tok := models.APIToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		Name:      strings.TrimSpace(req.Name),
		TokenHash: hash,
		Prefix:    prefix,
	}
	if req.ExpiresInDays > 0 {
		exp := time.Now().AddDate(0, 0, req.ExpiresInDays)
		tok.ExpiresAt = &exp
	}
	if err := s.DB.Create(&tok).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":   toTokenOut(tok),
		"plain":   plain,
		"warning": "此 Token 只显示这一次，请立即保存",
	})
}

// DELETE /api/tokens/:id — hard delete; a revoked-but-present row has no value
// to the user, and the audit trail that matters (uploads) references the user,
// not the token.
func (s *Server) handleDeleteToken(c *gin.Context) {
	u := auth.MustUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "token.bad_id")})
		return
	}
	res := s.DB.Where("id = ? AND user_id = ?", id, u.ID).Delete(&models.APIToken{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": i18n.T(c, "token.not_found")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
