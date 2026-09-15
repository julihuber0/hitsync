// Package config loads the media service's environment-based configuration.
package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds every environment-configurable setting for the media service.
type Config struct {
	MediaDomain       string `env:"MEDIA_DOMAIN,required"`
	AppDomain         string `env:"APP_DOMAIN,required"`
	MediaSharedSecret string `env:"MEDIA_SHARED_SECRET,required" secret:"true"`

	NavidromeURL        string `env:"NAVIDROME_URL,required"`
	NavidromeUsername   string `env:"NAVIDROME_USERNAME,required"`
	NavidromePassword   string `env:"NAVIDROME_PASSWORD,required" secret:"true"`
	NavidromeClientName string `env:"NAVIDROME_CLIENT_NAME" envDefault:"hitsync"`

	AudioFormat  string `env:"AUDIO_FORMAT" envDefault:"mp3"`
	AudioBitrate int    `env:"AUDIO_BITRATE" envDefault:"192"`

	MediaCacheDir      string `env:"MEDIA_CACHE_DIR" envDefault:"/cache"`
	MediaCacheMaxBytes int64  `env:"MEDIA_CACHE_MAX_BYTES" envDefault:"2147483648"`

	MediaDirectPort string `env:"MEDIA_DIRECT_PORT" envDefault:""`
	MediaTLSCert    string `env:"MEDIA_TLS_CERT_FILE" envDefault:""`
	MediaTLSKey     string `env:"MEDIA_TLS_KEY_FILE" envDefault:""`

	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// HTTPAddr is not part of the spec's env var table; it exists so local
	// dev (docs/local-development.md) can run the media service on a free
	// port without editing code, since the container always exposes 8090
	// either way.
	HTTPAddr string `env:"MEDIA_HTTP_ADDR" envDefault:":8090"`
}

// Load parses and validates the media service configuration.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing environment: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate fails fast with a clear message when a required setting is missing or invalid.
func (c *Config) Validate() error {
	var errs []string
	if c.MediaDomain == "" {
		errs = append(errs, "MEDIA_DOMAIN is required")
	}
	if c.AppDomain == "" {
		errs = append(errs, "APP_DOMAIN is required")
	}
	if len(c.MediaSharedSecret) < 32 {
		errs = append(errs, "MEDIA_SHARED_SECRET must be at least 32 characters")
	}
	if c.NavidromeURL == "" {
		errs = append(errs, "NAVIDROME_URL is required")
	}
	if c.NavidromeUsername == "" {
		errs = append(errs, "NAVIDROME_USERNAME is required")
	}
	if c.NavidromePassword == "" {
		errs = append(errs, "NAVIDROME_PASSWORD is required")
	}
	if (c.MediaTLSCert == "") != (c.MediaTLSKey == "") {
		errs = append(errs, "MEDIA_TLS_CERT_FILE and MEDIA_TLS_KEY_FILE must both be set or both be empty")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// isLocalDomain reports whether domain (host, optionally with :port) refers
// to the local machine, so local dev can run over plain HTTP without a
// certificate.
func isLocalDomain(domain string) bool {
	host := domain
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "127.0.0.1" || strings.HasSuffix(host, ".localhost")
}

// AppOrigin returns the app's own origin, e.g. "https://hitsync.example.com"
// in production or "http://localhost:5173" in local dev, for the media
// service's CORS header.
func (c *Config) AppOrigin() string {
	if isLocalDomain(c.AppDomain) {
		return "http://" + c.AppDomain
	}
	return "https://" + c.AppDomain
}
