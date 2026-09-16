// Package discogs resolves a track's earliest known release year via the
// Discogs database search API: search for the exact track title, restricted
// to master releases, then take the earliest year found across every
// matching result.
package discogs

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
// a safety limit against runaway pagination — a genuine exact-title match
// set is expected to be tiny compared to this.
const maxPages = 20

// Client is a Discogs API client with mandatory rate limiting.
type Client struct {
	baseURL    string
	token      string
	userAgent  string
	httpClient *http.Client
	limiter    *RateLimiter
}

// Config configures a Client.
type Config struct {
	BaseURL    string
	Token      string
	RatePerSec float64
	Timeout    time.Duration
}

// New creates a Discogs client.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		token:      cfg.Token,
		userAgent:  "Hitsync/1.0",
		httpClient: &http.Client{Timeout: cfg.Timeout},
		limiter:    NewRateLimiter(cfg.RatePerSec),
	}
}

// Result is the outcome of resolving a track's earliest release year.
type Result struct {
	Year   int
	Source string // "discogs"
}

// searchResponse is the shape of GET /database/search?...&fmt=json (implicit).
type searchResponse struct {
	Results    []searchResult `json:"results"`
	Pagination pagination     `json:"pagination"`
}

type searchResult struct {
	Type string `json:"type"`
	Year string `json:"year"`
}

type pagination struct {
	Pages int `json:"pages"`
	Page  int `json:"page"`
}

// doRequest performs a rate-limited GET with the required User-Agent and
// token headers, retrying on HTTP 503/429 with exponential backoff (1s, 2s,
// 4s), never on 404.
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
		if c.token != "" {
			req.Header.Set("Authorization", "Discogs token="+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}

		if (resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests) && attempt < len(backoffs) {
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

// Resolve searches Discogs for master releases matching title exactly, and
// returns the earliest year found across every result on every page. artist
// is accepted for interface compatibility with years.DiscogsResolver but is
// not part of the search: results are not filtered by artist.
func (c *Client) Resolve(ctx context.Context, title, artist string) (*Result, error) {
	var years []int

	for page := 1; page <= maxPages; page++ {
		results, pages, err := c.search(ctx, title, page)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			if r.Type != "master" {
				continue
			}
			if y := parseYear(r.Year); y != 0 {
				years = append(years, y)
			}
		}
		if page >= pages {
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

	return &Result{Year: best, Source: "discogs"}, nil
}

func (c *Client) search(ctx context.Context, title string, page int) (results []searchResult, pages int, err error) {
	v := url.Values{}
	v.Set("q", title)
	v.Set("type", "master")
	v.Set("per_page", "100")
	v.Set("page", strconv.Itoa(page))

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, v.Encode())
	body, status, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, 0, err
	}
	if status == http.StatusNotFound {
		return nil, 0, nil
	}
	if status != http.StatusOK {
		return nil, 0, fmt.Errorf("discogs: search returned status %d", status)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("discogs: decoding search response: %w", err)
	}
	return resp.Results, resp.Pagination.Pages, nil
}

func parseYear(s string) int {
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return y
}
