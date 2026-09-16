DROP VIEW eligible_tracks;
CREATE VIEW eligible_tracks AS
SELECT t.*
FROM tracks t
WHERE NOT EXISTS (
    SELECT 1 FROM exclusions e
    WHERE (e.kind = 'track'  AND e.ref_id = t.id)
       OR (e.kind = 'album'  AND e.ref_id = t.album_id)
       OR (e.kind = 'artist' AND e.ref_id = t.artist_id)
);

ALTER TABLE tracks DROP COLUMN hitsync_exclude;
ALTER TABLE tracks DROP COLUMN hitsync_year;
