package gamesvc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/store"
)

// SnapshotStore persists crash-recovery snapshots (implemented by
// *store.Store).
type SnapshotStore interface {
	SaveSnapshot(ctx context.Context, gameID, inviteCode string, state []byte) error
	DeleteSnapshot(ctx context.Context, gameID string) error
	RecentSnapshots(ctx context.Context, maxAge time.Duration) ([]store.GameSnapshot, error)
	DeleteSnapshotsExcept(ctx context.Context, keepIDs []string) error
}

// saveSnapshotAsync writes the current state to Postgres for crash recovery.
// Never blocks the game's command loop; a failed write is logged and
// dropped (§3.4). A finished game is not saved: there is nothing to resume.
func (mg *ManagedGame) saveSnapshotAsync() {
	if mg.g.Phase == game.PhaseGameOver {
		return
	}
	state := mg.g.ExportState()
	data, err := json.Marshal(state)
	if err != nil {
		mg.log.Error("failed to marshal game snapshot", "game_id", mg.id, "error", err)
		return
	}
	st := mg.store
	id, code := mg.id, mg.inviteCode
	log := mg.log
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := st.SaveSnapshot(ctx, id, code, data); err != nil {
			log.Warn("failed to save game snapshot", "game_id", id, "error", err)
		}
	}()
}

// deleteSnapshotAsync drops the game's crash-recovery snapshot once the game
// has ended or is forgotten. A save still in flight may land afterwards; the
// janitor removes such orphans.
func (mg *ManagedGame) deleteSnapshotAsync() {
	st := mg.store
	id := mg.id
	log := mg.log
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := st.DeleteSnapshot(ctx, id); err != nil {
			log.Warn("failed to delete game snapshot", "game_id", id, "error", err)
		}
	}()
}

// restoreFromSnapshot rehydrates a ManagedGame's rules-engine state from a
// crash-recovery snapshot (§17). Any game whose phase involved audio resumes
// in PLACING with a fresh timer, with the turn's track restarting from the
// beginning for players as they reconnect. Nobody is connected after a
// restart, so every player gets the usual reconnect grace.
func (mg *ManagedGame) restoreFromSnapshot(data []byte) error {
	var state game.State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	mg.g = game.RestoreState(state)
	mg.emptySince = time.Now()
	for _, p := range mg.g.Players {
		p.Connected = false
		playerID := p.ID
		mg.reconnectTimers[playerID] = time.AfterFunc(mg.cfg.PlayerReconnectGrace, func() {
			mg.enqueue(func() { mg.onReconnectGraceExpired(playerID) })
		})
	}

	switch mg.g.Phase {
	case game.PhasePreparing, game.PhaseChallenging, game.PhaseRevealing:
		mg.g.Phase = game.PhasePlacing
		if mg.g.Turn != nil {
			mg.g.Turn.PlacementSubmitted = false
			mg.g.Turn.PlacementSlot = -2
		}
		fallthrough
	case game.PhasePlacing:
		mg.schedulePhaseTimeout(mg.cfg.TurnPlacementTimeout, mg.onPlacementTimeout)
		if mg.g.Turn != nil {
			mg.track = &activeTrack{prepareID: newID(), trackID: mg.g.Turn.Track.TrackID, startAtServerMs: nowMs()}
		}
	}
	return nil
}
