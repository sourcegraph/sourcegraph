BEGIN;

ALTER TABLE insights_query_runner_jobs DROP COLUMN record_time;

COMMIT;
