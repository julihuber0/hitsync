package gamesvc

import (
	"context"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/livekit"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// startGameFlow validates and starts the game (§8.4), dealing starting
// cards, then kicks off the first turn.
func (mg *ManagedGame) startGameFlow(hostID string) error {
	if hostID != mg.g.HostID {
		return game.ErrNotHost
	}
	if err := mg.g.StartGame(mg.cfg.MinPlayers); err != nil {
		return err
	}
	mg.startedAt = time.Now()

	for _, p := range mg.g.Players {
		cand, err := mg.trackSource.DrawAndResolve(context.Background(), context.Background(), mg.usedAndPendingIDs())
		if err != nil {
			mg.log.Error("failed to deal starting card", "game_id", mg.id, "player_id", p.ID, "error", err)
			continue
		}
		_ = mg.g.DealStartingCard(p.ID, cand.Card)
	}

	mg.ensureLookahead()
	mg.beginNextTurn(true)
	mg.broadcastState()
	return nil
}

// beginNextTurn draws (or reuses, for a skip) the next track and enters
// PREPARING (§8.5).
func (mg *ManagedGame) beginNextTurn(advance bool) {
	if advance {
		if err := mg.g.NextTurn(); err != nil {
			return
		}
	}
	mg.turnCount++

	cand, err := mg.nextCandidate()
	if err != nil || cand == nil {
		mg.log.Error("track pool exhausted", "game_id", mg.id, "error", err)
		_ = mg.g.EndGame(mg.g.HostID)
		mg.clearPhaseTimeout()
		mg.broadcastState()
		mg.persistOnGameOver()
		return
	}
	card := cand.Card
	mg.currentYearSource = cand.YearSource

	if err := mg.g.BeginTurn(card); err != nil {
		return
	}
	mg.pendingReady = map[string]bool{}
	mg.pendingSongGuess = nil

	prepareID := newID()
	durationMs := int64(cand.DurationSec) * 1000
	mediaToken, err := mg.mediaSigner.Issue(mg.id, card.TrackID, mg.cfg.MediaTTL)
	if err != nil {
		mg.endForBroadcastFailure("could not authorise audio broadcast", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	err = mg.broadcaster.Prepare(ctx, mg.id, card.TrackID, mediaToken)
	cancel()
	if err != nil {
		mg.endForBroadcastFailure("could not prepare audio broadcast", err)
		return
	}
	mg.lastPrep = &lastPrepare{prepareID: prepareID, trackID: card.TrackID, durationMs: durationMs}

	var guessOpts *ws.GuessOptions
	if mg.g.Settings.EnableSongGuess {
		titles, artists, err := mg.trackSource.SongGuessOptions(context.Background(), &card)
		if err == nil {
			guessOpts = &ws.GuessOptions{Titles: titles, Artists: artists}
		}
	}

	activeID := mg.g.Turn.ActivePlayerID
	for playerID, c := range mg.conns {
		if c == nil {
			continue
		}
		livekitToken, err := mg.livekitTokens.Issue(livekit.RoomName(mg.id), playerID)
		if err != nil {
			mg.log.Error("failed to issue LiveKit token", "game_id", mg.id, "player_id", playerID, "error", err)
			continue
		}
		payload := ws.TrackPreparePayload{PrepareID: prepareID, TrackID: card.TrackID, LiveKitURL: mg.cfg.LiveKitURL, LiveKitToken: livekitToken, RoomName: livekit.RoomName(mg.id), DurationMs: durationMs}
		if playerID == activeID {
			payload.GuessOptions = guessOpts
		}
		c.Send(ws.TypeTrackPrepare, payload)
	}

	mg.schedulePhaseTimeout(mg.cfg.PreparingCap, mg.onPrepareTimeout)
	mg.broadcastState()
}

func (mg *ManagedGame) handleReady(playerID, prepareID string) {
	if mg.g.Phase != game.PhasePreparing || mg.lastPrep == nil || prepareID != mg.lastPrep.prepareID {
		return
	}
	mg.pendingReady[playerID] = true
	if mg.allConnectedReady() {
		mg.beginPlacing()
	}
}

func (mg *ManagedGame) allConnectedReady() bool {
	for playerID, c := range mg.conns {
		if c == nil {
			continue
		}
		if !mg.pendingReady[playerID] {
			return false
		}
	}
	return true
}

func (mg *ManagedGame) onPrepareTimeout() {
	mg.beginPlacing()
}

// beginPlacing transitions PREPARING -> PLACING and starts the server-side
// LiveKit audio publication. No browser chooses or corrects a playback clock.
func (mg *ManagedGame) beginPlacing() {
	if err := mg.g.BeginPlacing(); err != nil {
		return
	}
	mediaToken, err := mg.mediaSigner.Issue(mg.id, mg.lastPrep.trackID, mg.cfg.MediaTTL)
	if err != nil {
		mg.endForBroadcastFailure("could not authorise audio broadcast", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = mg.broadcaster.Start(ctx, mg.id, mg.lastPrep.trackID, mediaToken)
	cancel()
	if err != nil {
		mg.endForBroadcastFailure("could not start audio broadcast", err)
		return
	}

	for _, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeTrackStart, ws.TrackStartPayload{PrepareID: mg.lastPrep.prepareID})
	}

	// PLACING has no timeout while the active player is connected: the track
	// loops until they submit or the host skips (§8.5). A disconnected active
	// player still gets a bounded fallback so the game can't stall forever.
	if active := mg.g.ActivePlayer(); active != nil && !active.Connected {
		mg.schedulePhaseTimeout(mg.cfg.DisconnectedPlacementTimeout, mg.onPlacementTimeout)
	} else {
		mg.clearPhaseTimeout()
	}
	mg.broadcastState()
}

func (mg *ManagedGame) handlePlacePreview(playerID string, slotIndex int) {
	if err := mg.g.PreviewPlacement(playerID, slotIndex); err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "invalid_placement_preview", err.Error())
		}
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) handlePlaceCard(playerID string, slotIndex int, titleGuess, artistGuess *string) {
	if titleGuess != nil && artistGuess != nil {
		mg.pendingSongGuess = &pendingSongGuess{titleGuess: *titleGuess, artistGuess: *artistGuess}
	}
	skip, err := mg.g.PlaceCard(playerID, slotIndex)
	if err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "invalid_placement", err.Error())
		}
		return
	}
	mg.afterPlacement(skip)
}

func (mg *ManagedGame) onPlacementTimeout() {
	skip, err := mg.g.PlacementTimeout()
	if err != nil {
		return
	}
	mg.afterPlacement(skip)
}

func (mg *ManagedGame) afterPlacement(skipChallenge bool) {
	if skipChallenge {
		mg.clearPhaseTimeout()
		mg.beginRevealing()
		return
	}
	mg.schedulePhaseTimeout(mg.cfg.TurnChallengeWindow, mg.onChallengeTimeout)
	mg.broadcastState()
}

func (mg *ManagedGame) handleChallenge(playerID string, slotIndex int) {
	done, err := mg.g.Challenge(playerID, slotIndex)
	if err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "invalid_challenge", err.Error())
		}
		return
	}
	if done {
		mg.clearPhaseTimeout()
		mg.beginRevealing()
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) handleChallengePreview(playerID string, slotIndex int) {
	if err := mg.g.PreviewChallenge(playerID, slotIndex); err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "invalid_steal_preview", err.Error())
		}
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) handlePassChallenge(playerID string) {
	done, err := mg.g.PassChallenge(playerID)
	if err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "invalid_pass", err.Error())
		}
		return
	}
	if done {
		mg.clearPhaseTimeout()
		mg.beginRevealing()
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) onChallengeTimeout() {
	_ = mg.g.ChallengeTimeout()
	mg.beginRevealing()
}

// beginRevealing resolves the turn, applies the song-guess bonus, broadcasts
// the reveal, and either ends the game or arms the next-turn timer (§8.5,
// §8.6, §8.9).
func (mg *ManagedGame) beginRevealing() {
	for _, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeTrackStop, ws.TrackStopPayload{FadeMs: 400})
	}
	mg.stopBroadcast()

	reveal, err := mg.g.Resolve()
	if err != nil {
		return
	}

	if mg.pendingSongGuess != nil {
		card := mg.g.Turn
		correct := card != nil &&
			normalizedEquals(mg.pendingSongGuess.titleGuess, reveal.Card.Title) &&
			normalizedEquals(mg.pendingSongGuess.artistGuess, reveal.Card.Artist)
		if active := mg.g.Player(reveal.ActivePlayerID); active != nil {
			reveal.ApplySongGuessReward(active, mg.g.Settings.MaxTokens, correct)
		}
	}
	mg.pendingSongGuess = nil

	payload := buildRevealPayload(reveal, mg.currentYearSource)
	for _, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeReveal, payload)
	}

	if mg.g.Phase == game.PhaseGameOver {
		mg.clearPhaseTimeout()
		mg.broadcastState()
		mg.persistOnGameOver()
		return
	}

	mg.schedulePhaseTimeout(mg.cfg.RevealDuration, mg.onRevealTimeout)
	mg.broadcastState()
}

func normalizedEquals(a, b string) bool {
	return trimLower(a) == trimLower(b)
}

func trimLower(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}

func (mg *ManagedGame) onRevealTimeout() {
	mg.beginNextTurn(true)
}

func (mg *ManagedGame) handleSkipTrack(playerID string) {
	if time.Since(mg.lastSkipAt) < mg.cfg.SkipRateLimit {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "rate_limited", "skip is rate-limited to once per 10 seconds")
		}
		return
	}
	if err := mg.g.SkipTrack(playerID); err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "skip_failed", err.Error())
		}
		return
	}
	mg.lastSkipAt = time.Now()
	mg.clearPhaseTimeout()
	mg.stopBroadcast()
	mg.beginNextTurn(false)
}

func (mg *ManagedGame) handleKickPlayer(hostID, targetID string) {
	wasActive := mg.g.Turn != nil && mg.g.Turn.ActivePlayerID == targetID
	ended, err := mg.g.Kick(hostID, targetID, mg.cfg.MinPlayers)
	if err != nil {
		if c := mg.conns[hostID]; c != nil {
			mg.sendError(c, "kick_failed", err.Error())
		}
		return
	}
	mg.dropConn(targetID, "kicked")
	if ended {
		mg.clearPhaseTimeout()
		mg.stopBroadcast()
		mg.broadcastState()
		mg.persistOnGameOver()
		return
	}
	if wasActive {
		mg.clearPhaseTimeout()
		mg.g.Turn = nil
		mg.beginNextTurn(true)
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) handleEndGame(hostID string) {
	if err := mg.g.EndGame(hostID); err != nil {
		return
	}
	mg.clearPhaseTimeout()
	mg.stopBroadcast()
	mg.broadcastState()
	mg.persistOnGameOver()
}

func (mg *ManagedGame) stopBroadcast() {
	if mg.lastPrep == nil {
		return
	}
	token, err := mg.mediaSigner.Issue(mg.id, mg.lastPrep.trackID, mg.cfg.MediaTTL)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mg.broadcaster.Stop(ctx, mg.id, mg.lastPrep.trackID, token); err != nil {
		mg.log.Warn("failed to stop audio broadcast", "game_id", mg.id, "track_id", mg.lastPrep.trackID, "error", err)
	}
}

func (mg *ManagedGame) endForBroadcastFailure(message string, err error) {
	mg.log.Error(message, "game_id", mg.id, "error", err)
	_ = mg.g.EndGame(mg.g.HostID)
	mg.clearPhaseTimeout()
	mg.broadcastState()
	mg.persistOnGameOver()
}

func (mg *ManagedGame) handlePlayAgain(hostID string) {
	if err := mg.g.PlayAgain(hostID); err != nil {
		return
	}
	mg.candidates = nil
	mg.turnCount = 0
	mg.broadcastState()
}

func (mg *ManagedGame) handleUpdateSettings(hostID string, targetCards, startTokens *int, enableSongGuess *bool) {
	if err := mg.g.UpdateSettings(hostID, targetCards, startTokens, enableSongGuess); err != nil {
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) dropConn(playerID, reason string) {
	if c := mg.conns[playerID]; c != nil {
		c.Close(reason)
	}
	delete(mg.conns, playerID)
}
