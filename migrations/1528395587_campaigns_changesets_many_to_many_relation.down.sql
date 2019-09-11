BEGIN;

ALTER TABLE campaigns DROP COLUMN changeset_ids;
ALTER TABLE changesets DROP COLUMN campaign_ids;
ALTER TABLE changesets ADD COLUMN campaign_id integer
NOT NULL REFERENCES campaigns(id) ON DELETE
CASCADE DEFERRABLE INITIALLY IMMEDIATE;

COMMIT;
