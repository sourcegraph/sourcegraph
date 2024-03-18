alter table lsif_last_index_scan
      drop constraint lsif_last_index_scan_pkey,
      add constraint lsif_last_index_scan_pkey primary key (repository_id);
