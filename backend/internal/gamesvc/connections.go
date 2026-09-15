package gamesvc

import (
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/livekit"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// registerConn attaches a newly-authenticated socket to its player (§13.4
// reconnection). Returns false if the player is no longer part of the game
// (e.g. removed after the reconnect grace period expired).
func (mg *ManagedGame) registerConn(playerID string, c *ws.Conn) bool {
	p := mg.g.Player(playerID)
	if p == nil {
		return false
	}
	p.Connected = true
	mg.conns[playerID] = c

	if t, ok := mg.reconnectTimers[playerID]; ok {
		t.Stop()
		delete(mg.reconnectTimers, playerID)
	}

	c.Send(ws.TypeState, mg.buildState(playerID))
	mg.resendActiveAudioBurst(c)
	mg.broadcastState()
	return true
}

// resendActiveAudioBurst replays the current track_prepare/track_start so a
// reconnecting client rejoins the currently-published LiveKit broadcast.
func (mg *ManagedGame) resendActiveAudioBurst(c *ws.Conn) {
	if mg.lastPrep == nil {
		return
	}
	if mg.g.Phase != game.PhasePlacing && mg.g.Phase != game.PhaseChallenging {
		return
	}
	livekitToken, err := mg.livekitTokens.Issue(livekit.RoomName(mg.id), c.PlayerID)
	if err != nil {
		mg.log.Error("failed to issue reconnect LiveKit token", "game_id", mg.id, "player_id", c.PlayerID, "error", err)
		return
	}
	c.Send(ws.TypeTrackPrepare, ws.TrackPreparePayload{
		PrepareID:    mg.lastPrep.prepareID,
		TrackID:      mg.lastPrep.trackID,
		LiveKitURL:   mg.cfg.LiveKitURL,
		LiveKitToken: livekitToken,
		RoomName:     livekit.RoomName(mg.id),
		DurationMs:   mg.lastPrep.durationMs,
	})
	c.Send(ws.TypeTrackStart, ws.TrackStartPayload{PrepareID: mg.lastPrep.prepareID})
}

// unregisterConn handles a socket closing: marks the player disconnected
// and arms the reconnect-grace timer (§8.10).
func (mg *ManagedGame) unregisterConn(playerID string, c *ws.Conn) {
	// Ignore a stale disconnect from a socket that has since been replaced
	// by a fresher reconnection.
	if mg.conns[playerID] != c {
		return
	}
	delete(mg.conns, playerID)

	p := mg.g.Player(playerID)
	if p == nil {
		return
	}
	p.Connected = false

	mg.reconnectTimers[playerID] = time.AfterFunc(mg.cfg.PlayerReconnectGrace, func() {
		mg.enqueue(func() { mg.onReconnectGraceExpired(playerID) })
	})
	mg.broadcastState()
}

func (mg *ManagedGame) onReconnectGraceExpired(playerID string) {
	delete(mg.reconnectTimers, playerID)
	p := mg.g.Player(playerID)
	if p == nil || p.Connected {
		return
	}

	wasActive := mg.g.Turn != nil && mg.g.Turn.ActivePlayerID == playerID
	ended := mg.g.RemovePlayer(playerID, mg.cfg.MinPlayers)
	if ended {
		mg.clearPhaseTimeout()
		mg.stopBroadcast()
		mg.broadcastState()
		mg.persistOnGameOver()
		return
	}
	if wasActive {
		mg.clearPhaseTimeout()
		mg.stopBroadcast()
		mg.g.Turn = nil
		mg.beginNextTurn(true)
		return
	}
	mg.broadcastState()
}
