// Command media is Hitsync's private LiveKit broadcast worker.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/julianhuber/hitsync/media/internal/broadcast"
	"github.com/julianhuber/hitsync/media/internal/cache"
	"github.com/julianhuber/hitsync/media/internal/config"
	"github.com/julianhuber/hitsync/media/internal/tokens"
	"github.com/julianhuber/hitsync/media/internal/upstream"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(log)

	c, err := cache.New(cfg.MediaCacheDir, cfg.MediaCacheMaxBytes)
	if err != nil {
		log.Error("failed to initialise cache", "error", err)
		os.Exit(1)
	}

	fetcher := upstream.New(upstream.Navidrome{
		BaseURL:    cfg.NavidromeURL,
		Username:   cfg.NavidromeUsername,
		Password:   cfg.NavidromePassword,
		ClientName: cfg.NavidromeClientName,
	}, cfg.AudioFormat, cfg.AudioBitrate, c, 60*time.Second)

	verifier := tokens.NewVerifier(cfg.MediaSharedSecret)
	b := broadcast.New(broadcast.Config{
		LiveKitURL: cfg.LiveKitURL, APIKey: cfg.LiveKitAPIKey, APISecret: cfg.LiveKitAPISecret, FFmpegPath: cfg.FFmpegPath,
	}, log)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /broadcast/{trackId}/preload", preloadHandler(log, c, fetcher, verifier))
	mux.HandleFunc("POST /broadcast/{trackId}/prepare", prepareHandler(log, c, fetcher, verifier, b))
	mux.HandleFunc("POST /broadcast/{trackId}/start", startHandler(log, c, verifier, b))
	mux.HandleFunc("POST /broadcast/{trackId}/stop", stopHandler(verifier, b))
	mux.HandleFunc("GET /healthz", healthzHandler(cfg))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		var err error
		log.Info("media broadcast service listening", "addr", srv.Addr)
		err = srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// preloadHandler fills the local source cache ahead of a future turn. It does
// not allocate a LiveKit publication, so several upcoming tracks can be
// cached concurrently without creating idle rooms.
func preloadHandler(log *slog.Logger, c *cache.Cache, fetcher *upstream.Fetcher, verifier *tokens.Verifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := verifyBroadcast(r, verifier); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		trackID := r.PathValue("trackId")
		if err := fetcher.Ensure(trackID); err != nil {
			log.Warn("failed to preload broadcast source", "track_id", trackID, "error", err)
			http.Error(w, "upstream fetch failed", http.StatusBadGateway)
			return
		}
		c.Touch(trackID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func verifyBroadcast(r *http.Request, verifier *tokens.Verifier) (string, bool) {
	trackID := r.PathValue("trackId")
	payload, err := verifier.Verify(r.URL.Query().Get("token"), trackID)
	if err != nil {
		return "", false
	}
	return payload.GameID, true
}

func prepareHandler(log *slog.Logger, c *cache.Cache, fetcher *upstream.Fetcher, verifier *tokens.Verifier, b *broadcast.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID, ok := verifyBroadcast(r, verifier)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		trackID := r.PathValue("trackId")
		if err := fetcher.Ensure(trackID); err != nil {
			log.Error("failed to cache broadcast source", "track_id", trackID, "error", err)
			http.Error(w, "upstream fetch failed", http.StatusBadGateway)
			return
		}
		c.Touch(trackID)
		if err := b.Prepare(r.Context(), gameID, trackID); err != nil {
			log.Error("failed to prepare LiveKit broadcast", "game_id", gameID, "track_id", trackID, "error", err)
			http.Error(w, "broadcast unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func startHandler(log *slog.Logger, c *cache.Cache, verifier *tokens.Verifier, b *broadcast.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID, ok := verifyBroadcast(r, verifier)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		trackID := r.PathValue("trackId")
		if err := b.Start(gameID, trackID, c.Path(trackID)); err != nil {
			log.Error("failed to start LiveKit broadcast", "game_id", gameID, "track_id", trackID, "error", err)
			http.Error(w, "broadcast unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func stopHandler(verifier *tokens.Verifier, b *broadcast.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID, ok := verifyBroadcast(r, verifier)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		b.Stop(gameID, r.PathValue("trackId"))
		w.WriteHeader(http.StatusNoContent)
	}
}

func healthzHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		testFile := cfg.MediaCacheDir + "/.healthcheck"
		if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"error"}`))
			return
		}
		_ = os.Remove(testFile)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
