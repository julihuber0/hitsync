package gamesvc

import (
	"context"
	"errors"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/years"
)

// ErrPoolExhausted is returned when no eligible track remains for a game.
var ErrPoolExhausted = errors.New("no eligible tracks remain")

// Candidate pairs a resolved game.Card with the track duration needed for
// playback (§10.4), which is not part of game.Card's persisted shape (§8.1).
type Candidate struct {
	Card        game.Card
	DurationSec int
	YearSource  string
}

// LibraryFreshness reports the timestamp of the most recent successful
// library sync, used to filter out stale rows (§7.2).
type LibraryFreshness interface {
	LastSyncStart() time.Time
}

// TrackSource draws and resolves candidate tracks for gameplay (§9.5).
type TrackSource struct {
	st          *store.Store
	resolver    *years.Resolver
	freshness   LibraryFreshness
	minDurSec   int
	maxDurSec   int
	maxBackdate int
}

// NewTrackSource creates a TrackSource.
func NewTrackSource(st *store.Store, resolver *years.Resolver, freshness LibraryFreshness, minDur, maxDur time.Duration, maxBackdate int) *TrackSource {
	return &TrackSource{
		st:          st,
		resolver:    resolver,
		freshness:   freshness,
		minDurSec:   int(minDur.Seconds()),
		maxDurSec:   int(maxDur.Seconds()),
		maxBackdate: maxBackdate,
	}
}

// DrawAndResolve draws a random eligible track not in excludeIDs and
// resolves its gameplay year, retrying up to 5 times if a candidate has no
// year from either source (§9.5 point 5). dbCtx bounds the (fast) database
// query; mbCtx bounds the MusicBrainz lookup specifically — pass an
// already-expired context to force an immediate fall back to the Navidrome
// year alone (§9.5 point 4) without also aborting the database query.
func (ts *TrackSource) DrawAndResolve(dbCtx, mbCtx context.Context, excludeIDs []string) (*Candidate, error) {
	exclude := append([]string(nil), excludeIDs...)

	for attempt := 0; attempt < 5; attempt++ {
		tracks, err := ts.st.RandomEligibleTracks(dbCtx, exclude, ts.minDurSec, ts.maxDurSec, ts.freshness.LastSyncStart(), 1)
		if err != nil {
			return nil, err
		}
		if len(tracks) == 0 {
			return nil, ErrPoolExhausted
		}
		tr := tracks[0]
		exclude = append(exclude, tr.ID)

		cand, ok, err := ts.resolveTrack(dbCtx, mbCtx, tr)
		if err != nil {
			return nil, err
		}
		if ok {
			return cand, nil
		}
		// No year from either source: discard and draw another (§7.2, §9.5.5).
	}
	return nil, ErrPoolExhausted
}

func (ts *TrackSource) resolveTrack(dbCtx, mbCtx context.Context, tr store.Track) (*Candidate, bool, error) {
	var overrideYear *int
	override, err := ts.st.GetYearOverride(dbCtx, tr.ID)
	if err != nil {
		return nil, false, err
	}
	if override != nil {
		overrideYear = &override.Year
	}

	var mbYearPtr *int
	mbYear, mbFound, err := ts.resolver.ResolveMusicBrainzYear(mbCtx, tr.Title, tr.Artist)
	if err == nil && mbFound {
		mbYearPtr = &mbYear
	}
	// A resolution error (including an expired mbCtx) is treated as
	// "MusicBrainz absent" rather than a hard failure.

	year, source, ok := years.Combine(overrideYear, tr.NavidromeYear, mbYearPtr, ts.maxBackdate)
	if !ok {
		return nil, false, nil
	}
	card := game.Card{TrackID: tr.ID, Title: tr.Title, Artist: tr.Artist, Year: year}
	return &Candidate{Card: card, DurationSec: tr.DurationSec, YearSource: source}, true, nil
}

// SongGuessOptions builds the decoy sets for the "Name that tune" panel
// (§8.6): three random decoys per group, distinct from the correct value.
func (ts *TrackSource) SongGuessOptions(ctx context.Context, card *game.Card) (titles, artists []string, err error) {
	decoyTitles, err := ts.st.RandomTrackTitles(ctx, card.TrackID, 6)
	if err != nil {
		return nil, nil, err
	}
	decoyArtists, err := ts.st.RandomArtistNames(ctx, card.Artist, 6)
	if err != nil {
		return nil, nil, err
	}

	titles = append([]string{card.Title}, dedupeExcept(decoyTitles, card.Title, 3)...)
	artists = append([]string{card.Artist}, dedupeExcept(decoyArtists, card.Artist, 3)...)
	shuffleStrings(titles)
	shuffleStrings(artists)
	return titles, artists, nil
}

func dedupeExcept(candidates []string, exclude string, n int) []string {
	seen := map[string]bool{exclude: true}
	out := make([]string, 0, n)
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
		if len(out) == n {
			break
		}
	}
	return out
}
