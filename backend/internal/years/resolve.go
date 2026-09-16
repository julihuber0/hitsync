package years

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

// Source labels shown on the reveal card (§9.4).
const (
	SourceDiscogs = "Discogs"
	SourceLibrary = "Library"
	SourceBoth    = "Both"
	SourceManual  = "Manual"
)

// DiscogsResolver is the subset of *discogs.Client used here, so tests can
// inject a fake without hitting the network.
type DiscogsResolver interface {
	Resolve(ctx context.Context, title, artist string) (*DiscogsResult, error)
}

// DiscogsResult mirrors discogs.Result to avoid an import cycle; the
// concrete client returns a compatible type.
type DiscogsResult struct {
	Year   int
	Source string
}

// Resolver orchestrates on-demand Discogs year lookups with an in-memory
// cache and per-key request de-duplication (§9.5).
type Resolver struct {
	discogs DiscogsResolver
	enabled bool
	cache   *lruCache
	sf      singleflight.Group
}

// NewResolver creates a Resolver. If enabled is false, Discogs is never
// queried and Navidrome years are used exclusively.
func NewResolver(discogs DiscogsResolver, enabled bool, cacheEntries int, cacheTTL time.Duration) *Resolver {
	return &Resolver{
		discogs: discogs,
		enabled: enabled,
		cache:   newLRUCache(cacheEntries, cacheTTL),
	}
}

// ResolveDiscogsYear returns the Discogs year for a (title, artist) pair,
// using the in-memory cache (including negative results) and collapsing
// concurrent identical lookups into one upstream call.
func (r *Resolver) ResolveDiscogsYear(ctx context.Context, title, artist string) (year int, found bool, err error) {
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
		res, err := r.discogs.Resolve(ctx, title, artist)
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
// navidromeYear and discogsYear are nil when absent. maxBackdate is
// YEAR_MAX_BACKDATE (0 disables the guard).
func Combine(overrideYear, navidromeYear, discogsYear *int, maxBackdate int) (year int, source string, ok bool) {
	if overrideYear != nil {
		return *overrideYear, SourceManual, true
	}

	nd, dg := navidromeYear, discogsYear

	if maxBackdate > 0 && nd != nil && dg != nil && (*nd-*dg) > maxBackdate {
		dg = nil
	}

	switch {
	case nd != nil && dg != nil:
		switch {
		case *nd == *dg:
			return *nd, SourceBoth, true
		case *dg < *nd:
			return *dg, SourceDiscogs, true
		default:
			return *nd, SourceLibrary, true
		}
	case nd != nil:
		return *nd, SourceLibrary, true
	case dg != nil:
		return *dg, SourceDiscogs, true
	default:
		return 0, "", false
	}
}
