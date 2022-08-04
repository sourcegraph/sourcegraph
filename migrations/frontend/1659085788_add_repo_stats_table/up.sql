--------------------------------------------------------------------------------
--                              repos table                                   --
--------------------------------------------------------------------------------

-- repo_statistics holds statistics for the repo table (hence the singular
-- "repo" in the name)
CREATE TABLE repo_statistics (
  -- We only allow one row in this table.
  id bool PRIMARY KEY DEFAULT TRUE,
  -- Constraint ensures that the `id` must be true and `PRIMARY KEY` ensures
  -- that it's unique.
  CONSTRAINT id CHECK (id),

  total        BIGINT NOT NULL DEFAULT 0,
  soft_deleted BIGINT NOT NULL DEFAULT 0
);

COMMENT ON COLUMN repo_statistics.total IS 'Number of repositories that are not soft-deleted and not blocked';
COMMENT ON COLUMN repo_statistics.soft_deleted IS 'Number of repositories that are soft-deleted and not blocked';

-- Insert initial values into repo_statistics table
INSERT INTO repo_statistics (total, soft_deleted)
VALUES (
  (SELECT COUNT(1) FROM repo WHERE deleted_at is NULL AND blocked IS NULL),
  (SELECT COUNT(1) FROM repo WHERE deleted_at is NOT NULL AND blocked IS NULL)
);

-- UPDATE
CREATE OR REPLACE FUNCTION recalc_repo_statistics_on_repo_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
      INSERT INTO
        repo_statistics (total, soft_deleted)
      VALUES (
        (SELECT COUNT(1) FROM newtab WHERE deleted_at IS NULL     AND blocked IS NULL) - (SELECT COUNT(1) FROM oldtab WHERE deleted_at IS NULL     AND blocked IS NULL),
        (SELECT COUNT(1) FROM newtab WHERE deleted_at IS NOT NULL AND blocked IS NULL) - (SELECT COUNT(1) FROM oldtab WHERE deleted_at IS NOT NULL AND blocked IS NULL)
      )
      ON CONFLICT(id) DO UPDATE
      SET
        total        = repo_statistics.total        + excluded.total,
        soft_deleted = repo_statistics.soft_deleted + excluded.soft_deleted
      ;
      RETURN NULL;
  END
$$;
DROP TRIGGER IF EXISTS trig_recalc_repo_statistics_on_repo_update ON repo;
CREATE TRIGGER trig_recalc_repo_statistics_on_repo_update AFTER UPDATE ON repo REFERENCING OLD TABLE AS oldtab NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_repo_statistics_on_repo_update();

-- INSERT
CREATE OR REPLACE FUNCTION recalc_repo_statistics_on_repo_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
      INSERT INTO
        repo_statistics (total, soft_deleted)
      VALUES (
        (SELECT COUNT(1) FROM newtab WHERE deleted_at IS NULL     AND blocked IS NULL),
        (SELECT COUNT(1) FROM newtab WHERE deleted_at IS NOT NULL AND blocked IS NULL)
      )
      ON CONFLICT(id) DO UPDATE
      SET
        total        = repo_statistics.total        + excluded.total,
        soft_deleted = repo_statistics.soft_deleted + excluded.soft_deleted
      ;
      RETURN NULL;
  END
$$;
DROP TRIGGER IF EXISTS trig_recalc_repo_statistics_on_repo_insert ON repo;
CREATE TRIGGER trig_recalc_repo_statistics_on_repo_insert AFTER INSERT ON repo REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_repo_statistics_on_repo_insert();

-- DELETE
CREATE OR REPLACE FUNCTION recalc_repo_statistics_on_repo_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
      UPDATE repo_statistics
      SET
        total        = repo_statistics.total        - (SELECT COUNT(1) FROM oldtab WHERE deleted_at IS NULL     AND blocked IS NULL),
        soft_deleted = repo_statistics.soft_deleted - (SELECT COUNT(1) FROM oldtab WHERE deleted_at IS NOT NULL AND blocked IS NULL)
      WHERE id = TRUE;
      RETURN NULL;
  END
$$;
DROP TRIGGER IF EXISTS trig_recalc_repo_statistics_on_repo_delete ON repo;
CREATE TRIGGER trig_recalc_repo_statistics_on_repo_delete AFTER DELETE ON repo REFERENCING OLD TABLE AS oldtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_repo_statistics_on_repo_delete();

--------------------------------------------------------------------------------
--                       gitserver_repos table                                --
--------------------------------------------------------------------------------

-- gitserver_repos_statistics holds statistics for the gitserver_repos table
CREATE TABLE gitserver_repos_statistics (
  -- In this table we have one row per shard_id
  shard_id text PRIMARY KEY,

  total       BIGINT NOT NULL DEFAULT 0,
  not_cloned  BIGINT NOT NULL DEFAULT 0,
  cloning     BIGINT NOT NULL DEFAULT 0,
  cloned      BIGINT NOT NULL DEFAULT 0
);

COMMENT ON COLUMN gitserver_repos_statistics.cloned IS 'Number of repositories in gitserver_repos table that are cloned';
COMMENT ON COLUMN gitserver_repos_statistics.cloning IS 'Number of repositories in gitserver_repos table that cloning';
COMMENT ON COLUMN gitserver_repos_statistics.not_cloned IS 'Number of repositories in gitserver_repos table that are not cloned yet';

-- Insert initial values into gitserver_repos_statistics
INSERT INTO
  gitserver_repos_statistics (shard_id, total, not_cloned, cloning, cloned)
SELECT
  shard_id,
  COUNT(*) AS total,
  COUNT(*) FILTER(WHERE clone_status = 'not_cloned') AS not_cloned,
  COUNT(*) FILTER(WHERE clone_status = 'cloning') AS cloning,
  COUNT(*) FILTER(WHERE clone_status = 'cloned') AS cloned
FROM
  gitserver_repos
GROUP BY shard_id;


-- UPDATE
CREATE OR REPLACE FUNCTION recalc_gitserver_repos_statistics_on_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
      INSERT INTO gitserver_repos_statistics AS grs (shard_id, total, not_cloned, cloning, cloned)
      SELECT
        newtab.shard_id AS shard_id,
        COUNT(*) AS total,
        COUNT(*) FILTER(WHERE clone_status = 'not_cloned')  AS not_cloned,
        COUNT(*) FILTER(WHERE clone_status = 'cloning') AS cloning,
        COUNT(*) FILTER(WHERE clone_status = 'cloned') AS cloned
      FROM
        newtab
      GROUP BY newtab.shard_id
      ON CONFLICT(shard_id) DO
      UPDATE
      SET
        total      = grs.total      + (excluded.total -      (SELECT COUNT(*)                                              FROM oldtab ot WHERE ot.shard_id = excluded.shard_id)),
        not_cloned = grs.not_cloned + (excluded.not_cloned - (SELECT COUNT(*) FILTER(WHERE ot.clone_status = 'not_cloned') FROM oldtab ot WHERE ot.shard_id = excluded.shard_id)),
        cloning    = grs.cloning    + (excluded.cloning -    (SELECT COUNT(*) FILTER(WHERE ot.clone_status = 'cloning')    FROM oldtab ot WHERE ot.shard_id = excluded.shard_id)),
        cloned     = grs.cloned     + (excluded.cloned -     (SELECT COUNT(*) FILTER(WHERE ot.clone_status = 'cloned')     FROM oldtab ot WHERE ot.shard_id = excluded.shard_id))
      ;

      RETURN NULL;
  END
$$;

DROP TRIGGER IF EXISTS trig_recalc_gitserver_repos_statistics_on_update ON gitserver_repos;
CREATE TRIGGER trig_recalc_gitserver_repos_statistics_on_update AFTER UPDATE ON gitserver_repos REFERENCING OLD TABLE AS oldtab NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_gitserver_repos_statistics_on_update();

-- INSERT
CREATE OR REPLACE FUNCTION recalc_gitserver_repos_statistics_on_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
      INSERT INTO gitserver_repos_statistics AS grs (shard_id, total, not_cloned, cloning, cloned)
      SELECT
        shard_id,
        COUNT(*) AS total,
        COUNT(*) FILTER(WHERE clone_status = 'not_cloned') AS not_cloned,
        COUNT(*) FILTER(WHERE clone_status = 'cloning') AS cloning,
        COUNT(*) FILTER(WHERE clone_status = 'cloned') AS cloned
      FROM
        newtab
      GROUP BY shard_id
      ON CONFLICT(shard_id)
      DO UPDATE
      SET
        total      = grs.total      + excluded.total,
        not_cloned = grs.not_cloned + excluded.not_cloned,
        cloning    = grs.cloning    + excluded.cloning,
        cloned     = grs.cloned     + excluded.cloned
      ;

      RETURN NULL;
  END
$$;
DROP TRIGGER IF EXISTS trig_recalc_gitserver_repos_statistics_on_insert ON gitserver_repos;
CREATE TRIGGER trig_recalc_gitserver_repos_statistics_on_insert AFTER INSERT ON gitserver_repos REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_gitserver_repos_statistics_on_insert();

-- DELETE
CREATE OR REPLACE FUNCTION recalc_gitserver_repos_statistics_on_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
      UPDATE gitserver_repos_statistics grs
      SET
        total      = grs.total      - (SELECT COUNT(*)                                           FROM oldtab WHERE oldtab.shard_id = grs.shard_id),
        not_cloned = grs.not_cloned - (SELECT COUNT(*) FILTER(WHERE clone_status = 'not_cloned') FROM oldtab WHERE oldtab.shard_id = grs.shard_id),
        cloning    = grs.cloning    - (SELECT COUNT(*) FILTER(WHERE clone_status = 'cloning')    FROM oldtab WHERE oldtab.shard_id = grs.shard_id),
        cloned     = grs.cloned     - (SELECT COUNT(*) FILTER(WHERE clone_status = 'cloned')     FROM oldtab WHERE oldtab.shard_id = grs.shard_id)
      ;

      RETURN NULL;
  END
$$;
DROP TRIGGER IF EXISTS trig_recalc_gitserver_repos_statistics_on_delete ON gitserver_repos;
CREATE TRIGGER trig_recalc_gitserver_repos_statistics_on_delete AFTER DELETE ON gitserver_repos REFERENCING OLD TABLE AS oldtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_gitserver_repos_statistics_on_delete();
