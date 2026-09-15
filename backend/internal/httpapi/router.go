package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// Router builds the chi router for every REST endpoint plus the WebSocket
// upgrade (§12, §13).
func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(jsonBodyLimit)
	r.Use(cors("https://" + a.cfg.AppDomain))

	r.Get("/healthz", a.handleHealthz)

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/access", rateLimited(a.accessLimiter, a.handleAccess))
			r.Post("/logout", a.handleLogout)
		})

		r.Get("/config", a.requireAppScope(a.handleConfig))

		r.Route("/games", func(r chi.Router) {
			r.Post("/", a.requireAppScope(a.handleCreateGame))
			r.Post("/join", a.requireAppScope(a.handleJoinGame))
			r.Get("/{inviteCode}/preview", a.requireAppScope(a.handlePreview))
		})

		r.Route("/admin", func(r chi.Router) {
			r.Post("/login", rateLimited(a.adminLimiter, a.handleAdminLogin))
			r.Post("/logout", a.handleAdminLogout)

			r.Group(func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return a.requireAdminScope(next.ServeHTTP)
				})
				r.Get("/stats", a.handleAdminStats)
				r.Get("/tracks", a.handleAdminTracks)
				r.Get("/artists", a.handleAdminArtists)
				r.Get("/albums", a.handleAdminAlbums)
				r.Post("/exclusions", a.handleCreateExclusion)
				r.Delete("/exclusions/{kind}/{refId}", a.handleDeleteExclusion)
				r.Get("/exclusions", a.handleListExclusions)
				r.Put("/year-overrides/{trackId}", a.handleSetYearOverride)
				r.Delete("/year-overrides/{trackId}", a.handleDeleteYearOverride)
				r.Post("/resync-library", a.handleResyncLibrary)
				r.Post("/resolve-year/{trackId}", a.handleResolveYear)
				r.Get("/games", a.handleAdminGames)
				r.Delete("/games/{gameId}", a.handleForceEndGame)
			})
		})
	})

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.Serve(w, r, a.cfg.AppDomain, a.manager, a.log)
	})

	return r
}
