package game

// DealStartingCard gives a player their starting card (§8.4), revealed
// immediately and without audio. Must be called once per player, in seat
// order, before the first real turn. Marks the track used.
func (g *Game) DealStartingCard(playerID string, card Card) error {
	p := g.Player(playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	p.Timeline = InsertCard(p.Timeline, len(p.Timeline), card)
	g.MarkTrackUsed(card.TrackID)
	return nil
}

// MarkTrackUsed records a track as used in this game without dealing it to
// anyone, e.g. for starting cards or externally-drawn candidates.
func (g *Game) MarkTrackUsed(trackID string) {
	g.UsedTrackIDs[trackID] = true
	g.TracksUsedCount++
}

// StartGame fixes seat order (already the join order), deals no cards
// itself (the caller deals starting cards via DealStartingCard first), and
// moves the game out of the lobby so the first turn can begin. ActivePlayerIdx
// starts at -1; call NextTurn to advance to seat 0 for the first turn.
func (g *Game) StartGame(minPlayers int) error {
	if g.Phase != PhaseLobby {
		return ErrWrongPhase
	}
	if len(g.Players) < minPlayers {
		return ErrTooFewPlayers
	}
	g.ActivePlayerIdx = -1
	g.currentTurnNumber = 0
	return nil
}

// NextTurn advances to the next seat and increments the turn counter. Call
// once after game start and once after each REVEALING phase completes, but
// not when redoing a turn via SkipTrack (§8.11).
func (g *Game) NextTurn() error {
	if len(g.Players) == 0 {
		return ErrPlayerNotFound
	}
	g.ActivePlayerIdx = (g.ActivePlayerIdx + 1) % len(g.Players)
	g.currentTurnNumber++
	return nil
}

// BeginTurn starts a turn's PREPARING phase with the given track drawn for
// the current active player (§8.5). The track is marked used. Reusable for
// SkipTrack's "new PREPARING phase for the same active player" without
// advancing the turn counter.
func (g *Game) BeginTurn(track Card) error {
	active := g.ActivePlayer()
	if active == nil {
		return ErrPlayerNotFound
	}
	g.Phase = PhasePreparing
	g.Turn = &Turn{
		Number:         g.currentTurnNumber,
		ActivePlayerID: active.ID,
		Track:          track,
		PlacementSlot:  -2, // sentinel: not yet submitted
		Order:          g.seatOrderFrom(g.ActivePlayerIdx),
		Challenges:     map[string]int{},
		Passed:         map[string]bool{},
		Spent:          map[string]int{},
	}
	g.UsedTrackIDs[track.TrackID] = true
	g.TracksUsedCount++
	return nil
}

// seatOrderFrom returns player ids starting at fromIdx and wrapping around,
// i.e. the order in which challenges are evaluated (§8.8 rule 2).
func (g *Game) seatOrderFrom(fromIdx int) []string {
	n := len(g.Players)
	order := make([]string, 0, n-1)
	for i := 1; i < n; i++ {
		order = append(order, g.Players[(fromIdx+i)%n].ID)
	}
	return order
}

// BeginPlacing transitions PREPARING -> PLACING once clients are ready or
// the prepare cap elapses.
func (g *Game) BeginPlacing() error {
	if g.Phase != PhasePreparing {
		return ErrWrongPhase
	}
	g.Phase = PhasePlacing
	return nil
}

// PlaceCard records the active player's placement (§8.5 PLACING). Returns
// whether the challenge phase should be skipped entirely because no other
// connected player holds a token.
func (g *Game) PlaceCard(playerID string, slotIndex int) (skipChallenge bool, err error) {
	if g.Phase != PhasePlacing {
		return false, ErrWrongPhase
	}
	if g.Turn == nil || playerID != g.Turn.ActivePlayerID {
		return false, ErrNotActivePlayer
	}
	active := g.ActivePlayer()
	n := len(active.Timeline)
	if slotIndex < 0 || slotIndex > n {
		return false, ErrInvalidSlot
	}
	g.Turn.PlacementSlot = slotIndex
	g.Turn.PlacementSubmitted = true
	return g.advanceFromPlacing(), nil
}

// PlacementTimeout auto-fails the active placement (§8.5: slotIndex = -1).
func (g *Game) PlacementTimeout() (skipChallenge bool, err error) {
	if g.Phase != PhasePlacing {
		return false, ErrWrongPhase
	}
	g.Turn.PlacementSlot = -1
	g.Turn.PlacementSubmitted = true
	return g.advanceFromPlacing(), nil
}

// advanceFromPlacing moves to CHALLENGING, or reports that CHALLENGING
// should be skipped because nobody else holds a token (§8.5).
func (g *Game) advanceFromPlacing() bool {
	if !g.anyChallengerHasTokens() {
		g.finishChallenging()
		return true
	}
	g.Phase = PhaseChallenging
	return false
}

func (g *Game) finishChallenging() {
	g.Phase = PhaseRevealing
}

func (g *Game) anyChallengerHasTokens() bool {
	for _, p := range g.Players {
		if p.ID != g.Turn.ActivePlayerID && p.Tokens > 0 {
			return true
		}
	}
	return false
}

// Challenge records a non-active player's slot claim, spending one token
// immediately (§8.5 CHALLENGING, §8.8). Returns whether every eligible
// player has now challenged or passed, so the phase can end immediately.
func (g *Game) Challenge(playerID string, slotIndex int) (phaseComplete bool, err error) {
	if g.Phase != PhaseChallenging {
		return false, ErrWrongPhase
	}
	if playerID == g.Turn.ActivePlayerID {
		return false, ErrIsActivePlayer
	}
	p := g.Player(playerID)
	if p == nil {
		return false, ErrPlayerNotFound
	}
	if _, done := g.Turn.Challenges[playerID]; done {
		return false, ErrAlreadyActed
	}
	if g.Turn.Passed[playerID] {
		return false, ErrAlreadyActed
	}
	if p.Tokens <= 0 {
		return false, ErrNoTokens
	}
	active := g.ActivePlayer()
	n := len(active.Timeline)
	if slotIndex < 0 || slotIndex > n {
		return false, ErrInvalidSlot
	}
	if slotIndex == g.Turn.PlacementSlot {
		return false, ErrSlotIsActiveSlot
	}
	for _, taken := range g.Turn.Challenges {
		if taken == slotIndex {
			return false, ErrSlotTaken
		}
	}

	p.Tokens--
	g.Turn.Challenges[playerID] = slotIndex
	g.Turn.Spent[playerID]++
	done := g.allEligibleDone()
	if done {
		g.finishChallenging()
	}
	return done, nil
}

// PassChallenge records a player's explicit early exit from challenging.
func (g *Game) PassChallenge(playerID string) (phaseComplete bool, err error) {
	if g.Phase != PhaseChallenging {
		return false, ErrWrongPhase
	}
	if playerID == g.Turn.ActivePlayerID {
		return false, ErrIsActivePlayer
	}
	if g.Player(playerID) == nil {
		return false, ErrPlayerNotFound
	}
	if _, done := g.Turn.Challenges[playerID]; done {
		return false, ErrAlreadyActed
	}
	if g.Turn.Passed[playerID] {
		return false, ErrAlreadyActed
	}
	g.Turn.Passed[playerID] = true
	done := g.allEligibleDone()
	if done {
		g.finishChallenging()
	}
	return done, nil
}

// allEligibleDone reports whether every player who could still challenge
// (not active, holds >=1 token, hasn't already acted) has challenged or
// passed.
func (g *Game) allEligibleDone() bool {
	for _, p := range g.Players {
		if p.ID == g.Turn.ActivePlayerID {
			continue
		}
		_, challenged := g.Turn.Challenges[p.ID]
		passed := g.Turn.Passed[p.ID]
		if challenged || passed {
			continue
		}
		if p.Tokens > 0 {
			return false
		}
	}
	return true
}

// ChallengeTimeout forces the end of the CHALLENGING phase when its window
// elapses, regardless of stragglers.
func (g *Game) ChallengeTimeout() error {
	if g.Phase != PhaseChallenging {
		return ErrWrongPhase
	}
	g.Phase = PhaseRevealing
	return nil
}

// SkipTrack abandons the current turn without scoring (§8.11). The caller
// must then draw a fresh track and call BeginTurn again for the same active
// player; the turn counter is not advanced.
func (g *Game) SkipTrack(playerID string) error {
	if playerID != g.HostID {
		return ErrNotHost
	}
	if g.Phase != PhasePreparing && g.Phase != PhasePlacing && g.Phase != PhaseChallenging {
		return ErrWrongPhase
	}
	g.Turn = nil
	return nil
}
