// Package library syncs the Navidrome catalogue into the local tracks table
// (§9.2 of the spec).
package library

import (
	"context"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/navidrome"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/years"
)

const pageSize = 500

// Summary reports the outcome of a sync pass.
type Summary struct {
	Added, Updated, Removed int
	Duration                time.Duration
	Failed                  bool
}

// Syncer pages through the Navidrome library and upserts tracks.
type Syncer struct {
	nav           *navidrome.Client
	st            *store.Store
	log           *slog.Logger
	lastSyncStart atomic.Int64 // unix nanos of the most recent successful sync start
}

// New creates a Syncer.
func New(nav *navidrome.Client, st *store.Store, log *slog.Logger) *Syncer {
	return &Syncer{nav: nav, st: st, log: log}
}

// LastSyncStart returns the start time of the most recent successful sync
// pass, used to filter stale rows from the eligible pool (§7.2). Zero until
// the first successful sync completes.
func (s *Syncer) LastSyncStart() time.Time {
	ns := s.lastSyncStart.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// SyncOnce performs one full library sync pass.
func (s *Syncer) SyncOnce(ctx context.Context) Summary {
	start := time.Now()
	summary := Summary{}

	offset := 0
	for {
		songs, err := s.nav.SearchPage(ctx, offset, pageSize)
		if err != nil {
			s.log.Error("library sync: page fetch failed", "offset", offset, "error", err)
			summary.Failed = true
			summary.Duration = time.Since(start)
			return summary
		}
		if len(songs) == 0 {
			break
		}

		for _, song := range songs {
			var yr *int
			if song.Year > 0 {
				y := song.Year
				yr = &y
			}
			inserted, err := s.st.UpsertTrack(ctx, store.UpsertTrackParams{
				ID:            song.ID,
				Title:         song.Title,
				Artist:        song.Artist,
				ArtistID:      song.ArtistID,
				Album:         song.Album,
				AlbumID:       song.AlbumID,
				NavidromeYear: yr,
				DurationSec:   song.Duration,
				NormTitle:     years.NormalizeTitle(song.Title),
				NormArtist:    years.NormalizeArtist(song.Artist),
				LastSeenAt:    start,
			})
			if err != nil {
				s.log.Error("library sync: upsert failed", "track_id", song.ID, "error", err)
				summary.Failed = true
				continue
			}
			if inserted {
				summary.Added++
			} else {
				summary.Updated++
			}
		}

		if len(songs) < pageSize {
			break
		}
		offset += pageSize
	}

	if !summary.Failed {
		res, err := s.st.DeleteStaleTracks(ctx, start)
		if err != nil {
			s.log.Error("library sync: stale cleanup failed", "error", err)
		} else {
			summary.Removed = int(res.Removed)
		}
		s.lastSyncStart.Store(start.UnixNano())
	}

	summary.Duration = time.Since(start)
	s.log.Info("library sync complete",
		"added", summary.Added, "updated", summary.Updated, "removed", summary.Removed,
		"failed", summary.Failed, "duration_ms", summary.Duration.Milliseconds())
	return summary
}

// RunPeriodic runs an initial sync immediately, then repeats every interval
// with a small random jitter, until ctx is cancelled.
func (s *Syncer) RunPeriodic(ctx context.Context, interval time.Duration) {
	s.SyncOnce(ctx)

	for {
		jitter := time.Duration(rand.Int63n(int64(interval)/10 + 1))
		timer := time.NewTimer(interval + jitter)
		select {
		case <-timer.C:
			s.SyncOnce(ctx)
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}
