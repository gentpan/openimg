package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gentpan/openimg/backend/internal/i18n"
	"github.com/gin-gonic/gin"
)

// Sweeps the whole catalogue through the middleware rather than asserting on
// one message: any key whose two languages are identical is either untranslated
// or a proper noun, and this is the cheapest place to notice.
func TestEveryMessageDiffersByLanguage(t *testing.T) {
	same := []string{}
	for _, k := range i18n.Keys() {
		zh, en := i18n.TL(i18n.ZH, k), i18n.TL(i18n.EN, k)
		if zh == en {
			same = append(same, k+" = "+zh)
		}
	}
	// Proper nouns legitimately match; a long list means something was skipped.
	if len(same) > 5 {
		t.Errorf("有 %d 条中英文完全相同，可能漏翻：\n  %s", len(same), strings.Join(same, "\n  "))
	}
}

func TestHandlerAnswersInRequestLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(i18n.Middleware())
	r.POST("/api/upload", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(c, "upload.missing_field")})
	})
	for header, want := range map[string]string{
		"zh-CN": "缺少上传文件字段 file",
		"en-US": "Missing upload field: file",
		"":      "缺少上传文件字段 file",
	} {
		req := httptest.NewRequest("POST", "/api/upload", nil)
		if header != "" {
			req.Header.Set("Accept-Language", header)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("Accept-Language %q → %s，期望 %q", header, w.Body.String(), want)
		}
	}
}
