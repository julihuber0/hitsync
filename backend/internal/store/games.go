package store

import (
	"context"
	"time"
)

// GameSnapshot mirrors a row of the game_snapshots table (§3.4).
type GameSnapshot struct {
	GameID     string
	InviteCode string
	State      []byte // raw JSONB
	UpdatedAt  time.Time
}

// SaveSnapshot writes (or overwrites) a crash-recovery snapshot. Callers
// must never block gameplay on this; failures are logged and dropped.
func (s *Store) SaveSnapshot(ctx context.Context, gameID, inviteCode string, state []byte) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO game_snapshots (game_id, invite_code, state, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (game_id) DO UPDATE SET invite_code = EXCLUDED.invite_code, state = EXCLUDED.state, updated_at = now()
	`, gameID, inviteCode, state)
	return err
}

// DeleteSnapshot removes a game's snapshot, e.g. once it has ended.
func (s *Store) DeleteSnapshot(ctx context.Context, gameID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM game_snapshots WHERE game_id = $1`, gameID)
	return err
}

// RecentSnapshots returns snapshots updated within the last maxAge, for
// crash-recovery rehydration on startup (§17). Older rows are deleted.
func (s *Store) RecentSnapshots(ctx context.Context, maxAge time.Duration) ([]GameSnapshot, error) {
	cutoff := time.Now().Add(-maxAge)

	if _, err := s.Pool.Exec(ctx, `DELETE FROM game_snapshots WHERE updated_at < $1`, cutoff); err != nil {
		return nil, err
	}

	rows, err := s.Pool.Query(ctx, `SELECT game_id, invite_code, state, updated_at FROM game_snapshots WHERE updated_at >= $1`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GameSnapshot
	for rows.Next() {
		var g GameSnapshot
		if err := rows.Scan(&g.GameID, &g.InviteCode, &g.State, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeleteSnapshotsExcept removes every snapshot whose game is not in keepIDs,
// i.e. games the server has already forgotten.
func (s *Store) DeleteSnapshotsExcept(ctx context.Context, keepIDs []string) error {
	if keepIDs == nil {
		keepIDs = []string{}
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM game_snapshots WHERE game_id <> ALL($1::text[])`, keepIDs)
	return err
}
