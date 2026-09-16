// Package httpapi implements the REST surface (§12): the chi router, access
// gate, game creation/join, and admin endpoints.
package httpapi

import (
	"log/slog"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/cards"
	"github.com/julianhuber/hitsync/backend/internal/config"
	"github.com/julianhuber/hitsync/backend/internal/gamesvc"
	"github.com/julianhuber/hitsync/backend/internal/media"
	"github.com/julianhuber/hitsync/backend/internal/navidrome"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
)

// API holds every dependency the HTTP handlers need.
type API struct {
	cfg         *config.Config
	issuer      *tokens.Issuer
	mediaSigner *tokens.MediaSigner
	transcoder  *media.Transcoder
	st          *store.Store
	manager     *gamesvc.Manager
	cards       *cards.Collection
	nav         *navidrome.Client
	log         *slog.Logger

	accessLimiter *ipRateLimiter
	adminLimiter  *ipRateLimiter
}

// New creates an API instance.
func New(cfg *config.Config, issuer *tokens.Issuer, mediaSigner *tokens.MediaSigner, transcoder *media.Transcoder, st *store.Store, manager *gamesvc.Manager, collection *cards.Collection, nav *navidrome.Client, log *slog.Logger) *API {
	return &API{
		cfg:           cfg,
		issuer:        issuer,
		mediaSigner:   mediaSigner,
		transcoder:    transcoder,
		st:            st,
		manager:       manager,
		cards:         collection,
		nav:           nav,
		log:           log,
		accessLimiter: newIPRateLimiter(10, time.Minute),
		adminLimiter:  newIPRateLimiter(10, time.Minute),
	}
}
