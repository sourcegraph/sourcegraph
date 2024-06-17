ALTER TABLE IF EXISTS github_apps
    ADD COLUMN IF NOT EXISTS kind VARCHAR(255) NULL;

UPDATE github_apps
    SET kind = 'COMMIT_SIGNING'
    WHERE domain = 'batches';

UPDATE github_apps
    SET kind = 'REPO_SYNC'
    WHERE domain = 'repos';

ALTER TABLE IF EXISTS github_apps
    ALTER COLUMN kind SET NOT NULL;
