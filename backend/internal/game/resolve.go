package game

// Resolve computes the outcome of the current turn (§8.8) and transitions to
// GAME_OVER if the target card count is reached (§8.9), otherwise leaves the
// phase at REVEALING for the caller to time out into the next turn.
func (g *Game) Resolve() (*Reveal, error) {
	if g.Phase != PhaseRevealing {
		return nil, ErrWrongPhase
	}
	t := g.Turn
	active := g.ActivePlayer()

	reveal := &Reveal{
		Card:            t.Track,
		ActivePlayerID:  t.ActivePlayerID,
		ActivePlacement: t.PlacementSlot,
		TokenChanges:    map[string]int{},
	}
	for playerID, spent := range t.Spent {
		reveal.TokenChanges[playerID] = -spent
	}

	activeCorrect := PlacementCorrect(active.Timeline, t.Track.Year, t.PlacementSlot)
	reveal.ActiveCorrect = activeCorrect

	if activeCorrect {
		active.Timeline = InsertCard(active.Timeline, t.PlacementSlot, t.Track)
		reveal.WinnerPlayerID = active.ID
	} else {
		for _, playerID := range t.Order {
			slot, challenged := t.Challenges[playerID]
			if !challenged {
				continue
			}
			correct := PlacementCorrect(active.Timeline, t.Track.Year, slot)
			reveal.Challenges = append(reveal.Challenges, ChallengeOutcome{PlayerID: playerID, Slot: slot, Correct: correct})
			if correct && reveal.WinnerPlayerID == "" {
				winner := g.Player(playerID)
				insertAt := sortedInsertIndex(winner.Timeline, t.Track.Year)
				winner.Timeline = InsertCard(winner.Timeline, insertAt, t.Track)
				winner.Tokens++
				if winner.Tokens > g.Settings.MaxTokens {
					winner.Tokens = g.Settings.MaxTokens
				}
				reveal.TokenChanges[playerID] = reveal.TokenChanges[playerID] + 1
				reveal.WinnerPlayerID = playerID
			}
		}
	}

	g.checkWinCondition()
	return reveal, nil
}

// sortedInsertIndex returns the leftmost index at which year can be inserted
// into timeline while keeping it sorted ascending by year.
func sortedInsertIndex(timeline []Card, year int) int {
	for i, c := range timeline {
		if year <= c.Year {
			return i
		}
	}
	return len(timeline)
}

// checkWinCondition ends the game immediately if any player has reached the
// target card count (§8.9).
func (g *Game) checkWinCondition() {
	for _, p := range g.Players {
		if len(p.Timeline) >= g.Settings.TargetCards {
			g.Phase = PhaseGameOver
			g.WinnerID = p.ID
			return
		}
	}
}

// ApplySongGuessReward grants +1 token (capped at MaxTokens) to the active
// player when both the title and artist guesses were correct (§8.6). Call
// during Resolve's turn, before or after Resolve itself.
func (r *Reveal) ApplySongGuessReward(player *Player, maxTokens int, bothCorrect bool) {
	r.SongGuessCorrect = bothCorrect
	if !bothCorrect {
		return
	}
	if player.Tokens < maxTokens {
		player.Tokens++
		r.TokenChanges[player.ID] = r.TokenChanges[player.ID] + 1
	}
	r.SongGuessAwarded = true
}
