alter table lsif_configuration_policies drop column if exists syntactic_indexing_enabled;
alter table lsif_last_index_scan drop column if exists indexing_type;
drop type if exists indexing_type;
