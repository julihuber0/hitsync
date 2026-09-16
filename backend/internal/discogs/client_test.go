package discogs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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

func TestResolve_TakesEarliestYearAcrossAllPages(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 2 {
			w.Write(fixture(t, "search_page2.json"))
			return
		}
		w.Write(fixture(t, "search_page1.json"))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Token: "test-token", RatePerSec: 1000})
	res, err := c.Resolve(context.Background(), "Wish You Were Here", "Pink Floyd")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res == nil {
		t.Fatal("expected a result")
	}
	// 1971 (a "master" on page 2) beats 1975 and 1999, and the non-master
	// (type "release") and the empty-year entry are both ignored.
	if res.Year != 1971 {
		t.Errorf("year = %d, want 1971 (earliest master across both pages)", res.Year)
	}
	if res.Source != "discogs" {
		t.Errorf("source = %q, want discogs", res.Source)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("expected 2 requests (one per page), got %d", calls)
	}
}

func TestResolve_NoMatchesReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, RatePerSec: 1000})
	res, err := c.Resolve(context.Background(), "Totally Unknown Song", "Nobody")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result, got %+v", res)
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
