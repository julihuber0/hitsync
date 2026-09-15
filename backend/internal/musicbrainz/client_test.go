package musicbrainz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

// newTestServer wires specific fixture files to specific request patterns:
// search requests get searchFixture, recording lookups get lookupFixture.
func newTestServer(t *testing.T, searchFixture, lookupFixture string, calls *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			atomic.AddInt32(calls, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/recording/") && r.URL.Path != "/recording" {
			w.Write(fixture(t, lookupFixture))
			return
		}
		w.Write(fixture(t, searchFixture))
	}))
}

func TestResolve_FiltersCompilationInFavourOfOriginal(t *testing.T) {
	srv := newTestServer(t, "search_match.json", "lookup_original_and_compilation.json", nil)
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Contact: "test@example.com", MinScore: 90, RatePerSec: 1000})
	res, err := c.Resolve(context.Background(), "Wish You Were Here", "Pink Floyd")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res == nil {
		t.Fatal("expected a result")
	}
	if res.Year != 1975 {
		t.Errorf("year = %d, want 1975 (original, not the 2004 compilation)", res.Year)
	}
	if res.Source != "musicbrainz" {
		t.Errorf("source = %q, want musicbrainz", res.Source)
	}
}

func TestResolve_CompilationOnlyFallsBackToLoose(t *testing.T) {
	srv := newTestServer(t, "search_match.json", "lookup_compilation_only.json", nil)
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Contact: "test@example.com", MinScore: 90, RatePerSec: 1000})
	res, err := c.Resolve(context.Background(), "Wish You Were Here", "Pink Floyd")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res == nil {
		t.Fatal("expected a loose result")
	}
	if res.Year != 1999 {
		t.Errorf("year = %d, want 1999 (earliest of the compilations)", res.Year)
	}
	if res.Source != "musicbrainz_loose" {
		t.Errorf("source = %q, want musicbrainz_loose", res.Source)
	}
}

func TestResolve_RejectsArtistMismatch(t *testing.T) {
	srv := newTestServer(t, "search_wrong_artist.json", "lookup_original_and_compilation.json", nil)
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Contact: "test@example.com", MinScore: 90, RatePerSec: 1000})
	res, err := c.Resolve(context.Background(), "Wish You Were Here", "Pink Floyd")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result for artist mismatch, got %+v", res)
	}
}

func TestRateLimiterSerialisesConcurrentCallers(t *testing.T) {
	limiter := NewRateLimiter(20) // 50ms apart
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = limiter.Wait(context.Background())
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	// 5 calls at 20/sec (50ms apart) must take at least 4*50ms serialised.
	if elapsed < 190*time.Millisecond {
		t.Errorf("elapsed = %v, expected calls to be serialised to >=~200ms", elapsed)
	}
}
