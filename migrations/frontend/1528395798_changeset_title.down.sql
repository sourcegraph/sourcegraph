BEGIN;

DROP INDEX IF EXISTS changesets_title_idx;
ALTER TABLE changesets DROP COLUMN IF EXISTS title;

COMMIT;
