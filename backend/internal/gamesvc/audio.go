package gamesvc

import (
	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// Every client plays its own downloaded copy of a turn's track. The server
// only decides which track plays and when:
//
//   - track_preload: while a turn runs, clients fetch the next turn's track.
//   - track_prepare: PREPARING; clients make sure the track is downloaded and
//     answer with ready.
//   - track_start:   PLACING; a shared start instant on the server clock that
//     clients align their playback position to.
//   - track_stop:    the round is over; clients fade out and discard the file.

// TrackWarmer transcodes a track into the server's media cache ahead of time,
// so players' downloads of it don't wait on FFmpeg.
type TrackWarmer interface {
	Warm(trackID string)
}

// stopFadeMs is the client-side fade-out applied on track_stop.
const stopFadeMs = 400

// activeTrack is the current turn's track, kept so a reconnecting client can
// be brought back into the same playback (§13.4).
type activeTrack struct {
	prepareID  string
	trackID    string
	durationMs int64
	// startAtServerMs is 0 until PLACING begins.
	startAtServerMs int64
}

func turnInProgress(phase game.Phase) bool {
	switch phase {
	case game.PhasePreparing, game.PhasePlacing, game.PhaseChallenging, game.PhaseRevealing:
		return true
	}
	return false
}

// mediaURL returns the download URL for trackID, authorised for this game.
func (mg *ManagedGame) mediaURL(trackID string) (string, bool) {
	token, err := mg.mediaSigner.Issue(mg.id, trackID, mg.cfg.MediaTTL)
	if err != nil {
		mg.log.Error("failed to issue media token", "game_id", mg.id, "track_id", trackID, "error", err)
		return "", false
	}
	return "/api/media/" + token, true
}

func (mg *ManagedGame) sendTrackPrepare(c *ws.Conn, mediaURL string) {
	t := mg.track
	c.Send(ws.TypeTrackPrepare, ws.TrackPreparePayload{PrepareID: t.prepareID, TrackID: t.trackID, MediaURL: mediaURL, DurationMs: t.durationMs})
}

func (mg *ManagedGame) sendTrackStart(c *ws.Conn) {
	c.Send(ws.TypeTrackStart, ws.TrackStartPayload{PrepareID: mg.track.prepareID, StartAtServerMs: mg.track.startAtServerMs})
}

// stopTrack ends the current track on every client, which then discards its
// local copy. Idempotent.
func (mg *ManagedGame) stopTrack() {
	if mg.track == nil {
		return
	}
	for _, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeTrackStop, ws.TrackStopPayload{PrepareID: mg.track.prepareID, FadeMs: stopFadeMs})
	}
	mg.track = nil
}

// announceUpcoming tells clients to download the next turn's track, so that
// turn starts without a download delay.
func (mg *ManagedGame) announceUpcoming() {
	if mg.upcoming == nil || !turnInProgress(mg.g.Phase) {
		return
	}
	next := mg.upcoming.Card.TrackID
	url, ok := mg.mediaURL(next)
	if !ok {
		return
	}
	for _, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeTrackPreload, ws.TrackPreloadPayload{TrackID: next, MediaURL: url})
	}
}

// resendAudio replays the current turn's audio messages to a reconnecting
// client so it resumes the same playback, in sync, and preloads the next
// track again.
func (mg *ManagedGame) resendAudio(c *ws.Conn) {
	if !turnInProgress(mg.g.Phase) {
		return
	}
	if mg.track != nil {
		if url, ok := mg.mediaURL(mg.track.trackID); ok {
			mg.sendTrackPrepare(c, url)
			if mg.track.startAtServerMs != 0 {
				mg.sendTrackStart(c)
			}
		}
	}
	if mg.upcoming != nil {
		next := mg.upcoming.Card.TrackID
		if url, ok := mg.mediaURL(next); ok {
			c.Send(ws.TypeTrackPreload, ws.TrackPreloadPayload{TrackID: next, MediaURL: url})
		}
	}
}
