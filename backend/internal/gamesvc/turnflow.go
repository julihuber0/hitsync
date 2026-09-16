package gamesvc

import (
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// startGameFlow validates and starts the game (§8.4), dealing starting
// cards, then kicks off the first turn.
func (mg *ManagedGame) startGameFlow(hostID string) error {
	if hostID != mg.g.HostID {
		return game.ErrNotHost
	}
	// One starting card per player plus at least one turn.
	if mg.trackSource.PlayableCount() <= len(mg.g.Players) {
		return ErrNoCards
	}
	if err := mg.g.StartGame(mg.cfg.MinPlayers); err != nil {
		return err
	}
	mg.startedAt = time.Now()

	for _, p := range mg.g.Players {
		cand, err := mg.trackSource.Draw(mg.excludedTrackIDs())
		if err != nil {
			mg.log.Error("failed to deal starting card", "game_id", mg.id, "player_id", p.ID, "error", err)
			continue
		}
		_ = mg.g.DealStartingCard(p.ID, cand.Card)
	}

	mg.beginNextTurn(true)
	mg.broadcastState()
	return nil
}

// beginNextTurn draws (or reuses, for a skip) the next track and enters
// PREPARING (§8.5). Any track still playing is stopped first.
func (mg *ManagedGame) beginNextTurn(advance bool) {
	mg.stopTrack()
	if advance {
		if err := mg.g.NextTurn(); err != nil {
			return
		}
	}
	mg.turnCount++

	cand, err := mg.takeCandidate()
	if err != nil {
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

	mg.track = &activeTrack{prepareID: newID(), trackID: card.TrackID, durationMs: int64(cand.DurationSec) * 1000}
	if mg.g.Settings.EnableSongGuess {
		titles, artists := mg.trackSource.SongGuessOptions(&card)
		mg.track.guessOptions = &ws.GuessOptions{Titles: titles, Artists: artists}
	}

	if url, ok := mg.mediaURL(card.TrackID); ok {
		for _, c := range mg.conns {
			if c == nil {
				continue
			}
			mg.sendTrackPrepare(c, url)
		}
	}
	mg.drawUpcoming()

	mg.schedulePhaseTimeout(mg.cfg.PreparingCap, mg.onPrepareTimeout)
	mg.broadcastState()
}

func (mg *ManagedGame) handleReady(playerID, prepareID string) {
	if mg.g.Phase != game.PhasePreparing || mg.track == nil || prepareID != mg.track.prepareID {
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

// beginPlacing transitions PREPARING -> PLACING, once every connected client
// has the track or the preparing cap expires, and fixes the shared playback
// start slightly in the future so clients can schedule it. A client that is
// still downloading joins in at the right position once it's done.
func (mg *ManagedGame) beginPlacing() {
	if err := mg.g.BeginPlacing(); err != nil {
		return
	}
	if mg.track != nil {
		mg.track.startAtServerMs = nowMs() + mg.cfg.StartAtLeadMs
		for _, c := range mg.conns {
			if c == nil {
				continue
			}
			mg.sendTrackStart(c)
		}
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
	mg.stopTrack()

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
		// The winning card's reveal should still be shown for RevealDuration,
		// the same as any other turn, before the end screen appears (§8.9).
		// The win itself is already decided (WinnerID captured); only the
		// client-visible phase transition is delayed, by keeping the phase at
		// REVEALING for the wait and re-asserting the winner in
		// finalizeGameOver in case a player-count drop in the meantime would
		// otherwise have cleared it (game.Game.RemovePlayer).
		mg.pendingWinnerID = mg.g.WinnerID
		mg.g.Phase = game.PhaseRevealing
		mg.schedulePhaseTimeout(mg.cfg.RevealDuration, mg.finalizeGameOver)
		mg.broadcastState()
		return
	}

	mg.schedulePhaseTimeout(mg.cfg.RevealDuration, mg.onRevealTimeout)
	mg.broadcastState()
}

// finalizeGameOver transitions to GAME_OVER once the winning turn's reveal
// has been shown for RevealDuration, mirroring the non-winning
// onRevealTimeout path (§8.9).
func (mg *ManagedGame) finalizeGameOver() {
	mg.g.Phase = game.PhaseGameOver
	mg.g.WinnerID = mg.pendingWinnerID
	mg.clearPhaseTimeout()
	mg.broadcastState()
	mg.persistOnGameOver()
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
		mg.stopTrack()
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

func (mg *ManagedGame) handleAdjustTokens(hostID, targetID string, delta int) {
	if err := mg.g.AdjustTokens(hostID, targetID, delta); err != nil {
		if c := mg.conns[hostID]; c != nil {
			mg.sendError(c, "adjust_tokens_failed", err.Error())
		}
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) handleEndGame(hostID string) {
	if err := mg.g.EndGame(hostID); err != nil {
		return
	}
	mg.clearPhaseTimeout()
	mg.stopTrack()
	mg.broadcastState()
	mg.persistOnGameOver()
}

func (mg *ManagedGame) handlePlayAgain(hostID string) {
	if err := mg.g.PlayAgain(hostID); err != nil {
		return
	}
	mg.upcoming = nil
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
