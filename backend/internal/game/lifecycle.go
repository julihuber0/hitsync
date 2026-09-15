package game

// EndGame jumps straight to GAME_OVER with no winner (§8.11 host action).
func (g *Game) EndGame(playerID string) error {
	if playerID != g.HostID {
		return ErrNotHost
	}
	if g.Phase == PhaseGameOver {
		return ErrWrongPhase
	}
	g.Phase = PhaseGameOver
	g.WinnerID = ""
	return nil
}

// PlayAgain returns everyone to the lobby with the same players and
// settings, a fresh used-track set, empty timelines and reset tokens (§8.9).
func (g *Game) PlayAgain(playerID string) error {
	if playerID != g.HostID {
		return ErrNotHost
	}
	if g.Phase != PhaseGameOver {
		return ErrWrongPhase
	}
	g.Phase = PhaseLobby
	g.WinnerID = ""
	g.UsedTrackIDs = map[string]bool{}
	g.TracksUsedCount = 0
	g.currentTurnNumber = 0
	g.ActivePlayerIdx = -1
	g.Turn = nil
	for _, p := range g.Players {
		p.Timeline = nil
		p.Tokens = g.Settings.StartTokens
	}
	return nil
}

// Kick removes a player by host action (§8.11). minPlayers behaviour matches
// RemovePlayer.
func (g *Game) Kick(hostID, targetPlayerID string, minPlayers int) (endedNoWinner bool, err error) {
	if hostID != g.HostID {
		return false, ErrNotHost
	}
	if g.Player(targetPlayerID) == nil {
		return false, ErrPlayerNotFound
	}
	return g.RemovePlayer(targetPlayerID, minPlayers), nil
}
