BEGIN;

UPDATE user_credentials SET domain = 'batches' WHERE domain = 'campaigns';
ALTER TABLE IF EXISTS campaign_specs RENAME TO batch_specs;
ALTER TABLE IF EXISTS campaigns RENAME TO batch_changes;
ALTER TABLE IF EXISTS batch_changes RENAME COLUMN campaign_spec_id TO batch_spec_id;
ALTER TABLE IF EXISTS changeset_specs RENAME COLUMN campaign_spec_id TO batch_spec_id;
DROP VIEW IF EXISTS reconciler_changesets;
DROP VIEW IF EXISTS branch_changeset_specs_and_changesets;
DROP VIEW IF EXISTS tracking_changeset_specs_and_changesets;
ALTER TABLE IF EXISTS changesets RENAME COLUMN campaign_ids TO batch_change_ids;
ALTER TABLE IF EXISTS changesets RENAME COLUMN owned_by_campaign_id TO owned_by_batch_change_id;
CREATE VIEW tracking_changeset_specs_and_changesets AS (
    SELECT
        changeset_specs.id AS changeset_spec_id,
        COALESCE(changesets.id, 0::bigint) AS changeset_id,
        changeset_specs.repo_id,
        changeset_specs.batch_spec_id,
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
        changeset_specs.batch_spec_id,
        changesets.owned_by_batch_change_id AS owner_batch_change_id,
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
            SELECT 1 FROM batch_changes
            LEFT JOIN users namespace_user ON batch_changes.namespace_user_id = namespace_user.id
            LEFT JOIN orgs namespace_org ON batch_changes.namespace_org_id = namespace_org.id
            WHERE
                c.batch_change_ids ? batch_changes.id::text AND
                namespace_user.deleted_at IS NULL AND
                namespace_org.deleted_at IS NULL
        )
;
DROP TRIGGER IF EXISTS trig_delete_campaign_reference_on_changesets ON batch_changes;
DROP FUNCTION IF EXISTS delete_campaign_reference_on_changesets();
CREATE FUNCTION delete_batch_change_reference_on_changesets() RETURNS trigger
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
CREATE TRIGGER trig_delete_batch_change_reference_on_changesets AFTER DELETE ON batch_changes FOR EACH ROW EXECUTE PROCEDURE delete_batch_change_reference_on_changesets();
ALTER SEQUENCE campaigns_id_seq RENAME TO batch_changes_id_seq;
ALTER SEQUENCE campaign_specs_id_seq RENAME TO batch_specs_id_seq;
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_campaign_spec_id_fkey" TO "batch_changes_batch_spec_id_fkey";
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_initial_applier_id_fkey" TO "batch_changes_initial_applier_id_fkey";
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_last_applier_id_fkey" TO "batch_changes_last_applier_id_fkey";
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_namespace_org_id_fkey" TO "batch_changes_namespace_org_id_fkey";
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_namespace_user_id_fkey" TO "batch_changes_namespace_user_id_fkey";
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_has_1_namespace" TO "batch_changes_has_1_namespace";
ALTER TABLE batch_changes RENAME CONSTRAINT "campaigns_name_not_blank" TO "batch_changes_name_not_blank";
ALTER INDEX IF EXISTS campaigns_pkey RENAME TO batch_changes_pkey;
ALTER INDEX IF EXISTS campaigns_namespace_org_id RENAME TO batch_changes_namespace_org_id;
ALTER INDEX IF EXISTS campaigns_namespace_user_id RENAME TO batch_changes_namespace_user_id;
ALTER TABLE batch_specs RENAME CONSTRAINT "campaign_specs_has_1_namespace" TO "batch_specs_has_1_namespace";
ALTER TABLE batch_specs RENAME CONSTRAINT "campaign_specs_user_id_fkey" TO "batch_specs_user_id_fkey";
ALTER INDEX IF EXISTS campaign_specs_pkey RENAME TO batch_specs_pkey;
ALTER INDEX IF EXISTS campaign_specs_rand_id RENAME TO batch_specs_rand_id;
ALTER TABLE changeset_specs RENAME CONSTRAINT "changeset_specs_campaign_spec_id_fkey" TO "changeset_specs_batch_spec_id_fkey";
ALTER TABLE changesets RENAME CONSTRAINT "changesets_owned_by_campaign_id_fkey" TO "changesets_owned_by_batch_spec_id_fkey";
ALTER TABLE changesets RENAME CONSTRAINT "changesets_campaign_ids_check" TO "changesets_batch_change_ids_check";

COMMIT;
