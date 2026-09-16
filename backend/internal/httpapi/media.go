package httpapi

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// handleMedia implements GET /api/media/{token}: the transcoded MP3 a player
// downloads for the current or next turn. The token is minted by the game and
// names the track; the first request for an uncached track waits for its
// transcode.
func (a *API) handleMedia(w http.ResponseWriter, r *http.Request) {
	payload, err := a.mediaSigner.Verify(chi.URLParam(r, "token"))
	if err != nil {
		writeError(w, http.StatusForbidden, "invalid_media_token", "This media link is invalid or has expired.")
		return
	}

	path, err := a.transcoder.Ensure(r.Context(), payload.TrackID)
	if err != nil {
		if r.Context().Err() != nil {
			return // the client went away; the transcode continues for others
		}
		a.log.Warn("failed to prepare media", "game_id", payload.GameID, "track_id", payload.TrackID, "error", err)
		writeError(w, http.StatusBadGateway, "media_unavailable", "The track could not be prepared.")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "media_unavailable", "The track could not be prepared.")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	// Clients keep the file in memory only for the turn; a disk-cached copy
	// in the browser would outlive that.
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
