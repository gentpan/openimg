package api

import (
	"testing"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

// Router() panics on a duplicate method+path. That makes a mis-edited route
// table a boot-time crash rather than a test failure, which is the worst place
// to find out — and it is an easy edit to get wrong, since making a route
// token-accessible means *moving* the line, not adding one next to it.
func TestRouterRegistersWithoutDuplicates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{Auth: &auth.Service{}}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("路由注册 panic：%v", r)
		}
	}()
	r := s.Router()

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		if seen[key] {
			t.Errorf("重复注册：%s", key)
		}
		seen[key] = true
	}
	if len(seen) < 40 {
		t.Fatalf("只注册了 %d 条路由，构造似乎提前失败了", len(seen))
	}
}

// The client-facing contract: these must answer to a personal access token, or
// a native app cannot show which account it is on and cannot pre-validate a
// file against the tier limits before spending an upload on a 413.
func TestTokenReachableRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{Auth: &auth.Service{}}
	r := s.Router()

	want := []string{
		"POST /api/upload",
		"GET /api/images",
		"GET /api/images/:id",
		"DELETE /api/images/:id",
		"POST /api/images/bulk-delete",
		"POST /api/images/:id/variant",
		"GET /auth/me",
		"GET /api/quota",
		"GET /api/quota/transactions",
		"GET /api/storage/summary",
		"GET /api/checkin/history",
		"POST /api/checkin",
		"PATCH /api/preferences",
	}
	have := map[string]bool{}
	for _, route := range r.Routes() {
		have[route.Method+" "+route.Path] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("缺少路由 %s", w)
		}
	}

	// The other half of the contract — a leaked token must not be able to mint
	// more tokens, change the password, or delete the account — cannot be
	// asserted here: gin.RouteInfo exposes the path and the final handler, not
	// the middleware group, so cookie-only versus token-accessible is invisible
	// from the route table. What this does check is that those routes still
	// exist at all, so deleting one shows up as a failure rather than as a
	// feature that quietly disappeared.
	for _, p := range []string{"/api/tokens", "/api/account", "/api/account/password"} {
		var found bool
		for _, route := range r.Routes() {
			if route.Path == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("账号管理路由 %s 不见了", p)
		}
	}
}
