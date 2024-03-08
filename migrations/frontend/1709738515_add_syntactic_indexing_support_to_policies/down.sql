alter table lsif_configuration_policies drop column syntactic_indexing_enabled;
alter table lsif_last_index_scan drop column indexing_type;
drop type if exists indexing_type;
