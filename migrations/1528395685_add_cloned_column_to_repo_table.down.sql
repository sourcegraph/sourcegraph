BEGIN;

ALTER TABLE repo DROP COLUMN cloned;

COMMIT;
