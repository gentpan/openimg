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

// registerAppleAssociation mounts the document, or nothing when no application
// identifier is configured.
//
// Nothing, deliberately: an empty or placeholder document is worse than a 404,
// because the system caches what it fetches and a wrong answer sticks around
// after the real one is deployed.
func (s *Server) registerAppleAssociation(r *gin.Engine) {
	appID := strings.TrimSpace(s.AppleAppID)
	if appID == "" {
		return
	}
	r.GET(appleAssocPath, func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		c.JSON(http.StatusOK, gin.H{
			"webcredentials": gin.H{
				"apps": []string{appID},
			},
		})
	})
}
