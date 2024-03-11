alter table lsif_configuration_policies add column if not exists syntactic_indexing_enabled bool default true;
CREATE TYPE indexing_type AS ENUM ('precise', 'syntactic');
alter table lsif_last_index_scan add column if not exists indexing_type indexing_type not null default 'precise';
