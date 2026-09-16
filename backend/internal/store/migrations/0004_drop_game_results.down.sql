CREATE TABLE game_results (
    game_id     TEXT PRIMARY KEY,
    started_at  TIMESTAMPTZ NOT NULL,
    ended_at    TIMESTAMPTZ NOT NULL,
    player_count INT NOT NULL,
    winner_name TEXT,
    turns_played INT NOT NULL
);
