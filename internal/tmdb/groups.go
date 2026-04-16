package tmdb

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SortListItems returns a copy of items sorted by release/air date ascending.
func SortListItems(items []ListItem) []ListItem {
	sorted := make([]ListItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EffectiveDate() < sorted[j].EffectiveDate()
	})
	return sorted
}

var groupSeasonNameRe = regexp.MustCompile(`(?i)season\s*(\d+)`)

// GroupSeasonNumber infers the season number from an episode group season name
// and its positional order. Names like "Season 1" map to 1; names containing
// "special", "extra", "ova", or "bonus" map to 0 (specials).
func GroupSeasonNumber(name string, order int) int {
	if m := groupSeasonNameRe.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "special") || strings.Contains(lower, "extra") ||
		strings.Contains(lower, "ova") || strings.Contains(lower, "bonus") {
		return 0
	}
	return order + 1
}

// BuildGroupMap converts an EpisodeGroup into a map keyed by [season, episode]
// for fast lookup during pull.
func BuildGroupMap(group *EpisodeGroup) map[[2]int]*Episode {
	m := make(map[[2]int]*Episode)
	for _, gs := range group.Groups {
		seasonNum := GroupSeasonNumber(gs.Name, gs.Order)
		for _, gep := range gs.Episodes {
			epNum := gep.Order + 1
			ep := &Episode{
				ID:            gep.ID,
				Name:          gep.Name,
				Overview:      gep.Overview,
				SeasonNumber:  seasonNum,
				EpisodeNumber: epNum,
				AirDate:       gep.AirDate,
				StillPath:     gep.StillPath,
				VoteAverage:   gep.VoteAverage,
				VoteCount:     gep.VoteCount,
				Runtime:       gep.Runtime,
			}
			m[[2]int{seasonNum, epNum}] = ep
		}
	}
	return m
}
