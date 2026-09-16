// Package years resolves gameplay release years by combining Navidrome tags
// with MusicBrainz lookups (§9.2–9.5 of the spec), and provides the shared
// title/artist normalisation rules used for both the tracks table and
// MusicBrainz lookup keys.
package years

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// noisePattern matches bracket/trailing-segment content that should be
// stripped: remaster tags (optionally paired with a year, e.g. "Remastered
// 2011" or "2011 Remaster"), live/mono/stereo markers, edition markers, a
// bare 4-digit year, or a featured-artist credit.
var noisePattern = regexp.MustCompile(
	`^(\d{4}\s*)?(remaster(ed)?|live|mono|stereo|radio edit|single version|album version|bonus|deluxe|explicit|clean)(\s*\d{4})?$|^\d{4}$|^(feat|ft|featuring|with)\b.*$`,
)

var bracketPattern = regexp.MustCompile(`[\(\[\{]([^\)\]\}]*)[\)\]\}]`)
var trailingSegmentPattern = regexp.MustCompile(`\s*-\s*([^-]+)$`)
var nonAlphaNumSpace = regexp.MustCompile(`[^a-z0-9 ]`)
var whitespaceRun = regexp.MustCompile(`\s+`)

// artistSplitPattern splits a multi-artist credit into its primary segment.
// Applied before punctuation is stripped, so the separators must still be
// intact at that point.
var artistSplitPattern = regexp.MustCompile(`;|/|,| feat | ft `)

// NormalizeTitle applies the §9.2 normalisation rules to a track title.
func NormalizeTitle(s string) string {
	s = stripDiacritics(s)
	s = strings.ToLower(s)
	s = stripNoiseBrackets(s)
	s = stripTrailingNoise(s)
	s = strings.ReplaceAll(s, "&", "and")
	s = nonAlphaNumSpace.ReplaceAllString(s, "")
	s = whitespaceRun.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// NormalizeArtist applies the §9.2 normalisation rules to an artist name,
// additionally keeping only the primary artist from a multi-artist credit.
func NormalizeArtist(s string) string {
	s = stripDiacritics(s)
	s = strings.ToLower(s)

	// Keep only the primary artist before punctuation that separates
	// multiple credits is stripped away.
	if parts := artistSplitPattern.Split(s, 2); len(parts) > 0 {
		s = parts[0]
	}

	s = stripNoiseBrackets(s)
	s = stripTrailingNoise(s)
	s = strings.TrimPrefix(strings.TrimSpace(s), "the ")
	s = strings.ReplaceAll(s, "&", "and")
	s = nonAlphaNumSpace.ReplaceAllString(s, "")
	s = whitespaceRun.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func stripDiacritics(s string) string {
	t := transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

func stripNoiseBrackets(s string) string {
	return bracketPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := bracketPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		content := strings.TrimSpace(sub[1])
		if noisePattern.MatchString(content) {
			return ""
		}
		return match
	})
}

func stripTrailingNoise(s string) string {
	loc := trailingSegmentPattern.FindStringSubmatchIndex(s)
	if loc == nil {
		return s
	}
	content := strings.TrimSpace(s[loc[2]:loc[3]])
	if noisePattern.MatchString(content) {
		return s[:loc[0]]
	}
	return s
}
