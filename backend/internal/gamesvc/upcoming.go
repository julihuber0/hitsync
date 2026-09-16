package gamesvc

import (
	"context"
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
				if err != nil || card == nil {
					return
				}
				// A synchronous fallback draw in nextCandidate may have picked
				// the same track while this one was resolving.
				if mg.g.UsedTrackIDs[card.Card.TrackID] || mg.hasCandidate(card.Card.TrackID) {
					mg.ensureLookahead()
					return
				}
				mg.candidates = append(mg.candidates, card)
				mg.trackWarmer.Warm(card.Card.TrackID)
				mg.announceNextTrack()
			})
		}()
	}
}

func (mg *ManagedGame) hasCandidate(trackID string) bool {
	for _, c := range mg.candidates {
		if c.Card.TrackID == trackID {
			return true
		}
	}
	return false
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
