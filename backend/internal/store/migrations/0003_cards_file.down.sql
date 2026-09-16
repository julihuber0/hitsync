-- Recreates the (empty) library schema as of 0002_hitsync_tags.
CREATE TABLE tracks (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    artist          TEXT NOT NULL,
    artist_id       TEXT,
    album           TEXT,
    album_id        TEXT,
    navidrome_year  INT,
    duration_sec    INT NOT NULL,
    norm_title      TEXT NOT NULL,
    norm_artist     TEXT NOT NULL,
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    hitsync_year    INT,
    hitsync_exclude BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX tracks_norm_idx    ON tracks (norm_title, norm_artist);
CREATE INDEX tracks_artist_idx  ON tracks (artist_id);
CREATE INDEX tracks_album_idx   ON tracks (album_id);

CREATE TABLE year_overrides (
    track_id    TEXT PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    year        INT NOT NULL,
    note        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE exclusion_kind AS ENUM ('track', 'album', 'artist');

CREATE TABLE exclusions (
    kind        exclusion_kind NOT NULL,
    ref_id      TEXT NOT NULL,
    label       TEXT NOT NULL,
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, ref_id)
);

CREATE VIEW eligible_tracks AS
SELECT t.*
FROM tracks t
WHERE NOT t.hitsync_exclude
  AND NOT EXISTS (
    SELECT 1 FROM exclusions e
    WHERE (e.kind = 'track'  AND e.ref_id = t.id)
       OR (e.kind = 'album'  AND e.ref_id = t.album_id)
       OR (e.kind = 'artist' AND e.ref_id = t.artist_id)
);
