// Package songmatch decides whether a player's free-text guess names a song's
// title or artist. Matching is deliberately forgiving: case, accents,
// punctuation and spacing are ignored, a leading article may be left out,
// version suffixes such as "(2023 Remix)" or "- Live" are optional, and small
// spelling mistakes are tolerated in proportion to the name's length.
package songmatch

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Title reports whether guess names the song title actual. Bracketed parts
// and anything after " - " (remix, live, remaster, feat. credits, ...) may be
// omitted from the guess.
func Title(guess, actual string) bool {
	return matchAny(titleVariants(guess), titleVariants(actual))
}

// Album reports whether guess names the album actual. Like titles, edition
// suffixes such as "(Deluxe Edition)" or "[Disc 2]" may be left out.
func Album(guess, actual string) bool {
	return Title(guess, actual)
}

// Year reports whether guess is exactly the year actual.
func Year(guess string, actual int) bool {
	year, err := strconv.Atoi(strings.TrimSpace(guess))
	return err == nil && year == actual
}

// Artist reports whether guess names the artist credit actual. For a credit
// with several artists ("Queen & David Bowie", "Calvin Harris feat.
// Rihanna"), naming any one of them is enough, as is naming several of them
// in any order.
func Artist(guess, actual string) bool {
	guessFull := key(guess)
	if guessFull == "" {
		return false
	}
	actualParts := artistVariants(actual)
	if matchAny([]string{guessFull}, actualParts) {
		return true
	}
	// "David Bowie and Queen": every named artist must be credited.
	guessParts := splitArtists(guess)
	if len(guessParts) < 2 {
		return false
	}
	for _, part := range guessParts {
		if !matchAny([]string{key(part)}, actualParts) {
			return false
		}
	}
	return true
}

var (
	bracketed      = regexp.MustCompile(`\s*[\(\[\{][^\)\]\}]*[\)\]\}]`)
	dashSuffix     = regexp.MustCompile(`\s+[-–—]\s+.*$`)
	featSuffix     = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.?|featuring)\s+.*$`)
	artistSplitter = regexp.MustCompile(`(?i)\s*(,|;|/|\+|&|\s(and|und|x|vs\.?|with|feat\.?|ft\.?|featuring)\s)\s*`)
	apostrophes    = strings.NewReplacer("'", "", "’", "", "`", "", "´", "")
	letters        = strings.NewReplacer("ß", "ss", "æ", "ae", "œ", "oe", "ø", "o", "ł", "l", "đ", "d", "þ", "th")
	articles       = map[string]bool{
		"the": true, "a": true, "an": true,
		"der": true, "die": true, "das": true,
		"le": true, "la": true, "les": true, "l": true,
		"el": true, "los": true, "las": true,
		"il": true, "lo": true, "gli": true,
	}
)

// titleVariants returns the full title and the title without version
// suffixes, each normalised to a comparison key.
func titleVariants(s string) []string {
	base := bracketed.ReplaceAllString(s, "")
	base = dashSuffix.ReplaceAllString(base, "")
	base = featSuffix.ReplaceAllString(base, "")
	return nonEmptyKeys(s, base)
}

// artistVariants returns the full credit and each individual artist in it.
func artistVariants(s string) []string {
	return nonEmptyKeys(append([]string{s}, splitArtists(s)...)...)
}

func splitArtists(s string) []string {
	var parts []string
	for _, p := range artistSplitter.Split(bracketed.ReplaceAllString(s, ""), -1) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func nonEmptyKeys(values ...string) []string {
	var out []string
	for _, v := range values {
		if k := key(v); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// key normalises a name for comparison: accents removed, lowercase, "&" and
// "n" read as "and", punctuation and a leading article dropped, and spaces
// removed so "Bee Gees" equals "Beegees".
func key(s string) string {
	s = strings.ToLower(stripDiacritics(s))
	s = letters.Replace(s)
	s = apostrophes.Replace(s)
	s = strings.ReplaceAll(s, "&", " and ")
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) > 1 && articles[words[0]] {
		words = words[1:]
	}
	for i, w := range words {
		if w == "n" { // "Rock 'n' Roll", "Guns N' Roses"
			words[i] = "and"
		}
	}
	return strings.Join(words, "")
}

func stripDiacritics(s string) string {
	t := transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

func matchAny(guesses, actuals []string) bool {
	for _, g := range guesses {
		for _, a := range actuals {
			if similar(g, a) {
				return true
			}
		}
	}
	return false
}

// similar tolerates typos in proportion to the actual name's length (spaces
// and punctuation not counted). Short names must match exactly: one changed
// letter already turns "Line" into "Live". Checked against a real library,
// looser limits start matching different songs ("Waiting For The Day" vs
// "Waiting For The Sun").
func similar(guess, actual string) bool {
	if guess == actual {
		return true
	}
	g, a := []rune(guess), []rune(actual)
	var allowed int
	switch n := len(a); {
	case n <= 4:
		return false
	case n <= 11:
		allowed = 1
	case n <= 17:
		allowed = 2
	default:
		allowed = n / 6
	}
	if abs(len(g)-len(a)) > allowed {
		return false
	}
	return editDistance(g, a) <= allowed
}

// editDistance is the optimal string alignment distance: insertions,
// deletions, substitutions, and transpositions of adjacent characters.
func editDistance(a, b []rune) int {
	prev2 := make([]int, len(b)+1)
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				curr[j] = min(curr[j], prev2[j-2]+1)
			}
		}
		prev2, prev, curr = prev, curr, prev2
	}
	return prev[len(b)]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
