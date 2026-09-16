// Package gamesvc is the game manager: it owns every in-memory Game,
// dispatches WebSocket traffic to the right one, drives phase timers and
// track selection, and persists crash-recovery snapshots (§3.4).
package gamesvc

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/store"
	"github.com/julianhuber/hitsync/backend/internal/tokens"
	"github.com/julianhuber/hitsync/backend/internal/ws"
)

var (
	ErrGameNotFound = errors.New("game not found")
	ErrGameFull     = errors.New("game is full or already in progress")
	ErrTooManyGames = errors.New("too many concurrent games")
)

// PlayerIdentity is returned to a client on create/join (§11.2).
type PlayerIdentity struct {
	GameID      string
	PlayerID    string
	PlayerToken string
	InviteCode  string
}

// Manager owns every active game.
type Manager struct {
	mu       sync.RWMutex
	games    map[string]*ManagedGame
	byInvite map[string]string

	cfg         Config
	st          *store.Store
	trackSource *TrackSource
	mediaSigner *tokens.MediaSigner
	trackWarmer TrackWarmer
	issuer      *tokens.Issuer
	log         *slog.Logger

	stopJanitor chan struct{}
}

// NewManager creates a Manager.
func NewManager(cfg Config, st *store.Store, ts *TrackSource, mediaSigner *tokens.MediaSigner, warmer TrackWarmer, issuer *tokens.Issuer, log *slog.Logger) *Manager {
	return &Manager{
		games:       map[string]*ManagedGame{},
		byInvite:    map[string]string{},
		cfg:         cfg,
		st:          st,
		trackSource: ts,
		mediaSigner: mediaSigner,
		trackWarmer: warmer,
		issuer:      issuer,
		log:         log,
		stopJanitor: make(chan struct{}),
	}
}

// CreateGame creates a new game hosted by a fresh player (§12.2).
func (m *Manager) CreateGame(displayName string, targetCards, startTokens *int, enableSongGuess *bool) (PlayerIdentity, error) {
	m.mu.Lock()
	if len(m.games) >= m.cfg.MaxConcurrentGames {
		m.mu.Unlock()
		return PlayerIdentity{}, ErrTooManyGames
	}
	var code string
	for {
		c, err := newInviteCode()
		if err != nil {
			m.mu.Unlock()
			return PlayerIdentity{}, err
		}
		if _, taken := m.byInvite[c]; !taken {
			code = c
			break
		}
	}
	id := newID()
	settings := game.Settings{
		TargetCards:     m.cfg.DefaultTargetCards,
		StartTokens:     m.cfg.DefaultStartTokens,
		MaxTokens:       m.cfg.MaxTokens,
		EnableSongGuess: m.cfg.EnableSongGuess,
	}
	if targetCards != nil {
		settings.TargetCards = *targetCards
	}
	if startTokens != nil {
		settings.StartTokens = *startTokens
	}
	if enableSongGuess != nil {
		settings.EnableSongGuess = *enableSongGuess
	}

	mg := newManagedGame(id, code, settings, m.trackSource, m.mediaSigner, m.trackWarmer, m.st, m.cfg, m.log, m)
	m.games[id] = mg
	m.byInvite[code] = id
	m.mu.Unlock()

	var playerID string
	var addErr error
	mg.call(func() {
		p, err := mg.g.AddPlayer(newID(), displayName, m.cfg.MaxPlayers)
		if err != nil {
			addErr = err
			return
		}
		playerID = p.ID
	})
	if addErr != nil {
		m.removeGame(id, code)
		return PlayerIdentity{}, addErr
	}

	token, err := m.issuer.IssuePlayer(id, playerID, 6*time.Hour)
	if err != nil {
		return PlayerIdentity{}, err
	}
	return PlayerIdentity{GameID: id, PlayerID: playerID, PlayerToken: token, InviteCode: code}, nil
}

// JoinGame adds a player to an existing lobby (§12.2).
func (m *Manager) JoinGame(inviteCode, displayName string) (PlayerIdentity, error) {
	code := normalizeInviteCode(inviteCode)
	m.mu.RLock()
	gameID, ok := m.byInvite[code]
	var mg *ManagedGame
	if ok {
		mg = m.games[gameID]
	}
	m.mu.RUnlock()
	if !ok || mg == nil {
		return PlayerIdentity{}, ErrGameNotFound
	}

	var playerID string
	var joinErr error
	mg.call(func() {
		p, err := mg.g.AddPlayer(newID(), displayName, m.cfg.MaxPlayers)
		if err != nil {
			joinErr = err
			return
		}
		playerID = p.ID
		mg.broadcastState()
	})
	if joinErr != nil {
		return PlayerIdentity{}, joinErr
	}

	token, err := m.issuer.IssuePlayer(gameID, playerID, 6*time.Hour)
	if err != nil {
		return PlayerIdentity{}, err
	}
	return PlayerIdentity{GameID: gameID, PlayerID: playerID, PlayerToken: token, InviteCode: code}, nil
}

// PreviewResult answers the /j/{code} landing page (§12.2).
type PreviewResult struct {
	Exists      bool
	Phase       string
	PlayerCount int
	MaxPlayers  int
	HostName    string
	Joinable    bool
}

// Preview returns join-landing info for an invite code, leaking nothing else.
func (m *Manager) Preview(inviteCode string) PreviewResult {
	code := normalizeInviteCode(inviteCode)
	m.mu.RLock()
	gameID, ok := m.byInvite[code]
	var mg *ManagedGame
	if ok {
		mg = m.games[gameID]
	}
	m.mu.RUnlock()
	if !ok || mg == nil {
		return PreviewResult{Exists: false}
	}

	var result PreviewResult
	mg.call(func() {
		hostName := ""
		if h := mg.g.Player(mg.g.HostID); h != nil {
			hostName = h.Name
		}
		result = PreviewResult{
			Exists:      true,
			Phase:       string(mg.g.Phase),
			PlayerCount: len(mg.g.Players),
			MaxPlayers:  m.cfg.MaxPlayers,
			HostName:    hostName,
			Joinable:    mg.g.Phase == game.PhaseLobby && len(mg.g.Players) < m.cfg.MaxPlayers,
		}
	})
	return result
}

func (m *Manager) removeGame(gameID, inviteCode string) {
	m.mu.Lock()
	if mg, ok := m.games[gameID]; ok {
		mg.stop()
	}
	delete(m.games, gameID)
	delete(m.byInvite, inviteCode)
	m.mu.Unlock()
}

func (m *Manager) gameByID(gameID string) *ManagedGame {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.games[gameID]
}

// --- ws.Handler ---

// OnHello authenticates a socket by its player JWT and registers it with the
// owning game (§13.1, §13.4).
func (m *Manager) OnHello(c *ws.Conn, playerToken string) (gameID, playerID string, ok bool) {
	claims, err := m.issuer.VerifyPlayer(playerToken)
	if err != nil {
		return "", "", false
	}
	mg := m.gameByID(claims.GameID)
	if mg == nil {
		return "", "", false
	}

	var registered bool
	mg.call(func() {
		registered = mg.registerConn(claims.PlayerID, c)
	})
	if !registered {
		return "", "", false
	}
	return claims.GameID, claims.PlayerID, true
}

// OnMessage dispatches an authenticated client message (§13.1).
func (m *Manager) OnMessage(c *ws.Conn, env ws.Envelope) {
	mg := m.gameByID(c.GameID)
	if mg == nil {
		return
	}
	mg.enqueue(func() {
		dispatchMessage(mg, c, env)
	})
}

// OnDisconnect handles a socket closing (§8.10).
func (m *Manager) OnDisconnect(c *ws.Conn) {
	if c.GameID == "" {
		return
	}
	mg := m.gameByID(c.GameID)
	if mg == nil {
		return
	}
	mg.enqueue(func() {
		mg.unregisterConn(c.PlayerID, c)
	})
}

// --- Admin-facing ---

// ActiveGameSummary is one row of the admin games list (§12.3).
type ActiveGameSummary struct {
	GameID      string
	InviteCode  string
	Phase       string
	PlayerCount int
	TurnNumber  int
}

// ListActiveGames returns a summary of every in-memory game.
func (m *Manager) ListActiveGames() []ActiveGameSummary {
	m.mu.RLock()
	games := make([]*ManagedGame, 0, len(m.games))
	for _, mg := range m.games {
		games = append(games, mg)
	}
	m.mu.RUnlock()

	out := make([]ActiveGameSummary, len(games))
	for i, mg := range games {
		mg.call(func() {
			out[i] = ActiveGameSummary{
				GameID: mg.id, InviteCode: mg.inviteCode, Phase: string(mg.g.Phase),
				PlayerCount: len(mg.g.Players), TurnNumber: currentTurnNumber(mg.g),
			}
		})
	}
	return out
}

// ForceEndGame ends a game immediately for the admin console (§12.3).
func (m *Manager) ForceEndGame(gameID string) error {
	mg := m.gameByID(gameID)
	if mg == nil {
		return ErrGameNotFound
	}
	mg.call(func() {
		mg.g.Phase = game.PhaseGameOver
		mg.g.WinnerID = ""
		mg.clearPhaseTimeout()
		mg.stopTrack()
		mg.broadcastState()
		mg.persistOnGameOver()
	})
	m.removeGame(gameID, mg.inviteCode)
	return nil
}

// --- Lifecycle ---

// RunJanitor periodically reaps idle lobbies and disconnected games (§17).
func (m *Manager) RunJanitor(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.reapOnce()
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) reapOnce() {
	m.mu.RLock()
	games := make([]*ManagedGame, 0, len(m.games))
	for _, mg := range m.games {
		games = append(games, mg)
	}
	m.mu.RUnlock()

	for _, mg := range games {
		var shouldRemove bool
		mg.call(func() {
			idleTooLong := mg.g.Phase == game.PhaseLobby && time.Since(mg.createdAt) > m.cfg.LobbyIdleTimeout
			noConnections := len(mg.conns) == 0 && time.Since(mg.createdAt) > m.cfg.PlayerReconnectGrace
			if idleTooLong || noConnections {
				if mg.g.Phase != game.PhaseGameOver {
					mg.g.Phase = game.PhaseGameOver
					mg.persistOnGameOver()
				}
				shouldRemove = true
			}
		})
		if shouldRemove {
			m.removeGame(mg.id, mg.inviteCode)
		}
	}
}

// Shutdown notifies every connected client and stops accepting new games
// (§17). Callers are expected to also stop the HTTP/WS listener.
func (m *Manager) Shutdown() {
	close(m.stopJanitor)
	m.mu.RLock()
	games := make([]*ManagedGame, 0, len(m.games))
	for _, mg := range m.games {
		games = append(games, mg)
	}
	m.mu.RUnlock()

	for _, mg := range games {
		mg.call(func() {
			for _, c := range mg.conns {
				c.Send(ws.TypeError, ws.ErrorPayload{Code: "server_restarting", Message: "The server is restarting."})
			}
			mg.saveSnapshotAsync()
		})
	}
}

// RehydrateFromSnapshots restores in-progress games from Postgres on
// startup, for those updated within the last 30 minutes (§17).
func (m *Manager) RehydrateFromSnapshots(ctx context.Context) {
	snapshots, err := m.st.RecentSnapshots(ctx, 30*time.Minute)
	if err != nil {
		m.log.Error("failed to load snapshots for rehydration", "error", err)
		return
	}
	for _, snap := range snapshots {
		mg := newManagedGame(snap.GameID, snap.InviteCode, game.Settings{}, m.trackSource, m.mediaSigner, m.trackWarmer, m.st, m.cfg, m.log, m)
		if err := mg.restoreFromSnapshot(snap.State); err != nil {
			m.log.Error("failed to restore game snapshot", "game_id", snap.GameID, "error", err)
			mg.stop()
			continue
		}
		m.mu.Lock()
		m.games[snap.GameID] = mg
		m.byInvite[snap.InviteCode] = snap.GameID
		m.mu.Unlock()
		m.log.Info("rehydrated game from snapshot", "game_id", snap.GameID, "phase", mg.g.Phase)
	}
}
