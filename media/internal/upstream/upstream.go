// Package upstream fetches tracks from Navidrome on a cache miss, streaming
// the response simultaneously to disk and to the first waiting client while
// de-duplicating concurrent requests for the same track (§10.1).
package upstream

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/julianhuber/hitsync/media/internal/cache"
)

// Navidrome holds the credentials needed to build authenticated Subsonic
// stream URLs.
type Navidrome struct {
	BaseURL    string
	Username   string
	Password   string
	ClientName string
}

func (n Navidrome) streamURL(trackID, format string, maxBitRate int) (string, error) {
	salt, err := randomHex(16)
	if err != nil {
		return "", err
	}
	sum := md5.Sum([]byte(n.Password + salt))
	token := hex.EncodeToString(sum[:])

	v := url.Values{}
	v.Set("u", n.Username)
	v.Set("t", token)
	v.Set("s", salt)
	v.Set("v", "1.16.1")
	v.Set("c", n.ClientName)
	v.Set("f", "json")
	v.Set("id", trackID)
	v.Set("format", format)
	v.Set("maxBitRate", strconv.Itoa(maxBitRate))
	return fmt.Sprintf("%s/rest/stream.view?%s", n.BaseURL, v.Encode()), nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n/2)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Fetcher coordinates cache-miss downloads from Navidrome.
type Fetcher struct {
	nav        Navidrome
	format     string
	bitrate    int
	cache      *cache.Cache
	httpClient *http.Client

	mu       sync.Mutex
	inFlight map[string]chan struct{}
}

// New creates a Fetcher.
func New(nav Navidrome, format string, bitrate int, c *cache.Cache, timeout time.Duration) *Fetcher {
	return &Fetcher{
		nav:        nav,
		format:     format,
		bitrate:    bitrate,
		cache:      c,
		httpClient: &http.Client{Timeout: timeout},
		inFlight:   map[string]chan struct{}{},
	}
}

// Ensure downloads trackID to the local cache once. Audio files are never
// returned to a browser: the LiveKit publisher consumes the cached file.
// Concurrent calls for the same track wait for the leader rather than causing
// duplicate Navidrome transcodes.
func (f *Fetcher) Ensure(trackID string) error {
	if f.cache.Has(trackID) {
		return nil
	}

	f.mu.Lock()
	if ch, ok := f.inFlight[trackID]; ok {
		f.mu.Unlock()
		<-ch
		if f.cache.Has(trackID) {
			return nil
		}
		return fmt.Errorf("upstream fetch failed for track %s", trackID)
	}

	ch := make(chan struct{})
	f.inFlight[trackID] = ch
	f.mu.Unlock()

	err := f.fetchToCache(trackID)

	f.mu.Lock()
	delete(f.inFlight, trackID)
	f.mu.Unlock()
	close(ch)

	return err
}

func (f *Fetcher) fetchToCache(trackID string) error {
	reqURL, err := f.nav.streamURL(trackID, f.format, f.bitrate)
	if err != nil {
		return err
	}

	resp, err := f.httpClient.Get(reqURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("navidrome stream returned status %d", resp.StatusCode)
	}

	tmpPath := f.cache.TempPath(trackID)
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	n, copyErr := io.Copy(tmpFile, resp.Body)
	closeErr := tmpFile.Close()

	if copyErr != nil || closeErr != nil {
		os.Remove(tmpPath)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}

	if err := os.Rename(tmpPath, f.cache.Path(trackID)); err != nil {
		os.Remove(tmpPath)
		return err
	}
	f.cache.Commit(trackID, n)
	return nil
}
