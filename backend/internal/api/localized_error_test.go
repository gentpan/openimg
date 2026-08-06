package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gentpan/openimg/backend/internal/quota"
	"github.com/gin-gonic/gin"
)

// The status code depends on this. If localize stopped being transparent to
// errors.Is, an out-of-space upload would answer 500 instead of 507 and the
// client would show "server error" for a full disk.
func TestLocalizedStaysMatchable(t *testing.T) {
	err := localize("空间不足：需要 2 MB，剩余 1 MB", quota.ErrInsufficient)

	if !errors.Is(err, quota.ErrInsufficient) {
		t.Error("errors.Is 认不出被包装的哨兵错误，507 判断会失效")
	}
	// And the reverse: the sentinel's own English text must not leak into the
	// message, which is the bug this type exists to fix.
	if strings.Contains(err.Error(), quota.ErrInsufficient.Error()) {
		t.Errorf("哨兵原文混进了消息：%q", err.Error())
	}
}

// The deep layers translate through the request's context.Context, not gin's,
// so the middleware has to populate both.
func TestMiddlewarePopulatesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(i18n.Middleware())
	var fromCtx string
	r.GET("/x", func(c *gin.Context) {
		fromCtx = i18n.TCtx(c.Request.Context(), "upload.missing_field")
		c.Status(200)
	})
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Accept-Language", "en-US")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if fromCtx != "Missing upload field: file" {
		t.Errorf("请求 context 没带上语言，TCtx 得到 %q", fromCtx)
	}
	// No middleware at all must still produce a sentence, not a panic or "".
	if got := i18n.TCtx(context.Background(), "upload.missing_field"); got == "" {
		t.Error("裸 context 下 TCtx 返回空串")
	}
}
