ALTER TABLE IF EXISTS external_service_sync_jobs
ADD COLUMN IF NOT EXISTS repos_synced integer DEFAULT 0 NOT NULL,
ADD COLUMN IF NOT EXISTS repo_sync_errors integer DEFAULT 0 NOT NULL,
ADD COLUMN IF NOT EXISTS repos_added integer DEFAULT 0 NOT NULL,
ADD COLUMN IF NOT EXISTS repos_removed integer DEFAULT 0 NOT NULL,
ADD COLUMN IF NOT EXISTS repos_modified integer DEFAULT 0 NOT NULL,
ADD COLUMN IF NOT EXISTS repos_unmodified integer DEFAULT 0 NOT NULL,
ADD COLUMN IF NOT EXISTS repos_deleted integer DEFAULT 0 NOT NULL;
