package gamesvc

// Each turn's card is drawn one turn ahead, so clients can download its
// track while the current turn is still playing (see audio.go).

// excludedTrackIDs returns every track id that must not be drawn: used this
// game, plus the upcoming card.
func (mg *ManagedGame) excludedTrackIDs() map[string]bool {
	ids := make(map[string]bool, len(mg.g.UsedTrackIDs)+1)
	for id := range mg.g.UsedTrackIDs {
		ids[id] = true
	}
	if mg.upcoming != nil {
		ids[mg.upcoming.Card.TrackID] = true
	}
	return ids
}

// takeCandidate returns the card for the turn that is beginning: the
// upcoming card clients have preloaded, unless a library scan made it
// unplayable in the meantime, in which case a fresh one is drawn.
func (mg *ManagedGame) takeCandidate() (*Candidate, error) {
	if up := mg.upcoming; up != nil {
		mg.upcoming = nil
		if mg.trackSource.IsPlayable(up.Card.TrackID) {
			return up, nil
		}
	}
	return mg.trackSource.Draw(mg.excludedTrackIDs())
}

// drawUpcoming draws the next turn's card, has the server transcode its
// track, and tells clients to preload it. With the pool exhausted there is
// no upcoming card, and the next turn ends the game.
func (mg *ManagedGame) drawUpcoming() {
	mg.upcoming = nil
	cand, err := mg.trackSource.Draw(mg.excludedTrackIDs())
	if err != nil {
		return
	}
	mg.upcoming = cand
	mg.trackWarmer.Warm(cand.Card.TrackID)
	mg.announceUpcoming()
}
