package gamesvc

import (
	"context"
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
	streamURL := mg.buildStreamURL(card.TrackID)
	mg.lastPrep = &lastPrepare{prepareID: prepareID, trackID: card.TrackID, streamURL: streamURL, durationMs: durationMs}

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
		payload := ws.TrackPreparePayload{PrepareID: prepareID, TrackID: card.TrackID, StreamURL: streamURL, DurationMs: durationMs}
		if playerID == activeID {
			payload.GuessOptions = guessOpts
		}
		c.Send(ws.TypeTrackPrepare, payload)
	}

	mg.schedulePhaseTimeout(mg.cfg.PreparingCap, mg.onPrepareTimeout)
	mg.broadcastState()
}

func (mg *ManagedGame) buildStreamURL(trackID string) string {
	token, err := mg.mediaSigner.Issue(mg.id, trackID, mg.cfg.MediaTTL)
	if err != nil {
		return ""
	}
	return mg.cfg.MediaBaseURL + "/stream/" + trackID + "?token=" + token
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

// beginPlacing transitions PREPARING -> PLACING and broadcasts track_start
// (§8.5, §10.4).
func (mg *ManagedGame) beginPlacing() {
	if err := mg.g.BeginPlacing(); err != nil {
		return
	}
	startAt := time.Now().Add(time.Duration(mg.cfg.StartAtLeadMs) * time.Millisecond)
	mg.lastPrep.startAtServerMs = startAt.UnixMilli()

	for _, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeTrackStart, ws.TrackStartPayload{
			PrepareID:       mg.lastPrep.prepareID,
			StartAtServerMs: mg.lastPrep.startAtServerMs,
			DurationMs:      mg.lastPrep.durationMs,
		})
	}

	timeout := mg.cfg.TurnPlacementTimeout
	if active := mg.g.ActivePlayer(); active != nil && !active.Connected {
		timeout = mg.cfg.DisconnectedPlacementTimeout
	}
	mg.schedulePhaseTimeout(timeout, mg.onPlacementTimeout)
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
