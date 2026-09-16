package game

import "testing"

func TestPlacementCorrectness(t *testing.T) {
	cases := []struct {
		name     string
		timeline []Card
		year     int
		slot     int
		want     bool
	}{
		{"empty timeline slot 0", nil, 1980, 0, true},
		{"empty timeline invalid slot", nil, 1980, 1, false},
		{"empty timeline negative slot", nil, 1980, -1, false},
		{"single card before", []Card{{Year: 2000}}, 1990, 0, true},
		{"single card after", []Card{{Year: 2000}}, 2010, 1, true},
		{"single card wrong before", []Card{{Year: 2000}}, 2010, 0, false},
		{"single card wrong after", []Card{{Year: 2000}}, 1990, 1, false},
		{"single card equal at 0", []Card{{Year: 2000}}, 2000, 0, true},
		{"single card equal at 1", []Card{{Year: 2000}}, 2000, 1, true},
		{"boundary lower", []Card{{Year: 1980}, {Year: 2000}}, 1990, 1, true},
		{"boundary too low", []Card{{Year: 1980}, {Year: 2000}}, 1970, 1, false},
		{"boundary too high", []Card{{Year: 1980}, {Year: 2000}}, 2010, 1, false},
		{"duplicate years any slot 0", []Card{{Year: 1990}, {Year: 1990}}, 1990, 0, true},
		{"duplicate years any slot 1", []Card{{Year: 1990}, {Year: 1990}}, 1990, 1, true},
		{"duplicate years any slot 2", []Card{{Year: 1990}, {Year: 1990}}, 1990, 2, true},
		{"slot out of range high", []Card{{Year: 1990}}, 1990, 5, false},
		{"auto-fail sentinel", []Card{{Year: 1990}}, 1990, -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PlacementCorrect(c.timeline, c.year, c.slot)
			if got != c.want {
				t.Errorf("PlacementCorrect(%v, %d, %d) = %v, want %v", c.timeline, c.year, c.slot, got, c.want)
			}
		})
	}
}

func newTestGame(t *testing.T, names ...string) (*Game, map[string]string) {
	t.Helper()
	g := New("game1", "ABC123", Settings{TargetCards: 10, StartTokens: 2, MaxTokens: 5, EnableSongGuess: false})
	ids := map[string]string{}
	for i, name := range names {
		id := name + "-id"
		if _, err := g.AddPlayer(id, name, 12); err != nil {
			t.Fatalf("add player %s: %v", name, err)
		}
		_ = i
		ids[name] = id
	}
	if err := g.StartGame(2); err != nil {
		t.Fatalf("start game: %v", err)
	}
	return g, ids
}

func startTurn(t *testing.T, g *Game, track Card) {
	t.Helper()
	if err := g.NextTurn(); err != nil {
		t.Fatalf("next turn: %v", err)
	}
	if err := g.BeginTurn(track); err != nil {
		t.Fatalf("begin turn: %v", err)
	}
	if err := g.BeginPlacing(); err != nil {
		t.Fatalf("begin placing: %v", err)
	}
}

// steal claims a steal for playerID and places it on slot.
func steal(t *testing.T, g *Game, playerID string, slot int) (phaseComplete bool) {
	t.Helper()
	if err := g.ClaimSteal(playerID); err != nil {
		t.Fatalf("claim steal %s: %v", playerID, err)
	}
	done, err := g.Challenge(playerID, slot)
	if err != nil {
		t.Fatalf("place steal %s: %v", playerID, err)
	}
	return done
}

func TestChallengeResolution(t *testing.T) {
	t.Run("active correct, challenges ignored", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		g.Player(ids["Anna"]).Timeline = []Card{{TrackID: "seed", Year: 1980}}
		g.Player(ids["Bob"]).Timeline = []Card{{TrackID: "seed2", Year: 1980}}
		g.ActivePlayerIdx = -1 // Anna goes first after NextTurn
		startTurn(t, g, Card{TrackID: "t1", Title: "Song", Artist: "Artist", Year: 1990})
		if g.Turn.ActivePlayerID != ids["Anna"] {
			t.Fatalf("expected Anna active, got %s", g.Turn.ActivePlayerID)
		}
		if _, err := g.PlaceCard(ids["Anna"], 1); err != nil { // correct: after 1980
			t.Fatalf("place: %v", err)
		}
		if g.Phase != PhaseChallenging {
			t.Fatalf("expected CHALLENGING, got %s", g.Phase)
		}
		steal(t, g, ids["Bob"], 0)
		if _, err := g.CloseStealWindow(); err != nil {
			t.Fatalf("close steal window: %v", err)
		}
		reveal, err := g.Resolve()
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if !reveal.ActiveCorrect || reveal.WinnerPlayerID != ids["Anna"] {
			t.Fatalf("expected Anna to win, got %+v", reveal)
		}
		if len(g.Player(ids["Anna"]).Timeline) != 2 {
			t.Errorf("expected Anna to have 2 cards, got %d", len(g.Player(ids["Anna"]).Timeline))
		}
		// Bob's spent token is not refunded even though ignored.
		if g.Player(ids["Bob"]).Tokens != 1 {
			t.Errorf("expected Bob to have 1 token left (spent, not refunded), got %d", g.Player(ids["Bob"]).Tokens)
		}
	})

	t.Run("active wrong, one correct challenger wins", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		g.Player(ids["Anna"]).Timeline = []Card{{TrackID: "seed", Year: 1980}}
		g.Player(ids["Bob"]).Timeline = []Card{{TrackID: "seed2", Year: 1980}}
		startTurn(t, g, Card{TrackID: "t1", Title: "Song", Artist: "Artist", Year: 1990})
		if _, err := g.PlaceCard(ids["Anna"], 0); err != nil { // wrong: before 1980, but year 1990
			t.Fatalf("place: %v", err)
		}
		steal(t, g, ids["Bob"], 1) // correct: after 1980
		if _, err := g.PassChallenge(ids["Cara"]); err != nil {
			t.Fatalf("pass: %v", err)
		}
		reveal, err := g.Resolve()
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if reveal.ActiveCorrect {
			t.Fatal("expected active placement to be wrong")
		}
		if reveal.WinnerPlayerID != ids["Bob"] {
			t.Fatalf("expected Bob to win the card, got %q", reveal.WinnerPlayerID)
		}
		if len(g.Player(ids["Bob"]).Timeline) != 2 {
			t.Errorf("expected Bob to have 2 cards, got %d", len(g.Player(ids["Bob"]).Timeline))
		}
		if g.Player(ids["Bob"]).Tokens != 2 {
			t.Errorf("expected Bob's token refunded (back to 2), got %d", g.Player(ids["Bob"]).Tokens)
		}
		if len(g.Player(ids["Anna"]).Timeline) != 1 {
			t.Errorf("expected Anna to still have only 1 card, got %d", len(g.Player(ids["Anna"]).Timeline))
		}
	})

	t.Run("second challenger cannot claim an already-taken slot", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		g.Player(ids["Anna"]).Timeline = []Card{{TrackID: "seed", Year: 1980}}
		startTurn(t, g, Card{TrackID: "t1", Title: "Song", Artist: "Artist", Year: 1990})
		if _, err := g.PlaceCard(ids["Anna"], 0); err != nil { // wrong
			t.Fatalf("place: %v", err)
		}
		steal(t, g, ids["Bob"], 1)
		if err := g.ClaimSteal(ids["Cara"]); err != nil {
			t.Fatalf("claim cara: %v", err)
		}
		if _, err := g.Challenge(ids["Cara"], 1); err != ErrSlotTaken {
			t.Fatalf("expected ErrSlotTaken, got %v", err)
		}
	})

	t.Run("seat order wins among distinct correct slots", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara", "Dan")
		g.Player(ids["Anna"]).Timeline = []Card{{TrackID: "s1", Year: 1960}, {TrackID: "s2", Year: 1970}, {TrackID: "s3", Year: 2010}}
		startTurn(t, g, Card{TrackID: "t1", Title: "Song", Artist: "Artist", Year: 1990})
		// Timeline [1960, 1970, 2010]; the only correct slot for year 1990 is 2
		// (between 1970 and 2010). Slots 0, 1 and 3 are all wrong.
		if _, err := g.PlaceCard(ids["Anna"], 0); err != nil { // wrong
			t.Fatalf("place: %v", err)
		}
		// Bob and Cara claim distinct wrong slots; only Dan (last in seat order:
		// Bob, Cara, Dan) picks the actually-correct slot.
		steal(t, g, ids["Bob"], 1)         // wrong slot
		steal(t, g, ids["Cara"], 3)        // wrong slot
		done := steal(t, g, ids["Dan"], 2) // correct slot
		if !done {
			t.Fatal("expected challenging phase complete once everyone has acted")
		}
		reveal, err := g.Resolve()
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if reveal.WinnerPlayerID != ids["Dan"] {
			t.Fatalf("expected Dan to win (only correct challenger), got %q", reveal.WinnerPlayerID)
		}
	})

	t.Run("no correct challenger discards the card", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		g.Player(ids["Anna"]).Timeline = []Card{{TrackID: "s1", Year: 1970}, {TrackID: "s2", Year: 1990}}
		startTurn(t, g, Card{TrackID: "t1", Title: "Song", Artist: "Artist", Year: 1980})
		if _, err := g.PlaceCard(ids["Anna"], 0); err != nil { // wrong: 1980 doesn't fit before 1970
			t.Fatalf("place: %v", err)
		}
		steal(t, g, ids["Bob"], 2) // also wrong: 1980 doesn't fit after 1990
		if _, err := g.PassChallenge(ids["Cara"]); err != nil {
			t.Fatalf("pass: %v", err)
		}
		reveal, err := g.Resolve()
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if reveal.WinnerPlayerID != "" {
			t.Fatalf("expected no winner, got %q", reveal.WinnerPlayerID)
		}
		if len(g.Player(ids["Anna"]).Timeline) != 2 || len(g.Player(ids["Bob"]).Timeline) != 0 {
			t.Fatal("expected no timeline changes")
		}
	})

	t.Run("wrap-around seat ordering", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		// Make Cara active by rotating twice.
		startTurn(t, g, Card{TrackID: "warmup", Year: 2000})
		_, _ = g.PlaceCard(g.Turn.ActivePlayerID, 0)
		_, _ = g.Resolve()
		g.Phase = PhaseRevealing
		startTurn(t, g, Card{TrackID: "warmup2", Year: 2000})
		_, _ = g.PlaceCard(g.Turn.ActivePlayerID, 0)
		_, _ = g.Resolve()
		g.Phase = PhaseRevealing

		startTurn(t, g, Card{TrackID: "t1", Title: "Song", Artist: "Artist", Year: 1990})
		active := g.Turn.ActivePlayerID
		if active != ids["Cara"] {
			t.Fatalf("expected Cara active on third turn, got %s", active)
		}
		// Seat order after Cara wraps around to Anna, then Bob.
		wantOrder := []string{ids["Anna"], ids["Bob"]}
		if len(g.Turn.Order) != 2 || g.Turn.Order[0] != wantOrder[0] || g.Turn.Order[1] != wantOrder[1] {
			t.Fatalf("seat order = %v, want %v", g.Turn.Order, wantOrder)
		}
	})
}

func TestChallengePreviewRequiresClaimAndIsRevisable(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob")
	g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}, {Year: 2000}}
	startTurn(t, g, Card{TrackID: "t1", Year: 1990})
	if _, err := g.PlaceCard(ids["Anna"], 0); err != nil {
		t.Fatalf("place: %v", err)
	}

	if err := g.PreviewChallenge(ids["Bob"], 1); err != ErrStealNotClaimed {
		t.Fatalf("preview before claim: got %v, want ErrStealNotClaimed", err)
	}
	before := g.Player(ids["Bob"]).Tokens
	if err := g.ClaimSteal(ids["Bob"]); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if got := g.Player(ids["Bob"]).Tokens; got != before-1 {
		t.Fatalf("claim tokens = %d, want %d", got, before-1)
	}
	if err := g.PreviewChallenge(ids["Bob"], 1); err != nil {
		t.Fatalf("preview: %v", err)
	}
	if err := g.PreviewChallenge(ids["Bob"], 2); err != nil {
		t.Fatalf("replace preview: %v", err)
	}
	if got := g.Turn.ChallengePreviews[ids["Bob"]]; got != 2 {
		t.Fatalf("revised preview slot = %d, want 2", got)
	}
	if _, err := g.Challenge(ids["Bob"], 1); err != nil {
		t.Fatalf("final challenge: %v", err)
	}
	if _, ok := g.Turn.ChallengePreviews[ids["Bob"]]; ok {
		t.Error("expected placement to clear preview")
	}
	if got := g.Player(ids["Bob"]).Tokens; got != before-1 {
		t.Fatalf("placing must not spend another token: %d, want %d", got, before-1)
	}
}

func TestStealWindow(t *testing.T) {
	setup := func(t *testing.T) (*Game, map[string]string) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
		startTurn(t, g, Card{TrackID: "t1", Year: 1990})
		if _, err := g.PlaceCard(ids["Anna"], 0); err != nil {
			t.Fatalf("place: %v", err)
		}
		return g, ids
	}

	t.Run("claimant has unlimited time after the window closes", func(t *testing.T) {
		g, ids := setup(t)
		if err := g.ClaimSteal(ids["Bob"]); err != nil {
			t.Fatalf("claim: %v", err)
		}
		done, err := g.CloseStealWindow()
		if err != nil || done {
			t.Fatalf("close window: done=%v err=%v; want phase to wait for Bob", done, err)
		}
		if g.Phase != PhaseChallenging {
			t.Fatalf("phase = %s, want CHALLENGING", g.Phase)
		}
		if err := g.ClaimSteal(ids["Cara"]); err != ErrStealWindowOver {
			t.Fatalf("late claim: got %v, want ErrStealWindowOver", err)
		}
		if _, err := g.PassChallenge(ids["Cara"]); err != ErrStealWindowOver {
			t.Fatalf("late pass: got %v, want ErrStealWindowOver", err)
		}
		if pending := g.PendingStealers(); len(pending) != 1 || pending[0] != ids["Bob"] {
			t.Fatalf("pending = %v, want [Bob]", pending)
		}
		done, err = g.Challenge(ids["Bob"], 1)
		if err != nil || !done || g.Phase != PhaseRevealing {
			t.Fatalf("place: done=%v err=%v phase=%s", done, err, g.Phase)
		}
	})

	t.Run("no claims ends stealing when the window closes", func(t *testing.T) {
		g, _ := setup(t)
		done, err := g.CloseStealWindow()
		if err != nil || !done || g.Phase != PhaseRevealing {
			t.Fatalf("done=%v err=%v phase=%s", done, err, g.Phase)
		}
	})

	t.Run("open window waits for undecided players", func(t *testing.T) {
		g, ids := setup(t)
		if done := steal(t, g, ids["Bob"], 1); done {
			t.Fatal("Cara can still claim; stealing must not be complete")
		}
		done, err := g.PassChallenge(ids["Cara"])
		if err != nil || !done {
			t.Fatalf("pass: done=%v err=%v", done, err)
		}
	})

	t.Run("claim is binding", func(t *testing.T) {
		g, ids := setup(t)
		if err := g.ClaimSteal(ids["Bob"]); err != nil {
			t.Fatalf("claim: %v", err)
		}
		if _, err := g.PassChallenge(ids["Bob"]); err != ErrAlreadyActed {
			t.Fatalf("pass after claim: got %v, want ErrAlreadyActed", err)
		}
		if err := g.ClaimSteal(ids["Bob"]); err != ErrAlreadyActed {
			t.Fatalf("second claim: got %v, want ErrAlreadyActed", err)
		}
	})

	t.Run("a departed claimant no longer holds up the turn", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara", "Dan")
		g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
		startTurn(t, g, Card{TrackID: "t1", Year: 1990})
		if _, err := g.PlaceCard(ids["Anna"], 0); err != nil {
			t.Fatalf("place: %v", err)
		}
		if err := g.ClaimSteal(ids["Bob"]); err != nil {
			t.Fatalf("claim: %v", err)
		}
		if done, _ := g.CloseStealWindow(); done {
			t.Fatal("expected to wait for Bob")
		}
		if ended := g.RemovePlayer(ids["Bob"], 2); ended {
			t.Fatal("game should continue with 3 players")
		}
		if !g.FinishStealingIfComplete() || g.Phase != PhaseRevealing {
			t.Fatalf("phase = %s, want REVEALING", g.Phase)
		}
	})
}

func TestTokenAccounting(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob", "Cara", "Dan")
	g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}, {Year: 2000}}
	startTurn(t, g, Card{TrackID: "t1", Year: 1990})
	_, _ = g.PlaceCard(ids["Anna"], 0) // wrong

	bob := g.Player(ids["Bob"])
	if bob.Tokens != 2 {
		t.Fatalf("expected Bob to start with 2 tokens, got %d", bob.Tokens)
	}
	if err := g.ClaimSteal(ids["Bob"]); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if bob.Tokens != 1 {
		t.Errorf("expected token spent on pressing steal, got %d", bob.Tokens)
	}
	if _, err := g.Challenge(ids["Bob"], 1); err != nil { // correct
		t.Fatalf("challenge: %v", err)
	}

	cara := g.Player(ids["Cara"])
	if err := g.ClaimSteal(ids["Cara"]); err != nil {
		t.Fatalf("claim cara: %v", err)
	}
	// Cannot claim an already-taken slot.
	if _, err := g.Challenge(ids["Cara"], 1); err != ErrSlotTaken {
		t.Errorf("expected ErrSlotTaken, got %v", err)
	}
	// Cannot claim the active player's own slot.
	if _, err := g.Challenge(ids["Cara"], 0); err != ErrSlotIsActiveSlot {
		t.Errorf("expected ErrSlotIsActiveSlot, got %v", err)
	}
	if cara.Tokens != 1 {
		t.Errorf("expected exactly one token spent by Cara, got %d left", cara.Tokens)
	}

	g.Player(ids["Dan"]).Tokens = 0
	if err := g.ClaimSteal(ids["Dan"]); err != ErrNoTokens {
		t.Errorf("expected ErrNoTokens, got %v", err)
	}

	if done, err := g.CloseStealWindow(); err != nil || done {
		t.Fatalf("close steal window: done=%v err=%v; want to wait for Cara", done, err)
	}
	if done, err := g.Challenge(ids["Cara"], 2); err != nil || !done {
		t.Fatalf("cara places after window: done=%v err=%v", done, err)
	}
	reveal, err := g.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reveal.WinnerPlayerID != ids["Bob"] {
		t.Fatalf("expected Bob to win, got %q", reveal.WinnerPlayerID)
	}
	if bob.Tokens != 2 {
		t.Errorf("expected Bob's token refunded to 2, got %d", bob.Tokens)
	}
	if cara.Tokens != 1 {
		t.Errorf("expected Cara's wrong steal to cost her token, got %d", cara.Tokens)
	}
}

func TestTokenCapOnRefund(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob")
	g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
	g.Player(ids["Bob"]).Tokens = g.Settings.MaxTokens
	startTurn(t, g, Card{TrackID: "t1", Year: 1990})
	_, _ = g.PlaceCard(ids["Anna"], 0) // wrong
	steal(t, g, ids["Bob"], 1)
	reveal, err := g.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reveal.WinnerPlayerID != ids["Bob"] {
		t.Fatalf("expected bob to win")
	}
	if g.Player(ids["Bob"]).Tokens != g.Settings.MaxTokens {
		t.Errorf("expected tokens capped at %d, got %d", g.Settings.MaxTokens, g.Player(ids["Bob"]).Tokens)
	}
}

func TestWinCondition(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob")
	g.Settings.TargetCards = 2
	g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
	startTurn(t, g, Card{TrackID: "t1", Year: 1990})
	if _, err := g.PlaceCard(ids["Anna"], 1); err != nil {
		t.Fatalf("place: %v", err)
	}
	if g.Phase == PhaseChallenging {
		_, _ = g.CloseStealWindow()
	}
	reveal, err := g.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reveal.WinnerPlayerID != ids["Anna"] {
		t.Fatalf("expected anna to win the card")
	}
	if g.Phase != PhaseGameOver {
		t.Fatalf("expected GAME_OVER, got %s", g.Phase)
	}
	if g.WinnerID != ids["Anna"] {
		t.Fatalf("expected winner Anna, got %s", g.WinnerID)
	}
}

func TestTurnRotation(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob", "Cara")
	var seen []string
	for i := 0; i < 4; i++ {
		startTurn(t, g, Card{TrackID: "t" + string(rune('a'+i)), Year: 2000 + i})
		seen = append(seen, g.Turn.ActivePlayerID)
		_, _ = g.PlaceCard(g.Turn.ActivePlayerID, len(g.ActivePlayer().Timeline))
		if g.Phase == PhaseChallenging {
			_, _ = g.CloseStealWindow()
		}
		_, _ = g.Resolve()
		if g.Phase == PhaseGameOver {
			break
		}
		g.Phase = PhaseRevealing
	}
	want := []string{ids["Anna"], ids["Bob"], ids["Cara"], ids["Anna"]}
	for i := range want {
		if i >= len(seen) {
			break
		}
		if seen[i] != want[i] {
			t.Errorf("turn %d active = %s, want %s", i, seen[i], want[i])
		}
	}
}

func TestTurnRotationPastRemovedPlayer(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob", "Cara")
	startTurn(t, g, Card{TrackID: "t1", Year: 2000})
	if g.Turn.ActivePlayerID != ids["Anna"] {
		t.Fatalf("expected Anna active")
	}
	_, _ = g.PlaceCard(ids["Anna"], len(g.ActivePlayer().Timeline))
	if g.Phase == PhaseChallenging {
		_, _ = g.CloseStealWindow()
	}
	_, _ = g.Resolve()
	g.Phase = PhaseRevealing

	// Remove Bob (next in line) between turns.
	g.RemovePlayer(ids["Bob"], 2)

	startTurn(t, g, Card{TrackID: "t2", Year: 2001})
	if g.Turn.ActivePlayerID != ids["Cara"] {
		t.Fatalf("expected Cara active after Bob removed, got %s", g.Turn.ActivePlayerID)
	}
}

func TestPhaseTransitions(t *testing.T) {
	t.Run("placement timeout auto-fails", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob")
		g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
		startTurn(t, g, Card{TrackID: "t1", Year: 1990})
		if _, err := g.PlacementTimeout(); err != nil {
			t.Fatalf("timeout: %v", err)
		}
		if g.Turn.PlacementSlot != -1 {
			t.Errorf("expected slot -1 after timeout, got %d", g.Turn.PlacementSlot)
		}
	})

	t.Run("challenge phase skipped when nobody has tokens", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		g.Player(ids["Bob"]).Tokens = 0
		g.Player(ids["Cara"]).Tokens = 0
		startTurn(t, g, Card{TrackID: "t1", Year: 1990})
		skip, err := g.PlaceCard(g.Turn.ActivePlayerID, 0)
		if err != nil {
			t.Fatalf("place: %v", err)
		}
		if !skip {
			t.Fatal("expected challenge phase to be skipped")
		}
		if g.Phase != PhaseRevealing {
			t.Fatalf("expected REVEALING immediately, got %s", g.Phase)
		}
	})

	t.Run("challenge phase entered when someone has tokens", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob", "Cara")
		_ = ids
		startTurn(t, g, Card{TrackID: "t1", Year: 1990})
		skip, err := g.PlaceCard(g.Turn.ActivePlayerID, 0)
		if err != nil {
			t.Fatalf("place: %v", err)
		}
		if skip {
			t.Fatal("expected challenge phase to run")
		}
		if g.Phase != PhaseChallenging {
			t.Fatalf("expected CHALLENGING, got %s", g.Phase)
		}
	})

	t.Run("wrong phase errors", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob")
		if _, err := g.PlaceCard(ids["Anna"], 0); err != ErrWrongPhase {
			t.Errorf("expected ErrWrongPhase placing in lobby-adjacent state, got %v", err)
		}
	})

	t.Run("only active player may place", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob")
		startTurn(t, g, Card{TrackID: "t1", Year: 1990})
		other := ids["Bob"]
		if g.Turn.ActivePlayerID == other {
			other = ids["Anna"]
		}
		if _, err := g.PlaceCard(other, 0); err != ErrNotActivePlayer {
			t.Errorf("expected ErrNotActivePlayer, got %v", err)
		}
	})
}

func TestSingleplayer(t *testing.T) {
	g := New("game1", "ABC123", Settings{TargetCards: 3, StartTokens: 2, MaxTokens: 5})
	if _, err := g.AddPlayer("solo-id", "Solo", 12); err != nil {
		t.Fatalf("add player: %v", err)
	}
	if err := g.StartGame(1); err != nil {
		t.Fatalf("start singleplayer game: %v", err)
	}
	g.Player("solo-id").Timeline = []Card{{Year: 1980}}

	for turn := 0; turn < 2; turn++ {
		startTurn(t, g, Card{TrackID: "t" + string(rune('a'+turn)), Year: 1990 + turn})
		if g.Turn.ActivePlayerID != "solo-id" {
			t.Fatalf("turn %d: active = %q", turn, g.Turn.ActivePlayerID)
		}
		skip, err := g.PlaceCard("solo-id", len(g.ActivePlayer().Timeline))
		if err != nil {
			t.Fatalf("turn %d place: %v", turn, err)
		}
		if !skip || g.Phase != PhaseRevealing {
			t.Fatalf("turn %d: stealing must be skipped, phase = %s", turn, g.Phase)
		}
		if _, err := g.Resolve(); err != nil {
			t.Fatalf("turn %d resolve: %v", turn, err)
		}
	}
	if g.Phase != PhaseGameOver || g.WinnerID != "solo-id" {
		t.Fatalf("phase = %s, winner = %q; want solo win at 3 cards", g.Phase, g.WinnerID)
	}
}

func TestAdjustTokens(t *testing.T) {
	t.Run("host can grant and remove tokens", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob") // Anna is host, StartTokens: 2, MaxTokens: 5
		if err := g.AdjustTokens(ids["Anna"], ids["Bob"], 1); err != nil {
			t.Fatalf("adjust: %v", err)
		}
		if got := g.Player(ids["Bob"]).Tokens; got != 3 {
			t.Errorf("tokens = %d, want 3", got)
		}
		if err := g.AdjustTokens(ids["Anna"], ids["Anna"], -1); err != nil {
			t.Fatalf("adjust self: %v", err)
		}
		if got := g.Player(ids["Anna"]).Tokens; got != 1 {
			t.Errorf("tokens = %d, want 1", got)
		}
	})

	t.Run("clamps to [0, MaxTokens]", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob")
		if err := g.AdjustTokens(ids["Anna"], ids["Bob"], -10); err != nil {
			t.Fatalf("adjust: %v", err)
		}
		if got := g.Player(ids["Bob"]).Tokens; got != 0 {
			t.Errorf("tokens = %d, want 0", got)
		}
		if err := g.AdjustTokens(ids["Anna"], ids["Bob"], 10); err != nil {
			t.Fatalf("adjust: %v", err)
		}
		if got := g.Player(ids["Bob"]).Tokens; got != g.Settings.MaxTokens {
			t.Errorf("tokens = %d, want %d", got, g.Settings.MaxTokens)
		}
	})

	t.Run("non-host is rejected", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob")
		if err := g.AdjustTokens(ids["Bob"], ids["Anna"], 1); err != ErrNotHost {
			t.Errorf("expected ErrNotHost, got %v", err)
		}
	})

	t.Run("unknown target is rejected", func(t *testing.T) {
		g, ids := newTestGame(t, "Anna", "Bob")
		if err := g.AdjustTokens(ids["Anna"], "nope", 1); err != ErrPlayerNotFound {
			t.Errorf("expected ErrPlayerNotFound, got %v", err)
		}
	})
}

func TestUpdateSettingsTokens(t *testing.T) {
	g := New("game1", "ABC123", Settings{TargetCards: 10, StartTokens: 2, MaxTokens: 5})
	host, _ := g.AddPlayer("host-id", "Host", 12)
	guest, _ := g.AddPlayer("guest-id", "Guest", 12)
	intp := func(v int) *int { return &v }

	if err := g.UpdateSettings(guest.ID, SettingsUpdate{MaxTokens: intp(3)}); err != ErrNotHost {
		t.Fatalf("guest update: got %v, want ErrNotHost", err)
	}
	if err := g.UpdateSettings(host.ID, SettingsUpdate{StartTokens: intp(4)}); err != nil {
		t.Fatalf("start tokens: %v", err)
	}
	if host.Tokens != 4 || guest.Tokens != 4 {
		t.Errorf("joined players must follow start tokens: host %d, guest %d", host.Tokens, guest.Tokens)
	}
	if err := g.UpdateSettings(host.ID, SettingsUpdate{MaxTokens: intp(3)}); err != nil {
		t.Fatalf("max tokens: %v", err)
	}
	if g.Settings.MaxTokens != 3 || g.Settings.StartTokens != 3 || guest.Tokens != 3 {
		t.Errorf("lowering max must lower start tokens: %+v, guest %d", g.Settings, guest.Tokens)
	}
	for _, u := range []SettingsUpdate{
		{MaxTokens: intp(0)},
		{MaxTokens: intp(MaxTokensLimit + 1)},
		{StartTokens: intp(4)}, // above max 3
		{StartTokens: intp(-1)},
		{MaxTokens: intp(2), StartTokens: intp(3)},
	} {
		if err := g.UpdateSettings(host.ID, u); err != ErrInvalidSettings {
			t.Errorf("UpdateSettings(%+v) = %v, want ErrInvalidSettings", u, err)
		}
	}
	if g.Settings.MaxTokens != 3 || g.Settings.StartTokens != 3 {
		t.Errorf("rejected updates changed settings: %+v", g.Settings)
	}
}
