BEGIN;

UPDATE changesets SET reconciler_state = 'QUEUED' WHERE reconciler_state = 'queued';

COMMIT;
