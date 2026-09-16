package gamesvc

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/cards"
	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

type fakeSnapshots struct {
	mu      sync.Mutex
	saved   map[string]bool
	deleted map[string]bool
	kept    []string
}

func (f *fakeSnapshots) SaveSnapshot(_ context.Context, gameID, _ string, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved[gameID] = true
	return nil
}

func (f *fakeSnapshots) DeleteSnapshot(_ context.Context, gameID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted[gameID] = true
	return nil
}

func (f *fakeSnapshots) RecentSnapshots(context.Context, time.Duration) ([]store.GameSnapshot, error) {
	return nil, nil
}

func (f *fakeSnapshots) DeleteSnapshotsExcept(_ context.Context, keepIDs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kept = keepIDs
	return nil
}

func (f *fakeSnapshots) wasDeleted(gameID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleted[gameID]
}

type noWarmer struct{}

func (noWarmer) Warm(string) {}

func newTestManager(t *testing.T, grace time.Duration) (*Manager, *fakeSnapshots) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	snaps := &fakeSnapshots{saved: map[string]bool{}, deleted: map[string]bool{}}
	collection := cards.NewCollection(filepath.Join(t.TempDir(), "cards.json"), nil, time.Second, time.Hour, log)
	cfg := Config{
		MinPlayers: 1, MaxPlayers: 12, MaxConcurrentGames: 10,
		DefaultTargetCards: 10, DefaultStartTokens: 2, DefaultMaxTokens: 5,
		PlayerReconnectGrace: grace, LobbyIdleTimeout: time.Hour, MediaTTL: time.Minute,
	}
	const secret = "a-very-secret-key-that-is-32chars!!"
	m := NewManager(cfg, snaps, NewTrackSource(collection), tokens.NewMediaSigner(secret), noWarmer{}, tokens.NewIssuer(secret), log)
	return m, snaps
}

// connect opens a (message-discarding) socket for a player, like a browser
// loading the game page.
func connect(t *testing.T, m *Manager, token string) *ws.Conn {
	t.Helper()
	c := &ws.Conn{}
	gameID, playerID, ok := m.OnHello(c, token)
	if !ok {
		t.Fatal("hello rejected")
	}
	c.GameID, c.PlayerID = gameID, playerID
	return c
}

// disconnect simulates the socket closing and waits until the game has
// processed it.
func disconnect(m *Manager, c *ws.Conn) {
	m.OnDisconnect(c)
	if mg := m.gameByID(c.GameID); mg != nil {
		mg.call(func() {})
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestResumableGames(t *testing.T) {
	m, _ := newTestManager(t, time.Hour)
	host, err := m.CreateGame("Host", game.SettingsUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.CreateGame("Other", game.SettingsUpdate{})
	if err != nil {
		t.Fatal(err)
	}

	got := m.ResumableGames([]string{host.PlayerToken, "not-a-token", other.PlayerToken, host.PlayerToken})
	if !slices.Equal(got, []string{host.GameID, other.GameID}) {
		t.Fatalf("resumable = %v", got)
	}

	// A finished game is not offered again.
	m.gameByID(other.GameID).call(func() { m.gameByID(other.GameID).g.Phase = game.PhaseGameOver })
	if got := m.ResumableGames([]string{host.PlayerToken, other.PlayerToken}); !slices.Equal(got, []string{host.GameID}) {
		t.Fatalf("after game over: resumable = %v", got)
	}
}

func TestDisconnectedPlayerCanRejoinWithinGrace(t *testing.T) {
	m, _ := newTestManager(t, time.Hour)
	host, _ := m.CreateGame("Host", game.SettingsUpdate{})
	guest, err := m.JoinGame(host.InviteCode, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	connect(t, m, host.PlayerToken)
	guestConn := connect(t, m, guest.PlayerToken)

	disconnect(m, guestConn)
	if got := m.ResumableGames([]string{guest.PlayerToken}); len(got) != 1 {
		t.Fatal("disconnected player must still be able to rejoin")
	}
	connect(t, m, guest.PlayerToken) // rejoin with a new socket
	var connected bool
	m.gameByID(host.GameID).call(func() { connected = m.gameByID(host.GameID).g.Player(guest.PlayerID).Connected })
	if !connected {
		t.Fatal("rejoined player not marked connected")
	}
}

func TestPlayerRemovedAfterGraceGameContinuesForOthers(t *testing.T) {
	m, snaps := newTestManager(t, 50*time.Millisecond)
	host, _ := m.CreateGame("Host", game.SettingsUpdate{})
	guest, _ := m.JoinGame(host.InviteCode, "Guest")
	connect(t, m, host.PlayerToken)
	guestConn := connect(t, m, guest.PlayerToken)

	disconnect(m, guestConn)
	waitFor(t, "guest removal", func() bool { return len(m.ResumableGames([]string{guest.PlayerToken})) == 0 })

	if m.gameByID(host.GameID) == nil || len(m.ResumableGames([]string{host.PlayerToken})) != 1 {
		t.Fatal("game must continue for the remaining player")
	}
	if _, _, ok := m.OnHello(&ws.Conn{}, guest.PlayerToken); ok {
		t.Fatal("removed player must not rejoin")
	}
	if snaps.wasDeleted(host.GameID) {
		t.Fatal("snapshot of a running game must be kept")
	}
}

func TestGameForgottenWhenEveryoneIsGone(t *testing.T) {
	m, snaps := newTestManager(t, 50*time.Millisecond)
	host, _ := m.CreateGame("Host", game.SettingsUpdate{})
	guest, _ := m.JoinGame(host.InviteCode, "Guest")
	hostConn := connect(t, m, host.PlayerToken)
	guestConn := connect(t, m, guest.PlayerToken)

	disconnect(m, hostConn)
	disconnect(m, guestConn)
	waitFor(t, "game to be forgotten", func() bool { return m.gameByID(host.GameID) == nil })

	waitFor(t, "snapshot deletion", func() bool { return snaps.wasDeleted(host.GameID) })
	if got := m.ResumableGames([]string{host.PlayerToken, guest.PlayerToken}); len(got) != 0 {
		t.Fatalf("resumable = %v", got)
	}
	if m.Preview(host.InviteCode).Exists {
		t.Fatal("invite code must be released")
	}
}

func TestJanitorUsesTimeSinceLastDisconnect(t *testing.T) {
	m, snaps := newTestManager(t, time.Minute)
	solo, _ := m.CreateGame("Solo", game.SettingsUpdate{})
	mg := m.gameByID(solo.GameID)
	// An old game whose only player dropped out a moment ago...
	mg.call(func() { mg.createdAt = time.Now().Add(-30 * time.Minute) })
	disconnect(m, connect(t, m, solo.PlayerToken))

	m.reapOnce()
	if m.gameByID(solo.GameID) == nil {
		t.Fatal("a game that only just lost its connection must survive the janitor")
	}
	if !slices.Equal(snaps.kept, []string{solo.GameID}) {
		t.Fatalf("orphan cleanup must keep active games, kept %v", snaps.kept)
	}

	// ...but not one that has been empty for longer than the grace period.
	mg.call(func() { mg.emptySince = time.Now().Add(-2 * time.Minute) })
	m.reapOnce()
	if m.gameByID(solo.GameID) != nil {
		t.Fatal("janitor must forget a game that stayed empty past the grace period")
	}
}

func TestGameNeverConnectedIsForgotten(t *testing.T) {
	m, _ := newTestManager(t, time.Minute)
	g, _ := m.CreateGame("Host", game.SettingsUpdate{})
	mg := m.gameByID(g.GameID)
	mg.call(func() { mg.emptySince = time.Now().Add(-2 * time.Minute) })
	m.reapOnce()
	if m.gameByID(g.GameID) != nil {
		t.Fatal("a game nobody ever connected to must be forgotten")
	}
	// Calls into a forgotten game must not hang.
	done := make(chan struct{})
	go func() {
		mg.call(func() {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("call on a stopped game hung")
	}
}
