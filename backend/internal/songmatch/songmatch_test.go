package songmatch

import "testing"

func TestTitle(t *testing.T) {
	cases := []struct {
		guess, actual string
		want          bool
	}{
		{"Bohemian Rhapsody", "Bohemian Rhapsody", true},
		{"bohemian rhapsody", "Bohemian Rhapsody", true},
		{"Bohemian Rapsody", "Bohemian Rhapsody", true},           // typo
		{"Bohemain Rhapsody", "Bohemian Rhapsody", true},          // transposition
		{"Final Countdown", "The Final Countdown", true},          // forgotten article
		{"Dont Stop Me Now", "Don't Stop Me Now", true},           // apostrophe
		{"Dont stop me now!", "Don't Stop Me Now", true},          // punctuation
		{"Blinding Lights", "Blinding Lights (2023 Remix)", true}, // bracket suffix
		{"Blinding Lights", "Blinding Lights - Live at Wembley", true},
		{"Wonderwall", "Wonderwall - Remastered 2014", true},
		{"This Is What You Came For", "This Is What You Came For (feat. Rihanna)", true},
		{"Blinding Lights (Remix)", "Blinding Lights", true},
		{"99 Luftballons", "99 Luftballons", true},
		{"Sonne", "Sonne", true},
		{"Uber den Wolken", "Über den Wolken", true}, // accents
		{"Ueber den Wolken", "Über den Wolken", true},
		{"Rock n Roll", "Rock 'n' Roll", true},
		{"Rock and Roll", "Rock & Roll", true},
		{"Somebody", "Somebody That I Used to Know", false},
		{"Yesterday", "Tomorrow", false},
		{"Hello", "Help", false},
		{"Live", "The Line", false},
		{"Waiting For The Day", "Waiting For The Sun", false},
		{"ABC", "ABBA", false},
		{"", "Anything", false},
		{"   ", "Anything", false},
	}
	for _, c := range cases {
		if got := Title(c.guess, c.actual); got != c.want {
			t.Errorf("Title(%q, %q) = %v, want %v", c.guess, c.actual, got, c.want)
		}
	}
}

func TestArtist(t *testing.T) {
	cases := []struct {
		guess, actual string
		want          bool
	}{
		{"Queen", "Queen", true},
		{"Beatles", "The Beatles", true},
		{"the beatles", "The Beatles", true},
		{"Bee Gees", "Bee Gees", true},
		{"Beegees", "Bee Gees", true},
		{"Beyonce", "Beyoncé", true},
		{"AC DC", "AC/DC", true},
		{"acdc", "AC/DC", true},
		{"Guns and Roses", "Guns N' Roses", true},
		{"Michael Jakson", "Michael Jackson", true},
		{"David Bowie", "Queen & David Bowie", true},
		{"Queen", "Queen & David Bowie", true},
		{"Queen and David Bowie", "Queen & David Bowie", true},
		{"David Bowie & Queen", "Queen & David Bowie", true},
		{"Rihanna", "Calvin Harris feat. Rihanna", true},
		{"Calvin Harris", "Calvin Harris feat. Rihanna", true},
		{"Simon & Garfunkel", "Simon & Garfunkel", true},
		{"Die Ärzte", "Die Ärzte", true},
		{"Arzte", "Die Ärzte", true},
		{"Queen and Elton John", "Queen & David Bowie", false},
		{"Madonna", "Queen & David Bowie", false},
		{"ABC", "ABBA", false},
		{"", "Queen", false},
	}
	for _, c := range cases {
		if got := Artist(c.guess, c.actual); got != c.want {
			t.Errorf("Artist(%q, %q) = %v, want %v", c.guess, c.actual, got, c.want)
		}
	}
}
