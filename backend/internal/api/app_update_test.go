package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 挂一个和生产同形状的 SPA 兜底:任何未匹配路径返回 200 + HTML。
// 更新清单这条路由必须自己答,不能落进它。
func routerWithSPAFallback(s *Server) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s.registerMacUpdate(r)
	r.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!doctype html><html></html>"))
	})
	return r
}

func get(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// 没配置时必须是明确的 404,而不是被兜底吞成 200 + HTML。
//
// 这是整个更新链路上最阴的一个失败模式:客户端只要检查状态码就会把一份 HTML
// 当成合法清单,表现是「静默地永远没有更新」——不报错、不打日志,没人会在发布
// 当天发现。apple_assoc.go 已经为同一个坑立过一次法。
func TestMacUpdateNotSwallowedBySPAFallback(t *testing.T) {
	r := routerWithSPAFallback(&Server{})
	w := get(r, macUpdatePath)

	if w.Code != http.StatusNotFound {
		t.Errorf("没配置时应当 404,得到 %d —— 说明这条路由被 SPA 兜底吞了", w.Code)
	}
	if strings.Contains(w.Body.String(), "<!doctype") {
		t.Error("返回了 HTML —— 客户端会把它当成清单")
	}
	// 对照:随便一个别的路径确实会被兜底接走,证明上面那条不是因为兜底没生效。
	if other := get(r, "/definitely-not-a-route"); other.Code != http.StatusOK {
		t.Fatalf("对照组失败:兜底本身没生效(得到 %d),上面那条测试不成立", other.Code)
	}
}

// 配了但文件不在,同样是 404 而不是 200。
func TestMacUpdateMissingFileIs404(t *testing.T) {
	s := &Server{MacUpdateManifest: filepath.Join(t.TempDir(), "nope.json")}
	if w := get(routerWithSPAFallback(s), macUpdatePath); w.Code != http.StatusNotFound {
		t.Errorf("文件不在时应当 404,得到 %d", w.Code)
	}
}

// 正常情况:原样吐出字节,Content-Type 是 JSON,缓存 TTL 短。
func TestMacUpdateServesManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update.json")
	body := `{"payload":"eyJ4IjoxfQ==","sig":"AA==","keyId":"k1"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	w := get(routerWithSPAFallback(&Server{MacUpdateManifest: path}), macUpdatePath)
	if w.Code != http.StatusOK {
		t.Fatalf("应当 200,得到 %d", w.Code)
	}
	if got := w.Body.String(); got != body {
		t.Errorf("字节被改动了:\n得到 %s\n想要 %s", got, body)
	}
	// 客户端的第一道闸就是这个头。它不对,一份完全合法的清单也会被拒。
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q,客户端只认 application/json 开头", ct)
	}
	// 撤回一个坏版本靠推新清单,TTL 长了就推不出去。
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=300") {
		t.Errorf("Cache-Control = %q,想要短 TTL", cc)
	}
}
