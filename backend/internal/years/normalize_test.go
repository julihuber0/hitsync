package years

import "testing"

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "Hello World", "hello world"},
		{"diacritics", "Café Éclair", "cafe eclair"},
		{"remaster bracket", "Comfortably Numb (Remastered 2011)", "comfortably numb"},
		{"live bracket", "Money [Live]", "money"},
		{"radio edit bracket", "Song Title (Radio Edit)", "song title"},
		{"bare year bracket", "Track Name (1999)", "track name"},
		{"feat bracket kept out", "Song (feat. Someone)", "song"},
		{"non-noise bracket kept", "Song (Acoustic Version)", "song acoustic version"},
		{"trailing dash noise", "Song Title - Remastered", "song title"},
		{"trailing dash kept", "Song Title - Reprise", "song title reprise"},
		{"ampersand", "Rock & Roll", "rock and roll"},
		{"punctuation stripped", "Don't Stop Me Now!", "dont stop me now"},
		{"whitespace collapse", "Too   Many   Spaces", "too many spaces"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeTitle(c.in)
			if got != c.want {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeArtist(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"leading the", "The Beatles", "beatles"},
		{"feat split", "Artist One feat Artist Two", "artist one"},
		{"ft split", "Artist One ft Artist Two", "artist one"},
		{"semicolon split", "Artist One; Artist Two", "artist one"},
		{"slash split", "Artist One/Artist Two", "artist one"},
		{"comma split", "Artist One, Artist Two", "artist one"},
		{"diacritics", "Sigur Rós", "sigur ros"},
		{"ampersand", "Simon & Garfunkel", "simon and garfunkel"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeArtist(c.in)
			if got != c.want {
				t.Errorf("NormalizeArtist(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
