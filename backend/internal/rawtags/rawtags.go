// Package rawtags reads the HITSYNCYEAR and HITSYNCEXCLUDE custom tags
// directly out of an audio file's embedded metadata (ID3v2 TXXX frames,
// Vorbis comments, or MP4 freeform atoms). Navidrome does not expose custom
// tags through its API, so this fetches only the byte ranges of the file
// that the tag parser actually touches (via HTTP Range requests against
// Navidrome's raw, untranscoded stream endpoint) rather than downloading the
// whole track.
package rawtags

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

// Result holds the two custom tags, both optional.
type Result struct {
	Year    *int // from HITSYNCYEAR, nil if absent or unparsable
	Exclude bool // true only if HITSYNCEXCLUDE is exactly "1"
}

// Fetch reads title metadata from rawURL (a direct, authenticated URL to the
// track's raw/untranscoded bytes) and extracts the HITSYNCYEAR/
// HITSYNCEXCLUDE tags. Any error (unsupported format, network failure,
// unparseable file) is returned so the caller can decide how to treat it;
// callers that want "tag absent" semantics on failure should ignore the
// error and use the zero Result.
func Fetch(ctx context.Context, client *http.Client, rawURL string) (Result, error) {
	rs, err := newRangeReadSeeker(ctx, client, rawURL)
	if err != nil {
		return Result{}, err
	}

	m, err := tag.ReadFrom(rs)
	if err != nil {
		return Result{}, err
	}

	return extract(m.Raw()), nil
}

const (
	tagYear    = "hitsyncyear"
	tagExclude = "hitsyncexclude"
)

func extract(raw map[string]interface{}) Result {
	var res Result

	// Vorbis comments (FLAC/OGG) are lowercased by dhowden/tag; MP4 freeform
	// atoms keep the case as written in the file. Match case-insensitively
	// against a plain string value either way.
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			continue
		}
		switch strings.ToLower(k) {
		case tagYear:
			if y, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				res.Year = &y
			}
		case tagExclude:
			res.Exclude = strings.TrimSpace(s) == "1"
		}
	}

	// ID3v2 stores user-defined text frames under "TXXX" (or "TXXX_0",
	// "TXXX_1", ... when there are several), each holding a *tag.Comm whose
	// Description is the tag name and Text is the value.
	for k, v := range raw {
		if !strings.HasPrefix(k, "TXXX") {
			continue
		}
		c, ok := v.(*tag.Comm)
		if !ok {
			continue
		}
		switch strings.ToLower(c.Description) {
		case tagYear:
			if y, err := strconv.Atoi(strings.TrimSpace(c.Text)); err == nil {
				res.Year = &y
			}
		case tagExclude:
			res.Exclude = strings.TrimSpace(c.Text) == "1"
		}
	}

	return res
}
