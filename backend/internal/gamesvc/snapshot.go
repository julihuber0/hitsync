package gamesvc

import "github.com/julianhuber/hitsync/backend/internal/game"

// CardView is a revealed card as sent to clients (§8.1, §13.3).
type CardView struct {
	TrackID string `json:"trackId"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Year    int    `json:"year"`
}

func cardView(c game.Card) CardView {
	return CardView{TrackID: c.TrackID, Title: c.Title, Artist: c.Artist, Year: c.Year}
}

// PlayerView is one player's entry in the state snapshot (§13.3).
type PlayerView struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Colour               string     `json:"colour"`
	Connected            bool       `json:"connected"`
	IsHost               bool       `json:"isHost"`
	Tokens               int        `json:"tokens"`
	Timeline             []CardView `json:"timeline"`
	PendingChallengeSlot *int       `json:"pendingChallengeSlot"`
}

// SettingsView mirrors game.Settings for the wire (§13.3).
type SettingsView struct {
	TargetCards     int  `json:"targetCards"`
	StartTokens     int  `json:"startTokens"`
	EnableSongGuess bool `json:"enableSongGuess"`
}

// CurrentTurnView is the in-progress turn's visible state (§13.3).
type CurrentTurnView struct {
	ActivePlacementSubmitted bool     `json:"activePlacementSubmitted"`
	ActivePlacementSlot      *int     `json:"activePlacementSlot"`
	ChallengeSlotsTaken      []int    `json:"challengeSlotsTaken"`
	HasPassed                []string `json:"hasPassed"`
}

// StatePayload is the full per-recipient game state snapshot (§13.3).
// Information hiding is applied here, server-side, per recipient.
type StatePayload struct {
	GameID              string           `json:"gameId"`
	InviteCode          string           `json:"inviteCode"`
	Phase               string           `json:"phase"`
	Settings            SettingsView     `json:"settings"`
	HostID              string           `json:"hostId"`
	YouID               string           `json:"youId"`
	ActivePlayerID      string           `json:"activePlayerId"`
	TurnNumber          int              `json:"turnNumber"`
	PhaseEndsAtServerMs *int64           `json:"phaseEndsAtServerMs"`
	Players             []PlayerView     `json:"players"`
	CurrentTurn         *CurrentTurnView `json:"currentTurn"`
	WinnerID            *string          `json:"winnerId"`
	TracksUsed          int              `json:"tracksUsed"`
}

// buildState renders the snapshot for a specific recipient (forPlayerID may
// be "" for an as-yet-unassociated viewer, e.g. an admin).
func (mg *ManagedGame) buildState(forPlayerID string) StatePayload {
	g := mg.g

	players := make([]PlayerView, 0, len(g.Players))
	for _, p := range g.Players {
		pv := PlayerView{
			ID:        p.ID,
			Name:      p.Name,
			Colour:    p.Colour,
			Connected: p.Connected,
			IsHost:    p.IsHost,
			Tokens:    p.Tokens,
			Timeline:  make([]CardView, len(p.Timeline)),
		}
		for i, c := range p.Timeline {
			pv.Timeline[i] = cardView(c)
		}
		if g.Turn != nil {
			if slot, ok := g.Turn.Challenges[p.ID]; ok {
				if p.ID == forPlayerID || g.Phase == game.PhaseRevealing || g.Phase == game.PhaseGameOver {
					s := slot
					pv.PendingChallengeSlot = &s
				}
			}
		}
		players = append(players, pv)
	}

	var currentTurn *CurrentTurnView
	if g.Turn != nil {
		ct := &CurrentTurnView{
			ActivePlacementSubmitted: g.Turn.PlacementSubmitted,
			ChallengeSlotsTaken:      make([]int, 0),
			HasPassed:                make([]string, 0),
		}
		if g.Turn.PlacementSubmitted && (forPlayerID == g.Turn.ActivePlayerID || g.Phase == game.PhaseRevealing || g.Phase == game.PhaseGameOver) {
			slot := g.Turn.PlacementSlot
			ct.ActivePlacementSlot = &slot
		}
		for _, slot := range g.Turn.Challenges {
			ct.ChallengeSlotsTaken = append(ct.ChallengeSlotsTaken, slot)
		}
		for playerID, passed := range g.Turn.Passed {
			if passed {
				ct.HasPassed = append(ct.HasPassed, playerID)
			}
		}
		currentTurn = ct
	}

	var winnerID *string
	if g.Phase == game.PhaseGameOver && g.WinnerID != "" {
		w := g.WinnerID
		winnerID = &w
	}

	var deadline *int64
	if !mg.phaseDeadline.IsZero() {
		ms := mg.phaseDeadline.UnixMilli()
		deadline = &ms
	}

	activePlayerID := ""
	if ap := g.ActivePlayer(); ap != nil {
		activePlayerID = ap.ID
	}

	return StatePayload{
		GameID:     mg.id,
		InviteCode: mg.inviteCode,
		Phase:      string(g.Phase),
		Settings: SettingsView{
			TargetCards:     g.Settings.TargetCards,
			StartTokens:     g.Settings.StartTokens,
			EnableSongGuess: g.Settings.EnableSongGuess,
		},
		HostID:              g.HostID,
		YouID:               forPlayerID,
		ActivePlayerID:      activePlayerID,
		TurnNumber:          currentTurnNumber(g),
		PhaseEndsAtServerMs: deadline,
		Players:             players,
		CurrentTurn:         currentTurn,
		WinnerID:            winnerID,
		TracksUsed:          g.TracksUsedCount,
	}
}

func currentTurnNumber(g *game.Game) int {
	if g.Turn != nil {
		return g.Turn.Number
	}
	return 0
}

// RevealChallengeView is one challenger's outcome at reveal time (§13.2).
type RevealChallengeView struct {
	PlayerID string `json:"playerId"`
	Slot     int    `json:"slot"`
	Correct  bool   `json:"correct"`
}

// TokenChangeView is one player's net token delta for the turn (§13.2).
type TokenChangeView struct {
	PlayerID string `json:"playerId"`
	Delta    int    `json:"delta"`
}

// SongGuessResultView reports the optional bonus round's outcome (§8.6).
type SongGuessResultView struct {
	Correct bool `json:"correct"`
	Awarded bool `json:"awarded"`
}

// RevealPayload is the server->client `reveal` message (§13.2).
type RevealPayload struct {
	Card            CardView              `json:"card"`
	ActivePlayerID  string                `json:"activePlayerId"`
	ActivePlacement int                   `json:"activePlacement"`
	ActiveCorrect   bool                  `json:"activeCorrect"`
	WinnerPlayerID  string                `json:"winnerPlayerId"`
	Challenges      []RevealChallengeView `json:"challenges"`
	Outcome         string                `json:"outcome"`
	TokenChanges    []TokenChangeView     `json:"tokenChanges"`
	SongGuessResult *SongGuessResultView  `json:"songGuessResult,omitempty"`
	YearSource      string                `json:"yearSource"`
}

func buildRevealPayload(reveal *game.Reveal, yearSource string) RevealPayload {
	outcome := "discarded"
	if reveal.ActiveCorrect {
		outcome = "active_correct"
	} else if reveal.WinnerPlayerID != "" {
		outcome = "challenger_correct"
	}

	challenges := make([]RevealChallengeView, 0, len(reveal.Challenges))
	for _, c := range reveal.Challenges {
		challenges = append(challenges, RevealChallengeView{PlayerID: c.PlayerID, Slot: c.Slot, Correct: c.Correct})
	}

	tokenChanges := make([]TokenChangeView, 0, len(reveal.TokenChanges))
	for playerID, delta := range reveal.TokenChanges {
		tokenChanges = append(tokenChanges, TokenChangeView{PlayerID: playerID, Delta: delta})
	}

	var songGuess *SongGuessResultView
	if reveal.SongGuessCorrect || reveal.SongGuessAwarded {
		songGuess = &SongGuessResultView{Correct: reveal.SongGuessCorrect, Awarded: reveal.SongGuessAwarded}
	}

	return RevealPayload{
		Card:            cardView(reveal.Card),
		ActivePlayerID:  reveal.ActivePlayerID,
		ActivePlacement: reveal.ActivePlacement,
		ActiveCorrect:   reveal.ActiveCorrect,
		WinnerPlayerID:  reveal.WinnerPlayerID,
		Challenges:      challenges,
		Outcome:         outcome,
		TokenChanges:    tokenChanges,
		SongGuessResult: songGuess,
		YearSource:      yearSource,
	}
}
