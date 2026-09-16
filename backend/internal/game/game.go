package game

import "strings"

// New creates a fresh game in LOBBY, hosted by no one yet — the first
// player added via AddPlayer becomes the host.
func New(id, inviteCode string, settings Settings) *Game {
	return &Game{
		ID:           id,
		InviteCode:   inviteCode,
		Phase:        PhaseLobby,
		Settings:     settings,
		UsedTrackIDs: map[string]bool{},
	}
}

// AddPlayer validates and adds a new player to the lobby (§8.3). The first
// player added becomes the host. Returns the created player.
func (g *Game) AddPlayer(id, name string, maxPlayers int) (*Player, error) {
	if g.Phase != PhaseLobby {
		return nil, ErrWrongPhase
	}
	if len(g.Players) >= maxPlayers {
		return nil, ErrTooManyPlayers
	}

	trimmed, err := ValidateDisplayName(name)
	if err != nil {
		return nil, err
	}
	for _, p := range g.Players {
		if strings.EqualFold(p.Name, trimmed) {
			return nil, ErrDuplicateName
		}
	}

	colour, err := g.nextColour()
	if err != nil {
		return nil, err
	}

	p := &Player{
		ID:        id,
		Name:      trimmed,
		Colour:    colour,
		Tokens:    g.Settings.StartTokens,
		Connected: true,
		joinOrder: g.nextJoinOrder,
	}
	g.nextJoinOrder++

	if len(g.Players) == 0 {
		p.IsHost = true
		g.HostID = id
	}
	g.Players = append(g.Players, p)
	return p, nil
}

func (g *Game) nextColour() (string, error) {
	used := map[string]bool{}
	for _, p := range g.Players {
		used[p.Colour] = true
	}
	for _, c := range Palette {
		if !used[c] {
			return c, nil
		}
	}
	return "", ErrPaletteExhausted
}

// Player looks up a player by id.
func (g *Game) Player(id string) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// ActivePlayer returns the player whose turn it currently is, or nil in the lobby.
func (g *Game) ActivePlayer() *Player {
	if g.ActivePlayerIdx < 0 || g.ActivePlayerIdx >= len(g.Players) {
		return nil
	}
	return g.Players[g.ActivePlayerIdx]
}

// RemovePlayer removes a player (kick, or reconnect-grace expiry, §8.10) and
// discards their timeline. If the host is removed, host status transfers to
// the longest-connected remaining player. If the resulting player count
// drops below minPlayers, the game ends with no winner and endedNoWinner is
// true.
func (g *Game) RemovePlayer(playerID string, minPlayers int) (endedNoWinner bool) {
	idx := -1
	for i, p := range g.Players {
		if p.ID == playerID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}

	wasHost := g.Players[idx].IsHost
	g.Players = append(g.Players[:idx], g.Players[idx+1:]...)

	if g.Turn != nil {
		delete(g.Turn.StealClaims, playerID)
		delete(g.Turn.Challenges, playerID)
		delete(g.Turn.ChallengePreviews, playerID)
		delete(g.Turn.Passed, playerID)
	}

	if idx < g.ActivePlayerIdx {
		g.ActivePlayerIdx--
	}
	if len(g.Players) > 0 && g.ActivePlayerIdx >= len(g.Players) {
		g.ActivePlayerIdx = 0
	}
	if len(g.Players) == 0 {
		g.ActivePlayerIdx = 0
	}

	if wasHost && len(g.Players) > 0 {
		g.transferHost()
	}

	if g.Phase != PhaseLobby && g.Phase != PhaseGameOver && len(g.Players) < minPlayers {
		g.Phase = PhaseGameOver
		g.WinnerID = ""
		return true
	}
	return false
}

// transferHost assigns host to the longest-connected remaining player, i.e.
// the one with the smallest joinOrder (§8.3).
func (g *Game) transferHost() {
	var newHost *Player
	for _, p := range g.Players {
		p.IsHost = false
		if newHost == nil || p.joinOrder < newHost.joinOrder {
			newHost = p
		}
	}
	if newHost != nil {
		newHost.IsHost = true
		g.HostID = newHost.ID
	}
}

// UpdateSettings applies lobby setting changes (§13.1 update_settings), host
// only, lobby only.
func (g *Game) UpdateSettings(playerID string, targetCards, startTokens *int, enableSongGuess *bool) error {
	if playerID != g.HostID {
		return ErrNotHost
	}
	if g.Phase != PhaseLobby {
		return ErrWrongPhase
	}
	if targetCards != nil {
		g.Settings.TargetCards = *targetCards
	}
	if startTokens != nil {
		g.Settings.StartTokens = *startTokens
	}
	if enableSongGuess != nil {
		g.Settings.EnableSongGuess = *enableSongGuess
	}
	return nil
}
