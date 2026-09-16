package gamesvc

import (
	"strings"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/songmatch"
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

	cand, err := mg.takeCandidate()
	if err != nil {
		mg.log.Error("track pool exhausted", "game_id", mg.id, "error", err)
		_ = mg.g.EndGame(mg.g.HostID)
		mg.clearPhaseTimeout()
		mg.broadcastState()
		mg.deleteSnapshotAsync()
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

func (mg *ManagedGame) handlePlaceCard(playerID string, slotIndex int) {
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

// afterPlacement opens the steal window: other players get
// TurnChallengeWindow to press Steal; placing a claimed steal has no limit.
func (mg *ManagedGame) afterPlacement(skipChallenge bool) {
	if skipChallenge {
		mg.clearPhaseTimeout()
		mg.beginRevealing()
		return
	}
	mg.schedulePhaseTimeout(mg.cfg.TurnChallengeWindow, mg.onStealWindowTimeout)
	mg.broadcastState()
}

func (mg *ManagedGame) handleClaimSteal(playerID string) {
	if err := mg.g.ClaimSteal(playerID); err != nil {
		if c := mg.conns[playerID]; c != nil {
			mg.sendError(c, "invalid_steal", err.Error())
		}
		return
	}
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

// onStealWindowTimeout closes the steal window. Claimants who haven't
// placed yet keep the turn open, without a deadline, until they do.
func (mg *ManagedGame) onStealWindowTimeout() {
	done, err := mg.g.CloseStealWindow()
	if err != nil {
		return
	}
	mg.clearPhaseTimeout()
	if done {
		mg.beginRevealing()
		return
	}
	mg.broadcastState()
}

// afterPlayerRemoved finishes stealing if the removed player was the last
// claimant still to place, and otherwise just publishes the new state.
func (mg *ManagedGame) afterPlayerRemoved() {
	if mg.g.FinishStealingIfComplete() {
		mg.clearPhaseTimeout()
		mg.beginRevealing()
		return
	}
	mg.broadcastState()
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

	// The song guess is independent of placement and stealing: only the
	// active player's own guess decides their bonus token. Every field the
	// host selected must be right; the others are not checked.
	var guessResult *SongGuessResultView
	if guess, fields := mg.pendingSongGuess, mg.g.Settings.GuessFields; guess != nil && fields.Any() {
		result, allCorrect := evaluateSongGuess(guess, fields, reveal.Card)
		if active := mg.g.Player(reveal.ActivePlayerID); active != nil {
			reveal.ApplySongGuessReward(active, mg.g.Settings.MaxTokens, allCorrect)
		}
		result.Correct, result.Awarded = reveal.SongGuessCorrect, reveal.SongGuessAwarded
		guessResult = &result
	}
	mg.pendingSongGuess = nil

	payload := buildRevealPayload(reveal, mg.currentYearSource, guessResult)
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
	mg.deleteSnapshotAsync()
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
	if mg.forgetIfEmpty() {
		return
	}
	if ended {
		mg.clearPhaseTimeout()
		mg.stopTrack()
		mg.broadcastState()
		mg.deleteSnapshotAsync()
		return
	}
	if wasActive {
		mg.clearPhaseTimeout()
		mg.g.Turn = nil
		mg.beginNextTurn(true)
		return
	}
	mg.afterPlayerRemoved()
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
	mg.deleteSnapshotAsync()
}

func (mg *ManagedGame) handlePlayAgain(hostID string) {
	if err := mg.g.PlayAgain(hostID); err != nil {
		return
	}
	mg.upcoming = nil
	mg.broadcastState()
}

// maxGuessRunes bounds each stored song guess field.
const maxGuessRunes = 200

// handleSongGuess stores the active player's guess. It may be changed freely
// until the reveal checks it; sending only empty fields withdraws it. Every
// change is relayed to the other players so they can watch it being typed.
func (mg *ManagedGame) handleSongGuess(c *ws.Conn, p ws.SongGuessPayload) {
	if !mg.g.Settings.GuessFields.Any() || mg.g.Turn == nil || c.PlayerID != mg.g.Turn.ActivePlayerID {
		mg.sendError(c, "invalid_song_guess", "only the active player may guess, when the song guess bonus is enabled")
		return
	}
	switch mg.g.Phase {
	case game.PhasePreparing, game.PhasePlacing, game.PhaseChallenging:
	default:
		mg.sendError(c, "invalid_song_guess", "guesses are closed once the card is revealed")
		return
	}
	guess := &pendingSongGuess{
		title:  truncateRunes(p.Title, maxGuessRunes),
		artist: truncateRunes(p.Artist, maxGuessRunes),
		album:  truncateRunes(p.Album, maxGuessRunes),
		year:   truncateRunes(p.Year, 8),
	}
	if strings.TrimSpace(guess.title+guess.artist+guess.album+guess.year) == "" {
		guess = nil
	}
	mg.pendingSongGuess = guess

	var update ws.SongGuessPayload
	if guess != nil {
		v := guess.view(mg.g.Settings.GuessFields)
		update = ws.SongGuessPayload{Title: v.Title, Artist: v.Artist, Album: v.Album, Year: v.Year}
	}
	for playerID, other := range mg.conns {
		if other != nil && playerID != c.PlayerID {
			other.Send(ws.TypeSongGuessUpdate, update)
		}
	}
}

// evaluateSongGuess checks the fields the host selected against the card;
// allCorrect requires every one of them to match.
func evaluateSongGuess(guess *pendingSongGuess, fields game.GuessFields, card game.Card) (result SongGuessResultView, allCorrect bool) {
	result = SongGuessResultView{
		SongGuessView: guess.view(fields),
		TitleCorrect:  fields.Title && songmatch.Title(guess.title, card.Title),
		ArtistCorrect: fields.Artist && songmatch.Artist(guess.artist, card.Artist),
		AlbumCorrect:  fields.Album && songmatch.Album(guess.album, card.Album),
		YearCorrect:   fields.Year && songmatch.Year(guess.year, card.Year),
	}
	allCorrect = fields.Any() &&
		(!fields.Title || result.TitleCorrect) && (!fields.Artist || result.ArtistCorrect) &&
		(!fields.Album || result.AlbumCorrect) && (!fields.Year || result.YearCorrect)
	return result, allCorrect
}

// view returns the guess limited to the fields the game asks for.
func (g *pendingSongGuess) view(fields game.GuessFields) SongGuessView {
	var v SongGuessView
	if fields.Title {
		v.Title = g.title
	}
	if fields.Artist {
		v.Artist = g.artist
	}
	if fields.Album {
		v.Album = g.album
	}
	if fields.Year {
		v.Year = g.year
	}
	return v
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func (mg *ManagedGame) handleUpdateSettings(hostID string, u game.SettingsUpdate) {
	if err := mg.g.UpdateSettings(hostID, u); err != nil {
		if c := mg.conns[hostID]; c != nil {
			mg.sendError(c, "invalid_settings", err.Error())
		}
		return
	}
	mg.broadcastState()
}

func (mg *ManagedGame) dropConn(playerID, reason string) {
	if c := mg.conns[playerID]; c != nil {
		c.Close(reason)
	}
	delete(mg.conns, playerID)
	if len(mg.conns) == 0 && mg.emptySince.IsZero() {
		mg.emptySince = time.Now()
	}
}
