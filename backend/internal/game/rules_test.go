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
		if _, err := g.Challenge(ids["Bob"], 0); err != nil {
			t.Fatalf("challenge: %v", err)
		}
		if err := g.ChallengeTimeout(); err != nil {
			t.Fatalf("challenge timeout: %v", err)
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
		if _, err := g.Challenge(ids["Bob"], 1); err != nil { // correct: after 1980
			t.Fatalf("challenge: %v", err)
		}
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
		if _, err := g.Challenge(ids["Bob"], 1); err != nil {
			t.Fatalf("challenge bob: %v", err)
		}
		caraTokensBefore := g.Player(ids["Cara"]).Tokens
		if _, err := g.Challenge(ids["Cara"], 1); err != ErrSlotTaken {
			t.Fatalf("expected ErrSlotTaken, got %v", err)
		}
		if g.Player(ids["Cara"]).Tokens != caraTokensBefore {
			t.Error("expected no token spent on a rejected challenge")
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
		if _, err := g.Challenge(ids["Bob"], 1); err != nil { // wrong slot
			t.Fatalf("challenge bob: %v", err)
		}
		if _, err := g.Challenge(ids["Cara"], 3); err != nil { // wrong slot
			t.Fatalf("challenge cara: %v", err)
		}
		done, err := g.Challenge(ids["Dan"], 2) // correct slot
		if err != nil {
			t.Fatalf("challenge dan: %v", err)
		}
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
		if _, err := g.Challenge(ids["Bob"], 2); err != nil { // also wrong: 1980 doesn't fit after 1990
			t.Fatalf("challenge: %v", err)
		}
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

func TestTokenAccounting(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob", "Cara")
	g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
	startTurn(t, g, Card{TrackID: "t1", Year: 1990})
	_, _ = g.PlaceCard(ids["Anna"], 0) // wrong

	bob := g.Player(ids["Bob"])
	if bob.Tokens != 2 {
		t.Fatalf("expected Bob to start with 2 tokens, got %d", bob.Tokens)
	}
	if _, err := g.Challenge(ids["Bob"], 1); err != nil {
		t.Fatalf("challenge: %v", err)
	}
	if bob.Tokens != 1 {
		t.Errorf("expected token spent immediately, got %d", bob.Tokens)
	}

	// Cannot claim an already-taken slot.
	if _, err := g.Challenge(ids["Cara"], 1); err != ErrSlotTaken {
		t.Errorf("expected ErrSlotTaken, got %v", err)
	}
	// Cannot claim the active player's own slot.
	if _, err := g.Challenge(ids["Cara"], 0); err != ErrSlotIsActiveSlot {
		t.Errorf("expected ErrSlotIsActiveSlot, got %v", err)
	}

	cara := g.Player(ids["Cara"])
	cara.Tokens = 0
	if _, err := g.Challenge(ids["Cara"], 2); err != ErrNoTokens {
		t.Errorf("expected ErrNoTokens, got %v", err)
	}

	if err := g.ChallengeTimeout(); err != nil {
		t.Fatalf("challenge timeout: %v", err)
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
}

func TestTokenCapOnRefund(t *testing.T) {
	g, ids := newTestGame(t, "Anna", "Bob")
	g.Player(ids["Anna"]).Timeline = []Card{{Year: 1980}}
	g.Player(ids["Bob"]).Tokens = g.Settings.MaxTokens
	startTurn(t, g, Card{TrackID: "t1", Year: 1990})
	_, _ = g.PlaceCard(ids["Anna"], 0) // wrong
	if _, err := g.Challenge(ids["Bob"], 1); err != nil {
		t.Fatalf("challenge: %v", err)
	}
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
		_ = g.ChallengeTimeout()
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
			_ = g.ChallengeTimeout()
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
		_ = g.ChallengeTimeout()
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
