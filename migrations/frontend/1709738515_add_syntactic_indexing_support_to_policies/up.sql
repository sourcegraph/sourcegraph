alter table lsif_configuration_policies add column syntactic_indexing_enabled bool default true;
CREATE TYPE indexing_type AS ENUM ('precise', 'syntactic');
alter table lsif_last_index_scan add column indexing_type indexing_type not null default 'precise';
