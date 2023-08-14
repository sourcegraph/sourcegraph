# Table "public.codeintel_last_reconcile"
```
      Column       |           Type           | Collation | Nullable | Default 
-------------------+--------------------------+-----------+----------+---------
 dump_id           | integer                  |           | not null | 
 last_reconcile_at | timestamp with time zone |           | not null | 
Indexes:
    "codeintel_last_reconcile_dump_id_key" UNIQUE CONSTRAINT, btree (dump_id)
    "codeintel_last_reconcile_last_reconcile_at_dump_id" btree (last_reconcile_at, dump_id)

```

Stores the last time processed LSIF data was reconciled with the other database.

# Table "public.codeintel_scip_document_lookup"
```
    Column     |  Type   | Collation | Nullable |                          Default                           
---------------+---------+-----------+----------+------------------------------------------------------------
 id            | bigint  |           | not null | nextval('codeintel_scip_document_lookup_id_seq'::regclass)
 upload_id     | integer |           | not null | 
 document_path | text    |           | not null | 
 document_id   | bigint  |           | not null | 
Indexes:
    "codeintel_scip_document_lookup_pkey" PRIMARY KEY, btree (id)
    "codeintel_scip_document_lookup_upload_id_document_path_key" UNIQUE CONSTRAINT, btree (upload_id, document_path)
    "codeintel_scip_document_lookup_document_id" hash (document_id)
Foreign-key constraints:
    "codeintel_scip_document_lookup_document_id_fk" FOREIGN KEY (document_id) REFERENCES codeintel_scip_documents(id)
Referenced by:
    TABLE "codeintel_scip_symbols" CONSTRAINT "codeintel_scip_symbols_document_lookup_id_fk" FOREIGN KEY (document_lookup_id) REFERENCES codeintel_scip_document_lookup(id) ON DELETE CASCADE
Triggers:
    codeintel_scip_document_lookup_schema_versions_insert AFTER INSERT ON codeintel_scip_document_lookup REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION update_codeintel_scip_document_lookup_schema_versions_insert()
    codeintel_scip_documents_dereference_logs_insert AFTER DELETE ON codeintel_scip_document_lookup REFERENCING OLD TABLE AS oldtab FOR EACH STATEMENT EXECUTE FUNCTION update_codeintel_scip_documents_dereference_logs_delete()

```

A mapping from file paths to document references within a particular SCIP index.

**document_id**: The foreign key to the shared document payload (see the table [`codeintel_scip_document_lookup`](#table-publiccodeintel_scip_document_lookup)).

**document_path**: The file path to the document relative to the root of the index.

**id**: An auto-generated identifier. This column is used as a foreign key target to reduce occurrences of the full document path value.

**upload_id**: The identifier of the upload that provided this SCIP index.

# Table "public.codeintel_scip_document_lookup_schema_versions"
```
       Column       |  Type   | Collation | Nullable | Default 
--------------------+---------+-----------+----------+---------
 upload_id          | integer |           | not null | 
 min_schema_version | integer |           |          | 
 max_schema_version | integer |           |          | 
Indexes:
    "codeintel_scip_document_lookup_schema_versions_pkey" PRIMARY KEY, btree (upload_id)

```

Tracks the range of `schema_versions` values associated with each SCIP index in the [`codeintel_scip_document_lookup`](#table-publiccodeintel_scip_document_lookup) table.

**max_schema_version**: An upper-bound on the `schema_version` values of the records in the table [`codeintel_scip_document_lookup`](#table-publiccodeintel_scip_document_lookup) where the `upload_id` column matches the associated SCIP index.

**min_schema_version**: A lower-bound on the `schema_version` values of the records in the table [`codeintel_scip_document_lookup`](#table-publiccodeintel_scip_document_lookup) where the `upload_id` column matches the associated SCIP index.

**upload_id**: The identifier of the associated SCIP index.

# Table "public.codeintel_scip_documents"
```
      Column      |  Type   | Collation | Nullable |                       Default                        
------------------+---------+-----------+----------+------------------------------------------------------
 id               | bigint  |           | not null | nextval('codeintel_scip_documents_id_seq'::regclass)
 payload_hash     | bytea   |           | not null | 
 schema_version   | integer |           | not null | 
 raw_scip_payload | bytea   |           | not null | 
Indexes:
    "codeintel_scip_documents_pkey" PRIMARY KEY, btree (id)
    "codeintel_scip_documents_payload_hash_key" UNIQUE CONSTRAINT, btree (payload_hash)
Referenced by:
    TABLE "codeintel_scip_document_lookup" CONSTRAINT "codeintel_scip_document_lookup_document_id_fk" FOREIGN KEY (document_id) REFERENCES codeintel_scip_documents(id)

```

A lookup of SCIP [Document](https://sourcegraph.com/search?q=context:%40sourcegraph/all+repo:%5Egithub%5C.com/sourcegraph/scip%24+file:%5Escip%5C.proto+message+Document&amp;patternType=standard) payloads by their hash.

**id**: An auto-generated identifier. This column is used as a foreign key target to reduce occurrences of the full payload hash value.

**payload_hash**: A deterministic hash of the raw SCIP payload. We use this as a unique value to enforce deduplication between indexes with the same document data.

**raw_scip_payload**: The raw, canonicalized SCIP [Document](https://sourcegraph.com/search?q=context:%40sourcegraph/all+repo:%5Egithub%5C.com/sourcegraph/scip%24+file:%5Escip%5C.proto+message+Document&amp;patternType=standard) payload.

**schema_version**: The schema version of this row - used to determine presence and encoding of (future) denormalized data.

# Table "public.codeintel_scip_documents_dereference_logs"
```
      Column       |           Type           | Collation | Nullable |                                Default                                
-------------------+--------------------------+-----------+----------+-----------------------------------------------------------------------
 id                | bigint                   |           | not null | nextval('codeintel_scip_documents_dereference_logs_id_seq'::regclass)
 document_id       | bigint                   |           | not null | 
 last_removal_time | timestamp with time zone |           | not null | now()
Indexes:
    "codeintel_scip_documents_dereference_logs_pkey" PRIMARY KEY, btree (id)
    "codeintel_scip_documents_dereference_logs_last_removal_time_des" btree (last_removal_time DESC, document_id)

```

A list of document rows that were recently dereferenced by the deletion of an index.

**document_id**: The identifier of the document that was dereferenced.

**last_removal_time**: The time that the log entry was inserted.

# Table "public.codeintel_scip_metadata"
```
         Column         |  Type   | Collation | Nullable |                       Default                       
------------------------+---------+-----------+----------+-----------------------------------------------------
 id                     | bigint  |           | not null | nextval('codeintel_scip_metadata_id_seq'::regclass)
 upload_id              | integer |           | not null | 
 tool_name              | text    |           | not null | 
 tool_version           | text    |           | not null | 
 tool_arguments         | text[]  |           | not null | 
 text_document_encoding | text    |           | not null | 
 protocol_version       | integer |           | not null | 
Indexes:
    "codeintel_scip_metadata_pkey" PRIMARY KEY, btree (id)
    "codeintel_scip_metadata_upload_id" btree (upload_id)

```

Global metadatadata about a single processed upload.

**id**: An auto-generated identifier.

**protocol_version**: The version of the SCIP protocol used to encode this index.

**text_document_encoding**: The encoding of the text documents within this index. May affect range boundaries.

**tool_arguments**: Command-line arguments that were used to invoke this indexer.

**tool_name**: Name of the indexer that produced this index.

**tool_version**: Version of the indexer that produced this index.

**upload_id**: The identifier of the upload that provided this SCIP index.

# Table "public.codeintel_scip_symbol_names"
```
    Column    |  Type   | Collation | Nullable | Default 
--------------+---------+-----------+----------+---------
 id           | integer |           | not null | 
 upload_id    | integer |           | not null | 
 name_segment | text    |           | not null | 
 prefix_id    | integer |           |          | 
Indexes:
    "codeintel_scip_symbol_names_pkey" PRIMARY KEY, btree (upload_id, id)
    "codeintel_scip_symbol_names_upload_id_roots" btree (upload_id) WHERE prefix_id IS NULL
    "codeisdntel_scip_symbol_names_upload_id_children" btree (upload_id, prefix_id) WHERE prefix_id IS NOT NULL

```

Stores a prefix tree of symbol names within a particular upload.

**id**: An identifier unique within the index for this symbol name segment.

**name_segment**: The portion of the symbol name that is unique to this symbol and its children.

**prefix_id**: The identifier of the segment that forms the prefix of this symbol, if any.

**upload_id**: The identifier of the upload that provided this SCIP index.

# Table "public.codeintel_scip_symbols"
```
         Column         |  Type   | Collation | Nullable | Default 
------------------------+---------+-----------+----------+---------
 upload_id              | integer |           | not null | 
 document_lookup_id     | bigint  |           | not null | 
 schema_version         | integer |           | not null | 
 definition_ranges      | bytea   |           |          | 
 reference_ranges       | bytea   |           |          | 
 implementation_ranges  | bytea   |           |          | 
 type_definition_ranges | bytea   |           |          | 
 symbol_id              | integer |           | not null | 
Indexes:
    "codeintel_scip_symbols_pkey" PRIMARY KEY, btree (upload_id, symbol_id, document_lookup_id)
    "codeintel_scip_symbols_document_lookup_id" btree (document_lookup_id)
Foreign-key constraints:
    "codeintel_scip_symbols_document_lookup_id_fk" FOREIGN KEY (document_lookup_id) REFERENCES codeintel_scip_document_lookup(id) ON DELETE CASCADE
Triggers:
    codeintel_scip_symbols_schema_versions_insert AFTER INSERT ON codeintel_scip_symbols REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION update_codeintel_scip_symbols_schema_versions_insert()

```

A mapping from SCIP [Symbol names](https://sourcegraph.com/search?q=context:%40sourcegraph/all+repo:%5Egithub%5C.com/sourcegraph/scip%24+file:%5Escip%5C.proto+message+Symbol&amp;patternType=standard) to path and ranges where that symbol occurs within a particular SCIP index.

**definition_ranges**: An encoded set of ranges within the associated document that have a **definition** relationship to the associated symbol.

**document_lookup_id**: A reference to the `id` column of [`codeintel_scip_document_lookup`](#table-publiccodeintel_scip_document_lookup). Joining on this table yields the document path relative to the index root.

**implementation_ranges**: An encoded set of ranges within the associated document that have a **implementation** relationship to the associated symbol.

**reference_ranges**: An encoded set of ranges within the associated document that have a **reference** relationship to the associated symbol.

**schema_version**: The schema version of this row - used to determine presence and encoding of denormalized data.

**symbol_id**: The identifier of the segment that terminates the name of this symbol. See the table [`codeintel_scip_symbol_names`](#table-publiccodeintel_scip_symbol_names) on how to reconstruct the full symbol name.

**type_definition_ranges**: An encoded set of ranges within the associated document that have a **type definition** relationship to the associated symbol.

**upload_id**: The identifier of the upload that provided this SCIP index.

# Table "public.codeintel_scip_symbols_lookup"
```
     Column      |           Type           | Collation | Nullable | Default 
-----------------+--------------------------+-----------+----------+---------
 upload_id       | integer                  |           | not null | 
 segment_type    | symbolnamesegmenttype    |           | not null | 
 segment_quality | symbolnamesegmentquality |           |          | 
 name            | text                     |           | not null | 
 id              | integer                  |           | not null | 
 parent_id       | integer                  |           |          | 
Indexes:
    "codeintel_scip_symbols_lookup_id" UNIQUE, btree (upload_id, id)
    "codeintel_scip_symbols_lookup_reversed_descriptor_suffix_name" btree (upload_id, reverse(name) text_pattern_ops) WHERE segment_type = 'DESCRIPTOR_SUFFIX'::symbolnamesegmenttype

```

# Table "public.codeintel_scip_symbols_lookup_leaves"
```
           Column           |  Type   | Collation | Nullable | Default 
----------------------------+---------+-----------+----------+---------
 upload_id                  | integer |           | not null | 
 symbol_id                  | integer |           | not null | 
 descriptor_suffix_id       | integer |           | not null | 
 fuzzy_descriptor_suffix_id | integer |           |          | 
Indexes:
    "codeintel_scip_symbols_lookup_leaves_descriptor_suffix_id" btree (upload_id, descriptor_suffix_id)
    "codeintel_scip_symbols_lookup_leaves_fuzzy_descriptor_suffix_id" btree (upload_id, fuzzy_descriptor_suffix_id) WHERE fuzzy_descriptor_suffix_id IS NOT NULL

```

# Table "public.codeintel_scip_symbols_migration_progress"
```
  Column   |  Type   | Collation | Nullable | Default 
-----------+---------+-----------+----------+---------
 upload_id | integer |           | not null | 
 symbol_id | integer |           | not null | 
Indexes:
    "codeintel_scip_symbols_migration_progress_pkey" PRIMARY KEY, btree (upload_id)

```

# Table "public.codeintel_scip_symbols_schema_versions"
```
       Column       |  Type   | Collation | Nullable | Default 
--------------------+---------+-----------+----------+---------
 upload_id          | integer |           | not null | 
 min_schema_version | integer |           |          | 
 max_schema_version | integer |           |          | 
Indexes:
    "codeintel_scip_symbols_schema_versions_pkey" PRIMARY KEY, btree (upload_id)

```

Tracks the range of `schema_versions` for each index in the [`codeintel_scip_symbols`](#table-publiccodeintel_scip_symbols) table.

**max_schema_version**: An upper-bound on the `schema_version` values of the records in the table [`codeintel_scip_symbols`](#table-publiccodeintel_scip_symbols) where the `upload_id` column matches the associated SCIP index.

**min_schema_version**: A lower-bound on the `schema_version` values of the records in the table [`codeintel_scip_symbols`](#table-publiccodeintel_scip_symbols) where the `upload_id` column matches the associated SCIP index.

**upload_id**: The identifier of the associated SCIP index.

# Table "public.migration_logs"
```
            Column             |           Type           | Collation | Nullable |                  Default                   
-------------------------------+--------------------------+-----------+----------+--------------------------------------------
 id                            | integer                  |           | not null | nextval('migration_logs_id_seq'::regclass)
 migration_logs_schema_version | integer                  |           | not null | 
 schema                        | text                     |           | not null | 
 version                       | integer                  |           | not null | 
 up                            | boolean                  |           | not null | 
 started_at                    | timestamp with time zone |           | not null | 
 finished_at                   | timestamp with time zone |           |          | 
 success                       | boolean                  |           |          | 
 error_message                 | text                     |           |          | 
 backfilled                    | boolean                  |           | not null | false
Indexes:
    "migration_logs_pkey" PRIMARY KEY, btree (id)

```

# Table "public.rockskip_ancestry"
```
  Column   |         Type          | Collation | Nullable |                    Default                    
-----------+-----------------------+-----------+----------+-----------------------------------------------
 id        | integer               |           | not null | nextval('rockskip_ancestry_id_seq'::regclass)
 repo_id   | integer               |           | not null | 
 commit_id | character varying(40) |           | not null | 
 height    | integer               |           | not null | 
 ancestor  | integer               |           | not null | 
Indexes:
    "rockskip_ancestry_pkey" PRIMARY KEY, btree (id)
    "rockskip_ancestry_repo_id_commit_id_key" UNIQUE CONSTRAINT, btree (repo_id, commit_id)
    "rockskip_ancestry_repo_commit_id" btree (repo_id, commit_id)

```

# Table "public.rockskip_repos"
```
      Column      |           Type           | Collation | Nullable |                  Default                   
------------------+--------------------------+-----------+----------+--------------------------------------------
 id               | integer                  |           | not null | nextval('rockskip_repos_id_seq'::regclass)
 repo             | text                     |           | not null | 
 last_accessed_at | timestamp with time zone |           | not null | 
Indexes:
    "rockskip_repos_pkey" PRIMARY KEY, btree (id)
    "rockskip_repos_repo_key" UNIQUE CONSTRAINT, btree (repo)
    "rockskip_repos_last_accessed_at" btree (last_accessed_at)
    "rockskip_repos_repo" btree (repo)

```

# Table "public.rockskip_symbols"
```
 Column  |   Type    | Collation | Nullable |                   Default                    
---------+-----------+-----------+----------+----------------------------------------------
 id      | integer   |           | not null | nextval('rockskip_symbols_id_seq'::regclass)
 added   | integer[] |           | not null | 
 deleted | integer[] |           | not null | 
 repo_id | integer   |           | not null | 
 path    | text      |           | not null | 
 name    | text      |           | not null | 
Indexes:
    "rockskip_symbols_pkey" PRIMARY KEY, btree (id)
    "rockskip_symbols_gin" gin (singleton_integer(repo_id) gin__int_ops, added gin__int_ops, deleted gin__int_ops, name gin_trgm_ops, singleton(name), singleton(lower(name)), path gin_trgm_ops, singleton(path), path_prefixes(path), singleton(lower(path)), path_prefixes(lower(path)), singleton(get_file_extension(path)), singleton(get_file_extension(lower(path))))
    "rockskip_symbols_repo_id_path_name" btree (repo_id, path, name)

```

# Type symbolnamesegmentquality

- FUZZY
- PRECISE
- BOTH

# Type symbolnamesegmenttype

- SCHEME
- PACKAGE_MANAGER
- PACKAGE_NAME
- PACKAGE_VERSION
- DESCRIPTOR_NAMESPACE
- DESCRIPTOR_SUFFIX
