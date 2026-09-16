// Package musicbrainz resolves a track's earliest known release year via the
// MusicBrainz API: search recordings by exact track title and artist (no
// album/release information in the query), then take the earliest date
// found across every official release of every matching recording.
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
)

// maxPages bounds how many result pages are fetched for a single search, as
// a safety limit against runaway pagination — a genuine exact-title-and-
// artist match set is expected to be tiny compared to this.
const maxPages = 20

// pageLimit is the number of recordings requested per search page (the
// MusicBrainz webservice's maximum).
const pageLimit = 100

// Client is a MusicBrainz API client with mandatory rate limiting.
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
	limiter    *RateLimiter
}

// Config configures a Client.
type Config struct {
	BaseURL    string
	Contact    string
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
		httpClient: &http.Client{Timeout: cfg.Timeout},
		limiter:    NewRateLimiter(cfg.RatePerSec),
	}
}

// Result is the outcome of resolving a (title, artist) pair.
type Result struct {
	Year   int
	Source string // "musicbrainz"
}

// searchResponse is the shape of GET /recording?query=...&fmt=json.
type searchResponse struct {
	Count      int         `json:"count"`
	Recordings []recording `json:"recordings"`
}

type recording struct {
	ID       string    `json:"id"`
	Releases []release `json:"releases"`
}

// release is a release embedded in a recording search result. Status is
// "Official", "Promotion", "Bootleg", or "Pseudo-Release"; only "Official"
// releases are used to determine the earliest year.
type release struct {
	Status string `json:"status"`
	Date   string `json:"date"`
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

// Resolve searches MusicBrainz for recordings matching title and artist
// exactly, and returns the earliest year found across every official
// release of every matching recording, across every result page.
func (c *Client) Resolve(ctx context.Context, title, artist string) (*Result, error) {
	var years []int

	for page := 0; page < maxPages; page++ {
		offset := page * pageLimit
		recs, count, err := c.search(ctx, title, artist, offset)
		if err != nil {
			return nil, err
		}
		for _, rec := range recs {
			for _, rel := range rec.Releases {
				if rel.Status != "Official" {
					continue
				}
				if y := parseYear(rel.Date); y != 0 {
					years = append(years, y)
				}
			}
		}
		if offset+pageLimit >= count {
			break
		}
	}

	if len(years) == 0 {
		return nil, nil
	}

	best := years[0]
	for _, y := range years[1:] {
		if y < best {
			best = y
		}
	}

	currentYear := time.Now().Year()
	if best < 1860 || best > currentYear+1 {
		return nil, nil
	}

	return &Result{Year: best, Source: "musicbrainz"}, nil
}

func (c *Client) search(ctx context.Context, title, artist string, offset int) (recs []recording, count int, err error) {
	query := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, escapeLucene(title), escapeLucene(artist))
	v := url.Values{}
	v.Set("query", query)
	v.Set("fmt", "json")
	v.Set("limit", strconv.Itoa(pageLimit))
	v.Set("offset", strconv.Itoa(offset))

	reqURL := fmt.Sprintf("%s/recording?%s", c.baseURL, v.Encode())
	body, status, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, 0, err
	}
	if status == http.StatusNotFound {
		return nil, 0, nil
	}
	if status != http.StatusOK {
		return nil, 0, fmt.Errorf("musicbrainz: search returned status %d", status)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("musicbrainz: decoding search response: %w", err)
	}
	return resp.Recordings, resp.Count, nil
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
