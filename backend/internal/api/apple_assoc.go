package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Apple's associated-domains file, which is what makes a passkey usable from
// the Mac app instead of from a web sheet.
//
// The app declares `webcredentials:openimg.io`; the system then fetches this
// document over HTTPS and will only honour the entitlement if the app's
// identifier appears in it. Both halves are required — an entitlement with no
// document, or a document with no entitlement, fails the same way
// (ASAuthorizationError 1004, "the calling process does not have an
// application identifier").
//
// Three things Apple is strict about, all of which the SPA fallback would
// otherwise get wrong:
//
//   - Content-Type must be application/json. Served through the frontend
//     fallback this path has no extension, so it would go out as octet-stream.
//   - No redirects. Not even http→https at this exact path.
//   - No SPA fallback. Returning index.html here answers 200 with HTML, which
//     looks fine to a status-code check and is rejected by the system.
//
// The application identifier is `TEAMID.bundleid`. It is configuration rather
// than a constant because the team is the developer account's, not the
// project's — a fork signs with its own.
const appleAssocPath = "/.well-known/apple-app-site-association"

// registerAppleAssociation mounts the document, and mounts a 404 in its place
// when no application identifier is configured.
//
// The route is registered either way, and that is the whole point. Skipping it
// when unconfigured looks like the careful choice — no document is better than
// a placeholder one, since the system caches what it fetches — but the path
// then falls through to the SPA handler, which answers 200 with index.html.
// That is worse than both: Apple fetches it, gets HTML where it wanted JSON,
// and caches *that*. Measured on the live site before this was fixed:
//
//	HTTP/2 200
//	content-type: text/html; charset=utf-8
//
// An explicit 404 is the honest answer for "no Mac app is configured here".
func (s *Server) registerAppleAssociation(r *gin.Engine) {
	appID := strings.TrimSpace(s.AppleAppID)
	r.GET(appleAssocPath, func(c *gin.Context) {
		if appID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.JSON(http.StatusOK, gin.H{
			"webcredentials": gin.H{
				"apps": []string{appID},
			},
		})
	})
}
