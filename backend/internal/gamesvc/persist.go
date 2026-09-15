package gamesvc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/game"
	"github.com/julianhuber/hitsync/backend/internal/store"
)

// saveSnapshotAsync writes the current state to Postgres for crash recovery.
// Never blocks the game's command loop; a failed write is logged and
// dropped (§3.4).
func (mg *ManagedGame) saveSnapshotAsync() {
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

// persistOnGameOver records the finished game for the admin stats page and
// drops its crash-recovery snapshot.
func (mg *ManagedGame) persistOnGameOver() {
	winnerName := ""
	if mg.g.WinnerID != "" {
		if p := mg.g.Player(mg.g.WinnerID); p != nil {
			winnerName = p.Name
		}
	}
	result := store.GameResult{
		GameID:      mg.id,
		StartedAt:   mg.startedAt,
		EndedAt:     time.Now(),
		PlayerCount: len(mg.g.Players),
		WinnerName:  winnerName,
		TurnsPlayed: mg.turnCount,
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = mg.createdAt
	}

	st := mg.store
	id := mg.id
	log := mg.log
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := st.SaveGameResult(ctx, result); err != nil {
			log.Warn("failed to save game result", "game_id", id, "error", err)
		}
		if err := st.DeleteSnapshot(ctx, id); err != nil {
			log.Warn("failed to delete game snapshot", "game_id", id, "error", err)
		}
	}()
}

// restoreFromSnapshot rehydrates a ManagedGame's rules-engine state from a
// crash-recovery snapshot (§17). Any game whose phase involved audio resumes
// in PLACING with a fresh timer.
func (mg *ManagedGame) restoreFromSnapshot(data []byte) error {
	var state game.State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	mg.g = game.RestoreState(state)

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
	}
	return nil
}
