package api

import (
	"github.com/gentpan/openimg/backend/internal/i18n"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

// Rate limiting is per-IP and in-process. That's the right scope for this
// deployment — a single Go binary behind Caddy — and it deliberately doesn't
// reach for Redis for something a map and a mutex handle. It is a blunt
// backstop against scripted abuse, not a fairness mechanism: the real upload
// limits are the per-tier daily counts, which survive restarts.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int           // tokens per window
	window  time.Duration // refill period
}

type bucket struct {
	tokens   int
	resetsAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		buckets: map[string]*bucket{},
		limit:   limit,
		window:  window,
	}
	go rl.reap()
	return rl
}

// allow consumes a token, reporting whether the caller may proceed and how
// long until the window resets.
func (rl *rateLimiter) allow(key string) (bool, time.Duration) {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	if !ok || now.After(b.resetsAt) {
		rl.buckets[key] = &bucket{tokens: rl.limit - 1, resetsAt: now.Add(rl.window)}
		return true, 0
	}
	if b.tokens <= 0 {
		return false, time.Until(b.resetsAt)
	}
	b.tokens--
	return true, 0
}

// reap drops expired buckets so a long-running server doesn't accumulate one
// entry per IP that ever touched it.
func (rl *rateLimiter) reap() {
	for range time.Tick(5 * time.Minute) {
		now := time.Now()
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if now.After(b.resetsAt) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// middleware returns a gin handler enforcing this limiter. Admins bypass it —
// they're the ones running bulk cleanups.
//
// Takes a catalogue key rather than a message: routes are registered at
// startup, long before any request, so there is no language to translate into
// yet. Resolving the key inside the handler is what makes the 429 speak the
// caller's language.
func (rl *rateLimiter) middleware(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := auth.UserFrom(c); ok && u.IsAdmin() {
			c.Next()
			return
		}
		ok, retry := rl.allow(c.ClientIP())
		if !ok {
			secs := int(retry.Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(secs))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       i18n.T(c, key),
				"retry_after": secs,
			})
			return
		}
		c.Next()
	}
}
