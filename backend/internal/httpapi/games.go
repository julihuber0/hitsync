package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/gamesvc"
)

// handleConfig implements GET /api/config (§12.1).
func (a *API) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"minPlayers":         a.cfg.MinPlayers,
		"maxPlayers":         a.cfg.MaxPlayers,
		"defaultTargetCards": a.cfg.DefaultTargetCards,
		"defaultStartTokens": a.cfg.DefaultStartTokens,
		"songGuessAvailable": a.cfg.RuleEnableSongGuess,
		"mediaBaseUrl":       "https://" + a.cfg.MediaDomain,
	})
}

// handleHealthz implements GET /healthz (§12.1, §17).
func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbOK := a.st.Ping(ctx) == nil
	navOK := a.nav.Ping(ctx) == nil

	libraryTracks, _ := a.st.CountTracks(ctx)
	eligibleTracks, _ := a.st.CountEligibleTracks(ctx, int(a.cfg.TrackMinDuration.Seconds()), int(a.cfg.TrackMaxDuration.Seconds()), a.syncer.LastSyncStart())

	status := http.StatusOK
	if !dbOK {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"status":         boolStatus(dbOK),
		"db":             boolStatus(dbOK),
		"navidrome":      boolStatus(navOK),
		"libraryTracks":  libraryTracks,
		"eligibleTracks": eligibleTracks,
	})
}

func boolStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

type createGameRequest struct {
	DisplayName string `json:"displayName"`
	Settings    *struct {
		TargetCards     *int  `json:"targetCards"`
		StartTokens     *int  `json:"startTokens"`
		EnableSongGuess *bool `json:"enableSongGuess"`
	} `json:"settings"`
}

func identityResponse(id gamesvc.PlayerIdentity) map[string]any {
	return map[string]any{
		"gameId":      id.GameID,
		"playerId":    id.PlayerID,
		"playerToken": id.PlayerToken,
		"inviteCode":  id.InviteCode,
	}
}

// handleCreateGame implements POST /api/games (§12.2).
func (a *API) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Malformed request body.")
		return
	}
	var targetCards, startTokens *int
	var enableSongGuess *bool
	if req.Settings != nil {
		targetCards, startTokens, enableSongGuess = req.Settings.TargetCards, req.Settings.StartTokens, req.Settings.EnableSongGuess
	}

	id, err := a.manager.CreateGame(req.DisplayName, targetCards, startTokens, enableSongGuess)
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, identityResponse(id))
}

type joinGameRequest struct {
	InviteCode  string `json:"inviteCode"`
	DisplayName string `json:"displayName"`
}

// handleJoinGame implements POST /api/games/join (§12.2).
func (a *API) handleJoinGame(w http.ResponseWriter, r *http.Request) {
	var req joinGameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Malformed request body.")
		return
	}
	id, err := a.manager.JoinGame(req.InviteCode, req.DisplayName)
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, identityResponse(id))
}

// handlePreview implements GET /api/games/{inviteCode}/preview (§12.2).
func (a *API) handlePreview(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "inviteCode")
	result := a.manager.Preview(code)
	writeJSON(w, http.StatusOK, map[string]any{
		"exists":      result.Exists,
		"phase":       result.Phase,
		"playerCount": result.PlayerCount,
		"maxPlayers":  result.MaxPlayers,
		"hostName":    result.HostName,
		"joinable":    result.Joinable,
	})
}

func writeGameError(w http.ResponseWriter, err error) {
	switch err {
	case gamesvc.ErrGameNotFound:
		writeError(w, http.StatusNotFound, "invalid_invite_code", "No game found with that code.")
	case gamesvc.ErrTooManyGames:
		writeError(w, http.StatusServiceUnavailable, "too_many_games", "The server is at capacity. Please try again shortly.")
	case game.ErrTooManyPlayers:
		writeError(w, http.StatusConflict, "game_full", "This game is full.")
	case game.ErrDuplicateName:
		writeError(w, http.StatusConflict, "duplicate_name", "That name is already taken in this game.")
	case game.ErrInvalidName:
		writeError(w, http.StatusBadRequest, "invalid_name", "Display names must be 2-20 characters.")
	case game.ErrWrongPhase:
		writeError(w, http.StatusConflict, "game_in_progress", "This game has already started.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
	}
}
