package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Locks in the ClientIP contract that the rate limiter and every recorded IP
// depend on. If this breaks, per-IP limiting silently degrades to per-CF-edge
// shared buckets again — which is invisible until users start seeing 429s
// they did not earn.
func TestClientIPBehindCloudflare(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		peer string // what Caddy looks like to the app
		cfIP string // CF-Connecting-IP, already filtered by Caddy
		xff  string
		want string
	}{
		// The normal path: Cloudflare said who connected, believe it.
		{"via cloudflare", "127.0.0.1:9000", "203.0.113.7", "203.0.113.7, 162.158.1.1", "203.0.113.7"},
		// Direct to origin: Caddy stripped the CF header, and its own
		// X-Forwarded-For names the real peer.
		{"direct, header stripped", "127.0.0.1:9000", "", "198.51.100.9", "198.51.100.9"},
		// A forged XFF prefix from an untrusted peer must not win: only the
		// rightmost hop added by trusted Caddy counts.
		{"forged xff prefix", "127.0.0.1:9000", "", "10.0.0.1, 198.51.100.9", "198.51.100.9"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := gin.New()
			configureClientIP(r)
			var got string
			r.GET("/x", func(ctx *gin.Context) { got = ctx.ClientIP(); ctx.Status(200) })

			req := httptest.NewRequest("GET", "/x", nil)
			req.RemoteAddr = c.peer
			if c.cfIP != "" {
				req.Header.Set("CF-Connecting-IP", c.cfIP)
			}
			if c.xff != "" {
				req.Header.Set("X-Forwarded-For", c.xff)
			}
			r.ServeHTTP(httptest.NewRecorder(), req)

			if got != c.want {
				t.Errorf("ClientIP = %q, want %q", got, c.want)
			}
		})
	}
}
