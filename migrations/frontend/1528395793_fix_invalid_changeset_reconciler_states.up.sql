BEGIN;

UPDATE changesets SET reconciler_state = 'queued' WHERE reconciler_state = 'QUEUED';

COMMIT;
