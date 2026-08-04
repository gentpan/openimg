package api

import (
	"net/http"
	"sort"

	"github.com/gentpan/openimg/backend/internal/auth"
	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GET /api/storage/summary — where the user's space actually went.
//
// Aggregated in the database rather than by summing the rows the dashboard
// happens to have loaded. The panels used to compute their totals from the 12
// most recent images, which meant "你省了 24 MB" was really "你最近 12 张省了
// 24 MB" — a number that moves for reasons the user cannot see.

type storageSummary struct {
	Images int64 `json:"images"`
	// Orig is what the user handed us; Stored is what we keep. The difference
	// is the whole point of the service, so it gets its own panel.
	Orig   int64 `json:"size_orig"`
	Stored int64 `json:"size_stored"`
	// Breakdown of Stored, demoted to a detail line.
	Primary  int64 `json:"size_primary"`
	Variants int64 `json:"size_variants"`
	Thumbs   int64 `json:"size_thumbs"`
	// Unclassified is Stored minus the three above. Images uploaded before the
	// breakdown columns existed carry zeros in them, so the parts do not add up
	// to the whole for historical rows. Reporting the gap explicitly beats
	// silently drawing a chart whose segments don't reach 100%.
	Unclassified int64 `json:"size_unclassified"`

	ByProfile []profileUsage `json:"by_profile"`
	ByFormat  []formatUsage  `json:"by_format"`
}

type profileUsage struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Kind   string    `json:"kind"`
	Bytes  int64     `json:"bytes"`
	Images int64     `json:"images"`
}

type formatUsage struct {
	Ext    string `json:"ext"`
	Bytes  int64  `json:"bytes"`
	Images int64  `json:"images"`
}

func (s *Server) handleStorageSummary(c *gin.Context) {
	u := auth.MustUser(c)
	out := storageSummary{ByProfile: []profileUsage{}, ByFormat: []formatUsage{}}

	// A fresh query per aggregate: reusing one *gorm.DB across Select calls
	// carries the previous statement's clauses into the next one.
	live := func() *gorm.DB {
		return s.DB.Model(&models.Image{}).Where("user_id = ? AND deleted_at IS NULL", u.ID)
	}

	var totals struct {
		Images                              int64
		Orig, Stored, Prim, Vars, ThumbsSum int64
	}
	live().Select(
		"COUNT(*) AS images," +
			"COALESCE(SUM(size_orig),0) AS orig," +
			"COALESCE(SUM(size_stored),0) AS stored," +
			"COALESCE(SUM(size_primary),0) AS prim," +
			"COALESCE(SUM(size_variants),0) AS vars," +
			"COALESCE(SUM(size_thumbs),0) AS thumbs_sum").Scan(&totals)

	out.Images, out.Orig, out.Stored = totals.Images, totals.Orig, totals.Stored
	out.Primary, out.Variants, out.Thumbs = totals.Prim, totals.Vars, totals.ThumbsSum
	if gap := out.Stored - (out.Primary + out.Variants + out.Thumbs); gap > 0 {
		out.Unclassified = gap
	}

	// Per storage location. Profile names come from a second lookup rather than
	// a join so a row whose profile has been deleted still reports its bytes
	// instead of vanishing from a total the user can see elsewhere.
	var perProfile []struct {
		ProfileID uuid.UUID
		Bytes     int64
		Images    int64
	}
	live().
		Select("profile_id, COALESCE(SUM(size_stored),0) AS bytes, COUNT(*) AS images").
		Group("profile_id").Order("bytes DESC").Scan(&perProfile)

	if len(perProfile) > 0 {
		ids := make([]uuid.UUID, 0, len(perProfile))
		for _, r := range perProfile {
			ids = append(ids, r.ProfileID)
		}
		var profiles []models.StorageProfile
		s.DB.Where("id IN ?", ids).Find(&profiles)
		byID := map[uuid.UUID]models.StorageProfile{}
		for _, p := range profiles {
			byID[p.ID] = p
		}
		for _, r := range perProfile {
			p, ok := byID[r.ProfileID]
			name, kind := "已移除的存储", "unknown"
			switch {
			case ok:
				name, kind = p.Name, string(p.Kind)
				if p.IsPlatform() {
					name = "平台空间"
				}
			case r.ProfileID == uuid.Nil:
				// No platform profile row yet — local disk in development, and
				// the first uploads on a fresh install. Still the platform.
				name, kind = "平台空间", string(models.ProfilePlatform)
			}
			out.ByProfile = append(out.ByProfile, profileUsage{
				ID: r.ProfileID, Name: name, Kind: kind, Bytes: r.Bytes, Images: r.Images,
			})
		}
	}

	var perFormat []formatUsage
	live().
		Select("ext, COALESCE(SUM(size_stored),0) AS bytes, COUNT(*) AS images").
		Group("ext").Order("bytes DESC").Scan(&perFormat)
	// Fold the aliases: rows predating the extension normalisation still say
	// "jpeg", and showing jpg and jpeg as two formats is just confusing.
	merged := map[string]*formatUsage{}
	order := []string{}
	for _, f := range perFormat {
		key := models.CanonFormat(f.Ext)
		if m, ok := merged[key]; ok {
			m.Bytes += f.Bytes
			m.Images += f.Images
			continue
		}
		merged[key] = &formatUsage{Ext: key, Bytes: f.Bytes, Images: f.Images}
		order = append(order, key)
	}
	sort.SliceStable(order, func(i, j int) bool { return merged[order[i]].Bytes > merged[order[j]].Bytes })
	for i, k := range order {
		if i >= 8 {
			break
		}
		out.ByFormat = append(out.ByFormat, *merged[k])
	}

	c.JSON(http.StatusOK, out)
}
