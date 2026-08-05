package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAppleAssociation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, appID string
		want        int
		wantType    string
	}{
		{"未配置时 404 而不是让 SPA 兜底", "", 404, "application/json"},
		{"配置后返回 JSON 文档", "ABCDE12345.io.openimg.mac", 200, "application/json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{AppleAppID: tc.appID}
			r := gin.New()
			s.registerAppleAssociation(r)
			// SPA 兜底也挂上，复现线上那条路径
			r.NoRoute(func(c *gin.Context) { c.Data(200, "text/html; charset=utf-8", []byte("<html>")) })

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, appleAssocPath, nil))
			if w.Code != tc.want {
				t.Fatalf("状态码 = %d，期望 %d", w.Code, tc.want)
			}
			if ct := w.Header().Get("Content-Type"); ct[:16] != tc.wantType {
				t.Fatalf("Content-Type = %q，期望 %s", ct, tc.wantType)
			}
			if tc.want == 200 {
				var got struct {
					WebCredentials struct{ Apps []string } `json:"webcredentials"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if len(got.WebCredentials.Apps) != 1 || got.WebCredentials.Apps[0] != tc.appID {
					t.Fatalf("apps = %v", got.WebCredentials.Apps)
				}
			}
		})
	}
}
