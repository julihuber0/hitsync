package store

import "context"

// AdminTrackRow is one row of the admin library table (§12.3 GET /api/admin/tracks).
type AdminTrackRow struct {
	ID              string
	Title           string
	Artist          string
	Album           string
	NavidromeYear   *int
	HitsyncYear     *int
	HitsyncExcluded bool
	OverrideYear    *int
	ExcludedKind    *string // "track" | "album" | "artist" | nil
}

// SearchAdminTracks searches title/artist/album and reports override/exclusion
// status per row, without triggering any MusicBrainz lookups (§12.3).
func (s *Store) SearchAdminTracks(ctx context.Context, q string, excludedFilter *bool, page, pageSize int) ([]AdminTrackRow, int, error) {
	offset := page * pageSize
	var excludedStr any
	if excludedFilter != nil {
		if *excludedFilter {
			excludedStr = "true"
		} else {
			excludedStr = "false"
		}
	}

	const base = `
		WITH matched AS (
			SELECT t.id, t.title, t.artist, t.album, t.navidrome_year, t.hitsync_year, t.hitsync_exclude, yo.year AS override_year,
				(SELECT e.kind::text FROM exclusions e
				 WHERE (e.kind = 'track' AND e.ref_id = t.id)
					OR (e.kind = 'album' AND e.ref_id = t.album_id)
					OR (e.kind = 'artist' AND e.ref_id = t.artist_id)
				 LIMIT 1) AS excluded_kind
			FROM tracks t
			LEFT JOIN year_overrides yo ON yo.track_id = t.id
		)
		SELECT id, title, artist, album, navidrome_year, hitsync_year, hitsync_exclude, override_year, excluded_kind
		FROM matched
		WHERE ($1 = '' OR title ILIKE '%' || $1 || '%' OR artist ILIKE '%' || $1 || '%' OR album ILIKE '%' || $1 || '%')
		  AND ($2::text IS NULL
		       OR ($2 = 'true' AND (excluded_kind IS NOT NULL OR hitsync_exclude))
		       OR ($2 = 'false' AND excluded_kind IS NULL AND NOT hitsync_exclude))
	`

	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM (`+base+`) c`, q, excludedStr).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.Pool.Query(ctx, base+` ORDER BY title LIMIT $3 OFFSET $4`, q, excludedStr, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []AdminTrackRow
	for rows.Next() {
		var r AdminTrackRow
		var album *string
		if err := rows.Scan(&r.ID, &r.Title, &r.Artist, &album, &r.NavidromeYear, &r.HitsyncYear, &r.HitsyncExcluded, &r.OverrideYear, &r.ExcludedKind); err != nil {
			return nil, 0, err
		}
		if album != nil {
			r.Album = *album
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// RefOption is a selectable artist/album for the admin exclusion pickers.
type RefOption struct {
	ID   string
	Name string
}

// SearchArtists returns distinct artists matching q, for admin exclusion by artist.
func (s *Store) SearchArtists(ctx context.Context, q string, limit int) ([]RefOption, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT artist_id, artist FROM tracks
		WHERE artist_id IS NOT NULL AND ($1 = '' OR artist ILIKE '%' || $1 || '%')
		ORDER BY artist LIMIT $2
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RefOption
	for rows.Next() {
		var o RefOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SearchAlbums returns distinct albums matching q, for admin exclusion by album.
func (s *Store) SearchAlbums(ctx context.Context, q string, limit int) ([]RefOption, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT album_id, album FROM tracks
		WHERE album_id IS NOT NULL AND ($1 = '' OR album ILIKE '%' || $1 || '%')
		ORDER BY album LIMIT $2
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RefOption
	for rows.Next() {
		var o RefOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
