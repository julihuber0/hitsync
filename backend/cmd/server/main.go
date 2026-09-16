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
	"strings"
	"syscall"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/broadcast"
	"github.com/julianhuber/hitsync/backend/internal/config"
	"github.com/julianhuber/hitsync/backend/internal/discogs"
	"github.com/julianhuber/hitsync/backend/internal/gamesvc"
	"github.com/julianhuber/hitsync/backend/internal/httpapi"
	"github.com/julianhuber/hitsync/backend/internal/library"
	"github.com/julianhuber/hitsync/backend/internal/livekit"
	"github.com/julianhuber/hitsync/backend/internal/navidrome"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
	"github.com/julianhuber/hitsync/backend/internal/years"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		runHealthcheckProbe()
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "resolve-year" {
		runResolveYearCLI(os.Args[2], os.Args[3])
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
	syncer := library.New(nav, st, log)
	go syncer.RunPeriodic(ctx, cfg.LibrarySyncInterval)

	discogsClient := discogs.New(discogs.Config{
		BaseURL:    cfg.DiscogsBaseURL,
		Token:      cfg.DiscogsToken,
		RatePerSec: cfg.DiscogsRatePerSec,
	})
	resolver := years.NewResolver(discogs.YearsAdapter{Client: discogsClient}, cfg.DiscogsEnabled, cfg.DiscogsCacheEntries, cfg.DiscogsCacheTTL)

	trackSource := gamesvc.NewTrackSource(st, resolver, syncer, cfg.TrackMinDuration, cfg.TrackMaxDuration, cfg.YearMaxBackdate)

	issuer := tokens.NewIssuer(cfg.JWTSecret)
	mediaSigner := tokens.NewMediaSigner(cfg.MediaSharedSecret)
	broadcaster := broadcast.New(cfg.MediaInternalURL)
	livekitTokens := livekit.NewTokenIssuer(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret, 2*time.Hour)

	gsCfg := gamesvc.Config{
		MinPlayers: cfg.MinPlayers, MaxPlayers: cfg.MaxPlayers, MaxConcurrentGames: cfg.MaxConcurrentGames,
		DefaultTargetCards: cfg.DefaultTargetCards, DefaultStartTokens: cfg.DefaultStartTokens,
		MaxTokens: cfg.MaxTokens, EnableSongGuess: cfg.RuleEnableSongGuess,
		TurnPlacementTimeout: cfg.TurnPlacementTimeout, DisconnectedPlacementTimeout: 20 * time.Second,
		TurnChallengeWindow: cfg.TurnChallengeWindow, RevealDuration: cfg.RevealDuration,
		PreparingCap: 8 * time.Second, StartAtLeadMs: 400,
		PlayerReconnectGrace: cfg.PlayerReconnectGrace, LobbyIdleTimeout: cfg.LobbyIdleTimeout,
		SkipRateLimit:      10 * time.Second,
		YearLookaheadDepth: cfg.YearLookaheadDepth, YearLookupTimeout: cfg.YearLookupTimeout,
		AppDomain: cfg.AppDomain, LiveKitURL: cfg.LiveKitURL, LiveKitTokenTTL: 2 * time.Hour, MediaTTL: 30 * time.Minute,
	}
	manager := gamesvc.NewManager(gsCfg, st, trackSource, mediaSigner, broadcaster, livekitTokens, issuer, log)
	manager.RehydrateFromSnapshots(ctx)
	go manager.RunJanitor(ctx)

	api := httpapi.New(cfg, issuer, st, manager, syncer, resolver, discogsClient, nav, log)

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

// runResolveYearCLI is the §19 build-order CLI subcommand for manually
// resolving a title/artist pair and printing both year sources.
func runResolveYearCLI(title, artist string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	discogsClient := discogs.New(discogs.Config{
		BaseURL: cfg.DiscogsBaseURL, Token: cfg.DiscogsToken,
		RatePerSec: cfg.DiscogsRatePerSec,
	})
	res, err := discogsClient.Resolve(context.Background(), title, artist)
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve error:", err)
		os.Exit(1)
	}
	if res == nil {
		fmt.Printf("No Discogs match for %q by %q\n", title, artist)
		return
	}
	fmt.Printf("Title: %s\nArtist: %s\nDiscogs year: %d\nSource: %s\n", title, artist, res.Year, res.Source)
}
