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

// GameResult mirrors a row of the game_results table.
type GameResult struct {
	GameID      string
	StartedAt   time.Time
	EndedAt     time.Time
	PlayerCount int
	WinnerName  string
	TurnsPlayed int
}

// SaveGameResult records a finished game for the admin stats page.
func (s *Store) SaveGameResult(ctx context.Context, r GameResult) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO game_results (game_id, started_at, ended_at, player_count, winner_name, turns_played)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id) DO NOTHING
	`, r.GameID, r.StartedAt, r.EndedAt, r.PlayerCount, nullableString(r.WinnerName), r.TurnsPlayed)
	return err
}
