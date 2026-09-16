package rawtags

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var timeZero time.Time

func TestRangeReadSeeker_MatchesFullContent(t *testing.T) {
	payload := make([]byte, 3*chunkSize+123) // spans multiple chunks, non-aligned tail
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.ServeContent(w, r, "track.mp3", timeZero, bytes.NewReader(payload))
	}))
	defer srv.Close()

	rs, err := newRangeReadSeeker(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("newRangeReadSeeker: %v", err)
	}
	if rs.size != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", rs.size, len(payload))
	}

	got, err := io.ReadAll(rs)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(got), len(payload))
	}

	// One HEAD for size, then one GET per chunk touched (4 chunks).
	if requests > 6 {
		t.Errorf("expected a small, bounded number of requests, got %d", requests)
	}
}

func TestRangeReadSeeker_SeekAndReadArbitraryOffset(t *testing.T) {
	payload := []byte("the quick brown fox jumps over the lazy dog")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "track.mp3", timeZero, bytes.NewReader(payload))
	}))
	defer srv.Close()

	rs, err := newRangeReadSeeker(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("newRangeReadSeeker: %v", err)
	}

	if _, err := rs.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	buf := make([]byte, 5)
	n, err := rs.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "quick" {
		t.Errorf("read = %q, want %q", got, "quick")
	}

	if _, err := rs.Seek(-3, io.SeekEnd); err != nil {
		t.Fatalf("seek from end: %v", err)
	}
	buf = make([]byte, 3)
	n, err = rs.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "dog" {
		t.Errorf("read = %q, want %q", got, "dog")
	}
}

func TestRangeReadSeeker_ReadPastEOFReturnsEOF(t *testing.T) {
	payload := []byte("short")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "track.mp3", timeZero, bytes.NewReader(payload))
	}))
	defer srv.Close()

	rs, err := newRangeReadSeeker(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("newRangeReadSeeker: %v", err)
	}
	if _, err := rs.Seek(0, io.SeekEnd); err != nil {
		t.Fatalf("seek: %v", err)
	}
	buf := make([]byte, 10)
	n, err := rs.Read(buf)
	if n != 0 || err != io.EOF {
		t.Errorf("Read at EOF = (%d, %v), want (0, io.EOF)", n, err)
	}
}
