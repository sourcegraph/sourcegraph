alter table lsif_configuration_policies add column if not exists syntactic_indexing_enabled bool not null default false;

CREATE TABLE IF NOT EXISTS syntactic_scip_index_last_scan(
    repository_id int NOT NULL,
    last_index_scan_at timestamp with time zone NOT NULL,
    PRIMARY KEY(repository_id)
);

COMMENT ON TABLE syntactic_scip_index_last_scan IS 'Tracks the last time repository was checked for syntactic indexing job scheduling.';
COMMENT ON COLUMN syntactic_scip_index_last_scan.last_index_scan_at IS 'The last time uploads of this repository were considered for syntactic indexing job scheduling.';
