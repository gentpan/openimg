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
		"warning": i18n.T(c, "token.shown_once"),
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

// DELETE /api/tokens/current — 让一枚令牌注销它自己。
//
// 为什么它可以挂在令牌组,而 /api/tokens/:id 只认 cookie:那一条能删掉**任意
// 一枚**令牌,是账号管理;这一条只能删自己,方向是收权不是放权。一枚被粘进
// 第三方客户端的令牌把自己作废,最坏的后果是那个客户端要重新登录。
//
// 没有这条的时候,app 退出只是把令牌从本机抹掉,服务器上那枚仍然有效——而
// 提示语让用户"去网站的账号设置里删",现实是没人会去。于是每台退出过的机器
// 背后都留着一枚活令牌,这正是退出想避免的事。
func (s *Server) handleRevokeCurrentToken(c *gin.Context) {
	id, ok := auth.TokenID(c)
	if !ok {
		// cookie 会话没有令牌可撤。不报错:退出流程照常走完,前端也不必为
		// "我是哪种会话"分岔。
		c.JSON(http.StatusOK, gin.H{"ok": true, "revoked": false})
		return
	}
	u := auth.MustUser(c)
	res := s.DB.Where("id = ? AND user_id = ?", id, u.ID).Delete(&models.APIToken{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "revoked": res.RowsAffected > 0})
}
