-- The song catalogue, year corrections, and exclusions now live in the
-- cards.json file (see internal/cards). Postgres keeps only game snapshots
-- and results.
DROP VIEW IF EXISTS eligible_tracks;
DROP TABLE IF EXISTS year_overrides;
DROP TABLE IF EXISTS exclusions;
DROP TABLE IF EXISTS tracks;
DROP TYPE IF EXISTS exclusion_kind;
