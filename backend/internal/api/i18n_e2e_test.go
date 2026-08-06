package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gin-gonic/gin"
)

// The whole point of the middleware, checked end to end: one handler, one
// request path, two languages out.
func TestUploadErrorsFollowAcceptLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(i18n.Middleware())
	// Stands in for the real handler's early-exit branch: same T call, no DB.
	r.POST("/api/upload", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "upload.missing_field")})
	})

	for header, want := range map[string]string{
		"zh-CN": "缺少上传文件字段 file",
		"en-US": "Missing upload field: file",
		"":      "缺少上传文件字段 file", // 不带语言头 → 默认中文，老客户端不受影响
	} {
		req := httptest.NewRequest("POST", "/api/upload", nil)
		if header != "" {
			req.Header.Set("Accept-Language", header)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("Accept-Language %q → %s，期望包含 %q", header, w.Body.String(), want)
		}
	}
}

// Formatted messages must interpolate in both languages, with the argument
// landing wherever that language puts it.
func TestFormattedMessageBothLanguages(t *testing.T) {
	if got := i18n.TL(i18n.ZH, "upload.too_large", 20.0); !strings.Contains(got, "20.0 MB") {
		t.Errorf("中文格式化失败: %q", got)
	}
	if got := i18n.TL(i18n.EN, "upload.too_large", 20.0); !strings.Contains(got, "20.0 MB") {
		t.Errorf("英文格式化失败: %q", got)
	}
}
