// Command server is the Hitsync backend: REST API, WebSocket hub, and game
// manager (§3, §19).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/cards"
	"github.com/julianhuber/hitsync/backend/internal/config"
	"github.com/julianhuber/hitsync/backend/internal/gamesvc"
	"github.com/julianhuber/hitsync/backend/internal/httpapi"
	"github.com/julianhuber/hitsync/backend/internal/media"
	"github.com/julianhuber/hitsync/backend/internal/navidrome"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		runHealthcheckProbe()
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(log)
	log.Info("starting hitsync backend", "config", cfg.Redacted())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	nav := navidrome.New(cfg.NavidromeURL, cfg.NavidromeUsername, cfg.NavidromePassword, cfg.NavidromeClientName, cfg.NavidromeTimeout)
	// Games draw from the card collection once the startup scan has merged
	// the Navidrome library into the cards file (or, if Navidrome is
	// unreachable, from the existing file as it is).
	collection := cards.NewCollection(cfg.CardsFile, nav, cfg.TrackMinDuration, cfg.TrackMaxDuration, log)
	go func() { _, _ = collection.Scan(ctx) }()
	trackSource := gamesvc.NewTrackSource(collection)

	// Files are keyed by output format so a changed AUDIO_BITRATE (or files
	// left by an older deployment) are never served in place of fresh ones.
	mediaCacheDir := filepath.Join(cfg.MediaCacheDir, fmt.Sprintf("mp3-%dk", cfg.AudioBitrate))
	mediaCache, err := media.NewCache(mediaCacheDir, cfg.MediaCacheMaxBytes)
	if err != nil {
		log.Error("failed to initialise media cache", "error", err)
		os.Exit(1)
	}
	transcoder := media.NewTranscoder(media.TranscoderConfig{
		Cache:         mediaCache,
		SourceURL:     nav.RawStreamURL,
		FFmpegPath:    cfg.FFmpegPath,
		BitrateKbps:   cfg.AudioBitrate,
		MaxConcurrent: 2,
		Timeout:       3 * time.Minute,
	}, log)

	issuer := tokens.NewIssuer(cfg.JWTSecret)
	mediaSigner := tokens.NewMediaSigner(cfg.JWTSecret)

	gsCfg := gamesvc.Config{
		MinPlayers: cfg.MinPlayers, MaxPlayers: cfg.MaxPlayers, MaxConcurrentGames: cfg.MaxConcurrentGames,
		DefaultTargetCards: cfg.DefaultTargetCards, DefaultStartTokens: cfg.DefaultStartTokens,
		MaxTokens: cfg.MaxTokens, EnableSongGuess: cfg.RuleEnableSongGuess,
		TurnPlacementTimeout: cfg.TurnPlacementTimeout, DisconnectedPlacementTimeout: 20 * time.Second,
		TurnChallengeWindow: cfg.TurnChallengeWindow, RevealDuration: cfg.RevealDuration,
		PreparingCap: 8 * time.Second, StartAtLeadMs: 400,
		PlayerReconnectGrace: cfg.PlayerReconnectGrace, LobbyIdleTimeout: cfg.LobbyIdleTimeout,
		SkipRateLimit: 10 * time.Second,
		AppDomain:     cfg.AppDomain, MediaTTL: 30 * time.Minute,
	}
	manager := gamesvc.NewManager(gsCfg, st, trackSource, mediaSigner, transcoder, issuer, log)
	manager.RehydrateFromSnapshots(ctx)
	go manager.RunJanitor(ctx)

	api := httpapi.New(cfg, issuer, mediaSigner, transcoder, st, manager, collection, nav, log)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	manager.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
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

// runHealthcheckProbe backs the Dockerfile HEALTHCHECK, which execs the
// server binary itself rather than requiring curl in a distroless image.
func runHealthcheckProbe() {
	addr := ":8080"
	if cfg, err := config.Load(); err == nil {
		addr = cfg.HTTPAddr
	}
	host := "localhost" + addr
	if !strings.HasPrefix(addr, ":") {
		host = addr
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + "/healthz")
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
