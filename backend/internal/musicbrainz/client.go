// Package musicbrainz resolves a track's original release year via the
// MusicBrainz API, following the algorithm in §9.3 of the spec: search,
// verify the artist credit, then take the earliest surviving release-group
// first-release-date, filtering out compilations/live/remix/etc.
package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/years"
)

// secondaryTypeBlocklist are release-group secondary types that disqualify a
// release group from being used as the "original" release (§9.3 step 4.3).
var secondaryTypeBlocklist = map[string]bool{
	"Compilation":    true,
	"Live":           true,
	"Remix":          true,
	"DJ-mix":         true,
	"Mixtape/Street": true,
	"Interview":      true,
	"Soundtrack":     true,
	"Demo":           true,
}

// Client is a MusicBrainz API client with mandatory rate limiting.
type Client struct {
	baseURL    string
	userAgent  string
	minScore   int
	httpClient *http.Client
	limiter    *RateLimiter
}

// Config configures a Client.
type Config struct {
	BaseURL    string
	Contact    string
	MinScore   int
	RatePerSec float64
	Timeout    time.Duration
}

// New creates a MusicBrainz client.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		userAgent:  fmt.Sprintf("Hitsync/1.0 ( %s )", cfg.Contact),
		minScore:   cfg.MinScore,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		limiter:    NewRateLimiter(cfg.RatePerSec),
	}
}

// Result is the outcome of resolving a (title, artist) pair.
type Result struct {
	Year   int
	Source string // "musicbrainz" or "musicbrainz_loose"
}

// searchResponse is the shape of GET /recording?query=...&fmt=json.
type searchResponse struct {
	Recordings []recording `json:"recordings"`
}

type recording struct {
	ID           string         `json:"id"`
	Score        int            `json:"score"`
	Title        string         `json:"title"`
	ArtistCredit []artistCredit `json:"artist-credit"`
}

type artistCredit struct {
	Name   string `json:"name"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
}

// lookupResponse is the shape of GET /recording/{mbid}?inc=releases+release-groups&fmt=json.
type lookupResponse struct {
	Releases []release `json:"releases"`
}

type release struct {
	ReleaseGroup releaseGroup `json:"release-group"`
}

type releaseGroup struct {
	ID               string   `json:"id"`
	FirstReleaseDate string   `json:"first-release-date"`
	SecondaryTypes   []string `json:"secondary-types"`
}

// doRequest performs a rate-limited GET with the required User-Agent header
// and retries on HTTP 503 with exponential backoff (1s, 2s, 4s), never on 404.
func (c *Client) doRequest(ctx context.Context, reqURL string) ([]byte, int, error) {
	backoffs := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; ; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, 0, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}

		if resp.StatusCode == http.StatusServiceUnavailable && attempt < len(backoffs) {
			timer := time.NewTimer(backoffs[attempt])
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return nil, resp.StatusCode, ctx.Err()
			}
			continue
		}
		return body, resp.StatusCode, nil
	}
}

// Resolve looks up the earliest original release year for a normalised
// (title, artist) pair, following §9.3 steps 1-7.
func (c *Client) Resolve(ctx context.Context, title, artist string) (*Result, error) {
	normArtist := years.NormalizeArtist(artist)

	recs, err := c.search(ctx, title, artist)
	if err != nil {
		return nil, err
	}

	var candidates []recording
	for _, r := range recs {
		if r.Score < c.minScore {
			continue
		}
		if !c.artistMatches(r, normArtist) {
			continue
		}
		candidates = append(candidates, r)
		if len(candidates) == 3 {
			break
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	bestYear := 0
	looseOnly := true
	found := false

	for _, cand := range candidates {
		year, loose, err := c.earliestYearForRecording(ctx, cand.ID)
		if err != nil {
			return nil, err
		}
		if year == 0 {
			continue
		}
		if !loose {
			looseOnly = false
		}
		if !found || year < bestYear {
			bestYear = year
			found = true
		}
	}

	if !found {
		return nil, nil
	}

	currentYear := time.Now().Year()
	if bestYear < 1860 || bestYear > currentYear+1 {
		return nil, nil
	}

	source := "musicbrainz"
	if looseOnly {
		source = "musicbrainz_loose"
	}
	return &Result{Year: bestYear, Source: source}, nil
}

func (c *Client) artistMatches(r recording, normArtist string) bool {
	for _, ac := range r.ArtistCredit {
		name := ac.Artist.Name
		if name == "" {
			name = ac.Name
		}
		if years.NormalizeArtist(name) == normArtist {
			return true
		}
	}
	return false
}

func (c *Client) search(ctx context.Context, title, artist string) ([]recording, error) {
	query := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, escapeLucene(title), escapeLucene(artist))
	v := url.Values{}
	v.Set("query", query)
	v.Set("fmt", "json")
	v.Set("limit", "8")

	reqURL := fmt.Sprintf("%s/recording?%s", c.baseURL, v.Encode())
	body, status, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz: search returned status %d", status)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("musicbrainz: decoding search response: %w", err)
	}
	return resp.Recordings, nil
}

// earliestYearForRecording implements §9.3 step 4: fetch the recording's
// releases, collect distinct release groups, filter out compilations etc.,
// and take the minimum first-release-date year. Falls back to the loose
// (unfiltered) minimum if every release group is filtered out.
func (c *Client) earliestYearForRecording(ctx context.Context, mbid string) (year int, loose bool, err error) {
	v := url.Values{}
	v.Set("inc", "releases+release-groups")
	v.Set("fmt", "json")
	reqURL := fmt.Sprintf("%s/recording/%s?%s", c.baseURL, mbid, v.Encode())

	body, status, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return 0, false, err
	}
	if status == http.StatusNotFound {
		return 0, false, nil
	}
	if status != http.StatusOK {
		return 0, false, fmt.Errorf("musicbrainz: lookup returned status %d", status)
	}

	var resp lookupResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, fmt.Errorf("musicbrainz: decoding lookup response: %w", err)
	}

	seen := map[string]releaseGroup{}
	for _, rel := range resp.Releases {
		rg := rel.ReleaseGroup
		if rg.ID == "" {
			continue
		}
		seen[rg.ID] = rg
	}

	filteredYear, filteredOK := minFirstReleaseYear(seen, true)
	if filteredOK {
		return filteredYear, false, nil
	}

	looseYear, looseOK := minFirstReleaseYear(seen, false)
	if looseOK {
		return looseYear, true, nil
	}
	return 0, false, nil
}

func minFirstReleaseYear(groups map[string]releaseGroup, applyFilter bool) (int, bool) {
	best := 0
	found := false
	for _, rg := range groups {
		if applyFilter && hasBlockedSecondaryType(rg.SecondaryTypes) {
			continue
		}
		y := parseYear(rg.FirstReleaseDate)
		if y == 0 {
			continue
		}
		if !found || y < best {
			best = y
			found = true
		}
	}
	return best, found
}

func hasBlockedSecondaryType(types []string) bool {
	for _, t := range types {
		if secondaryTypeBlocklist[t] {
			return true
		}
	}
	return false
}

func parseYear(dateStr string) int {
	if len(dateStr) < 4 {
		return 0
	}
	y, err := strconv.Atoi(dateStr[:4])
	if err != nil {
		return 0
	}
	return y
}

// escapeLucene escapes Lucene special characters in an interpolated query value.
func escapeLucene(s string) string {
	special := `+-&|!(){}[]^"~*?:\/`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
