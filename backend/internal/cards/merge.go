package cards

import (
	"sort"
	"strings"
)

// ScanSummary reports how a scan changed the collection.
type ScanSummary struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Total   int `json:"total"`
}

// Merge combines the cards from the existing file with the songs a scan found
// (whose hitsyncyear/excluded fields are ignored). Songs no longer in the
// library are dropped, new songs get hitsyncyear null and excluded false, and
// every surviving card takes the scanned Navidrome fields while keeping its
// own hitsyncyear and excluded. The result is sorted by artist, album, and
// title so the file stays easy to browse and diff.
func Merge(existing, scanned []Card) ([]Card, ScanSummary) {
	byID := make(map[string]Card, len(existing))
	for _, c := range existing {
		byID[c.NavidromeID] = c
	}

	var summary ScanSummary
	merged := make([]Card, 0, len(scanned))
	seen := make(map[string]bool, len(scanned))
	for _, s := range scanned {
		if seen[s.NavidromeID] {
			continue // a song can repeat across pages if the library changes mid-scan
		}
		seen[s.NavidromeID] = true

		card := s
		card.HitsyncYear = nil
		card.Excluded = false
		if old, ok := byID[s.NavidromeID]; ok {
			card.HitsyncYear = old.HitsyncYear
			card.Excluded = old.Excluded
			if !sameLibraryFields(old, card) {
				summary.Updated++
			}
		} else {
			summary.Added++
		}
		merged = append(merged, card)
	}
	for id := range byID {
		if !seen[id] {
			summary.Removed++
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		a, b := merged[i], merged[j]
		for _, pair := range [][2]string{{a.Artist, b.Artist}, {a.Album, b.Album}, {a.Title, b.Title}} {
			if x, y := strings.ToLower(pair[0]), strings.ToLower(pair[1]); x != y {
				return x < y
			}
		}
		return a.NavidromeID < b.NavidromeID
	})
	summary.Total = len(merged)
	return merged, summary
}

func sameLibraryFields(a, b Card) bool {
	return a.Title == b.Title && a.Artist == b.Artist && a.Album == b.Album &&
		a.DurationSec == b.DurationSec && equalYear(a.Year, b.Year)
}

func equalYear(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
