package gamesvc

import (
	"log/slog"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

// pendingSongGuess is the active player's optional "Name that tune" answer,
// evaluated at reveal time (§8.6).
type pendingSongGuess struct {
	titleGuess, artistGuess string
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
	startedAt time.Time
	turnCount int

	trackSource *TrackSource
	mediaSigner *tokens.MediaSigner
	trackWarmer TrackWarmer
	store       *store.Store
	log         *slog.Logger
	cfg         Config
	manager     *Manager
}

func newManagedGame(id, inviteCode string, settings game.Settings, ts *TrackSource, signer *tokens.MediaSigner, warmer TrackWarmer, st *store.Store, cfg Config, log *slog.Logger, mgr *Manager) *ManagedGame {
	mg := &ManagedGame{
		id:              id,
		inviteCode:      inviteCode,
		cmdCh:           make(chan func(), 64),
		stopCh:          make(chan struct{}),
		g:               game.New(id, inviteCode, settings),
		conns:           map[string]*ws.Conn{},
		reconnectTimers: map[string]*time.Timer{},
		pendingReady:    map[string]bool{},
		createdAt:       time.Now(),
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

// call runs fn on the game's goroutine and blocks until it completes.
func (mg *ManagedGame) call(fn func()) {
	done := make(chan struct{})
	mg.enqueue(func() {
		fn()
		close(done)
	})
	<-done
}

func (mg *ManagedGame) stop() {
	close(mg.stopCh)
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
