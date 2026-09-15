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
	MediaDomain         string `env:"MEDIA_DOMAIN,required"`
	TraefikNetwork      string `env:"TRAEFIK_NETWORK" envDefault:"proxy"`
	TraefikEntrypoint   string `env:"TRAEFIK_ENTRYPOINT" envDefault:"websecure"`
	TraefikCertResolver string `env:"TRAEFIK_CERTRESOLVER" envDefault:"letsencrypt"`
	LogLevel            string `env:"LOG_LEVEL" envDefault:"info"`
	TZ                  string `env:"TZ" envDefault:"Europe/Berlin"`

	// Secrets
	AppAccessCode     string `env:"APP_ACCESS_CODE,required" secret:"true"`
	AdminPassword     string `env:"ADMIN_PASSWORD,required" secret:"true"`
	JWTSecret         string `env:"JWT_SECRET,required" secret:"true"`
	MediaSharedSecret string `env:"MEDIA_SHARED_SECRET,required" secret:"true"`

	// Navidrome
	NavidromeURL        string        `env:"NAVIDROME_URL,required"`
	NavidromeUsername   string        `env:"NAVIDROME_USERNAME,required"`
	NavidromePassword   string        `env:"NAVIDROME_PASSWORD,required" secret:"true"`
	NavidromeClientName string        `env:"NAVIDROME_CLIENT_NAME" envDefault:"hitsync"`
	NavidromeTimeout    time.Duration `env:"NAVIDROME_TIMEOUT" envDefault:"30s"`

	// Audio
	AudioFormat        string `env:"AUDIO_FORMAT" envDefault:"mp3"`
	AudioBitrate       int    `env:"AUDIO_BITRATE" envDefault:"192"`
	MediaCacheDir      string `env:"MEDIA_CACHE_DIR" envDefault:"/cache"`
	MediaCacheMaxBytes int64  `env:"MEDIA_CACHE_MAX_BYTES" envDefault:"2147483648"`

	// Library and year resolution
	LibrarySyncInterval     time.Duration `env:"LIBRARY_SYNC_INTERVAL" envDefault:"6h"`
	MusicBrainzEnabled      bool          `env:"MUSICBRAINZ_ENABLED" envDefault:"true"`
	MusicBrainzBaseURL      string        `env:"MUSICBRAINZ_BASE_URL" envDefault:"https://musicbrainz.org/ws/2"`
	MusicBrainzContact      string        `env:"MUSICBRAINZ_CONTACT"`
	MusicBrainzRatePerSec   float64       `env:"MUSICBRAINZ_RATE_PER_SEC" envDefault:"1"`
	MusicBrainzMinScore     int           `env:"MUSICBRAINZ_MIN_SCORE" envDefault:"90"`
	MusicBrainzCacheEntries int           `env:"MUSICBRAINZ_CACHE_ENTRIES" envDefault:"2000"`
	MusicBrainzCacheTTL     time.Duration `env:"MUSICBRAINZ_CACHE_TTL" envDefault:"24h"`
	YearLookaheadDepth      int           `env:"YEAR_LOOKAHEAD_DEPTH" envDefault:"2"`
	YearLookupTimeout       time.Duration `env:"YEAR_LOOKUP_TIMEOUT" envDefault:"6s"`
	YearMaxBackdate         int           `env:"YEAR_MAX_BACKDATE" envDefault:"0"`

	// Game rules and limits
	MaxConcurrentGames   int           `env:"MAX_CONCURRENT_GAMES" envDefault:"10"`
	MinPlayers           int           `env:"MIN_PLAYERS" envDefault:"2"`
	MaxPlayers           int           `env:"MAX_PLAYERS" envDefault:"12"`
	DefaultTargetCards   int           `env:"DEFAULT_TARGET_CARDS" envDefault:"10"`
	DefaultStartTokens   int           `env:"DEFAULT_START_TOKENS" envDefault:"2"`
	MaxTokens            int           `env:"MAX_TOKENS" envDefault:"5"`
	RuleEnableSongGuess  bool          `env:"RULE_ENABLE_SONG_GUESS" envDefault:"true"`
	TurnPlacementTimeout time.Duration `env:"TURN_PLACEMENT_TIMEOUT" envDefault:"90s"`
	TurnChallengeWindow  time.Duration `env:"TURN_CHALLENGE_WINDOW" envDefault:"20s"`
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
	if c.MediaDomain == "" {
		errs = append(errs, "MEDIA_DOMAIN is required")
	}
	requireLen("APP_ACCESS_CODE", c.AppAccessCode, 4)
	requireLen("ADMIN_PASSWORD", c.AdminPassword, 8)
	requireLen("JWT_SECRET", c.JWTSecret, 32)
	requireLen("MEDIA_SHARED_SECRET", c.MediaSharedSecret, 32)
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
	if c.MusicBrainzEnabled && strings.TrimSpace(c.MusicBrainzContact) == "" {
		errs = append(errs, "MUSICBRAINZ_CONTACT is required when MUSICBRAINZ_ENABLED=true")
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
		"MEDIA_DOMAIN":         c.MediaDomain,
		"TRAEFIK_NETWORK":      c.TraefikNetwork,
		"LOG_LEVEL":            c.LogLevel,
		"TZ":                   c.TZ,
		"APP_ACCESS_CODE":      mask(c.AppAccessCode),
		"ADMIN_PASSWORD":       mask(c.AdminPassword),
		"JWT_SECRET":           mask(c.JWTSecret),
		"MEDIA_SHARED_SECRET":  mask(c.MediaSharedSecret),
		"NAVIDROME_URL":        c.NavidromeURL,
		"NAVIDROME_USERNAME":   c.NavidromeUsername,
		"NAVIDROME_PASSWORD":   mask(c.NavidromePassword),
		"POSTGRES_USER":        c.PostgresUser,
		"POSTGRES_PASSWORD":    mask(c.PostgresPassword),
		"POSTGRES_DB":          c.PostgresDB,
		"MUSICBRAINZ_ENABLED":  c.MusicBrainzEnabled,
		"MUSICBRAINZ_CONTACT":  c.MusicBrainzContact,
		"MAX_CONCURRENT_GAMES": c.MaxConcurrentGames,
		"MIN_PLAYERS":          c.MinPlayers,
		"MAX_PLAYERS":          c.MaxPlayers,
	}
}
