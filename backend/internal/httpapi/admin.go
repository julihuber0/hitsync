package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/julianhuber/hitsync/backend/internal/cards"
)

// handleAdminStats implements GET /api/admin/stats (§12.3).
func (a *API) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats := a.cards.Stats()
	var lastScan *string
	if !stats.LastScan.IsZero() {
		s := stats.LastScan.UTC().Format(time.RFC3339)
		lastScan = &s
	}
	var lastScanError *string
	if stats.LastScanError != "" {
		lastScanError = &stats.LastScanError
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cardsFile":        a.cfg.CardsFile,
		"cards":            stats.Total,
		"playableCards":    stats.Playable,
		"excludedCards":    stats.Excluded,
		"cardsMissingYear": stats.MissingYear,
		"lastScan":         lastScan,
		"lastScanError":    lastScanError,
		"scanning":         stats.Scanning,
		"activeGames":      len(a.manager.ListActiveGames()),
	})
}

// handleScanCards implements POST /api/admin/cards/scan: scans the Navidrome
// library, merges it into the cards file (keeping hitsyncyear and excluded),
// and makes the result the active collection. It also applies hand edits
// made to the file since the last scan. Responds when the scan is done.
func (a *API) handleScanCards(w http.ResponseWriter, r *http.Request) {
	// Detached from the request: an admin closing the tab should not abort a
	// scan halfway.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	summary, err := a.cards.Scan(ctx)
	switch {
	case errors.Is(err, cards.ErrScanInProgress):
		writeError(w, http.StatusConflict, "scan_in_progress", "A library scan is already running.")
	case err != nil:
		writeError(w, http.StatusBadGateway, "scan_failed", err.Error())
	default:
		writeJSON(w, http.StatusOK, summary)
	}
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
