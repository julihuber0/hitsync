package years

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

// Source labels shown on the reveal card (§9.4).
const (
	SourceMusicBrainz = "MusicBrainz"
	SourceLibrary     = "Library"
	SourceBoth        = "Both"
	SourceManual      = "Manual"
)

// MBResolver is the subset of *musicbrainz.Client used here, so tests can
// inject a fake without hitting the network.
type MBResolver interface {
	Resolve(ctx context.Context, title, artist string) (*MBResult, error)
}

// MBResult mirrors musicbrainz.Result to avoid an import cycle; the concrete
// client returns a compatible type.
type MBResult struct {
	Year   int
	Source string
}

// Resolver orchestrates on-demand MusicBrainz year lookups with an in-memory
// cache and per-key request de-duplication (§9.5).
type Resolver struct {
	mb      MBResolver
	enabled bool
	cache   *lruCache
	sf      singleflight.Group
}

// NewResolver creates a Resolver. If enabled is false, MusicBrainz is never
// queried and Navidrome years are used exclusively.
func NewResolver(mb MBResolver, enabled bool, cacheEntries int, cacheTTL time.Duration) *Resolver {
	return &Resolver{
		mb:      mb,
		enabled: enabled,
		cache:   newLRUCache(cacheEntries, cacheTTL),
	}
}

// ResolveMusicBrainzYear returns the MusicBrainz year for a (title, artist)
// pair, using the in-memory cache (including negative results) and
// collapsing concurrent identical lookups into one upstream call.
func (r *Resolver) ResolveMusicBrainzYear(ctx context.Context, title, artist string) (year int, found bool, err error) {
	if !r.enabled {
		return 0, false, nil
	}

	key := cacheKey(title, artist)
	if entry, ok := r.cache.get(key); ok {
		return entry.year, entry.found, nil
	}

	v, err, _ := r.sf.Do(key, func() (interface{}, error) {
		if entry, ok := r.cache.get(key); ok {
			return entry, nil
		}
		res, err := r.mb.Resolve(ctx, title, artist)
		if err != nil {
			return nil, err
		}
		entry := cacheEntry{key: key}
		if res != nil {
			entry.year = res.Year
			entry.found = true
			entry.source = res.Source
		}
		r.cache.set(key, entry.year, entry.found, entry.source)
		return entry, nil
	})
	if err != nil {
		return 0, false, err
	}
	entry := v.(cacheEntry)
	return entry.year, entry.found, nil
}

// CacheLen exposes the current cache size for admin stats.
func (r *Resolver) CacheLen() int {
	return r.cache.Len()
}

// Combine implements the earliest-wins combiner from §9.4. overrideYear,
// navidromeYear and mbYear are nil when absent. maxBackdate is
// YEAR_MAX_BACKDATE (0 disables the guard).
func Combine(overrideYear, navidromeYear, mbYear *int, maxBackdate int) (year int, source string, ok bool) {
	if overrideYear != nil {
		return *overrideYear, SourceManual, true
	}

	nd, mb := navidromeYear, mbYear

	if maxBackdate > 0 && nd != nil && mb != nil && (*nd-*mb) > maxBackdate {
		mb = nil
	}

	switch {
	case nd != nil && mb != nil:
		switch {
		case *nd == *mb:
			return *nd, SourceBoth, true
		case *mb < *nd:
			return *mb, SourceMusicBrainz, true
		default:
			return *nd, SourceLibrary, true
		}
	case nd != nil:
		return *nd, SourceLibrary, true
	case mb != nil:
		return *mb, SourceMusicBrainz, true
	default:
		return 0, "", false
	}
}
