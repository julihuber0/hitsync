package gamesvc

import (
	"testing"

	"github.com/julianhuber/hitsync/backend/internal/game"
)

func TestEvaluateSongGuess(t *testing.T) {
	card := game.Card{Title: "Take On Me", Artist: "a-ha", Album: "Hunting High and Low (Deluxe)", Year: 1985}
	guess := &pendingSongGuess{title: "take on me", artist: "A-ha", album: "Hunting High and Low", year: "1984"}

	cases := []struct {
		name   string
		fields game.GuessFields
		want   bool
	}{
		{"title and artist right", game.GuessFields{Title: true, Artist: true}, true},
		{"album right", game.GuessFields{Album: true}, true},
		{"wrong year fails the guess", game.GuessFields{Title: true, Year: true}, false},
		{"nothing selected earns nothing", game.GuessFields{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, allCorrect := evaluateSongGuess(guess, c.fields, card)
			if allCorrect != c.want {
				t.Errorf("allCorrect = %v, want %v (%+v)", allCorrect, c.want, result)
			}
		})
	}

	result, _ := evaluateSongGuess(guess, game.GuessFields{Title: true, Year: true}, card)
	if !result.TitleCorrect || result.YearCorrect || result.ArtistCorrect || result.Artist != "" {
		t.Errorf("unselected fields must be empty and unchecked: %+v", result)
	}
}
