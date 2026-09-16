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
		Card:        game.Card{TrackID: c.NavidromeID, Title: c.Title, Artist: c.Artist, Year: year},
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

// SongGuessOptions builds the decoy sets for the "Name that tune" panel
// (§8.6): three random decoys per group, distinct from the correct value.
func (ts *TrackSource) SongGuessOptions(card *game.Card) (titles, artists []string) {
	sample := ts.cards.RandomCards(24)
	decoyTitles := make([]string, 0, len(sample))
	decoyArtists := make([]string, 0, len(sample))
	for _, c := range sample {
		decoyTitles = append(decoyTitles, c.Title)
		decoyArtists = append(decoyArtists, c.Artist)
	}

	titles = append([]string{card.Title}, dedupeExcept(decoyTitles, card.Title, 3)...)
	artists = append([]string{card.Artist}, dedupeExcept(decoyArtists, card.Artist, 3)...)
	shuffleStrings(titles)
	shuffleStrings(artists)
	return titles, artists
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
