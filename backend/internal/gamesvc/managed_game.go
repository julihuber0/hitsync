package gamesvc

import (
	"log/slog"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// pendingSongGuess is the active player's latest guess for the token bonus,
// as typed; checked at reveal time (§8.6).
type pendingSongGuess struct {
	title, artist, album, year string
}

// ManagedGame owns one game's authoritative state, mutated only from its own
// goroutine via the command channel (§3.4).
type ManagedGame struct {
	id, inviteCode string
	cmdCh          chan func()
	stopCh         chan struct{}

	g     *game.Game
	conns map[string]*ws.Conn

	reconnectTimers map[string]*time.Timer

	phaseTimer      *time.Timer
	phaseGeneration int
	phaseDeadline   time.Time

	lastSkipAt time.Time

	// upcoming is the next turn's card, already announced for preloading.
	upcoming *Candidate

	pendingReady      map[string]bool
	pendingSongGuess  *pendingSongGuess
	currentYearSource string
	track             *activeTrack
	pendingWinnerID   string // captured winner while a game-ending reveal is still showing (§8.9)

	createdAt time.Time
	// emptySince is when the game last had no connected socket; zero while
	// anyone is connected. The janitor forgets games that stay empty for
	// longer than the reconnect grace.
	emptySince time.Time

	trackSource *TrackSource
	mediaSigner *tokens.MediaSigner
	trackWarmer TrackWarmer
	store       SnapshotStore
	log         *slog.Logger
	cfg         Config
	manager     *Manager
}

func newManagedGame(id, inviteCode string, settings game.Settings, ts *TrackSource, signer *tokens.MediaSigner, warmer TrackWarmer, st SnapshotStore, cfg Config, log *slog.Logger, mgr *Manager) *ManagedGame {
	now := time.Now()
	mg := &ManagedGame{
		id:              id,
		inviteCode:      inviteCode,
		cmdCh:           make(chan func(), 64),
		stopCh:          make(chan struct{}),
		g:               game.New(id, inviteCode, settings),
		conns:           map[string]*ws.Conn{},
		reconnectTimers: map[string]*time.Timer{},
		pendingReady:    map[string]bool{},
		createdAt:       now,
		emptySince:      now, // nobody has connected yet
		trackSource:     ts,
		mediaSigner:     signer,
		trackWarmer:     warmer,
		store:           st,
		log:             log,
		cfg:             cfg,
		manager:         mgr,
	}
	go mg.run()
	return mg
}

func (mg *ManagedGame) run() {
	for {
		select {
		case fn := <-mg.cmdCh:
			fn()
		case <-mg.stopCh:
			return
		}
	}
}

// enqueue schedules fn to run on the game's own goroutine. Safe from any
// goroutine. Silently dropped if the game has already stopped.
func (mg *ManagedGame) enqueue(fn func()) {
	select {
	case mg.cmdCh <- fn:
	case <-mg.stopCh:
	}
}

// call runs fn on the game's goroutine and blocks until it completes. If the
// game has stopped, fn may not run at all and call returns anyway.
func (mg *ManagedGame) call(fn func()) {
	done := make(chan struct{})
	mg.enqueue(func() {
		fn()
		close(done)
	})
	select {
	case <-done:
	case <-mg.stopCh:
	}
}

// stop ends the game's goroutine and its timers. Only Manager.removeGame
// calls it, exactly once.
func (mg *ManagedGame) stop() {
	close(mg.stopCh)
	if mg.phaseTimer != nil {
		mg.phaseTimer.Stop()
	}
	for _, t := range mg.reconnectTimers {
		t.Stop()
	}
}

// forget removes the game from the server entirely: from memory and its
// crash-recovery snapshot. Nobody can rejoin it afterwards. Must run on the
// game's goroutine.
func (mg *ManagedGame) forget(reason string) {
	mg.log.Info("forgetting game", "game_id", mg.id, "reason", reason)
	mg.deleteSnapshotAsync()
	mg.manager.removeGame(mg.id, mg.inviteCode)
}

// forgetIfEmpty forgets the game once its last player is gone. Reports
// whether it did.
func (mg *ManagedGame) forgetIfEmpty() bool {
	if len(mg.g.Players) > 0 {
		return false
	}
	mg.forget("all players left")
	return true
}

// broadcastState sends a freshly-rendered, per-recipient state snapshot to
// every connected client (§13.2).
func (mg *ManagedGame) broadcastState() {
	for playerID, c := range mg.conns {
		if c == nil {
			continue
		}
		c.Send(ws.TypeState, mg.buildState(playerID))
	}
	mg.saveSnapshotAsync()
}

func (mg *ManagedGame) sendError(c *ws.Conn, code, message string) {
	c.Send(ws.TypeError, ws.ErrorPayload{Code: code, Message: message})
}
