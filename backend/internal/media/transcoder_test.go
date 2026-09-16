package media

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestTranscoder transcodes a generated FLAC with embedded tags. Skipped
// when FFmpeg is not installed.
func newTestTranscoder(t *testing.T) (*Transcoder, *atomic.Int32) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.flac")
	gen := exec.Command(ffmpeg, "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-metadata", "title=Secret Title", "-metadata", "date=1984", "-c:a", "flac", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate source: %v: %s", err, out)
	}

	cache, err := NewCache(filepath.Join(dir, "cache"), 1<<30)
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	var sourceCalls atomic.Int32
	tr := NewTranscoder(TranscoderConfig{
		Cache: cache,
		SourceURL: func(string) (string, error) {
			sourceCalls.Add(1)
			return src, nil
		},
		FFmpegPath:    ffmpeg,
		BitrateKbps:   128,
		MaxConcurrent: 2,
		Timeout:       time.Minute,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return tr, &sourceCalls
}

func TestTranscoderStripsTags(t *testing.T) {
	tr, _ := newTestTranscoder(t)
	path, err := tr.Ensure(context.Background(), "track1")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if bytes.HasPrefix(data, []byte("ID3")) {
		t.Fatal("output starts with an ID3v2 tag")
	}
	for _, leak := range []string{"Secret Title", "1984"} {
		if bytes.Contains(data, []byte(leak)) {
			t.Fatalf("output leaks source metadata %q", leak)
		}
	}
	// 3 s at 128 kbit/s is roughly 48 kB.
	if len(data) < 30_000 || len(data) > 70_000 {
		t.Fatalf("unexpected output size %d for 3s at 128 kbit/s", len(data))
	}
}

func TestTranscoderDeduplicatesConcurrentRequests(t *testing.T) {
	tr, sourceCalls := newTestTranscoder(t)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tr.Ensure(context.Background(), "track1"); err != nil {
				t.Errorf("ensure: %v", err)
			}
		}()
	}
	wg.Wait()
	if _, err := tr.Ensure(context.Background(), "track1"); err != nil {
		t.Fatalf("cached ensure: %v", err)
	}
	if n := sourceCalls.Load(); n != 1 {
		t.Fatalf("expected one transcode, got %d", n)
	}
}

func TestTranscoderRejectsUnsafeTrackIDs(t *testing.T) {
	tr := NewTranscoder(TranscoderConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, id := range []string{"", "../etc/passwd", "a/b", "a b"} {
		if _, err := tr.Ensure(context.Background(), id); err != ErrInvalidTrackID {
			t.Errorf("Ensure(%q) = %v, want ErrInvalidTrackID", id, err)
		}
	}
}
