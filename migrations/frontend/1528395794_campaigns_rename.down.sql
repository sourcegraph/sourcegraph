BEGIN;

UPDATE user_credentials SET domain = 'campaigns' WHERE domain = 'batches';
ALTER TABLE IF EXISTS batch_specs RENAME TO campaign_specs;
ALTER TABLE IF EXISTS batch_changes RENAME TO campaigns;
ALTER TABLE IF EXISTS campaigns RENAME COLUMN batch_spec_id TO campaign_spec_id;
ALTER TABLE IF EXISTS changeset_specs RENAME COLUMN batch_spec_id TO campaign_spec_id;
DROP VIEW IF EXISTS reconciler_changesets;
DROP VIEW IF EXISTS branch_changeset_specs_and_changesets;
DROP VIEW IF EXISTS tracking_changeset_specs_and_changesets;
ALTER TABLE IF EXISTS changesets RENAME COLUMN batch_change_ids TO campaign_ids;
ALTER TABLE IF EXISTS changesets RENAME COLUMN owned_by_batch_change_id TO owned_by_campaign_id;
CREATE VIEW tracking_changeset_specs_and_changesets AS (
    SELECT
        changeset_specs.id AS changeset_spec_id,
        COALESCE(changesets.id, 0::bigint) AS changeset_id,
        changeset_specs.repo_id,
        changeset_specs.campaign_spec_id,
        repo.name AS repo_name,
        COALESCE(changesets.metadata ->> 'Title'::text, changesets.metadata ->> 'title'::text) AS changeset_name,
        changesets.external_state,
        changesets.publication_state,
        changesets.reconciler_state
    FROM
        changeset_specs
    LEFT JOIN
        changesets
    ON
        changesets.repo_id = changeset_specs.repo_id
        AND
        changesets.external_id = changeset_specs.external_id
    JOIN
        repo
    ON
        changeset_specs.repo_id = repo.id
    WHERE
        changeset_specs.external_id IS NOT NULL
        AND
        repo.deleted_at IS NULL
);
CREATE VIEW branch_changeset_specs_and_changesets AS (
    SELECT
        changeset_specs.id AS changeset_spec_id,
        COALESCE(changesets.id, 0::bigint) AS changeset_id,
        changeset_specs.repo_id,
        changeset_specs.campaign_spec_id,
        changesets.owned_by_campaign_id AS owner_campaign_id,
        repo.name AS repo_name,
        changeset_specs.title AS changeset_name,
        changesets.external_state,
        changesets.publication_state,
        changesets.reconciler_state
    FROM changeset_specs
    LEFT JOIN
        changesets
    ON
        changesets.repo_id = changeset_specs.repo_id
        AND
        changesets.current_spec_id IS NOT NULL
        AND
        EXISTS (
            SELECT 1
            FROM changeset_specs changeset_specs_1
            WHERE
                changeset_specs_1.id = changesets.current_spec_id
                AND
                changeset_specs_1.head_ref = changeset_specs.head_ref
        )
    JOIN
        repo
    ON
        changeset_specs.repo_id = repo.id
    WHERE
        changeset_specs.external_id IS NULL
        AND
        repo.deleted_at IS NULL
);
CREATE VIEW reconciler_changesets AS
    SELECT c.* FROM changesets c
    INNER JOIN repo r on r.id = c.repo_id
    WHERE
        r.deleted_at IS NULL AND
        EXISTS (
            SELECT 1 FROM campaigns
            LEFT JOIN users namespace_user ON campaigns.namespace_user_id = namespace_user.id
            LEFT JOIN orgs namespace_org ON campaigns.namespace_org_id = namespace_org.id
            WHERE
                c.campaign_ids ? campaigns.id::text AND
                namespace_user.deleted_at IS NULL AND
                namespace_org.deleted_at IS NULL
        )
;
DROP TRIGGER IF EXISTS trig_delete_batch_change_reference_on_changesets ON campaigns;
DROP FUNCTION IF EXISTS delete_batch_change_reference_on_changesets();
CREATE FUNCTION delete_campaign_reference_on_changesets() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
    BEGIN
        UPDATE
          changesets
        SET
          batch_change_ids = changesets.batch_change_ids - OLD.id::text
        WHERE
          changesets.batch_change_ids ? OLD.id::text;

        RETURN OLD;
    END;
$$;
CREATE TRIGGER trig_delete_campaign_reference_on_changesets AFTER DELETE ON campaigns FOR EACH ROW EXECUTE PROCEDURE delete_campaign_reference_on_changesets();
ALTER SEQUENCE batch_changes_id_seq RENAME TO campaigns_id_seq;
ALTER SEQUENCE batch_specs_id_seq RENAME TO campaign_specs_id_seq;
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_batch_spec_id_fkey" TO "campaigns_campaign_spec_id_fkey";
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_initial_applier_id_fkey" TO "campaigns_initial_applier_id_fkey";
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_last_applier_id_fkey" TO "campaigns_last_applier_id_fkey";
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_namespace_org_id_fkey" TO "campaigns_namespace_org_id_fkey";
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_namespace_user_id_fkey" TO "campaigns_namespace_user_id_fkey";
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_has_1_namespace" TO "campaigns_has_1_namespace";
ALTER TABLE campaigns RENAME CONSTRAINT "batch_changes_name_not_blank" TO "campaigns_name_not_blank";
ALTER INDEX IF EXISTS batch_changes_pkey RENAME TO campaigns_pkey;
ALTER INDEX IF EXISTS batch_changes_namespace_org_id RENAME TO campaigns_namespace_org_id;
ALTER INDEX IF EXISTS batch_changes_namespace_user_id RENAME TO campaigns_namespace_user_id;
ALTER TABLE campaign_specs RENAME CONSTRAINT "batch_specs_has_1_namespace" TO "campaign_specs_has_1_namespace";
ALTER TABLE campaign_specs RENAME CONSTRAINT "batch_specs_user_id_fkey" TO "campaign_specs_user_id_fkey";
ALTER INDEX IF EXISTS batch_specs_pkey RENAME TO campaign_specs_pkey;
ALTER INDEX IF EXISTS batch_specs_rand_id RENAME TO campaign_specs_rand_id;
ALTER TABLE changeset_specs RENAME CONSTRAINT "changeset_specs_batch_spec_id_fkey" TO "changeset_specs_campaign_spec_id_fkey";
ALTER TABLE changesets RENAME CONSTRAINT "changesets_owned_by_batch_spec_id_fkey" TO "changesets_owned_by_campaign_id_fkey";
ALTER TABLE changesets RENAME CONSTRAINT "changesets_batch_change_ids_check" TO "changesets_campaign_ids_check";

COMMIT;
