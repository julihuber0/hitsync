-- HITSYNCYEAR and HITSYNCEXCLUDE are optional custom tags the admin can
-- embed directly in a track's audio file. They are read straight out of the
-- file (not via Navidrome, which does not expose custom tags through its
-- API) during library sync.
ALTER TABLE tracks ADD COLUMN hitsync_year INT;
ALTER TABLE tracks ADD COLUMN hitsync_exclude BOOLEAN NOT NULL DEFAULT false;

DROP VIEW eligible_tracks;
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
