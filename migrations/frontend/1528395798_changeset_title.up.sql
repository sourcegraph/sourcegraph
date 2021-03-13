BEGIN;

ALTER TABLE changesets ADD COLUMN IF NOT EXISTS title text;
COMMENT ON COLUMN changesets.title IS 'Normalized property generated on save using Changeset.Title()';
-- initial migration, we'll need to continue doing this in an oob migrator.
UPDATE changesets SET title = COALESCE(changesets.metadata->>'Title', changesets.metadata->>'title', NULL);
CREATE INDEX IF NOT EXISTS changesets_title_idx ON changesets USING BTREE(title);

COMMIT;
