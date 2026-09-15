package gamesvc

import (
	"encoding/json"

	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// dispatchMessage routes one authenticated client message to the right
// handler. Always called on the game's own goroutine (§13.1).
func dispatchMessage(mg *ManagedGame, c *ws.Conn, env ws.Envelope) {
	switch env.Type {
	case ws.TypePing:
		var p ws.PingPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			c.Send(ws.TypePong, ws.PongPayload{C0: p.C0, S: nowMs()})
		}

	case ws.TypeReady:
		var p ws.ReadyPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handleReady(c.PlayerID, p.PrepareID)
		}

	case ws.TypeUpdateSettings:
		var p ws.UpdateSettingsPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handleUpdateSettings(c.PlayerID, p.TargetCards, p.StartTokens, p.EnableSongGuess)
		}

	case ws.TypeStartGame:
		if err := mg.startGameFlow(c.PlayerID); err != nil {
			mg.sendError(c, "start_failed", err.Error())
		}

	case ws.TypePlaceCard:
		var p ws.PlaceCardPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handlePlaceCard(c.PlayerID, p.SlotIndex, p.TitleGuess, p.ArtistGuess)
		}

	case ws.TypePlacePreview:
		var p ws.PlacePreviewPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handlePlacePreview(c.PlayerID, p.SlotIndex)
		}

	case ws.TypeChallenge:
		var p ws.ChallengePayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handleChallenge(c.PlayerID, p.SlotIndex)
		}

	case ws.TypeChallengePreview:
		var p ws.ChallengePreviewPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handleChallengePreview(c.PlayerID, p.SlotIndex)
		}

	case ws.TypePassChallenge:
		mg.handlePassChallenge(c.PlayerID)

	case ws.TypeSkipTrack:
		mg.handleSkipTrack(c.PlayerID)

	case ws.TypeKickPlayer:
		var p ws.KickPlayerPayload
		if json.Unmarshal(env.Payload, &p) == nil {
			mg.handleKickPlayer(c.PlayerID, p.PlayerID)
		}

	case ws.TypeEndGame:
		mg.handleEndGame(c.PlayerID)

	case ws.TypePlayAgain:
		mg.handlePlayAgain(c.PlayerID)

	case ws.TypeLeave:
		wasActive := mg.g.Turn != nil && mg.g.Turn.ActivePlayerID == c.PlayerID
		ended := mg.g.RemovePlayer(c.PlayerID, mg.cfg.MinPlayers)
		mg.dropConn(c.PlayerID, "left")
		switch {
		case ended:
			mg.clearPhaseTimeout()
			mg.stopBroadcast()
			mg.persistOnGameOver()
			mg.broadcastState()
		case wasActive:
			mg.clearPhaseTimeout()
			mg.stopBroadcast()
			mg.g.Turn = nil
			mg.beginNextTurn(true)
		default:
			mg.broadcastState()
		}

	default:
		mg.sendError(c, "unknown_message_type", "unrecognised message type")
	}
}
