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
	r.Use(cors(a.cfg.AppOrigin()))

	r.Get("/healthz", a.handleHealthz)

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/access", a.handleAccessStatus)
			r.Post("/access", rateLimited(a.accessLimiter, a.handleAccess))
			r.Post("/logout", a.handleLogout)
		})

		r.Get("/config", a.requireAppScope(a.handleConfig))
		r.Get("/media/{token}", a.requireAppScope(a.handleMedia))

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
				r.Post("/cards/scan", a.handleScanCards)
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
