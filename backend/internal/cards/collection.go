package cards

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/navidrome"
)

const scanPageSize = 500

// ErrScanInProgress is returned when a scan is requested while one runs.
var ErrScanInProgress = errors.New("a library scan is already running")

// Library is the part of the Navidrome client a scan needs.
type Library interface {
	SearchPage(ctx context.Context, songOffset, songCount int) ([]navidrome.Song, error)
}

// Stats summarises the collection for the admin console.
type Stats struct {
	Total         int
	Playable      int
	Excluded      int
	MissingYear   int
	LastScan      time.Time // zero until a scan succeeds
	LastScanError string
	Scanning      bool
}

// Collection is the in-memory copy of cards.json that games draw from. It is
// replaced as a whole after each scan; hand edits to the file take effect on
// the next scan.
type Collection struct {
	path           string
	lib            Library
	minDur, maxDur int
	log            *slog.Logger

	scanMu sync.Mutex

	mu            sync.RWMutex
	all           []Card
	playable      []Card
	playableIDs   map[string]bool
	loaded        bool
	lastScan      time.Time
	lastScanError string
	scanning      bool
}

// NewCollection creates an empty collection backed by the file at path.
// Songs outside [minDur, maxDur] are kept in the file but never played.
func NewCollection(path string, lib Library, minDur, maxDur time.Duration, log *slog.Logger) *Collection {
	return &Collection{
		path: path, lib: lib, log: log,
		minDur: int(minDur.Seconds()), maxDur: int(maxDur.Seconds()),
		playableIDs: map[string]bool{},
	}
}

// Scan reads the whole Navidrome library, merges it into cards.json, writes
// the file, and makes the result the active collection. If Navidrome cannot
// be read and nothing is loaded yet, the existing file is used as it is.
func (c *Collection) Scan(ctx context.Context) (ScanSummary, error) {
	if !c.scanMu.TryLock() {
		return ScanSummary{}, ErrScanInProgress
	}
	defer c.scanMu.Unlock()
	c.setScanning(true)
	defer c.setScanning(false)

	started := time.Now()
	summary, err := c.scan(ctx)
	if err != nil {
		c.log.Error("card scan failed", "path", c.path, "error", err)
		c.mu.Lock()
		c.lastScanError = err.Error()
		c.mu.Unlock()
		return summary, err
	}
	c.log.Info("card scan complete", "path", c.path, "added", summary.Added, "updated", summary.Updated,
		"removed", summary.Removed, "total", summary.Total, "duration_ms", time.Since(started).Milliseconds())
	return summary, nil
}

func (c *Collection) scan(ctx context.Context) (ScanSummary, error) {
	scanned, scanErr := c.fetchLibrary(ctx)

	// Read the file only now, after the slow Navidrome paging, so hand edits
	// made while the scan ran are not overwritten.
	existing, err := ReadFile(c.path)
	if err != nil {
		return ScanSummary{}, err
	}
	if scanErr != nil {
		if !c.isLoaded() && existing != nil {
			c.set(existing)
			c.log.Warn("using existing card file without a scan", "path", c.path, "cards", len(existing))
		}
		return ScanSummary{}, fmt.Errorf("reading the Navidrome library: %w", scanErr)
	}
	// An empty answer is far more likely a Navidrome problem than a library
	// that was really emptied; merging it would wipe every hand edit.
	if len(scanned) == 0 && len(existing) > 0 {
		if !c.isLoaded() {
			c.set(existing)
		}
		return ScanSummary{}, fmt.Errorf("navidrome returned no songs; refusing to remove all %d cards", len(existing))
	}

	merged, summary := Merge(existing, scanned)
	if err := WriteFile(c.path, merged); err != nil {
		return summary, fmt.Errorf("writing %s: %w", c.path, err)
	}
	c.set(merged)
	c.mu.Lock()
	c.lastScan = time.Now()
	c.lastScanError = ""
	c.mu.Unlock()
	return summary, nil
}

func (c *Collection) fetchLibrary(ctx context.Context) ([]Card, error) {
	var out []Card
	for offset := 0; ; offset += scanPageSize {
		songs, err := c.lib.SearchPage(ctx, offset, scanPageSize)
		if err != nil {
			return nil, err
		}
		for _, s := range songs {
			if s.ID == "" {
				continue
			}
			card := Card{NavidromeID: s.ID, Title: s.Title, Artist: s.Artist, Album: s.Album, DurationSec: s.Duration}
			if s.Year > 0 {
				year := s.Year
				card.Year = &year
			}
			out = append(out, card)
		}
		if len(songs) < scanPageSize {
			return out, nil
		}
	}
}

func (c *Collection) set(all []Card) {
	playable := make([]Card, 0, len(all))
	ids := make(map[string]bool, len(all))
	for _, card := range all {
		if c.isPlayable(card) {
			playable = append(playable, card)
			ids[card.NavidromeID] = true
		}
	}
	c.mu.Lock()
	c.all, c.playable, c.playableIDs, c.loaded = all, playable, ids, true
	c.mu.Unlock()
}

func (c *Collection) isPlayable(card Card) bool {
	_, _, hasYear := card.GameYear()
	return !card.Excluded && hasYear && card.DurationSec >= c.minDur && card.DurationSec <= c.maxDur
}

func (c *Collection) isLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

func (c *Collection) setScanning(scanning bool) {
	c.mu.Lock()
	c.scanning = scanning
	c.mu.Unlock()
}

// Draw returns a random playable card whose id is not in exclude.
func (c *Collection) Draw(exclude map[string]bool) (Card, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Reservoir sampling: one pass, no allocation.
	var picked Card
	n := 0
	for _, card := range c.playable {
		if exclude[card.NavidromeID] {
			continue
		}
		n++
		if rand.Intn(n) == 0 {
			picked = card
		}
	}
	return picked, n > 0
}

// IsPlayable reports whether a card is still in the playable pool, e.g.
// after a scan picked up a hand edit that excluded it.
func (c *Collection) IsPlayable(navidromeID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.playableIDs[navidromeID]
}

// PlayableCount returns the size of the playable pool.
func (c *Collection) PlayableCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.playable)
}

// Stats returns counts and scan status.
func (c *Collection) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s := Stats{
		Total: len(c.all), Playable: len(c.playable),
		LastScan: c.lastScan, LastScanError: c.lastScanError, Scanning: c.scanning,
	}
	for _, card := range c.all {
		if card.Excluded {
			s.Excluded++
		}
		if _, _, ok := card.GameYear(); !ok {
			s.MissingYear++
		}
	}
	return s
}
