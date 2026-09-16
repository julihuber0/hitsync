// Package cards maintains the song card collection: a JSON file generated
// from the Navidrome library, which an admin edits by hand to correct a
// song's year (hitsyncyear) or keep it out of games (excluded). Scans refresh
// the Navidrome-provided fields and never touch the hand-edited ones.
package cards

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Year sources shown on the reveal card.
const (
	SourceNavidrome   = "Navidrome"
	SourceHitsyncYear = "hitsyncyear"
)

// Card is one song in cards.json.
type Card struct {
	// Filled from Navidrome on every scan.
	NavidromeID string `json:"navidromeId"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	Year        *int   `json:"year"` // null when Navidrome has no year
	DurationSec int    `json:"durationSec"`

	// Edited by hand; preserved across scans.
	HitsyncYear *int `json:"hitsyncyear"` // overrides Year when set
	Excluded    bool `json:"excluded"`
}

// GameYear returns the year the game uses: hitsyncyear if set, otherwise the
// Navidrome year. ok is false when neither is available.
func (c Card) GameYear() (year int, source string, ok bool) {
	if c.HitsyncYear != nil {
		return *c.HitsyncYear, SourceHitsyncYear, true
	}
	if c.Year != nil {
		return *c.Year, SourceNavidrome, true
	}
	return 0, "", false
}

// ReadFile loads cards.json. A missing file yields (nil, nil). Decoding is
// strict: an unknown field (such as a misspelled "hitsync_year") or a
// duplicate id is an error rather than something a later write would
// silently drop.
func ReadFile(path string) ([]Card, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cards []Card
	if err := dec.Decode(&cards); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	seen := make(map[string]bool, len(cards))
	for i, c := range cards {
		if c.NavidromeID == "" {
			return nil, fmt.Errorf("parsing %s: card %d has no navidromeId", path, i)
		}
		if seen[c.NavidromeID] {
			return nil, fmt.Errorf("parsing %s: navidromeId %q appears more than once", path, c.NavidromeID)
		}
		seen[c.NavidromeID] = true
	}
	return cards, nil
}

// WriteFile replaces cards.json atomically, so a crash mid-write never leaves
// a truncated file behind.
func WriteFile(path string, cards []Card) error {
	if cards == nil {
		cards = []Card{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cards); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".cards-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
