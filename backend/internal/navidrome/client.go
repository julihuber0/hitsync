// Package navidrome implements a minimal Subsonic API client for Navidrome.
package navidrome

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const apiVersion = "1.16.1"

// Client talks to a Navidrome server's Subsonic-compatible API.
type Client struct {
	baseURL    string
	username   string
	password   string
	clientName string
	httpClient *http.Client
}

// New creates a Navidrome client.
func New(baseURL, username, password, clientName string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		clientName: clientName,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// authParams returns the salted-token auth query parameters. The plaintext
// password is never placed in a query string.
func (c *Client) authParams() (url.Values, error) {
	salt, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	sum := md5.Sum([]byte(c.password + salt))
	token := hex.EncodeToString(sum[:])

	v := url.Values{}
	v.Set("u", c.username)
	v.Set("t", token)
	v.Set("s", salt)
	v.Set("v", apiVersion)
	v.Set("c", c.clientName)
	v.Set("f", "json")
	return v, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n/2)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (c *Client) get(ctx context.Context, endpoint string, extra url.Values) (*subsonicResponse, error) {
	v, err := c.authParams()
	if err != nil {
		return nil, err
	}
	for k, vals := range extra {
		for _, val := range vals {
			v.Add(k, val)
		}
	}

	reqURL := fmt.Sprintf("%s/rest/%s?%s", c.baseURL, endpoint, v.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("navidrome: %s returned status %d", endpoint, resp.StatusCode)
	}

	var wrapper struct {
		SubsonicResponse subsonicResponse `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("navidrome: decoding response: %w", err)
	}
	if wrapper.SubsonicResponse.Status != "ok" {
		msg := "unknown error"
		if wrapper.SubsonicResponse.Error != nil {
			msg = wrapper.SubsonicResponse.Error.Message
		}
		return nil, fmt.Errorf("navidrome: %s failed: %s", endpoint, msg)
	}
	return &wrapper.SubsonicResponse, nil
}

type subsonicResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	SearchResult3 *searchResult3 `json:"searchResult3,omitempty"`
}

type searchResult3 struct {
	Song []Song `json:"song"`
}

// Song is a single track as returned by search3.
type Song struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	ArtistID string `json:"artistId"`
	Album    string `json:"album"`
	AlbumID  string `json:"albumId"`
	Year     int    `json:"year"`
	Duration int    `json:"duration"`
}

// Ping checks connectivity to the Navidrome server.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.get(ctx, "ping.view", nil)
	return err
}

// SearchPage returns up to songCount songs starting at songOffset, paging
// through the whole library with an empty query (§9.1).
func (c *Client) SearchPage(ctx context.Context, songOffset, songCount int) ([]Song, error) {
	extra := url.Values{}
	extra.Set("query", "")
	extra.Set("songCount", strconv.Itoa(songCount))
	extra.Set("songOffset", strconv.Itoa(songOffset))
	extra.Set("artistCount", "0")
	extra.Set("albumCount", "0")

	resp, err := c.get(ctx, "search3.view", extra)
	if err != nil {
		return nil, err
	}
	if resp.SearchResult3 == nil {
		return nil, nil
	}
	return resp.SearchResult3.Song, nil
}

// RawStreamURL builds a stream URL for the original, untranscoded file
// (format=raw). The library sync reads custom tags from it (these are not
// exposed by any of Navidrome's own metadata endpoints) and the transcoder
// uses it as its FFmpeg input. It is never exposed to browser clients.
func (c *Client) RawStreamURL(trackID string) (string, error) {
	v, err := c.authParams()
	if err != nil {
		return "", err
	}
	v.Set("id", trackID)
	v.Set("format", "raw")
	return fmt.Sprintf("%s/rest/stream.view?%s", c.baseURL, v.Encode()), nil
}
