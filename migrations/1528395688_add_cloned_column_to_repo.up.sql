BEGIN;

ALTER TABLE repo ADD COLUMN IF NOT EXISTS cloned BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS repo_cloned ON repo(cloned);

COMMIT;
