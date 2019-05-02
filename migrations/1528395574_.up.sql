BEGIN;

DROP INDEX IF EXISTS repo_name_unique;

ALTER TABLE repo ADD CONSTRAINT repo_name_unique
UNIQUE(name) DEFERRABLE INITIALLY IMMEDIATE;

COMMIT;
