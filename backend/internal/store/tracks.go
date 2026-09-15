package store

import (
	"context"
	"time"
)

// Track mirrors a row of the tracks table.
type Track struct {
	ID            string
	Title         string
	Artist        string
	ArtistID      string
	Album         string
	AlbumID       string
	NavidromeYear *int
	DurationSec   int
	NormTitle     string
	NormArtist    string
}

// UpsertTrackParams is the input to UpsertTrack.
type UpsertTrackParams struct {
	ID            string
	Title         string
	Artist        string
	ArtistID      string
	Album         string
	AlbumID       string
	NavidromeYear *int
	DurationSec   int
	NormTitle     string
	NormArtist    string
	LastSeenAt    time.Time
}

// UpsertTrack inserts or updates a track row from a library sync page. It
// reports whether the row was newly inserted (true) or updated (false).
func (s *Store) UpsertTrack(ctx context.Context, p UpsertTrackParams) (inserted bool, err error) {
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO tracks (id, title, artist, artist_id, album, album_id, navidrome_year, duration_sec, norm_title, norm_artist, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			artist = EXCLUDED.artist,
			artist_id = EXCLUDED.artist_id,
			album = EXCLUDED.album,
			album_id = EXCLUDED.album_id,
			navidrome_year = EXCLUDED.navidrome_year,
			duration_sec = EXCLUDED.duration_sec,
			norm_title = EXCLUDED.norm_title,
			norm_artist = EXCLUDED.norm_artist,
			last_seen_at = EXCLUDED.last_seen_at
		RETURNING (xmax = 0)
	`, p.ID, p.Title, p.Artist, nullableString(p.ArtistID), p.Album, nullableString(p.AlbumID), p.NavidromeYear, p.DurationSec, p.NormTitle, p.NormArtist, p.LastSeenAt).Scan(&inserted)
	return inserted, err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// DeleteStaleTracksResult reports how many rows were removed.
type DeleteStaleTracksResult struct {
	Removed int64
}

// DeleteStaleTracks removes tracks whose last_seen_at is older than
// syncStart, i.e. tracks that disappeared from the library during a fully
// successful sync pass (§9.2 step 3).
func (s *Store) DeleteStaleTracks(ctx context.Context, syncStart time.Time) (DeleteStaleTracksResult, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM tracks WHERE last_seen_at < $1`, syncStart)
	if err != nil {
		return DeleteStaleTracksResult{}, err
	}
	return DeleteStaleTracksResult{Removed: tag.RowsAffected()}, nil
}

// CountTracks returns the total number of tracks in the library index.
func (s *Store) CountTracks(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM tracks`).Scan(&n)
	return n, err
}

// CountEligibleTracks returns the size of the eligible pool given duration
// bounds and the freshest sync timestamp, ignoring the per-game used-track
// filter (§7.2).
func (s *Store) CountEligibleTracks(ctx context.Context, minDurationSec, maxDurationSec int, freshSince time.Time) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM eligible_tracks
		WHERE duration_sec BETWEEN $1 AND $2 AND last_seen_at >= $3
	`, minDurationSec, maxDurationSec, freshSince).Scan(&n)
	return n, err
}

// RandomEligibleTracks draws up to n random eligible tracks, excluding any
// id already present in excludeIDs (already used in the current game) and
// respecting the configured duration bounds and library freshness (§7.2).
func (s *Store) RandomEligibleTracks(ctx context.Context, excludeIDs []string, minDurationSec, maxDurationSec int, freshSince time.Time, n int) ([]Track, error) {
	if excludeIDs == nil {
		excludeIDs = []string{}
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, title, artist, artist_id, album, album_id, navidrome_year, duration_sec, norm_title, norm_artist
		FROM eligible_tracks
		WHERE duration_sec BETWEEN $1 AND $2
		  AND last_seen_at >= $3
		  AND id <> ALL($4::text[])
		ORDER BY random()
		LIMIT $5
	`, minDurationSec, maxDurationSec, freshSince, excludeIDs, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Track
	for rows.Next() {
		var t Track
		var artistID, album, albumID *string
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &artistID, &album, &albumID, &t.NavidromeYear, &t.DurationSec, &t.NormTitle, &t.NormArtist); err != nil {
			return nil, err
		}
		if artistID != nil {
			t.ArtistID = *artistID
		}
		if album != nil {
			t.Album = *album
		}
		if albumID != nil {
			t.AlbumID = *albumID
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTrack fetches a single track by id.
func (s *Store) GetTrack(ctx context.Context, id string) (*Track, error) {
	var t Track
	var artistID, album, albumID *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, title, artist, artist_id, album, album_id, navidrome_year, duration_sec, norm_title, norm_artist
		FROM tracks WHERE id = $1
	`, id).Scan(&t.ID, &t.Title, &t.Artist, &artistID, &album, &albumID, &t.NavidromeYear, &t.DurationSec, &t.NormTitle, &t.NormArtist)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if artistID != nil {
		t.ArtistID = *artistID
	}
	if album != nil {
		t.Album = *album
	}
	if albumID != nil {
		t.AlbumID = *albumID
	}
	return &t, nil
}

// RandomTrackTitles returns n random titles excluding the given track id,
// used to build song-guess decoys (§8.6).
func (s *Store) RandomTrackTitles(ctx context.Context, excludeTrackID string, n int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT title FROM tracks WHERE id <> $1 ORDER BY random() LIMIT $2
	`, excludeTrackID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		out = append(out, title)
	}
	return out, rows.Err()
}

// RandomArtistNames returns n random distinct artist names excluding the
// given artist, used to build song-guess decoys (§8.6).
func (s *Store) RandomArtistNames(ctx context.Context, excludeArtist string, n int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT artist FROM tracks WHERE artist <> $1 ORDER BY random() LIMIT $2
	`, excludeArtist, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			return nil, err
		}
		out = append(out, artist)
	}
	return out, rows.Err()
}
