BEGIN;

ALTER TABLE insights_query_runner_jobs ADD COLUMN record_time timestamptz;

COMMIT;
