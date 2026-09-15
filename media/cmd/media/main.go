// Command media is Hitsync's dedicated audio-streaming service (§10.1).
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /stream/{trackId}", streamHandler(log, cfg, c, fetcher, verifier))
	mux.HandleFunc("GET /healthz", healthzHandler(cfg))
	mux.HandleFunc("GET /time", timeHandler)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		var err error
		if cfg.MediaTLSCert != "" {
			log.Info("media service listening (TLS)", "addr", srv.Addr)
			err = srv.ListenAndServeTLS(cfg.MediaTLSCert, cfg.MediaTLSKey)
		} else {
			log.Info("media service listening", "addr", srv.Addr)
			err = srv.ListenAndServe()
		}
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

func streamHandler(log *slog.Logger, cfg *config.Config, c *cache.Cache, fetcher *upstream.Fetcher, verifier *tokens.Verifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trackID := r.PathValue("trackId")
		token := r.URL.Query().Get("token")

		if _, err := verifier.Verify(token, trackID); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", cfg.AppOrigin())
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.Header().Set("Cache-Control", "private, max-age=3600")

		if c.Has(trackID) {
			c.Touch(trackID)
			serveFromCache(w, r, c, trackID)
			return
		}

		servedDirectly, err := fetcher.Obtain(trackID, w)
		if err != nil {
			log.Error("upstream fetch failed", "track_id", trackID, "error", err)
			if !servedDirectly {
				http.Error(w, "upstream fetch failed", http.StatusBadGateway)
			}
			return
		}
		if servedDirectly {
			return
		}
		// This request was a follower: the file is now cached.
		c.Touch(trackID)
		serveFromCache(w, r, c, trackID)
	}
}

func serveFromCache(w http.ResponseWriter, r *http.Request, c *cache.Cache, trackID string) {
	path := c.Path(trackID)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, trackID+".mp3", info.ModTime(), f)
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

func timeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"serverTimeMs":%d}`, time.Now().UnixMilli())
}
