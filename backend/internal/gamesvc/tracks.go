package gamesvc

import (
	"errors"

	"github.com/julianhuber/hitsync/backend/internal/cards"
	"github.com/julianhuber/hitsync/backend/internal/game"
)

// ErrPoolExhausted is returned when no playable card remains for a game.
var ErrPoolExhausted = errors.New("no playable songs remain")

// ErrNoCards is returned when a game is started before the card collection
// holds enough playable songs, e.g. while the first library scan is running.
var ErrNoCards = errors.New("the song library is not ready or has too few playable songs")

// Candidate pairs a drawn game.Card with the track duration needed for
// playback, which is not part of game.Card's persisted shape (§8.1).
type Candidate struct {
	Card        game.Card
	DurationSec int
	YearSource  string
}

// TrackSource draws cards for gameplay from the card collection.
type TrackSource struct {
	cards *cards.Collection
}

// NewTrackSource creates a TrackSource.
func NewTrackSource(c *cards.Collection) *TrackSource {
	return &TrackSource{cards: c}
}

// Draw returns a random playable card whose track id is not in exclude.
func (ts *TrackSource) Draw(exclude map[string]bool) (*Candidate, error) {
	c, ok := ts.cards.Draw(exclude)
	if !ok {
		return nil, ErrPoolExhausted
	}
	year, source, _ := c.GameYear() // playable cards always have a year
	return &Candidate{
		Card:        game.Card{TrackID: c.NavidromeID, Title: c.Title, Artist: c.Artist, Album: c.Album, Year: year},
		DurationSec: c.DurationSec,
		YearSource:  source,
	}, nil
}

// IsPlayable reports whether a previously drawn card is still playable.
func (ts *TrackSource) IsPlayable(trackID string) bool {
	return ts.cards.IsPlayable(trackID)
}

// PlayableCount returns the number of playable cards.
func (ts *TrackSource) PlayableCount() int {
	return ts.cards.PlayableCount()
}
