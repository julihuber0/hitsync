// Package httpapi implements the REST surface (§12): the chi router, access
// gate, game creation/join, and admin endpoints.
package httpapi

import (
	"log/slog"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/config"
	"github.com/julianhuber/hitsync/backend/internal/gamesvc"
	"github.com/julianhuber/hitsync/backend/internal/library"
	"github.com/julianhuber/hitsync/backend/internal/musicbrainz"
	"github.com/julianhuber/hitsync/backend/internal/navidrome"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
	"github.com/julianhuber/hitsync/backend/internal/years"
)

// API holds every dependency the HTTP handlers need.
type API struct {
	cfg      *config.Config
	issuer   *tokens.Issuer
	st       *store.Store
	manager  *gamesvc.Manager
	syncer   *library.Syncer
	resolver *years.Resolver
	mbClient *musicbrainz.Client
	nav      *navidrome.Client
	log      *slog.Logger

	accessLimiter *ipRateLimiter
	adminLimiter  *ipRateLimiter
}

// New creates an API instance.
func New(cfg *config.Config, issuer *tokens.Issuer, st *store.Store, manager *gamesvc.Manager, syncer *library.Syncer, resolver *years.Resolver, mbClient *musicbrainz.Client, nav *navidrome.Client, log *slog.Logger) *API {
	return &API{
		cfg:           cfg,
		issuer:        issuer,
		st:            st,
		manager:       manager,
		syncer:        syncer,
		resolver:      resolver,
		mbClient:      mbClient,
		nav:           nav,
		log:           log,
		accessLimiter: newIPRateLimiter(10, time.Minute),
		adminLimiter:  newIPRateLimiter(10, time.Minute),
	}
}
