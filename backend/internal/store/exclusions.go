package store

import (
	"context"
	"strconv"
	"time"
)

// ExclusionKind matches the exclusion_kind Postgres enum.
type ExclusionKind string

const (
	ExclusionTrack  ExclusionKind = "track"
	ExclusionAlbum  ExclusionKind = "album"
	ExclusionArtist ExclusionKind = "artist"
)

// Exclusion mirrors a row of the exclusions table.
type Exclusion struct {
	Kind      ExclusionKind
	RefID     string
	Label     string
	Reason    string
	CreatedAt time.Time
}

// AddExclusion inserts or replaces an exclusion.
func (s *Store) AddExclusion(ctx context.Context, kind ExclusionKind, refID, label, reason string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO exclusions (kind, ref_id, label, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (kind, ref_id) DO UPDATE SET label = EXCLUDED.label, reason = EXCLUDED.reason
	`, kind, refID, label, nullableString(reason))
	return err
}

// RemoveExclusion deletes an exclusion by kind and ref id.
func (s *Store) RemoveExclusion(ctx context.Context, kind ExclusionKind, refID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM exclusions WHERE kind = $1 AND ref_id = $2`, kind, refID)
	return err
}

// ListExclusions returns a page of exclusions, optionally filtered by kind.
func (s *Store) ListExclusions(ctx context.Context, kind *ExclusionKind, page, pageSize int) ([]Exclusion, error) {
	offset := page * pageSize

	query := `SELECT kind, ref_id, label, reason, created_at FROM exclusions`
	args := []any{}
	if kind != nil {
		query += ` WHERE kind = $1`
		args = append(args, *kind)
	}
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, pageSize, offset)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Exclusion
	for rows.Next() {
		var e Exclusion
		var reason *string
		if err := rows.Scan(&e.Kind, &e.RefID, &e.Label, &reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		if reason != nil {
			e.Reason = *reason
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// IsExcluded reports whether a track is excluded at the track/album/artist
// level, and if so which kind matched.
func (s *Store) IsExcluded(ctx context.Context, trackID, albumID, artistID string) (excluded bool, kind ExclusionKind, err error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT kind FROM exclusions
		WHERE (kind = 'track' AND ref_id = $1)
		   OR (kind = 'album' AND ref_id = $2)
		   OR (kind = 'artist' AND ref_id = $3)
		LIMIT 1
	`, trackID, nullableString(albumID), nullableString(artistID))
	if err != nil {
		return false, "", err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&kind); err != nil {
			return false, "", err
		}
		return true, kind, nil
	}
	return false, "", rows.Err()
}

// YearOverride mirrors a row of the year_overrides table.
type YearOverride struct {
	TrackID string
	Year    int
	Note    string
}

// SetYearOverride inserts or replaces a manual year correction.
func (s *Store) SetYearOverride(ctx context.Context, trackID string, year int, note string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO year_overrides (track_id, year, note)
		VALUES ($1, $2, $3)
		ON CONFLICT (track_id) DO UPDATE SET year = EXCLUDED.year, note = EXCLUDED.note
	`, trackID, year, nullableString(note))
	return err
}

// DeleteYearOverride removes a manual year correction.
func (s *Store) DeleteYearOverride(ctx context.Context, trackID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM year_overrides WHERE track_id = $1`, trackID)
	return err
}

// GetYearOverride returns the manual override for a track, if any.
func (s *Store) GetYearOverride(ctx context.Context, trackID string) (*YearOverride, error) {
	var o YearOverride
	var note *string
	err := s.Pool.QueryRow(ctx, `SELECT track_id, year, note FROM year_overrides WHERE track_id = $1`, trackID).
		Scan(&o.TrackID, &o.Year, &note)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if note != nil {
		o.Note = *note
	}
	return &o, nil
}
