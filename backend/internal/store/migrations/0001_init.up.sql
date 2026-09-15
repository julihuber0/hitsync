CREATE TABLE tracks (
    id              TEXT PRIMARY KEY,           -- Navidrome/Subsonic song id
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
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX tracks_norm_idx    ON tracks (norm_title, norm_artist);
CREATE INDEX tracks_artist_idx  ON tracks (artist_id);
CREATE INDEX tracks_album_idx   ON tracks (album_id);

-- Manual corrections set by the admin. Highest precedence.
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

-- Crash-recovery snapshots. Not on the gameplay hot path.
CREATE TABLE game_snapshots (
    game_id     TEXT PRIMARY KEY,
    invite_code TEXT NOT NULL,
    state       JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX game_snapshots_updated_idx ON game_snapshots (updated_at);

-- Finished games, for the admin stats page.
CREATE TABLE game_results (
    game_id     TEXT PRIMARY KEY,
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ NOT NULL,
    player_count INT NOT NULL,
    winner_name TEXT,
    turns_played INT NOT NULL
);

-- A track is eligible when it is not excluded at the track/album/artist level.
-- Freshness (last_seen_at), duration bounds, and "not used in this game" are
-- applied by the caller as query parameters (§7.2), since they depend on
-- runtime configuration and per-game state that a static view cannot see.
CREATE VIEW eligible_tracks AS
SELECT t.*
FROM tracks t
WHERE NOT EXISTS (
    SELECT 1 FROM exclusions e
    WHERE (e.kind = 'track'  AND e.ref_id = t.id)
       OR (e.kind = 'album'  AND e.ref_id = t.album_id)
       OR (e.kind = 'artist' AND e.ref_id = t.artist_id)
);
