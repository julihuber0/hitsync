package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/julianhuber/hitsync/backend/internal/store"
)

// handleAdminStats implements GET /api/admin/stats (§12.3).
func (a *API) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	libraryTracks, _ := a.st.CountTracks(ctx)
	eligibleTracks, _ := a.st.CountEligibleTracks(ctx, int(a.cfg.TrackMinDuration.Seconds()), int(a.cfg.TrackMaxDuration.Seconds()), a.syncer.LastSyncStart())
	activeGames := a.manager.ListActiveGames()

	var lastSync *string
	if t := a.syncer.LastSyncStart(); !t.IsZero() {
		s := t.UTC().Format(time.RFC3339)
		lastSync = &s
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"libraryTracks":   libraryTracks,
		"eligibleTracks":  eligibleTracks,
		"activeGames":     len(activeGames),
		"lastLibrarySync": lastSync,
		"discogsCacheLen": a.resolver.CacheLen(),
	})
}

// handleAdminTracks implements GET /api/admin/tracks (§12.3). It never
// triggers Discogs lookups.
func (a *API) handleAdminTracks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	page, pageSize := parsePaging(r)

	var excluded *bool
	if v := r.URL.Query().Get("excluded"); v != "" {
		b := v == "true"
		excluded = &b
	}

	rows, total, err := a.st.SearchAdminTracks(r.Context(), q, excluded, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to search tracks.")
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":            row.ID,
			"title":         row.Title,
			"artist":        row.Artist,
			"album":         row.Album,
			"navidromeYear": row.NavidromeYear,
			"overrideYear":  row.OverrideYear,
			"excludedKind":  row.ExcludedKind,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": out, "total": total, "page": page, "pageSize": pageSize})
}

// handleAdminArtists implements GET /api/admin/artists (§12.3).
func (a *API) handleAdminArtists(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	opts, err := a.st.SearchArtists(r.Context(), q, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to search artists.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artists": refOptionsJSON(opts)})
}

// handleAdminAlbums implements GET /api/admin/albums (§12.3).
func (a *API) handleAdminAlbums(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	opts, err := a.st.SearchAlbums(r.Context(), q, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to search albums.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": refOptionsJSON(opts)})
}

func refOptionsJSON(opts []store.RefOption) []map[string]string {
	out := make([]map[string]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, map[string]string{"id": o.ID, "name": o.Name})
	}
	return out
}

type createExclusionRequest struct {
	Kind   string `json:"kind"`
	RefID  string `json:"refId"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

// handleCreateExclusion implements POST /api/admin/exclusions (§12.3).
func (a *API) handleCreateExclusion(w http.ResponseWriter, r *http.Request) {
	var req createExclusionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Malformed request body.")
		return
	}
	kind := store.ExclusionKind(req.Kind)
	if kind != store.ExclusionTrack && kind != store.ExclusionAlbum && kind != store.ExclusionArtist {
		writeError(w, http.StatusBadRequest, "invalid_kind", "kind must be track, album, or artist.")
		return
	}
	if req.RefID == "" || req.Label == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "refId and label are required.")
		return
	}
	if err := a.st.AddExclusion(r.Context(), kind, req.RefID, req.Label, req.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to add exclusion.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// handleDeleteExclusion implements DELETE /api/admin/exclusions/{kind}/{refId} (§12.3).
func (a *API) handleDeleteExclusion(w http.ResponseWriter, r *http.Request) {
	kind := store.ExclusionKind(chi.URLParam(r, "kind"))
	refID := chi.URLParam(r, "refId")
	if err := a.st.RemoveExclusion(r.Context(), kind, refID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to remove exclusion.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleListExclusions implements GET /api/admin/exclusions (§12.3).
func (a *API) handleListExclusions(w http.ResponseWriter, r *http.Request) {
	var kind *store.ExclusionKind
	if v := r.URL.Query().Get("kind"); v != "" {
		k := store.ExclusionKind(v)
		kind = &k
	}
	page, pageSize := parsePaging(r)

	exclusions, err := a.st.ListExclusions(r.Context(), kind, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list exclusions.")
		return
	}
	out := make([]map[string]any, 0, len(exclusions))
	for _, e := range exclusions {
		out = append(out, map[string]any{
			"kind": e.Kind, "refId": e.RefID, "label": e.Label, "reason": e.Reason, "createdAt": e.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"exclusions": out})
}

type yearOverrideRequest struct {
	Year int    `json:"year"`
	Note string `json:"note"`
}

// handleSetYearOverride implements PUT /api/admin/year-overrides/{trackId} (§12.3).
func (a *API) handleSetYearOverride(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "trackId")
	var req yearOverrideRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Malformed request body.")
		return
	}
	if err := a.st.SetYearOverride(r.Context(), trackID, req.Year, req.Note); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to set year override.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleDeleteYearOverride implements DELETE /api/admin/year-overrides/{trackId} (§12.3).
func (a *API) handleDeleteYearOverride(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "trackId")
	if err := a.st.DeleteYearOverride(r.Context(), trackID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete year override.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleResyncLibrary implements POST /api/admin/resync-library (§12.3).
func (a *API) handleResyncLibrary(w http.ResponseWriter, r *http.Request) {
	go a.syncer.SyncOnce(context.Background())
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// handleResolveYear implements POST /api/admin/resolve-year/{trackId} (§12.3):
// an on-demand lookup bypassing the in-memory cache, for diagnosing a
// suspicious card.
func (a *API) handleResolveYear(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "trackId")
	tr, err := a.st.GetTrack(r.Context(), trackID)
	if err != nil || tr == nil {
		writeError(w, http.StatusNotFound, "track_not_found", "No such track.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	dgResult, err := a.discogsClient.Resolve(ctx, tr.Title, tr.Artist)

	var dgYear *int
	var dgSource string
	if err == nil && dgResult != nil {
		y := dgResult.Year
		dgYear = &y
		dgSource = dgResult.Source
	}

	override, _ := a.st.GetYearOverride(r.Context(), trackID)
	var overrideYear *int
	if override != nil {
		overrideYear = &override.Year
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"trackId":       trackID,
		"navidromeYear": tr.NavidromeYear,
		"discogs": map[string]any{
			"year":   dgYear,
			"source": dgSource,
			"error":  errString(err),
		},
		"overrideYear": overrideYear,
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// handleAdminGames implements GET /api/admin/games (§12.3).
func (a *API) handleAdminGames(w http.ResponseWriter, r *http.Request) {
	games := a.manager.ListActiveGames()
	out := make([]map[string]any, 0, len(games))
	for _, g := range games {
		out = append(out, map[string]any{
			"gameId": g.GameID, "inviteCode": g.InviteCode, "phase": g.Phase,
			"playerCount": g.PlayerCount, "turnNumber": g.TurnNumber,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": out})
}

// handleForceEndGame implements DELETE /api/admin/games/{gameId} (§12.3).
func (a *API) handleForceEndGame(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "gameId")
	if err := a.manager.ForceEndGame(gameID); err != nil {
		writeError(w, http.StatusNotFound, "game_not_found", "No such game.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func parsePaging(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 {
		page = 0
	}
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	return page, pageSize
}
