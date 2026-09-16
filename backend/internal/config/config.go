// Package config loads and validates Hitsync's environment-based configuration.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds every environment-configurable setting for the backend.
type Config struct {
	// General
	AppDomain           string `env:"APP_DOMAIN,required"`
	TraefikNetwork      string `env:"TRAEFIK_NETWORK" envDefault:"proxy"`
	TraefikEntrypoint   string `env:"TRAEFIK_ENTRYPOINT" envDefault:"websecure"`
	TraefikCertResolver string `env:"TRAEFIK_CERTRESOLVER" envDefault:"letsencrypt"`
	// HTTPAddr is not part of the spec's env var table; it exists so local
	// dev (docs/local-development.md) can run the backend on a free port
	// without editing code, since the container always exposes 8080 either
	// way.
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	TZ       string `env:"TZ" envDefault:"Europe/Berlin"`

	// Secrets
	AppAccessCode string `env:"APP_ACCESS_CODE,required" secret:"true"`
	AdminPassword string `env:"ADMIN_PASSWORD,required" secret:"true"`
	JWTSecret     string `env:"JWT_SECRET,required" secret:"true"`

	// Navidrome
	NavidromeURL        string        `env:"NAVIDROME_URL,required"`
	NavidromeUsername   string        `env:"NAVIDROME_USERNAME,required"`
	NavidromePassword   string        `env:"NAVIDROME_PASSWORD,required" secret:"true"`
	NavidromeClientName string        `env:"NAVIDROME_CLIENT_NAME" envDefault:"hitsync"`
	NavidromeTimeout    time.Duration `env:"NAVIDROME_TIMEOUT" envDefault:"30s"`

	// Audio transcoding for player downloads
	AudioBitrate       int    `env:"AUDIO_BITRATE" envDefault:"128"` // kbit/s, MP3
	MediaCacheDir      string `env:"MEDIA_CACHE_DIR" envDefault:"/cache"`
	MediaCacheMaxBytes int64  `env:"MEDIA_CACHE_MAX_BYTES" envDefault:"2147483648"`
	FFmpegPath         string `env:"FFMPEG_PATH" envDefault:"ffmpeg"`

	// Song card collection, generated from Navidrome and hand-edited
	CardsFile string `env:"CARDS_FILE" envDefault:"/config/cards.json"`

	// Game rules and limits
	MaxConcurrentGames   int           `env:"MAX_CONCURRENT_GAMES" envDefault:"10"`
	MinPlayers           int           `env:"MIN_PLAYERS" envDefault:"1"` // 1 allows singleplayer
	MaxPlayers           int           `env:"MAX_PLAYERS" envDefault:"12"`
	DefaultTargetCards   int           `env:"DEFAULT_TARGET_CARDS" envDefault:"10"`
	DefaultStartTokens   int           `env:"DEFAULT_START_TOKENS" envDefault:"2"`
	DefaultMaxTokens     int           `env:"DEFAULT_MAX_TOKENS" envDefault:"5"`
	RuleEnableSongGuess  bool          `env:"RULE_ENABLE_SONG_GUESS" envDefault:"true"`
	TurnPlacementTimeout time.Duration `env:"TURN_PLACEMENT_TIMEOUT" envDefault:"90s"`
	TurnChallengeWindow  time.Duration `env:"TURN_CHALLENGE_WINDOW" envDefault:"5s"`
	RevealDuration       time.Duration `env:"REVEAL_DURATION" envDefault:"8s"`
	TrackMinDuration     time.Duration `env:"TRACK_MIN_DURATION" envDefault:"45s"`
	TrackMaxDuration     time.Duration `env:"TRACK_MAX_DURATION" envDefault:"600s"`
	PlayerReconnectGrace time.Duration `env:"PLAYER_RECONNECT_GRACE" envDefault:"120s"`
	LobbyIdleTimeout     time.Duration `env:"LOBBY_IDLE_TIMEOUT" envDefault:"30m"`

	// Database
	PostgresUser     string `env:"POSTGRES_USER" envDefault:"hitsync"`
	PostgresPassword string `env:"POSTGRES_PASSWORD,required" secret:"true"`
	PostgresDB       string `env:"POSTGRES_DB" envDefault:"hitsync"`
	DatabaseURL      string `env:"DATABASE_URL"`
}

// Load parses environment variables into a Config and validates it.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing environment: %w", err)
	}
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@postgres:5432/%s?sslmode=disable",
			cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDB,
		)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate fails fast with a clear message when a required setting is missing or invalid.
func (c *Config) Validate() error {
	var errs []string

	requireLen := func(name, val string, minLen int) {
		if len(val) < minLen {
			errs = append(errs, fmt.Sprintf("%s must be at least %d characters", name, minLen))
		}
	}

	if c.AppDomain == "" {
		errs = append(errs, "APP_DOMAIN is required")
	}
	requireLen("APP_ACCESS_CODE", c.AppAccessCode, 4)
	requireLen("ADMIN_PASSWORD", c.AdminPassword, 8)
	requireLen("JWT_SECRET", c.JWTSecret, 32)
	requireLen("POSTGRES_PASSWORD", c.PostgresPassword, 8)

	if c.NavidromeURL == "" {
		errs = append(errs, "NAVIDROME_URL is required")
	}
	if c.NavidromeUsername == "" {
		errs = append(errs, "NAVIDROME_USERNAME is required")
	}
	if c.NavidromePassword == "" {
		errs = append(errs, "NAVIDROME_PASSWORD is required")
	}
	if c.CardsFile == "" {
		errs = append(errs, "CARDS_FILE is required")
	}
	if c.AudioBitrate < 32 || c.AudioBitrate > 320 {
		errs = append(errs, "AUDIO_BITRATE must be between 32 and 320")
	}
	if c.DefaultMaxTokens < 1 || c.DefaultMaxTokens > 10 {
		errs = append(errs, "DEFAULT_MAX_TOKENS must be between 1 and 10")
	}
	if c.DefaultStartTokens < 0 || c.DefaultStartTokens > c.DefaultMaxTokens {
		errs = append(errs, "DEFAULT_START_TOKENS must be between 0 and DEFAULT_MAX_TOKENS")
	}
	if c.MinPlayers < 1 {
		errs = append(errs, "MIN_PLAYERS must be at least 1")
	}
	if c.MaxPlayers < c.MinPlayers {
		errs = append(errs, "MAX_PLAYERS must be >= MIN_PLAYERS")
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, "LOG_LEVEL must be one of debug, info, warn, error")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// isLocalDomain reports whether domain (host, optionally with :port) refers
// to the local machine, so local dev can run over plain HTTP/WS without a
// certificate.
func isLocalDomain(domain string) bool {
	host := domain
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "127.0.0.1" || strings.HasSuffix(host, ".localhost")
}

func httpSchemeFor(domain string) string {
	if isLocalDomain(domain) {
		return "http"
	}
	return "https"
}

// AppOrigin returns the app's own origin, e.g. "https://hitsync.example.com"
// in production or "http://localhost:5173" in local dev.
func (c *Config) AppOrigin() string {
	return httpSchemeFor(c.AppDomain) + "://" + c.AppDomain
}

// Redacted returns a copy suitable for logging, with secret fields masked.
func (c *Config) Redacted() map[string]any {
	mask := func(s string) string {
		if s == "" {
			return ""
		}
		return "***REDACTED***"
	}
	return map[string]any{
		"APP_DOMAIN":           c.AppDomain,
		"TRAEFIK_NETWORK":      c.TraefikNetwork,
		"LOG_LEVEL":            c.LogLevel,
		"TZ":                   c.TZ,
		"APP_ACCESS_CODE":      mask(c.AppAccessCode),
		"ADMIN_PASSWORD":       mask(c.AdminPassword),
		"JWT_SECRET":           mask(c.JWTSecret),
		"NAVIDROME_URL":        c.NavidromeURL,
		"NAVIDROME_USERNAME":   c.NavidromeUsername,
		"AUDIO_BITRATE":        c.AudioBitrate,
		"MEDIA_CACHE_DIR":      c.MediaCacheDir,
		"NAVIDROME_PASSWORD":   mask(c.NavidromePassword),
		"POSTGRES_USER":        c.PostgresUser,
		"POSTGRES_PASSWORD":    mask(c.PostgresPassword),
		"POSTGRES_DB":          c.PostgresDB,
		"CARDS_FILE":           c.CardsFile,
		"MAX_CONCURRENT_GAMES": c.MaxConcurrentGames,
		"MIN_PLAYERS":          c.MinPlayers,
		"MAX_PLAYERS":          c.MaxPlayers,
	}
}
