package api

import (
	"net/http"
	"strings"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/referral"
	"github.com/gin-gonic/gin"
)

// GET /api/referral/me — summary for the signed-in user's "Refer & earn" page.
// Lazily ensures a code exists if the user predates the feature.
func (s *Server) handleReferralMe(c *gin.Context) {
	u := auth.MustUser(c)
	_ = referral.EnsureCode(s.DB, u)
	sum, err := referral.SummaryFor(s.DB, u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sum)
}

// GET /api/referral/list — masked list of users this user invited.
func (s *Server) handleReferralList(c *gin.Context) {
	u := auth.MustUser(c)
	rows, err := referral.ListInvitees(s.DB, u.ID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invitees": rows})
}

// GET /api/referral/lookup?code=XXX — public; returns whether the code is
// valid and (if so) the referrer's display name. Used by the landing /
// register flow to confirm "you were invited by Alice — sign up to get N".
func (s *Server) handleReferralLookup(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Query("code")))
	if code == "" {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}
	ref, err := referral.LookupReferrer(s.DB, code)
	if err != nil || ref == nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}
	display := ref.Name
	if display == "" {
		display = referral.MaskEmail(ref.Email)
	}
	c.JSON(http.StatusOK, gin.H{
		"valid":         true,
		"referrer_name": display,
		"bonus_bytes":   s.groupFor(ref).ReferralSpace,
	})
}
