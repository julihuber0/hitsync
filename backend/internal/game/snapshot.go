package game

// State is a fully serialisable snapshot of a Game, used for crash-recovery
// persistence (§3.4, §17). It exists because several bookkeeping fields
// (join order, turn numbering) are intentionally unexported from normal
// gameplay code.
type State struct {
	ID         string
	InviteCode string
	Phase      Phase
	Settings   Settings
	HostID     string

	Players         []PlayerState
	ActivePlayerIdx int

	Turn *TurnState

	WinnerID          string
	UsedTrackIDs      []string
	TracksUsedCount   int
	NextJoinOrder     int
	CurrentTurnNumber int
}

// PlayerState is the serialisable form of Player.
type PlayerState struct {
	ID        string
	Name      string
	Colour    string
	Tokens    int
	Timeline  []Card
	Connected bool
	IsHost    bool
	JoinOrder int
}

// TurnState is the serialisable form of Turn.
type TurnState struct {
	Number             int
	ActivePlayerID     string
	Track              Card
	PlacementSubmitted bool
	PlacementSlot      int
	Order              []string
	Challenges         map[string]int
	ChallengePreviews  map[string]int
	Passed             map[string]bool
	Spent              map[string]int
}

// ExportState captures the full game state for persistence.
func (g *Game) ExportState() State {
	s := State{
		ID:                g.ID,
		InviteCode:        g.InviteCode,
		Phase:             g.Phase,
		Settings:          g.Settings,
		HostID:            g.HostID,
		ActivePlayerIdx:   g.ActivePlayerIdx,
		WinnerID:          g.WinnerID,
		TracksUsedCount:   g.TracksUsedCount,
		NextJoinOrder:     g.nextJoinOrder,
		CurrentTurnNumber: g.currentTurnNumber,
	}
	for _, p := range g.Players {
		s.Players = append(s.Players, PlayerState{
			ID: p.ID, Name: p.Name, Colour: p.Colour, Tokens: p.Tokens,
			Timeline:  append([]Card(nil), p.Timeline...),
			Connected: p.Connected, IsHost: p.IsHost, JoinOrder: p.joinOrder,
		})
	}
	for id := range g.UsedTrackIDs {
		s.UsedTrackIDs = append(s.UsedTrackIDs, id)
	}
	if g.Turn != nil {
		s.Turn = &TurnState{
			Number: g.Turn.Number, ActivePlayerID: g.Turn.ActivePlayerID, Track: g.Turn.Track,
			PlacementSubmitted: g.Turn.PlacementSubmitted, PlacementSlot: g.Turn.PlacementSlot,
			Order:      append([]string(nil), g.Turn.Order...),
			Challenges: copyIntMap(g.Turn.Challenges), ChallengePreviews: copyIntMap(g.Turn.ChallengePreviews), Passed: copyBoolMap(g.Turn.Passed),
			Spent: copyIntMap(g.Turn.Spent),
		}
	}
	return s
}

// RestoreState reconstructs a Game from a previously exported State.
func RestoreState(s State) *Game {
	g := &Game{
		ID: s.ID, InviteCode: s.InviteCode, Phase: s.Phase, Settings: s.Settings, HostID: s.HostID,
		ActivePlayerIdx: s.ActivePlayerIdx, WinnerID: s.WinnerID, TracksUsedCount: s.TracksUsedCount,
		nextJoinOrder: s.NextJoinOrder, currentTurnNumber: s.CurrentTurnNumber,
		UsedTrackIDs: map[string]bool{},
	}
	for _, p := range s.Players {
		g.Players = append(g.Players, &Player{
			ID: p.ID, Name: p.Name, Colour: p.Colour, Tokens: p.Tokens,
			Timeline:  append([]Card(nil), p.Timeline...),
			Connected: p.Connected, IsHost: p.IsHost, joinOrder: p.JoinOrder,
		})
	}
	for _, id := range s.UsedTrackIDs {
		g.UsedTrackIDs[id] = true
	}
	if s.Turn != nil {
		g.Turn = &Turn{
			Number: s.Turn.Number, ActivePlayerID: s.Turn.ActivePlayerID, Track: s.Turn.Track,
			PlacementSubmitted: s.Turn.PlacementSubmitted, PlacementSlot: s.Turn.PlacementSlot,
			Order:      append([]string(nil), s.Turn.Order...),
			Challenges: copyIntMap(s.Turn.Challenges), ChallengePreviews: copyIntMap(s.Turn.ChallengePreviews), Passed: copyBoolMap(s.Turn.Passed),
			Spent: copyIntMap(s.Turn.Spent),
		}
	}
	return g
}

func copyIntMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyBoolMap(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
