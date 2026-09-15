package gamesvc

import (
	"context"
	"time"
)

// usedAndPendingIDs returns every track id that must not be redrawn: used
// this game, plus ones already resolved and waiting in the pipeline.
func (mg *ManagedGame) usedAndPendingIDs() []string {
	ids := make([]string, 0, len(mg.g.UsedTrackIDs)+len(mg.candidates))
	for id := range mg.g.UsedTrackIDs {
		ids = append(ids, id)
	}
	for _, c := range mg.candidates {
		ids = append(ids, c.Card.TrackID)
	}
	return ids
}

// ensureLookahead tops up the resolved-candidate pipeline in the background
// so resolution for turn n+1 runs during turn n (§9.5).
func (mg *ManagedGame) ensureLookahead() {
	need := mg.cfg.YearLookaheadDepth - len(mg.candidates) - mg.resolving
	for i := 0; i < need; i++ {
		mg.resolving++
		exclude := mg.usedAndPendingIDs()
		go func() {
			card, err := mg.trackSource.DrawAndResolve(context.Background(), context.Background(), exclude)
			mg.enqueue(func() {
				mg.resolving--
				if err == nil && card != nil {
					mg.candidates = append(mg.candidates, card)
					mg.preloadCandidate(card)
				}
			})
		}()
	}
}

// preloadCandidate overlaps Navidrome download for an upcoming turn with the
// current turn. A cache hit makes its later PREPARING phase only a LiveKit
// publication setup instead of a full source download.
func (mg *ManagedGame) preloadCandidate(candidate *Candidate) {
	if candidate == nil {
		return
	}
	token, err := mg.mediaSigner.Issue(mg.id, candidate.Card.TrackID, mg.cfg.MediaTTL)
	if err != nil {
		mg.log.Warn("failed to authorise audio preload", "game_id", mg.id, "track_id", candidate.Card.TrackID, "error", err)
		return
	}
	go func(trackID, token string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := mg.broadcaster.Preload(ctx, mg.id, trackID, token); err != nil {
			mg.log.Warn("failed to preload audio source", "game_id", mg.id, "track_id", trackID, "error", err)
		}
	}(candidate.Card.TrackID, token)
}

// nextCandidate takes the head of the resolved pipeline, or falls back to a
// synchronous draw bounded by YEAR_LOOKUP_TIMEOUT (§9.5 point 4). Must be
// called from the game's own goroutine; the fallback path blocks it briefly
// while waiting on the result channel, which is acceptable since it only
// happens at game start or an unusually fast turn.
func (mg *ManagedGame) nextCandidate() (*Candidate, error) {
	if len(mg.candidates) > 0 {
		c := mg.candidates[0]
		mg.candidates = mg.candidates[1:]
		mg.ensureLookahead()
		return c, nil
	}

	exclude := mg.usedAndPendingIDs()
	type result struct {
		card *Candidate
		err  error
	}
	resultCh := make(chan result, 1)
	mbCtx, cancel := context.WithTimeout(context.Background(), mg.cfg.YearLookupTimeout)
	defer cancel()

	go func() {
		card, err := mg.trackSource.DrawAndResolve(context.Background(), mbCtx, exclude)
		resultCh <- result{card, err}
	}()

	select {
	case r := <-resultCh:
		mg.ensureLookahead()
		return r.card, r.err
	case <-mbCtx.Done():
		// The in-flight draw raced against excludeIDs that this turn is
		// about to invalidate, so its eventual result is discarded rather
		// than risking the same track being used twice. A fresh draw with
		// the now-expired mbCtx proceeds immediately, falling back to the
		// Navidrome year alone (resolveTrack treats the expired context as
		// "MusicBrainz absent") so the turn is never blocked further; the
		// database query itself still gets an unbounded context.
		go func() { <-resultCh }()
		return mg.trackSource.DrawAndResolve(context.Background(), mbCtx, exclude)
	}
}
