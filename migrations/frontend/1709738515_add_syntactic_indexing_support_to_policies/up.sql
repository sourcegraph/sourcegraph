alter table lsif_configuration_policies add column if not exists syntactic_indexing_enabled bool default true;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'indexing_type') THEN
        CREATE TYPE indexing_type AS ENUM ('precise', 'syntactic');
    END IF;
END$$;

alter table lsif_last_index_scan add column if not exists indexing_type indexing_type not null default 'precise';
