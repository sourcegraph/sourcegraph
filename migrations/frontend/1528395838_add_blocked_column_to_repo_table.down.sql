BEGIN;

DROP INDEX IF EXISTS repo_is_blocked_idx;

DROP FUNCTION IF EXISTS block_repo;

ALTER TABLE IF EXISTS repo DROP COLUMN IF EXISTS blocked;

COMMIT;
