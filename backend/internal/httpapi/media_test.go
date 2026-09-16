package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/julianhuber/hitsync/backend/internal/media"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
)

func mediaRequest(a *API, token string, header http.Header) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/api/media/{token}", a.handleMedia)
	req := httptest.NewRequest(http.MethodGet, "/api/media/"+token, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestHandleMediaRejectsInvalidToken(t *testing.T) {
	a := testAPI()
	a.mediaSigner = tokens.NewMediaSigner(a.cfg.JWTSecret)

	expired, _ := a.mediaSigner.Issue("game1", "track1", -time.Minute)
	for _, token := range []string{"garbage", expired} {
		if rr := mediaRequest(a, token, nil); rr.Code != http.StatusForbidden {
			t.Errorf("token %q: status = %d, want 403", token, rr.Code)
		}
	}
}

func TestHandleMediaServesTranscodedTrack(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.wav")
	if out, err := exec.Command(ffmpeg, "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "sine=duration=2", src).CombinedOutput(); err != nil {
		t.Fatalf("generate source: %v: %s", err, out)
	}
	cache, err := media.NewCache(filepath.Join(dir, "cache"), 1<<30)
	if err != nil {
		t.Fatal(err)
	}

	a := testAPI()
	a.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	a.mediaSigner = tokens.NewMediaSigner(a.cfg.JWTSecret)
	a.transcoder = media.NewTranscoder(media.TranscoderConfig{
		Cache:       cache,
		SourceURL:   func(string) (string, error) { return src, nil },
		FFmpegPath:  ffmpeg,
		BitrateKbps: 128,
		Timeout:     time.Minute,
	}, a.log)

	token, _ := a.mediaSigner.Issue("game1", "track1", time.Minute)
	rr := mediaRequest(a, token, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("Content-Type = %q", ct)
	}
	if rr.Body.Len() < 10_000 {
		t.Errorf("body too small: %d bytes", rr.Body.Len())
	}

	rr = mediaRequest(a, token, http.Header{"Range": {"bytes=0-99"}})
	if rr.Code != http.StatusPartialContent || rr.Body.Len() != 100 {
		t.Errorf("range request: status %d, %d bytes", rr.Code, rr.Body.Len())
	}
}
