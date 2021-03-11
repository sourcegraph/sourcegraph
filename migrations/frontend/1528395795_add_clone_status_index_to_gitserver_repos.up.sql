BEGIN;

CREATE INDEX IF NOT EXISTS gitserver_repos_clone_status_idx ON gitserver_repos (clone_status);

COMMIT;
