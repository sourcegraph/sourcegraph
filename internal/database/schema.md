# Table "public.access_requests"
```
       Column        |           Type           | Collation | Nullable |                   Default                   
---------------------+--------------------------+-----------+----------+---------------------------------------------
 id                  | integer                  |           | not null | nextval('access_requests_id_seq'::regclass)
 created_at          | timestamp with time zone |           | not null | now()
 updated_at          | timestamp with time zone |           | not null | now()
 name                | text                     |           | not null | 
 email               | text                     |           | not null | 
 additional_info     | text                     |           |          | 
 status              | text                     |           | not null | 
 decision_by_user_id | integer                  |           |          | 
Indexes:
    "access_requests_pkey" PRIMARY KEY, btree (id)
    "access_requests_email_key" UNIQUE CONSTRAINT, btree (email)
    "access_requests_created_at" btree (created_at)
    "access_requests_status" btree (status)
Foreign-key constraints:
    "access_requests_decision_by_user_id_fkey" FOREIGN KEY (decision_by_user_id) REFERENCES users(id) ON DELETE SET NULL

```

# Table "public.access_tokens"
```
     Column      |           Type           | Collation | Nullable |                  Default                  
-----------------+--------------------------+-----------+----------+-------------------------------------------
 id              | bigint                   |           | not null | nextval('access_tokens_id_seq'::regclass)
 subject_user_id | integer                  |           | not null | 
 value_sha256    | bytea                    |           | not null | 
 note            | text                     |           | not null | 
 created_at      | timestamp with time zone |           | not null | now()
 last_used_at    | timestamp with time zone |           |          | 
 deleted_at      | timestamp with time zone |           |          | 
 creator_user_id | integer                  |           | not null | 
 scopes          | text[]                   |           | not null | 
 internal        | boolean                  |           |          | false
 expires_at      | timestamp with time zone |           |          | 
Indexes:
    "access_tokens_pkey" PRIMARY KEY, btree (id)
    "access_tokens_value_sha256_key" UNIQUE CONSTRAINT, btree (value_sha256)
    "access_tokens_lookup" hash (value_sha256) WHERE deleted_at IS NULL
    "access_tokens_lookup_double_hash" hash (digest(value_sha256, 'sha256'::text)) WHERE deleted_at IS NULL
Foreign-key constraints:
    "access_tokens_creator_user_id_fkey" FOREIGN KEY (creator_user_id) REFERENCES users(id)
    "access_tokens_subject_user_id_fkey" FOREIGN KEY (subject_user_id) REFERENCES users(id)

```

# Table "public.aggregated_user_statistics"
```
       Column        |           Type           | Collation | Nullable | Default 
---------------------+--------------------------+-----------+----------+---------
 user_id             | bigint                   |           | not null | 
 created_at          | timestamp with time zone |           | not null | now()
 updated_at          | timestamp with time zone |           | not null | now()
 user_last_active_at | timestamp with time zone |           |          | 
 user_events_count   | bigint                   |           |          | 
Indexes:
    "aggregated_user_statistics_pkey" PRIMARY KEY, btree (user_id)
Foreign-key constraints:
    "aggregated_user_statistics_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.assigned_owners"
```
        Column        |            Type             | Collation | Nullable |                   Default                   
----------------------+-----------------------------+-----------+----------+---------------------------------------------
 id                   | integer                     |           | not null | nextval('assigned_owners_id_seq'::regclass)
 owner_user_id        | integer                     |           | not null | 
 file_path_id         | integer                     |           | not null | 
 who_assigned_user_id | integer                     |           |          | 
 assigned_at          | timestamp without time zone |           | not null | now()
Indexes:
    "assigned_owners_pkey" PRIMARY KEY, btree (id)
    "assigned_owners_file_path_owner" UNIQUE, btree (file_path_id, owner_user_id)
Foreign-key constraints:
    "assigned_owners_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    "assigned_owners_owner_user_id_fkey" FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    "assigned_owners_who_assigned_user_id_fkey" FOREIGN KEY (who_assigned_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE

```

Table for ownership assignments, one entry contains an assigned user ID, which repo_path is assigned and the date and user who assigned the owner.

# Table "public.assigned_teams"
```
        Column        |            Type             | Collation | Nullable |                  Default                   
----------------------+-----------------------------+-----------+----------+--------------------------------------------
 id                   | integer                     |           | not null | nextval('assigned_teams_id_seq'::regclass)
 owner_team_id        | integer                     |           | not null | 
 file_path_id         | integer                     |           | not null | 
 who_assigned_team_id | integer                     |           |          | 
 assigned_at          | timestamp without time zone |           | not null | now()
Indexes:
    "assigned_teams_pkey" PRIMARY KEY, btree (id)
    "assigned_teams_file_path_owner" UNIQUE, btree (file_path_id, owner_team_id)
Foreign-key constraints:
    "assigned_teams_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    "assigned_teams_owner_team_id_fkey" FOREIGN KEY (owner_team_id) REFERENCES teams(id) ON DELETE CASCADE DEFERRABLE
    "assigned_teams_who_assigned_team_id_fkey" FOREIGN KEY (who_assigned_team_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE

```

Table for team ownership assignments, one entry contains an assigned team ID, which repo_path is assigned and the date and user who assigned the owner team.

# Table "public.batch_changes"
```
      Column       |           Type           | Collation | Nullable |                  Default                  
-------------------+--------------------------+-----------+----------+-------------------------------------------
 id                | bigint                   |           | not null | nextval('batch_changes_id_seq'::regclass)
 name              | text                     |           | not null | 
 description       | text                     |           |          | 
 creator_id        | integer                  |           |          | 
 namespace_user_id | integer                  |           |          | 
 namespace_org_id  | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 closed_at         | timestamp with time zone |           |          | 
 batch_spec_id     | bigint                   |           | not null | 
 last_applier_id   | bigint                   |           |          | 
 last_applied_at   | timestamp with time zone |           |          | 
Indexes:
    "batch_changes_pkey" PRIMARY KEY, btree (id)
    "batch_changes_unique_org_id" UNIQUE, btree (name, namespace_org_id) WHERE namespace_org_id IS NOT NULL
    "batch_changes_unique_user_id" UNIQUE, btree (name, namespace_user_id) WHERE namespace_user_id IS NOT NULL
    "batch_changes_namespace_org_id" btree (namespace_org_id)
    "batch_changes_namespace_user_id" btree (namespace_user_id)
Check constraints:
    "batch_change_name_is_valid" CHECK (name ~ '^[\w.-]+$'::text)
    "batch_changes_has_1_namespace" CHECK ((namespace_user_id IS NULL) <> (namespace_org_id IS NULL))
    "batch_changes_name_not_blank" CHECK (name <> ''::text)
Foreign-key constraints:
    "batch_changes_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) DEFERRABLE
    "batch_changes_initial_applier_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    "batch_changes_last_applier_id_fkey" FOREIGN KEY (last_applier_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    "batch_changes_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE DEFERRABLE
    "batch_changes_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
Referenced by:
    TABLE "batch_specs" CONSTRAINT "batch_specs_batch_change_id_fkey" FOREIGN KEY (batch_change_id) REFERENCES batch_changes(id) ON DELETE SET NULL DEFERRABLE
    TABLE "changeset_jobs" CONSTRAINT "changeset_jobs_batch_change_id_fkey" FOREIGN KEY (batch_change_id) REFERENCES batch_changes(id) ON DELETE CASCADE DEFERRABLE
    TABLE "changesets" CONSTRAINT "changesets_owned_by_batch_spec_id_fkey" FOREIGN KEY (owned_by_batch_change_id) REFERENCES batch_changes(id) ON DELETE SET NULL DEFERRABLE
Triggers:
    trig_delete_batch_change_reference_on_changesets AFTER DELETE ON batch_changes FOR EACH ROW EXECUTE FUNCTION delete_batch_change_reference_on_changesets()

```

# Table "public.batch_changes_site_credentials"
```
        Column         |           Type           | Collation | Nullable |                          Default                           
-----------------------+--------------------------+-----------+----------+------------------------------------------------------------
 id                    | bigint                   |           | not null | nextval('batch_changes_site_credentials_id_seq'::regclass)
 external_service_type | text                     |           | not null | 
 external_service_id   | text                     |           | not null | 
 created_at            | timestamp with time zone |           | not null | now()
 updated_at            | timestamp with time zone |           | not null | now()
 credential            | bytea                    |           | not null | 
 encryption_key_id     | text                     |           | not null | ''::text
 github_app_id         | integer                  |           |          | 
Indexes:
    "batch_changes_site_credentials_pkey" PRIMARY KEY, btree (id)
    "batch_changes_site_credentials_unique" UNIQUE, btree (external_service_type, external_service_id)
    "batch_changes_site_credentials_credential_idx" btree ((encryption_key_id = ANY (ARRAY[''::text, 'previously-migrated'::text])))
Check constraints:
    "check_github_app_id_and_external_service_type" CHECK (github_app_id IS NULL OR external_service_type = 'github'::text)
Foreign-key constraints:
    "batch_changes_site_credentials_github_app_id_fkey" FOREIGN KEY (github_app_id) REFERENCES github_apps(id)

```

# Table "public.batch_spec_execution_cache_entries"
```
    Column    |           Type           | Collation | Nullable |                            Default                             
--------------+--------------------------+-----------+----------+----------------------------------------------------------------
 id           | bigint                   |           | not null | nextval('batch_spec_execution_cache_entries_id_seq'::regclass)
 key          | text                     |           | not null | 
 value        | text                     |           | not null | 
 version      | integer                  |           | not null | 
 last_used_at | timestamp with time zone |           |          | 
 created_at   | timestamp with time zone |           | not null | now()
 user_id      | integer                  |           | not null | 
Indexes:
    "batch_spec_execution_cache_entries_pkey" PRIMARY KEY, btree (id)
    "batch_spec_execution_cache_entries_user_id_key_unique" UNIQUE CONSTRAINT, btree (user_id, key)
Foreign-key constraints:
    "batch_spec_execution_cache_entries_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.batch_spec_resolution_jobs"
```
      Column       |           Type           | Collation | Nullable |                        Default                         
-------------------+--------------------------+-----------+----------+--------------------------------------------------------
 id                | bigint                   |           | not null | nextval('batch_spec_resolution_jobs_id_seq'::regclass)
 batch_spec_id     | integer                  |           | not null | 
 state             | text                     |           | not null | 'queued'::text
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 last_heartbeat_at | timestamp with time zone |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 queued_at         | timestamp with time zone |           |          | now()
 initiator_id      | integer                  |           | not null | 
 cancel            | boolean                  |           | not null | false
Indexes:
    "batch_spec_resolution_jobs_pkey" PRIMARY KEY, btree (id)
    "batch_spec_resolution_jobs_batch_spec_id_unique" UNIQUE CONSTRAINT, btree (batch_spec_id)
    "batch_spec_resolution_jobs_state" btree (state)
Foreign-key constraints:
    "batch_spec_resolution_jobs_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE DEFERRABLE
    "batch_spec_resolution_jobs_initiator_id_fkey" FOREIGN KEY (initiator_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE

```

# Table "public.batch_spec_workspace_execution_jobs"
```
         Column          |           Type           | Collation | Nullable |                             Default                             
-------------------------+--------------------------+-----------+----------+-----------------------------------------------------------------
 id                      | bigint                   |           | not null | nextval('batch_spec_workspace_execution_jobs_id_seq'::regclass)
 batch_spec_workspace_id | integer                  |           | not null | 
 state                   | text                     |           | not null | 'queued'::text
 failure_message         | text                     |           |          | 
 started_at              | timestamp with time zone |           |          | 
 finished_at             | timestamp with time zone |           |          | 
 process_after           | timestamp with time zone |           |          | 
 num_resets              | integer                  |           | not null | 0
 num_failures            | integer                  |           | not null | 0
 execution_logs          | json[]                   |           |          | 
 worker_hostname         | text                     |           | not null | ''::text
 last_heartbeat_at       | timestamp with time zone |           |          | 
 created_at              | timestamp with time zone |           | not null | now()
 updated_at              | timestamp with time zone |           | not null | now()
 cancel                  | boolean                  |           | not null | false
 queued_at               | timestamp with time zone |           |          | now()
 user_id                 | integer                  |           | not null | 
 version                 | integer                  |           | not null | 1
Indexes:
    "batch_spec_workspace_execution_jobs_pkey" PRIMARY KEY, btree (id)
    "batch_spec_workspace_execution_jobs_batch_spec_workspace_id" btree (batch_spec_workspace_id)
    "batch_spec_workspace_execution_jobs_cancel" btree (cancel)
    "batch_spec_workspace_execution_jobs_last_dequeue" btree (user_id, started_at DESC)
    "batch_spec_workspace_execution_jobs_state" btree (state)
Foreign-key constraints:
    "batch_spec_workspace_execution_job_batch_spec_workspace_id_fkey" FOREIGN KEY (batch_spec_workspace_id) REFERENCES batch_spec_workspaces(id) ON DELETE CASCADE DEFERRABLE
Triggers:
    batch_spec_workspace_execution_last_dequeues_insert AFTER INSERT ON batch_spec_workspace_execution_jobs REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION batch_spec_workspace_execution_last_dequeues_upsert()
    batch_spec_workspace_execution_last_dequeues_update AFTER UPDATE ON batch_spec_workspace_execution_jobs REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION batch_spec_workspace_execution_last_dequeues_upsert()

```

# Table "public.batch_spec_workspace_execution_last_dequeues"
```
     Column     |           Type           | Collation | Nullable | Default 
----------------+--------------------------+-----------+----------+---------
 user_id        | integer                  |           | not null | 
 latest_dequeue | timestamp with time zone |           |          | 
Indexes:
    "batch_spec_workspace_execution_last_dequeues_pkey" PRIMARY KEY, btree (user_id)
Foreign-key constraints:
    "batch_spec_workspace_execution_last_dequeues_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED

```

# Table "public.batch_spec_workspace_files"
```
    Column     |           Type           | Collation | Nullable |                        Default                         
---------------+--------------------------+-----------+----------+--------------------------------------------------------
 id            | integer                  |           | not null | nextval('batch_spec_workspace_files_id_seq'::regclass)
 rand_id       | text                     |           | not null | 
 batch_spec_id | bigint                   |           | not null | 
 filename      | text                     |           | not null | 
 path          | text                     |           | not null | 
 size          | bigint                   |           | not null | 
 content       | bytea                    |           | not null | 
 modified_at   | timestamp with time zone |           | not null | 
 created_at    | timestamp with time zone |           | not null | now()
 updated_at    | timestamp with time zone |           | not null | now()
Indexes:
    "batch_spec_workspace_files_pkey" PRIMARY KEY, btree (id)
    "batch_spec_workspace_files_batch_spec_id_filename_path" UNIQUE, btree (batch_spec_id, filename, path)
    "batch_spec_workspace_files_rand_id" btree (rand_id)
Foreign-key constraints:
    "batch_spec_workspace_files_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE

```

# Table "public.batch_spec_workspaces"
```
        Column        |           Type           | Collation | Nullable |                      Default                      
----------------------+--------------------------+-----------+----------+---------------------------------------------------
 id                   | bigint                   |           | not null | nextval('batch_spec_workspaces_id_seq'::regclass)
 batch_spec_id        | integer                  |           | not null | 
 changeset_spec_ids   | jsonb                    |           | not null | '{}'::jsonb
 repo_id              | integer                  |           | not null | 
 branch               | text                     |           | not null | 
 commit               | text                     |           | not null | 
 path                 | text                     |           | not null | 
 file_matches         | text[]                   |           | not null | 
 only_fetch_workspace | boolean                  |           | not null | false
 created_at           | timestamp with time zone |           | not null | now()
 updated_at           | timestamp with time zone |           | not null | now()
 ignored              | boolean                  |           | not null | false
 unsupported          | boolean                  |           | not null | false
 skipped              | boolean                  |           | not null | false
 cached_result_found  | boolean                  |           | not null | false
 step_cache_results   | jsonb                    |           | not null | '{}'::jsonb
Indexes:
    "batch_spec_workspaces_pkey" PRIMARY KEY, btree (id)
    "batch_spec_workspaces_batch_spec_id" btree (batch_spec_id)
    "batch_spec_workspaces_id_batch_spec_id" btree (id, batch_spec_id)
Foreign-key constraints:
    "batch_spec_workspaces_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE DEFERRABLE
    "batch_spec_workspaces_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) DEFERRABLE
Referenced by:
    TABLE "batch_spec_workspace_execution_jobs" CONSTRAINT "batch_spec_workspace_execution_job_batch_spec_workspace_id_fkey" FOREIGN KEY (batch_spec_workspace_id) REFERENCES batch_spec_workspaces(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.batch_specs"
```
      Column       |           Type           | Collation | Nullable |                 Default                 
-------------------+--------------------------+-----------+----------+-----------------------------------------
 id                | bigint                   |           | not null | nextval('batch_specs_id_seq'::regclass)
 rand_id           | text                     |           | not null | 
 raw_spec          | text                     |           | not null | 
 spec              | jsonb                    |           | not null | '{}'::jsonb
 namespace_user_id | integer                  |           |          | 
 namespace_org_id  | integer                  |           |          | 
 user_id           | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 created_from_raw  | boolean                  |           | not null | false
 allow_unsupported | boolean                  |           | not null | false
 allow_ignored     | boolean                  |           | not null | false
 no_cache          | boolean                  |           | not null | false
 batch_change_id   | bigint                   |           |          | 
Indexes:
    "batch_specs_pkey" PRIMARY KEY, btree (id)
    "batch_specs_unique_rand_id" UNIQUE, btree (rand_id)
Check constraints:
    "batch_specs_has_1_namespace" CHECK ((namespace_user_id IS NULL) <> (namespace_org_id IS NULL))
Foreign-key constraints:
    "batch_specs_batch_change_id_fkey" FOREIGN KEY (batch_change_id) REFERENCES batch_changes(id) ON DELETE SET NULL DEFERRABLE
    "batch_specs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
Referenced by:
    TABLE "batch_changes" CONSTRAINT "batch_changes_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) DEFERRABLE
    TABLE "batch_spec_resolution_jobs" CONSTRAINT "batch_spec_resolution_jobs_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE DEFERRABLE
    TABLE "batch_spec_workspace_files" CONSTRAINT "batch_spec_workspace_files_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE
    TABLE "batch_spec_workspaces" CONSTRAINT "batch_spec_workspaces_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE DEFERRABLE
    TABLE "changeset_specs" CONSTRAINT "changeset_specs_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.cached_available_indexers"
```
       Column       |  Type   | Collation | Nullable |                        Default                        
--------------------+---------+-----------+----------+-------------------------------------------------------
 id                 | integer |           | not null | nextval('cached_available_indexers_id_seq'::regclass)
 repository_id      | integer |           | not null | 
 num_events         | integer |           | not null | 
 available_indexers | jsonb   |           | not null | 
Indexes:
    "cached_available_indexers_pkey" PRIMARY KEY, btree (id)
    "cached_available_indexers_repository_id" UNIQUE, btree (repository_id)
    "cached_available_indexers_num_events" btree (num_events DESC) WHERE available_indexers::text <> '{}'::text

```

# Table "public.changeset_events"
```
    Column    |           Type           | Collation | Nullable |                   Default                    
--------------+--------------------------+-----------+----------+----------------------------------------------
 id           | bigint                   |           | not null | nextval('changeset_events_id_seq'::regclass)
 changeset_id | bigint                   |           | not null | 
 kind         | text                     |           | not null | 
 key          | text                     |           | not null | 
 created_at   | timestamp with time zone |           | not null | now()
 metadata     | jsonb                    |           | not null | '{}'::jsonb
 updated_at   | timestamp with time zone |           | not null | now()
Indexes:
    "changeset_events_pkey" PRIMARY KEY, btree (id)
    "changeset_events_changeset_id_kind_key_unique" UNIQUE CONSTRAINT, btree (changeset_id, kind, key)
Check constraints:
    "changeset_events_key_check" CHECK (key <> ''::text)
    "changeset_events_kind_check" CHECK (kind <> ''::text)
    "changeset_events_metadata_check" CHECK (jsonb_typeof(metadata) = 'object'::text)
Foreign-key constraints:
    "changeset_events_changeset_id_fkey" FOREIGN KEY (changeset_id) REFERENCES changesets(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.changeset_jobs"
```
      Column       |           Type           | Collation | Nullable |                  Default                   
-------------------+--------------------------+-----------+----------+--------------------------------------------
 id                | bigint                   |           | not null | nextval('changeset_jobs_id_seq'::regclass)
 bulk_group        | text                     |           | not null | 
 user_id           | integer                  |           | not null | 
 batch_change_id   | integer                  |           | not null | 
 changeset_id      | integer                  |           | not null | 
 job_type          | text                     |           | not null | 
 payload           | jsonb                    |           |          | '{}'::jsonb
 state             | text                     |           | not null | 'queued'::text
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 execution_logs    | json[]                   |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 worker_hostname   | text                     |           | not null | ''::text
 last_heartbeat_at | timestamp with time zone |           |          | 
 queued_at         | timestamp with time zone |           |          | now()
 cancel            | boolean                  |           | not null | false
Indexes:
    "changeset_jobs_pkey" PRIMARY KEY, btree (id)
    "changeset_jobs_bulk_group_idx" btree (bulk_group)
    "changeset_jobs_state_idx" btree (state)
Check constraints:
    "changeset_jobs_payload_check" CHECK (jsonb_typeof(payload) = 'object'::text)
Foreign-key constraints:
    "changeset_jobs_batch_change_id_fkey" FOREIGN KEY (batch_change_id) REFERENCES batch_changes(id) ON DELETE CASCADE DEFERRABLE
    "changeset_jobs_changeset_id_fkey" FOREIGN KEY (changeset_id) REFERENCES changesets(id) ON DELETE CASCADE DEFERRABLE
    "changeset_jobs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.changeset_specs"
```
       Column        |           Type           | Collation | Nullable |                   Default                   
---------------------+--------------------------+-----------+----------+---------------------------------------------
 id                  | bigint                   |           | not null | nextval('changeset_specs_id_seq'::regclass)
 rand_id             | text                     |           | not null | 
 spec                | jsonb                    |           |          | '{}'::jsonb
 batch_spec_id       | bigint                   |           |          | 
 repo_id             | integer                  |           | not null | 
 user_id             | integer                  |           |          | 
 diff_stat_added     | integer                  |           |          | 
 diff_stat_deleted   | integer                  |           |          | 
 created_at          | timestamp with time zone |           | not null | now()
 updated_at          | timestamp with time zone |           | not null | now()
 head_ref            | text                     |           |          | 
 title               | text                     |           |          | 
 external_id         | text                     |           |          | 
 fork_namespace      | citext                   |           |          | 
 diff                | bytea                    |           |          | 
 base_rev            | text                     |           |          | 
 base_ref            | text                     |           |          | 
 body                | text                     |           |          | 
 published           | text                     |           |          | 
 commit_message      | text                     |           |          | 
 commit_author_name  | text                     |           |          | 
 commit_author_email | text                     |           |          | 
 type                | text                     |           | not null | 
Indexes:
    "changeset_specs_pkey" PRIMARY KEY, btree (id)
    "changeset_specs_unique_rand_id" UNIQUE, btree (rand_id)
    "changeset_specs_batch_spec_id" btree (batch_spec_id)
    "changeset_specs_created_at" btree (created_at)
    "changeset_specs_external_id" btree (external_id)
    "changeset_specs_head_ref" btree (head_ref)
    "changeset_specs_title" btree (title)
Check constraints:
    "changeset_specs_published_valid_values" CHECK (published = 'true'::text OR published = 'false'::text OR published = '"draft"'::text OR published IS NULL)
Foreign-key constraints:
    "changeset_specs_batch_spec_id_fkey" FOREIGN KEY (batch_spec_id) REFERENCES batch_specs(id) ON DELETE CASCADE DEFERRABLE
    "changeset_specs_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) DEFERRABLE
    "changeset_specs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
Referenced by:
    TABLE "changesets" CONSTRAINT "changesets_changeset_spec_id_fkey" FOREIGN KEY (current_spec_id) REFERENCES changeset_specs(id) DEFERRABLE
    TABLE "changesets" CONSTRAINT "changesets_previous_spec_id_fkey" FOREIGN KEY (previous_spec_id) REFERENCES changeset_specs(id) DEFERRABLE

```

# Table "public.changesets"
```
          Column          |                     Type                     | Collation | Nullable |                Default                 
--------------------------+----------------------------------------------+-----------+----------+----------------------------------------
 id                       | bigint                                       |           | not null | nextval('changesets_id_seq'::regclass)
 batch_change_ids         | jsonb                                        |           | not null | '{}'::jsonb
 repo_id                  | integer                                      |           | not null | 
 created_at               | timestamp with time zone                     |           | not null | now()
 updated_at               | timestamp with time zone                     |           | not null | now()
 metadata                 | jsonb                                        |           |          | '{}'::jsonb
 external_id              | text                                         |           |          | 
 external_service_type    | text                                         |           | not null | 
 external_deleted_at      | timestamp with time zone                     |           |          | 
 external_branch          | text                                         |           |          | 
 external_updated_at      | timestamp with time zone                     |           |          | 
 external_state           | text                                         |           |          | 
 external_review_state    | text                                         |           |          | 
 external_check_state     | text                                         |           |          | 
 diff_stat_added          | integer                                      |           |          | 
 diff_stat_deleted        | integer                                      |           |          | 
 sync_state               | jsonb                                        |           | not null | '{}'::jsonb
 current_spec_id          | bigint                                       |           |          | 
 previous_spec_id         | bigint                                       |           |          | 
 publication_state        | text                                         |           |          | 'UNPUBLISHED'::text
 owned_by_batch_change_id | bigint                                       |           |          | 
 reconciler_state         | text                                         |           |          | 'queued'::text
 failure_message          | text                                         |           |          | 
 started_at               | timestamp with time zone                     |           |          | 
 finished_at              | timestamp with time zone                     |           |          | 
 process_after            | timestamp with time zone                     |           |          | 
 num_resets               | integer                                      |           | not null | 0
 closing                  | boolean                                      |           | not null | false
 num_failures             | integer                                      |           | not null | 0
 log_contents             | text                                         |           |          | 
 execution_logs           | json[]                                       |           |          | 
 syncer_error             | text                                         |           |          | 
 external_title           | text                                         |           |          | 
 worker_hostname          | text                                         |           | not null | ''::text
 ui_publication_state     | batch_changes_changeset_ui_publication_state |           |          | 
 last_heartbeat_at        | timestamp with time zone                     |           |          | 
 external_fork_namespace  | citext                                       |           |          | 
 queued_at                | timestamp with time zone                     |           |          | now()
 cancel                   | boolean                                      |           | not null | false
 detached_at              | timestamp with time zone                     |           |          | 
 computed_state           | text                                         |           | not null | 
 external_fork_name       | citext                                       |           |          | 
 previous_failure_message | text                                         |           |          | 
 commit_verification      | jsonb                                        |           | not null | '{}'::jsonb
Indexes:
    "changesets_pkey" PRIMARY KEY, btree (id)
    "changesets_repo_external_id_unique" UNIQUE CONSTRAINT, btree (repo_id, external_id)
    "changesets_batch_change_ids" gin (batch_change_ids)
    "changesets_bitbucket_cloud_metadata_source_commit_idx" btree ((((metadata -> 'source'::text) -> 'commit'::text) ->> 'hash'::text))
    "changesets_changeset_specs" btree (current_spec_id, previous_spec_id)
    "changesets_computed_state" btree (computed_state)
    "changesets_detached_at" btree (detached_at)
    "changesets_external_state_idx" btree (external_state)
    "changesets_external_title_idx" btree (external_title)
    "changesets_publication_state_idx" btree (publication_state)
    "changesets_reconciler_state_idx" btree (reconciler_state)
Check constraints:
    "changesets_batch_change_ids_check" CHECK (jsonb_typeof(batch_change_ids) = 'object'::text)
    "changesets_external_id_check" CHECK (external_id <> ''::text)
    "changesets_external_service_type_not_blank" CHECK (external_service_type <> ''::text)
    "changesets_metadata_check" CHECK (jsonb_typeof(metadata) = 'object'::text)
    "external_branch_ref_prefix" CHECK (external_branch ~~ 'refs/heads/%'::text)
Foreign-key constraints:
    "changesets_changeset_spec_id_fkey" FOREIGN KEY (current_spec_id) REFERENCES changeset_specs(id) DEFERRABLE
    "changesets_owned_by_batch_spec_id_fkey" FOREIGN KEY (owned_by_batch_change_id) REFERENCES batch_changes(id) ON DELETE SET NULL DEFERRABLE
    "changesets_previous_spec_id_fkey" FOREIGN KEY (previous_spec_id) REFERENCES changeset_specs(id) DEFERRABLE
    "changesets_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE
Referenced by:
    TABLE "changeset_events" CONSTRAINT "changeset_events_changeset_id_fkey" FOREIGN KEY (changeset_id) REFERENCES changesets(id) ON DELETE CASCADE DEFERRABLE
    TABLE "changeset_jobs" CONSTRAINT "changeset_jobs_changeset_id_fkey" FOREIGN KEY (changeset_id) REFERENCES changesets(id) ON DELETE CASCADE DEFERRABLE
Triggers:
    changesets_update_computed_state BEFORE INSERT OR UPDATE ON changesets FOR EACH ROW EXECUTE FUNCTION changesets_computed_state_ensure()

```

**external_title**: Normalized property generated on save using Changeset.Title()

# Table "public.cm_action_jobs"
```
      Column       |           Type           | Collation | Nullable |                  Default                   
-------------------+--------------------------+-----------+----------+--------------------------------------------
 id                | integer                  |           | not null | nextval('cm_action_jobs_id_seq'::regclass)
 email             | bigint                   |           |          | 
 state             | text                     |           |          | 'queued'::text
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 log_contents      | text                     |           |          | 
 trigger_event     | integer                  |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 webhook           | bigint                   |           |          | 
 slack_webhook     | bigint                   |           |          | 
 queued_at         | timestamp with time zone |           |          | now()
 cancel            | boolean                  |           | not null | false
Indexes:
    "cm_action_jobs_pkey" PRIMARY KEY, btree (id)
    "cm_action_jobs_state_idx" btree (state)
    "cm_action_jobs_trigger_event" btree (trigger_event)
Check constraints:
    "cm_action_jobs_only_one_action_type" CHECK ((
CASE
    WHEN email IS NULL THEN 0
    ELSE 1
END +
CASE
    WHEN webhook IS NULL THEN 0
    ELSE 1
END +
CASE
    WHEN slack_webhook IS NULL THEN 0
    ELSE 1
END) = 1)
Foreign-key constraints:
    "cm_action_jobs_email_fk" FOREIGN KEY (email) REFERENCES cm_emails(id) ON DELETE CASCADE
    "cm_action_jobs_slack_webhook_fkey" FOREIGN KEY (slack_webhook) REFERENCES cm_slack_webhooks(id) ON DELETE CASCADE
    "cm_action_jobs_trigger_event_fk" FOREIGN KEY (trigger_event) REFERENCES cm_trigger_jobs(id) ON DELETE CASCADE
    "cm_action_jobs_webhook_fkey" FOREIGN KEY (webhook) REFERENCES cm_webhooks(id) ON DELETE CASCADE

```

**email**: The ID of the cm_emails action to execute if this is an email job. Mutually exclusive with webhook and slack_webhook

**slack_webhook**: The ID of the cm_slack_webhook action to execute if this is a slack webhook job. Mutually exclusive with email and webhook

**webhook**: The ID of the cm_webhooks action to execute if this is a webhook job. Mutually exclusive with email and slack_webhook

# Table "public.cm_emails"
```
     Column      |           Type           | Collation | Nullable |                Default                
-----------------+--------------------------+-----------+----------+---------------------------------------
 id              | bigint                   |           | not null | nextval('cm_emails_id_seq'::regclass)
 monitor         | bigint                   |           | not null | 
 enabled         | boolean                  |           | not null | 
 priority        | cm_email_priority        |           | not null | 
 header          | text                     |           | not null | 
 created_by      | integer                  |           | not null | 
 created_at      | timestamp with time zone |           | not null | now()
 changed_by      | integer                  |           | not null | 
 changed_at      | timestamp with time zone |           | not null | now()
 include_results | boolean                  |           | not null | false
Indexes:
    "cm_emails_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "cm_emails_changed_by_fk" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_emails_created_by_fk" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_emails_monitor" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
Referenced by:
    TABLE "cm_action_jobs" CONSTRAINT "cm_action_jobs_email_fk" FOREIGN KEY (email) REFERENCES cm_emails(id) ON DELETE CASCADE
    TABLE "cm_recipients" CONSTRAINT "cm_recipients_emails" FOREIGN KEY (email) REFERENCES cm_emails(id) ON DELETE CASCADE

```

# Table "public.cm_last_searched"
```
   Column    |  Type   | Collation | Nullable | Default 
-------------+---------+-----------+----------+---------
 monitor_id  | bigint  |           | not null | 
 commit_oids | text[]  |           | not null | 
 repo_id     | integer |           | not null | 
Indexes:
    "cm_last_searched_pkey" PRIMARY KEY, btree (monitor_id, repo_id)
Foreign-key constraints:
    "cm_last_searched_monitor_id_fkey" FOREIGN KEY (monitor_id) REFERENCES cm_monitors(id) ON DELETE CASCADE
    "cm_last_searched_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE

```

The last searched commit hashes for the given code monitor and unique set of search arguments

**commit_oids**: The set of commit OIDs that was previously successfully searched and should be excluded on the next run

# Table "public.cm_monitors"
```
      Column       |           Type           | Collation | Nullable |                 Default                 
-------------------+--------------------------+-----------+----------+-----------------------------------------
 id                | bigint                   |           | not null | nextval('cm_monitors_id_seq'::regclass)
 created_by        | integer                  |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
 description       | text                     |           | not null | 
 changed_at        | timestamp with time zone |           | not null | now()
 changed_by        | integer                  |           | not null | 
 enabled           | boolean                  |           | not null | true
 namespace_user_id | integer                  |           | not null | 
 namespace_org_id  | integer                  |           |          | 
Indexes:
    "cm_monitors_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "cm_monitors_changed_by_fk" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_monitors_created_by_fk" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_monitors_org_id_fk" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    "cm_monitors_user_id_fk" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
Referenced by:
    TABLE "cm_emails" CONSTRAINT "cm_emails_monitor" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
    TABLE "cm_last_searched" CONSTRAINT "cm_last_searched_monitor_id_fkey" FOREIGN KEY (monitor_id) REFERENCES cm_monitors(id) ON DELETE CASCADE
    TABLE "cm_slack_webhooks" CONSTRAINT "cm_slack_webhooks_monitor_fkey" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
    TABLE "cm_queries" CONSTRAINT "cm_triggers_monitor" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
    TABLE "cm_webhooks" CONSTRAINT "cm_webhooks_monitor_fkey" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE

```

**namespace_org_id**: DEPRECATED: code monitors cannot be owned by an org

# Table "public.cm_queries"
```
    Column     |           Type           | Collation | Nullable |                Default                 
---------------+--------------------------+-----------+----------+----------------------------------------
 id            | bigint                   |           | not null | nextval('cm_queries_id_seq'::regclass)
 monitor       | bigint                   |           | not null | 
 query         | text                     |           | not null | 
 created_by    | integer                  |           | not null | 
 created_at    | timestamp with time zone |           | not null | now()
 changed_by    | integer                  |           | not null | 
 changed_at    | timestamp with time zone |           | not null | now()
 next_run      | timestamp with time zone |           |          | now()
 latest_result | timestamp with time zone |           |          | 
Indexes:
    "cm_queries_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "cm_triggers_changed_by_fk" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_triggers_created_by_fk" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_triggers_monitor" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
Referenced by:
    TABLE "cm_trigger_jobs" CONSTRAINT "cm_trigger_jobs_query_fk" FOREIGN KEY (query) REFERENCES cm_queries(id) ON DELETE CASCADE

```

# Table "public.cm_recipients"
```
      Column       |  Type   | Collation | Nullable |                  Default                  
-------------------+---------+-----------+----------+-------------------------------------------
 id                | bigint  |           | not null | nextval('cm_recipients_id_seq'::regclass)
 email             | bigint  |           | not null | 
 namespace_user_id | integer |           |          | 
 namespace_org_id  | integer |           |          | 
Indexes:
    "cm_recipients_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "cm_recipients_emails" FOREIGN KEY (email) REFERENCES cm_emails(id) ON DELETE CASCADE
    "cm_recipients_org_id_fk" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    "cm_recipients_user_id_fk" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.cm_slack_webhooks"
```
     Column      |           Type           | Collation | Nullable |                    Default                    
-----------------+--------------------------+-----------+----------+-----------------------------------------------
 id              | bigint                   |           | not null | nextval('cm_slack_webhooks_id_seq'::regclass)
 monitor         | bigint                   |           | not null | 
 url             | text                     |           | not null | 
 enabled         | boolean                  |           | not null | 
 created_by      | integer                  |           | not null | 
 created_at      | timestamp with time zone |           | not null | now()
 changed_by      | integer                  |           | not null | 
 changed_at      | timestamp with time zone |           | not null | now()
 include_results | boolean                  |           | not null | false
Indexes:
    "cm_slack_webhooks_pkey" PRIMARY KEY, btree (id)
    "cm_slack_webhooks_monitor" btree (monitor)
Foreign-key constraints:
    "cm_slack_webhooks_changed_by_fkey" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_slack_webhooks_created_by_fkey" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_slack_webhooks_monitor_fkey" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
Referenced by:
    TABLE "cm_action_jobs" CONSTRAINT "cm_action_jobs_slack_webhook_fkey" FOREIGN KEY (slack_webhook) REFERENCES cm_slack_webhooks(id) ON DELETE CASCADE

```

Slack webhook actions configured on code monitors

**monitor**: The code monitor that the action is defined on

**url**: The Slack webhook URL we send the code monitor event to

# Table "public.cm_trigger_jobs"
```
      Column       |           Type           | Collation | Nullable |                   Default                   
-------------------+--------------------------+-----------+----------+---------------------------------------------
 id                | integer                  |           | not null | nextval('cm_trigger_jobs_id_seq'::regclass)
 query             | bigint                   |           | not null | 
 state             | text                     |           |          | 'queued'::text
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 log_contents      | text                     |           |          | 
 query_string      | text                     |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 search_results    | jsonb                    |           |          | 
 queued_at         | timestamp with time zone |           |          | now()
 cancel            | boolean                  |           | not null | false
 logs              | json[]                   |           |          | 
Indexes:
    "cm_trigger_jobs_pkey" PRIMARY KEY, btree (id)
    "cm_trigger_jobs_finished_at" btree (finished_at)
    "cm_trigger_jobs_state_idx" btree (state)
Check constraints:
    "search_results_is_array" CHECK (jsonb_typeof(search_results) = 'array'::text)
Foreign-key constraints:
    "cm_trigger_jobs_query_fk" FOREIGN KEY (query) REFERENCES cm_queries(id) ON DELETE CASCADE
Referenced by:
    TABLE "cm_action_jobs" CONSTRAINT "cm_action_jobs_trigger_event_fk" FOREIGN KEY (trigger_event) REFERENCES cm_trigger_jobs(id) ON DELETE CASCADE

```

# Table "public.cm_webhooks"
```
     Column      |           Type           | Collation | Nullable |                 Default                 
-----------------+--------------------------+-----------+----------+-----------------------------------------
 id              | bigint                   |           | not null | nextval('cm_webhooks_id_seq'::regclass)
 monitor         | bigint                   |           | not null | 
 url             | text                     |           | not null | 
 enabled         | boolean                  |           | not null | 
 created_by      | integer                  |           | not null | 
 created_at      | timestamp with time zone |           | not null | now()
 changed_by      | integer                  |           | not null | 
 changed_at      | timestamp with time zone |           | not null | now()
 include_results | boolean                  |           | not null | false
Indexes:
    "cm_webhooks_pkey" PRIMARY KEY, btree (id)
    "cm_webhooks_monitor" btree (monitor)
Foreign-key constraints:
    "cm_webhooks_changed_by_fkey" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_webhooks_created_by_fkey" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    "cm_webhooks_monitor_fkey" FOREIGN KEY (monitor) REFERENCES cm_monitors(id) ON DELETE CASCADE
Referenced by:
    TABLE "cm_action_jobs" CONSTRAINT "cm_action_jobs_webhook_fkey" FOREIGN KEY (webhook) REFERENCES cm_webhooks(id) ON DELETE CASCADE

```

Webhook actions configured on code monitors

**enabled**: Whether this Slack webhook action is enabled. When not enabled, the action will not be run when its code monitor generates events

**monitor**: The code monitor that the action is defined on

**url**: The webhook URL we send the code monitor event to

# Table "public.code_hosts"
```
             Column              |           Type           | Collation | Nullable |                Default                 
---------------------------------+--------------------------+-----------+----------+----------------------------------------
 id                              | integer                  |           | not null | nextval('code_hosts_id_seq'::regclass)
 kind                            | text                     |           | not null | 
 url                             | text                     |           | not null | 
 api_rate_limit_quota            | integer                  |           |          | 
 api_rate_limit_interval_seconds | integer                  |           |          | 
 git_rate_limit_quota            | integer                  |           |          | 
 git_rate_limit_interval_seconds | integer                  |           |          | 
 created_at                      | timestamp with time zone |           | not null | now()
 updated_at                      | timestamp with time zone |           | not null | now()
Indexes:
    "code_hosts_pkey" PRIMARY KEY, btree (id)
    "code_hosts_url_key" UNIQUE CONSTRAINT, btree (url)
Referenced by:
    TABLE "external_services" CONSTRAINT "external_services_code_host_id_fkey" FOREIGN KEY (code_host_id) REFERENCES code_hosts(id) ON UPDATE CASCADE ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED

```

# Table "public.codeintel_autoindex_queue"
```
    Column     |           Type           | Collation | Nullable |                        Default                        
---------------+--------------------------+-----------+----------+-------------------------------------------------------
 id            | integer                  |           | not null | nextval('codeintel_autoindex_queue_id_seq'::regclass)
 repository_id | integer                  |           | not null | 
 rev           | text                     |           | not null | 
 queued_at     | timestamp with time zone |           | not null | now()
 processed_at  | timestamp with time zone |           |          | 
Indexes:
    "codeintel_autoindex_queue_pkey" PRIMARY KEY, btree (id)
    "codeintel_autoindex_queue_repository_id_commit" UNIQUE, btree (repository_id, rev)

```

# Table "public.codeintel_autoindexing_exceptions"
```
       Column       |  Type   | Collation | Nullable |                            Default                            
--------------------+---------+-----------+----------+---------------------------------------------------------------
 id                 | integer |           | not null | nextval('codeintel_autoindexing_exceptions_id_seq'::regclass)
 repository_id      | integer |           | not null | 
 disable_scheduling | boolean |           | not null | false
 disable_inference  | boolean |           | not null | false
Indexes:
    "codeintel_autoindexing_exceptions_pkey" PRIMARY KEY, btree (id)
    "codeintel_autoindexing_exceptions_repository_id_key" UNIQUE CONSTRAINT, btree (repository_id)
Foreign-key constraints:
    "codeintel_autoindexing_exceptions_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE

```

# Table "public.codeintel_commit_dates"
```
    Column     |           Type           | Collation | Nullable | Default 
---------------+--------------------------+-----------+----------+---------
 repository_id | integer                  |           | not null | 
 commit_bytea  | bytea                    |           | not null | 
 committed_at  | timestamp with time zone |           |          | 
Indexes:
    "codeintel_commit_dates_pkey" PRIMARY KEY, btree (repository_id, commit_bytea)

```

Maps commits within a repository to the commit date as reported by gitserver.

**commit_bytea**: Identifies the 40-character commit hash.

**committed_at**: The commit date (may be -infinity if unresolvable).

**repository_id**: Identifies a row in the `repo` table.

# Table "public.codeintel_inference_scripts"
```
      Column      |           Type           | Collation | Nullable | Default 
------------------+--------------------------+-----------+----------+---------
 insert_timestamp | timestamp with time zone |           | not null | now()
 script           | text                     |           | not null | 

```

Contains auto-index job inference Lua scripts as an alternative to setting via environment variables.

# Table "public.codeintel_initial_path_ranks"
```
       Column       |  Type   | Collation | Nullable |                         Default                          
--------------------+---------+-----------+----------+----------------------------------------------------------
 id                 | bigint  |           | not null | nextval('codeintel_initial_path_ranks_id_seq'::regclass)
 document_path      | text    |           | not null | ''::text
 graph_key          | text    |           | not null | 
 document_paths     | text[]  |           | not null | '{}'::text[]
 exported_upload_id | integer |           | not null | 
Indexes:
    "codeintel_initial_path_ranks_pkey" PRIMARY KEY, btree (id)
    "codeintel_initial_path_ranks_exported_upload_id" btree (exported_upload_id)
    "codeintel_initial_path_ranks_graph_key_id" btree (graph_key, id)
Foreign-key constraints:
    "codeintel_initial_path_ranks_exported_upload_id_fkey" FOREIGN KEY (exported_upload_id) REFERENCES codeintel_ranking_exports(id) ON DELETE CASCADE
Referenced by:
    TABLE "codeintel_initial_path_ranks_processed" CONSTRAINT "fk_codeintel_initial_path_ranks" FOREIGN KEY (codeintel_initial_path_ranks_id) REFERENCES codeintel_initial_path_ranks(id) ON DELETE CASCADE

```

# Table "public.codeintel_initial_path_ranks_processed"
```
             Column              |  Type  | Collation | Nullable |                              Default                               
---------------------------------+--------+-----------+----------+--------------------------------------------------------------------
 id                              | bigint |           | not null | nextval('codeintel_initial_path_ranks_processed_id_seq'::regclass)
 graph_key                       | text   |           | not null | 
 codeintel_initial_path_ranks_id | bigint |           | not null | 
Indexes:
    "codeintel_initial_path_ranks_processed_pkey" PRIMARY KEY, btree (id)
    "codeintel_initial_path_ranks_processed_cgraph_key_codeintel_ini" UNIQUE, btree (graph_key, codeintel_initial_path_ranks_id)
    "codeintel_initial_path_ranks_processed_codeintel_initial_path_r" btree (codeintel_initial_path_ranks_id)
Foreign-key constraints:
    "fk_codeintel_initial_path_ranks" FOREIGN KEY (codeintel_initial_path_ranks_id) REFERENCES codeintel_initial_path_ranks(id) ON DELETE CASCADE

```

# Table "public.codeintel_langugage_support_requests"
```
   Column    |  Type   | Collation | Nullable |                             Default                              
-------------+---------+-----------+----------+------------------------------------------------------------------
 id          | integer |           | not null | nextval('codeintel_langugage_support_requests_id_seq'::regclass)
 user_id     | integer |           | not null | 
 language_id | text    |           | not null | 
Indexes:
    "codeintel_langugage_support_requests_user_id_language" UNIQUE, btree (user_id, language_id)

```

# Table "public.codeintel_path_ranks"
```
     Column      |           Type           | Collation | Nullable |                     Default                      
-----------------+--------------------------+-----------+----------+--------------------------------------------------
 repository_id   | integer                  |           | not null | 
 payload         | jsonb                    |           | not null | 
 updated_at      | timestamp with time zone |           | not null | now()
 graph_key       | text                     |           | not null | 
 num_paths       | integer                  |           |          | 
 refcount_logsum | double precision         |           |          | 
 id              | bigint                   |           | not null | nextval('codeintel_path_ranks_id_seq'::regclass)
Indexes:
    "codeintel_path_ranks_pkey" PRIMARY KEY, btree (id)
    "codeintel_path_ranks_graph_key_repository_id" UNIQUE, btree (graph_key, repository_id)
    "codeintel_path_ranks_graph_key" btree (graph_key, updated_at NULLS FIRST, id)
    "codeintel_path_ranks_repository_id_updated_at_id" btree (repository_id, updated_at NULLS FIRST, id)
Triggers:
    insert_codeintel_path_ranks_statistics BEFORE INSERT ON codeintel_path_ranks FOR EACH ROW EXECUTE FUNCTION update_codeintel_path_ranks_statistics_columns()
    update_codeintel_path_ranks_statistics BEFORE UPDATE ON codeintel_path_ranks FOR EACH ROW WHEN (new.* IS DISTINCT FROM old.*) EXECUTE FUNCTION update_codeintel_path_ranks_statistics_columns()
    update_codeintel_path_ranks_updated_at BEFORE UPDATE ON codeintel_path_ranks FOR EACH ROW WHEN (new.* IS DISTINCT FROM old.*) EXECUTE FUNCTION update_codeintel_path_ranks_updated_at_column()

```

# Table "public.codeintel_ranking_definitions"
```
       Column       |  Type   | Collation | Nullable |                          Default                          
--------------------+---------+-----------+----------+-----------------------------------------------------------
 id                 | bigint  |           | not null | nextval('codeintel_ranking_definitions_id_seq'::regclass)
 symbol_name        | text    |           | not null | 
 document_path      | text    |           | not null | 
 graph_key          | text    |           | not null | 
 exported_upload_id | integer |           | not null | 
 symbol_checksum    | bytea   |           | not null | '\x'::bytea
Indexes:
    "codeintel_ranking_definitions_pkey" PRIMARY KEY, btree (id)
    "codeintel_ranking_definitions_exported_upload_id" btree (exported_upload_id)
    "codeintel_ranking_definitions_graph_key_symbol_checksum_search" btree (graph_key, symbol_checksum, exported_upload_id, document_path)
Foreign-key constraints:
    "codeintel_ranking_definitions_exported_upload_id_fkey" FOREIGN KEY (exported_upload_id) REFERENCES codeintel_ranking_exports(id) ON DELETE CASCADE

```

# Table "public.codeintel_ranking_exports"
```
     Column      |           Type           | Collation | Nullable |                        Default                        
-----------------+--------------------------+-----------+----------+-------------------------------------------------------
 upload_id       | integer                  |           |          | 
 graph_key       | text                     |           | not null | 
 locked_at       | timestamp with time zone |           | not null | now()
 id              | integer                  |           | not null | nextval('codeintel_ranking_exports_id_seq'::regclass)
 last_scanned_at | timestamp with time zone |           |          | 
 deleted_at      | timestamp with time zone |           |          | 
 upload_key      | text                     |           |          | 
Indexes:
    "codeintel_ranking_exports_pkey" PRIMARY KEY, btree (id)
    "codeintel_ranking_exports_graph_key_upload_id" UNIQUE, btree (graph_key, upload_id)
    "codeintel_ranking_exports_graph_key_deleted_at_id" btree (graph_key, deleted_at DESC, id)
    "codeintel_ranking_exports_graph_key_last_scanned_at" btree (graph_key, last_scanned_at NULLS FIRST, id)
Foreign-key constraints:
    "codeintel_ranking_exports_upload_id_fkey" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE SET NULL
Referenced by:
    TABLE "codeintel_initial_path_ranks" CONSTRAINT "codeintel_initial_path_ranks_exported_upload_id_fkey" FOREIGN KEY (exported_upload_id) REFERENCES codeintel_ranking_exports(id) ON DELETE CASCADE
    TABLE "codeintel_ranking_definitions" CONSTRAINT "codeintel_ranking_definitions_exported_upload_id_fkey" FOREIGN KEY (exported_upload_id) REFERENCES codeintel_ranking_exports(id) ON DELETE CASCADE
    TABLE "codeintel_ranking_references" CONSTRAINT "codeintel_ranking_references_exported_upload_id_fkey" FOREIGN KEY (exported_upload_id) REFERENCES codeintel_ranking_exports(id) ON DELETE CASCADE

```

# Table "public.codeintel_ranking_graph_keys"
```
   Column   |           Type           | Collation | Nullable |                         Default                          
------------+--------------------------+-----------+----------+----------------------------------------------------------
 id         | integer                  |           | not null | nextval('codeintel_ranking_graph_keys_id_seq'::regclass)
 graph_key  | text                     |           | not null | 
 created_at | timestamp with time zone |           |          | now()
Indexes:
    "codeintel_ranking_graph_keys_pkey" PRIMARY KEY, btree (id)

```

# Table "public.codeintel_ranking_path_counts_inputs"
```
    Column     |  Type   | Collation | Nullable |                             Default                              
---------------+---------+-----------+----------+------------------------------------------------------------------
 id            | bigint  |           | not null | nextval('codeintel_ranking_path_counts_inputs_id_seq'::regclass)
 count         | integer |           | not null | 
 graph_key     | text    |           | not null | 
 processed     | boolean |           | not null | false
 definition_id | bigint  |           |          | 
Indexes:
    "codeintel_ranking_path_counts_inputs_pkey" PRIMARY KEY, btree (id)
    "codeintel_ranking_path_counts_inputs_graph_key_unique_definitio" UNIQUE, btree (graph_key, definition_id) WHERE NOT processed
    "codeintel_ranking_path_counts_inputs_graph_key_id" btree (graph_key, id)

```

# Table "public.codeintel_ranking_progress"
```
               Column               |           Type           | Collation | Nullable |                        Default                         
------------------------------------+--------------------------+-----------+----------+--------------------------------------------------------
 id                                 | bigint                   |           | not null | nextval('codeintel_ranking_progress_id_seq'::regclass)
 graph_key                          | text                     |           | not null | 
 mappers_started_at                 | timestamp with time zone |           | not null | 
 mapper_completed_at                | timestamp with time zone |           |          | 
 seed_mapper_completed_at           | timestamp with time zone |           |          | 
 reducer_started_at                 | timestamp with time zone |           |          | 
 reducer_completed_at               | timestamp with time zone |           |          | 
 num_path_records_total             | integer                  |           |          | 
 num_reference_records_total        | integer                  |           |          | 
 num_count_records_total            | integer                  |           |          | 
 num_path_records_processed         | integer                  |           |          | 
 num_reference_records_processed    | integer                  |           |          | 
 num_count_records_processed        | integer                  |           |          | 
 max_export_id                      | bigint                   |           | not null | 
 reference_cursor_export_deleted_at | timestamp with time zone |           |          | 
 reference_cursor_export_id         | integer                  |           |          | 
 path_cursor_deleted_export_at      | timestamp with time zone |           |          | 
 path_cursor_export_id              | integer                  |           |          | 
Indexes:
    "codeintel_ranking_progress_pkey" PRIMARY KEY, btree (id)
    "codeintel_ranking_progress_graph_key_key" UNIQUE CONSTRAINT, btree (graph_key)

```

# Table "public.codeintel_ranking_references"
```
       Column       |  Type   | Collation | Nullable |                         Default                          
--------------------+---------+-----------+----------+----------------------------------------------------------
 id                 | bigint  |           | not null | nextval('codeintel_ranking_references_id_seq'::regclass)
 symbol_names       | text[]  |           | not null | 
 graph_key          | text    |           | not null | 
 exported_upload_id | integer |           | not null | 
 symbol_checksums   | bytea[] |           | not null | '{}'::bytea[]
Indexes:
    "codeintel_ranking_references_pkey" PRIMARY KEY, btree (id)
    "codeintel_ranking_references_exported_upload_id" btree (exported_upload_id)
    "codeintel_ranking_references_graph_key_id" btree (graph_key, id)
Foreign-key constraints:
    "codeintel_ranking_references_exported_upload_id_fkey" FOREIGN KEY (exported_upload_id) REFERENCES codeintel_ranking_exports(id) ON DELETE CASCADE
Referenced by:
    TABLE "codeintel_ranking_references_processed" CONSTRAINT "fk_codeintel_ranking_reference" FOREIGN KEY (codeintel_ranking_reference_id) REFERENCES codeintel_ranking_references(id) ON DELETE CASCADE

```

References for a given upload proceduced by background job consuming SCIP indexes.

# Table "public.codeintel_ranking_references_processed"
```
             Column             |  Type   | Collation | Nullable |                              Default                               
--------------------------------+---------+-----------+----------+--------------------------------------------------------------------
 graph_key                      | text    |           | not null | 
 codeintel_ranking_reference_id | integer |           | not null | 
 id                             | bigint  |           | not null | nextval('codeintel_ranking_references_processed_id_seq'::regclass)
Indexes:
    "codeintel_ranking_references_processed_pkey" PRIMARY KEY, btree (id)
    "codeintel_ranking_references_processed_graph_key_codeintel_rank" UNIQUE, btree (graph_key, codeintel_ranking_reference_id)
    "codeintel_ranking_references_processed_reference_id" btree (codeintel_ranking_reference_id)
Foreign-key constraints:
    "fk_codeintel_ranking_reference" FOREIGN KEY (codeintel_ranking_reference_id) REFERENCES codeintel_ranking_references(id) ON DELETE CASCADE

```

# Table "public.codeowners"
```
     Column     |           Type           | Collation | Nullable |                Default                 
----------------+--------------------------+-----------+----------+----------------------------------------
 id             | integer                  |           | not null | nextval('codeowners_id_seq'::regclass)
 contents       | text                     |           | not null | 
 contents_proto | bytea                    |           | not null | 
 repo_id        | integer                  |           | not null | 
 created_at     | timestamp with time zone |           | not null | now()
 updated_at     | timestamp with time zone |           | not null | now()
Indexes:
    "codeowners_pkey" PRIMARY KEY, btree (id)
    "codeowners_repo_id_key" UNIQUE CONSTRAINT, btree (repo_id)
Foreign-key constraints:
    "codeowners_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE

```

# Table "public.codeowners_individual_stats"
```
         Column         |            Type             | Collation | Nullable | Default 
------------------------+-----------------------------+-----------+----------+---------
 file_path_id           | integer                     |           | not null | 
 owner_id               | integer                     |           | not null | 
 tree_owned_files_count | integer                     |           | not null | 
 updated_at             | timestamp without time zone |           | not null | 
Indexes:
    "codeowners_individual_stats_pkey" PRIMARY KEY, btree (file_path_id, owner_id)
Foreign-key constraints:
    "codeowners_individual_stats_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    "codeowners_individual_stats_owner_id_fkey" FOREIGN KEY (owner_id) REFERENCES codeowners_owners(id)

```

Data on how many files in given tree are owned by given owner.

As opposed to ownership-general `ownership_path_stats` table, the individual &lt;path x owner&gt; stats
are stored in CODEOWNERS-specific table `codeowners_individual_stats`. The reason for that is that
we are also indexing on owner_id which is CODEOWNERS-specific.

**tree_owned_files_count**: Total owned file count by given owner at given file tree.

**updated_at**: When the last background job updating counts run.

# Table "public.codeowners_owners"
```
  Column   |  Type   | Collation | Nullable |                    Default                    
-----------+---------+-----------+----------+-----------------------------------------------
 id        | integer |           | not null | nextval('codeowners_owners_id_seq'::regclass)
 reference | text    |           | not null | 
Indexes:
    "codeowners_owners_pkey" PRIMARY KEY, btree (id)
    "codeowners_owners_reference" btree (reference)
Referenced by:
    TABLE "codeowners_individual_stats" CONSTRAINT "codeowners_individual_stats_owner_id_fkey" FOREIGN KEY (owner_id) REFERENCES codeowners_owners(id)

```

Text reference in CODEOWNERS entry to use in codeowners_individual_stats. Reference is either email or handle without @ in front.

**reference**: We just keep the reference as opposed to splitting it to handle or email
since the distinction is not relevant for query, and this makes indexing way easier.

# Table "public.commit_authors"
```
 Column |  Type   | Collation | Nullable |                  Default                   
--------+---------+-----------+----------+--------------------------------------------
 id     | integer |           | not null | nextval('commit_authors_id_seq'::regclass)
 email  | text    |           | not null | 
 name   | text    |           | not null | 
Indexes:
    "commit_authors_pkey" PRIMARY KEY, btree (id)
    "commit_authors_email_name" UNIQUE, btree (email, name)
Referenced by:
    TABLE "own_aggregate_recent_contribution" CONSTRAINT "own_aggregate_recent_contribution_commit_author_id_fkey" FOREIGN KEY (commit_author_id) REFERENCES commit_authors(id)
    TABLE "own_signal_recent_contribution" CONSTRAINT "own_signal_recent_contribution_commit_author_id_fkey" FOREIGN KEY (commit_author_id) REFERENCES commit_authors(id)

```

# Table "public.configuration_policies_audit_logs"
```
       Column       |           Type           | Collation | Nullable |                          Default                           
--------------------+--------------------------+-----------+----------+------------------------------------------------------------
 log_timestamp      | timestamp with time zone |           |          | clock_timestamp()
 record_deleted_at  | timestamp with time zone |           |          | 
 policy_id          | integer                  |           | not null | 
 transition_columns | USER-DEFINED[]           |           |          | 
 sequence           | bigint                   |           | not null | nextval('configuration_policies_audit_logs_seq'::regclass)
 operation          | audit_log_operation      |           | not null | 
Indexes:
    "configuration_policies_audit_logs_policy_id" btree (policy_id)
    "configuration_policies_audit_logs_timestamp" brin (log_timestamp)

```

**log_timestamp**: Timestamp for this log entry.

**record_deleted_at**: Set once the upload this entry is associated with is deleted. Once NOW() - record_deleted_at is above a certain threshold, this log entry will be deleted.

**transition_columns**: Array of changes that occurred to the upload for this entry, in the form of {&#34;column&#34;=&gt;&#34;&lt;column name&gt;&#34;, &#34;old&#34;=&gt;&#34;&lt;previous value&gt;&#34;, &#34;new&#34;=&gt;&#34;&lt;new value&gt;&#34;}.

# Table "public.context_detection_embedding_jobs"
```
      Column       |           Type           | Collation | Nullable |                           Default                            
-------------------+--------------------------+-----------+----------+--------------------------------------------------------------
 id                | integer                  |           | not null | nextval('context_detection_embedding_jobs_id_seq'::regclass)
 state             | text                     |           |          | 'queued'::text
 failure_message   | text                     |           |          | 
 queued_at         | timestamp with time zone |           |          | now()
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 cancel            | boolean                  |           | not null | false
Indexes:
    "context_detection_embedding_jobs_pkey" PRIMARY KEY, btree (id)

```

# Table "public.critical_and_site_config"
```
      Column       |           Type           | Collation | Nullable |                       Default                        
-------------------+--------------------------+-----------+----------+------------------------------------------------------
 id                | integer                  |           | not null | nextval('critical_and_site_config_id_seq'::regclass)
 type              | critical_or_site         |           | not null | 
 contents          | text                     |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 author_user_id    | integer                  |           |          | 
 redacted_contents | text                     |           |          | 
Indexes:
    "critical_and_site_config_pkey" PRIMARY KEY, btree (id)
    "critical_and_site_config_unique" UNIQUE, btree (id, type)

```

**author_user_id**: A null value indicates that this config was most likely added by code on the start-up path, for example from the SITE_CONFIG_FILE unless the config itself was added before this column existed in which case it could also have been a user.

**redacted_contents**: This column stores the contents but redacts all secrets. The redacted form is a sha256 hash of the secret appended to the REDACTED string. This is used to generate diffs between two subsequent changes in a way that allows us to detect changes to any secrets while also ensuring that we do not leak it in the diff. A null value indicates that this config was added before this column was added or redacting the secrets during write failed so we skipped writing to this column instead of a hard failure.

# Table "public.discussion_comments"
```
     Column     |           Type           | Collation | Nullable |                     Default                     
----------------+--------------------------+-----------+----------+-------------------------------------------------
 id             | bigint                   |           | not null | nextval('discussion_comments_id_seq'::regclass)
 thread_id      | bigint                   |           | not null | 
 author_user_id | integer                  |           | not null | 
 contents       | text                     |           | not null | 
 created_at     | timestamp with time zone |           | not null | now()
 updated_at     | timestamp with time zone |           | not null | now()
 deleted_at     | timestamp with time zone |           |          | 
 reports        | text[]                   |           | not null | '{}'::text[]
Indexes:
    "discussion_comments_pkey" PRIMARY KEY, btree (id)
    "discussion_comments_author_user_id_idx" btree (author_user_id)
    "discussion_comments_reports_array_length_idx" btree (array_length(reports, 1))
    "discussion_comments_thread_id_idx" btree (thread_id)
Foreign-key constraints:
    "discussion_comments_author_user_id_fkey" FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT
    "discussion_comments_thread_id_fkey" FOREIGN KEY (thread_id) REFERENCES discussion_threads(id) ON DELETE CASCADE

```

# Table "public.discussion_mail_reply_tokens"
```
   Column   |           Type           | Collation | Nullable | Default 
------------+--------------------------+-----------+----------+---------
 token      | text                     |           | not null | 
 user_id    | integer                  |           | not null | 
 thread_id  | bigint                   |           | not null | 
 deleted_at | timestamp with time zone |           |          | 
Indexes:
    "discussion_mail_reply_tokens_pkey" PRIMARY KEY, btree (token)
    "discussion_mail_reply_tokens_user_id_thread_id_idx" btree (user_id, thread_id)
Foreign-key constraints:
    "discussion_mail_reply_tokens_thread_id_fkey" FOREIGN KEY (thread_id) REFERENCES discussion_threads(id) ON DELETE CASCADE
    "discussion_mail_reply_tokens_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT

```

# Table "public.discussion_threads"
```
     Column     |           Type           | Collation | Nullable |                    Default                     
----------------+--------------------------+-----------+----------+------------------------------------------------
 id             | bigint                   |           | not null | nextval('discussion_threads_id_seq'::regclass)
 author_user_id | integer                  |           | not null | 
 title          | text                     |           |          | 
 target_repo_id | bigint                   |           |          | 
 created_at     | timestamp with time zone |           | not null | now()
 archived_at    | timestamp with time zone |           |          | 
 updated_at     | timestamp with time zone |           | not null | now()
 deleted_at     | timestamp with time zone |           |          | 
Indexes:
    "discussion_threads_pkey" PRIMARY KEY, btree (id)
    "discussion_threads_author_user_id_idx" btree (author_user_id)
Foreign-key constraints:
    "discussion_threads_author_user_id_fkey" FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT
    "discussion_threads_target_repo_id_fk" FOREIGN KEY (target_repo_id) REFERENCES discussion_threads_target_repo(id) ON DELETE CASCADE
Referenced by:
    TABLE "discussion_comments" CONSTRAINT "discussion_comments_thread_id_fkey" FOREIGN KEY (thread_id) REFERENCES discussion_threads(id) ON DELETE CASCADE
    TABLE "discussion_mail_reply_tokens" CONSTRAINT "discussion_mail_reply_tokens_thread_id_fkey" FOREIGN KEY (thread_id) REFERENCES discussion_threads(id) ON DELETE CASCADE
    TABLE "discussion_threads_target_repo" CONSTRAINT "discussion_threads_target_repo_thread_id_fkey" FOREIGN KEY (thread_id) REFERENCES discussion_threads(id) ON DELETE CASCADE

```

# Table "public.discussion_threads_target_repo"
```
     Column      |  Type   | Collation | Nullable |                          Default                           
-----------------+---------+-----------+----------+------------------------------------------------------------
 id              | bigint  |           | not null | nextval('discussion_threads_target_repo_id_seq'::regclass)
 thread_id       | bigint  |           | not null | 
 repo_id         | integer |           | not null | 
 path            | text    |           |          | 
 branch          | text    |           |          | 
 revision        | text    |           |          | 
 start_line      | integer |           |          | 
 end_line        | integer |           |          | 
 start_character | integer |           |          | 
 end_character   | integer |           |          | 
 lines_before    | text    |           |          | 
 lines           | text    |           |          | 
 lines_after     | text    |           |          | 
Indexes:
    "discussion_threads_target_repo_pkey" PRIMARY KEY, btree (id)
    "discussion_threads_target_repo_repo_id_path_idx" btree (repo_id, path)
Foreign-key constraints:
    "discussion_threads_target_repo_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    "discussion_threads_target_repo_thread_id_fkey" FOREIGN KEY (thread_id) REFERENCES discussion_threads(id) ON DELETE CASCADE
Referenced by:
    TABLE "discussion_threads" CONSTRAINT "discussion_threads_target_repo_id_fk" FOREIGN KEY (target_repo_id) REFERENCES discussion_threads_target_repo(id) ON DELETE CASCADE

```

# Table "public.event_logs"
```
          Column          |           Type           | Collation | Nullable |                Default                 
--------------------------+--------------------------+-----------+----------+----------------------------------------
 id                       | bigint                   |           | not null | nextval('event_logs_id_seq'::regclass)
 name                     | text                     |           | not null | 
 url                      | text                     |           | not null | 
 user_id                  | integer                  |           | not null | 
 anonymous_user_id        | text                     |           | not null | 
 source                   | text                     |           | not null | 
 argument                 | jsonb                    |           | not null | 
 version                  | text                     |           | not null | 
 timestamp                | timestamp with time zone |           | not null | 
 feature_flags            | jsonb                    |           |          | 
 cohort_id                | date                     |           |          | 
 public_argument          | jsonb                    |           | not null | '{}'::jsonb
 first_source_url         | text                     |           |          | 
 last_source_url          | text                     |           |          | 
 referrer                 | text                     |           |          | 
 device_id                | text                     |           |          | 
 insert_id                | text                     |           |          | 
 billing_product_category | text                     |           |          | 
 billing_event_id         | text                     |           |          | 
 client                   | text                     |           |          | 
Indexes:
    "event_logs_pkey" PRIMARY KEY, btree (id)
    "event_logs_anonymous_user_id" btree (anonymous_user_id)
    "event_logs_name_timestamp" btree (name, "timestamp" DESC)
    "event_logs_source" btree (source)
    "event_logs_timestamp" btree ("timestamp")
    "event_logs_timestamp_at_utc" btree (date(timezone('UTC'::text, "timestamp")))
    "event_logs_user_id_name" btree (user_id, name)
    "event_logs_user_id_timestamp" btree (user_id, "timestamp")
Check constraints:
    "event_logs_check_has_user" CHECK (user_id = 0 AND anonymous_user_id <> ''::text OR user_id <> 0 AND anonymous_user_id = ''::text OR user_id <> 0 AND anonymous_user_id <> ''::text)
    "event_logs_check_name_not_empty" CHECK (name <> ''::text)
    "event_logs_check_source_not_empty" CHECK (source <> ''::text)
    "event_logs_check_version_not_empty" CHECK (version <> ''::text)

```

# Table "public.event_logs_export_allowlist"
```
   Column   |  Type   | Collation | Nullable |                         Default                         
------------+---------+-----------+----------+---------------------------------------------------------
 id         | integer |           | not null | nextval('event_logs_export_allowlist_id_seq'::regclass)
 event_name | text    |           | not null | 
Indexes:
    "event_logs_export_allowlist_pkey" PRIMARY KEY, btree (id)
    "event_logs_export_allowlist_event_name_idx" UNIQUE, btree (event_name)

```

An allowlist of events that are approved for export if the scraping job is enabled

**event_name**: Name of the event that corresponds to event_logs.name

# Table "public.event_logs_scrape_state"
```
   Column    |  Type   | Collation | Nullable |                       Default                       
-------------+---------+-----------+----------+-----------------------------------------------------
 id          | integer |           | not null | nextval('event_logs_scrape_state_id_seq'::regclass)
 bookmark_id | integer |           | not null | 
Indexes:
    "event_logs_scrape_state_pk" PRIMARY KEY, btree (id)

```

Contains state for the periodic telemetry job that scrapes events if enabled.

**bookmark_id**: Bookmarks the maximum most recent successful event_logs.id that was scraped

# Table "public.event_logs_scrape_state_own"
```
   Column    |  Type   | Collation | Nullable |                         Default                         
-------------+---------+-----------+----------+---------------------------------------------------------
 id          | integer |           | not null | nextval('event_logs_scrape_state_own_id_seq'::regclass)
 bookmark_id | integer |           | not null | 
 job_type    | integer |           | not null | 
Indexes:
    "event_logs_scrape_state_own_pk" PRIMARY KEY, btree (id)

```

Contains state for own jobs that scrape events if enabled.

**bookmark_id**: Bookmarks the maximum most recent successful event_logs.id that was scraped

# Table "public.executor_heartbeats"
```
      Column      |           Type           | Collation | Nullable |                     Default                     
------------------+--------------------------+-----------+----------+-------------------------------------------------
 id               | integer                  |           | not null | nextval('executor_heartbeats_id_seq'::regclass)
 hostname         | text                     |           | not null | 
 queue_name       | text                     |           |          | 
 os               | text                     |           | not null | 
 architecture     | text                     |           | not null | 
 docker_version   | text                     |           | not null | 
 executor_version | text                     |           | not null | 
 git_version      | text                     |           | not null | 
 ignite_version   | text                     |           | not null | 
 src_cli_version  | text                     |           | not null | 
 first_seen_at    | timestamp with time zone |           | not null | now()
 last_seen_at     | timestamp with time zone |           | not null | now()
 queue_names      | text[]                   |           |          | 
Indexes:
    "executor_heartbeats_pkey" PRIMARY KEY, btree (id)
    "executor_heartbeats_hostname_key" UNIQUE CONSTRAINT, btree (hostname)
Check constraints:
    "one_of_queue_name_queue_names" CHECK (queue_name IS NOT NULL AND queue_names IS NULL OR queue_names IS NOT NULL AND queue_name IS NULL)

```

Tracks the most recent activity of executors attached to this Sourcegraph instance.

**architecture**: The machine architure running the executor.

**docker_version**: The version of Docker used by the executor.

**executor_version**: The version of the executor.

**first_seen_at**: The first time a heartbeat from the executor was received.

**git_version**: The version of Git used by the executor.

**hostname**: The uniquely identifying name of the executor.

**ignite_version**: The version of Ignite used by the executor.

**last_seen_at**: The last time a heartbeat from the executor was received.

**os**: The operating system running the executor.

**queue_name**: The queue name that the executor polls for work.

**queue_names**: The list of queue names that the executor polls for work.

**src_cli_version**: The version of src-cli used by the executor.

# Table "public.executor_job_tokens"
```
    Column    |           Type           | Collation | Nullable |                     Default                     
--------------+--------------------------+-----------+----------+-------------------------------------------------
 id           | integer                  |           | not null | nextval('executor_job_tokens_id_seq'::regclass)
 value_sha256 | bytea                    |           | not null | 
 job_id       | bigint                   |           | not null | 
 queue        | text                     |           | not null | 
 repo_id      | bigint                   |           | not null | 
 created_at   | timestamp with time zone |           | not null | now()
 updated_at   | timestamp with time zone |           | not null | now()
Indexes:
    "executor_job_tokens_pkey" PRIMARY KEY, btree (id)
    "executor_job_tokens_job_id_queue_repo_id_key" UNIQUE CONSTRAINT, btree (job_id, queue, repo_id)
    "executor_job_tokens_value_sha256_key" UNIQUE CONSTRAINT, btree (value_sha256)

```

# Table "public.executor_secret_access_logs"
```
       Column       |           Type           | Collation | Nullable |                         Default                         
--------------------+--------------------------+-----------+----------+---------------------------------------------------------
 id                 | integer                  |           | not null | nextval('executor_secret_access_logs_id_seq'::regclass)
 executor_secret_id | integer                  |           | not null | 
 user_id            | integer                  |           |          | 
 created_at         | timestamp with time zone |           | not null | now()
 machine_user       | text                     |           | not null | ''::text
Indexes:
    "executor_secret_access_logs_pkey" PRIMARY KEY, btree (id)
Check constraints:
    "user_id_or_machine_user" CHECK (user_id IS NULL AND machine_user <> ''::text OR user_id IS NOT NULL AND machine_user = ''::text)
Foreign-key constraints:
    "executor_secret_access_logs_executor_secret_id_fkey" FOREIGN KEY (executor_secret_id) REFERENCES executor_secrets(id) ON DELETE CASCADE
    "executor_secret_access_logs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.executor_secrets"
```
      Column       |           Type           | Collation | Nullable |                   Default                    
-------------------+--------------------------+-----------+----------+----------------------------------------------
 id                | integer                  |           | not null | nextval('executor_secrets_id_seq'::regclass)
 key               | text                     |           | not null | 
 value             | bytea                    |           | not null | 
 scope             | text                     |           | not null | 
 encryption_key_id | text                     |           |          | 
 namespace_user_id | integer                  |           |          | 
 namespace_org_id  | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 creator_id        | integer                  |           |          | 
Indexes:
    "executor_secrets_pkey" PRIMARY KEY, btree (id)
    "executor_secrets_unique_key_global" UNIQUE, btree (key, scope) WHERE namespace_user_id IS NULL AND namespace_org_id IS NULL
    "executor_secrets_unique_key_namespace_org" UNIQUE, btree (key, namespace_org_id, scope) WHERE namespace_org_id IS NOT NULL
    "executor_secrets_unique_key_namespace_user" UNIQUE, btree (key, namespace_user_id, scope) WHERE namespace_user_id IS NOT NULL
Foreign-key constraints:
    "executor_secrets_creator_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL
    "executor_secrets_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    "executor_secrets_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
Referenced by:
    TABLE "executor_secret_access_logs" CONSTRAINT "executor_secret_access_logs_executor_secret_id_fkey" FOREIGN KEY (executor_secret_id) REFERENCES executor_secrets(id) ON DELETE CASCADE

```

**creator_id**: NULL, if the user has been deleted.

# Table "public.exhaustive_search_jobs"
```
      Column       |           Type           | Collation | Nullable |                      Default                       
-------------------+--------------------------+-----------+----------+----------------------------------------------------
 id                | integer                  |           | not null | nextval('exhaustive_search_jobs_id_seq'::regclass)
 state             | text                     |           |          | 'queued'::text
 initiator_id      | integer                  |           | not null | 
 query             | text                     |           | not null | 
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 cancel            | boolean                  |           | not null | false
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 queued_at         | timestamp with time zone |           |          | now()
Indexes:
    "exhaustive_search_jobs_pkey" PRIMARY KEY, btree (id)
    "exhaustive_search_jobs_state" btree (state)
Foreign-key constraints:
    "exhaustive_search_jobs_initiator_id_fkey" FOREIGN KEY (initiator_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE
Referenced by:
    TABLE "exhaustive_search_repo_jobs" CONSTRAINT "exhaustive_search_repo_jobs_search_job_id_fkey" FOREIGN KEY (search_job_id) REFERENCES exhaustive_search_jobs(id) ON DELETE CASCADE

```

# Table "public.exhaustive_search_repo_jobs"
```
      Column       |           Type           | Collation | Nullable |                         Default                         
-------------------+--------------------------+-----------+----------+---------------------------------------------------------
 id                | integer                  |           | not null | nextval('exhaustive_search_repo_jobs_id_seq'::regclass)
 state             | text                     |           |          | 'queued'::text
 repo_id           | integer                  |           | not null | 
 ref_spec          | text                     |           | not null | 
 search_job_id     | integer                  |           | not null | 
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 cancel            | boolean                  |           | not null | false
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 queued_at         | timestamp with time zone |           |          | now()
Indexes:
    "exhaustive_search_repo_jobs_pkey" PRIMARY KEY, btree (id)
    "exhaustive_search_repo_jobs_state" btree (state)
Foreign-key constraints:
    "exhaustive_search_repo_jobs_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    "exhaustive_search_repo_jobs_search_job_id_fkey" FOREIGN KEY (search_job_id) REFERENCES exhaustive_search_jobs(id) ON DELETE CASCADE
Referenced by:
    TABLE "exhaustive_search_repo_revision_jobs" CONSTRAINT "exhaustive_search_repo_revision_jobs_search_repo_job_id_fkey" FOREIGN KEY (search_repo_job_id) REFERENCES exhaustive_search_repo_jobs(id) ON DELETE CASCADE

```

# Table "public.exhaustive_search_repo_revision_jobs"
```
       Column       |           Type           | Collation | Nullable |                             Default                              
--------------------+--------------------------+-----------+----------+------------------------------------------------------------------
 id                 | integer                  |           | not null | nextval('exhaustive_search_repo_revision_jobs_id_seq'::regclass)
 state              | text                     |           |          | 'queued'::text
 search_repo_job_id | integer                  |           | not null | 
 revision           | text                     |           | not null | 
 failure_message    | text                     |           |          | 
 started_at         | timestamp with time zone |           |          | 
 finished_at        | timestamp with time zone |           |          | 
 process_after      | timestamp with time zone |           |          | 
 num_resets         | integer                  |           | not null | 0
 num_failures       | integer                  |           | not null | 0
 last_heartbeat_at  | timestamp with time zone |           |          | 
 execution_logs     | json[]                   |           |          | 
 worker_hostname    | text                     |           | not null | ''::text
 cancel             | boolean                  |           | not null | false
 created_at         | timestamp with time zone |           | not null | now()
 updated_at         | timestamp with time zone |           | not null | now()
 queued_at          | timestamp with time zone |           |          | now()
Indexes:
    "exhaustive_search_repo_revision_jobs_pkey" PRIMARY KEY, btree (id)
    "exhaustive_search_repo_revision_jobs_state" btree (state)
Foreign-key constraints:
    "exhaustive_search_repo_revision_jobs_search_repo_job_id_fkey" FOREIGN KEY (search_repo_job_id) REFERENCES exhaustive_search_repo_jobs(id) ON DELETE CASCADE

```

# Table "public.explicit_permissions_bitbucket_projects_jobs"
```
       Column        |           Type           | Collation | Nullable |                                 Default                                  
---------------------+--------------------------+-----------+----------+--------------------------------------------------------------------------
 id                  | integer                  |           | not null | nextval('explicit_permissions_bitbucket_projects_jobs_id_seq'::regclass)
 state               | text                     |           |          | 'queued'::text
 failure_message     | text                     |           |          | 
 queued_at           | timestamp with time zone |           |          | now()
 started_at          | timestamp with time zone |           |          | 
 finished_at         | timestamp with time zone |           |          | 
 process_after       | timestamp with time zone |           |          | 
 num_resets          | integer                  |           | not null | 0
 num_failures        | integer                  |           | not null | 0
 last_heartbeat_at   | timestamp with time zone |           |          | 
 execution_logs      | json[]                   |           |          | 
 worker_hostname     | text                     |           | not null | ''::text
 project_key         | text                     |           | not null | 
 external_service_id | integer                  |           | not null | 
 permissions         | json[]                   |           |          | 
 unrestricted        | boolean                  |           | not null | false
 cancel              | boolean                  |           | not null | false
Indexes:
    "explicit_permissions_bitbucket_projects_jobs_pkey" PRIMARY KEY, btree (id)
    "explicit_permissions_bitbucket_projects_jobs_project_key_extern" btree (project_key, external_service_id, state)
    "explicit_permissions_bitbucket_projects_jobs_queued_at_idx" btree (queued_at)
    "explicit_permissions_bitbucket_projects_jobs_state_idx" btree (state)
Check constraints:
    "explicit_permissions_bitbucket_projects_jobs_check" CHECK (permissions IS NOT NULL AND unrestricted IS FALSE OR permissions IS NULL AND unrestricted IS TRUE)

```

# Table "public.external_service_repos"
```
       Column        |           Type           | Collation | Nullable |         Default         
---------------------+--------------------------+-----------+----------+-------------------------
 external_service_id | bigint                   |           | not null | 
 repo_id             | integer                  |           | not null | 
 clone_url           | text                     |           | not null | 
 created_at          | timestamp with time zone |           | not null | transaction_timestamp()
Indexes:
    "external_service_repos_repo_id_external_service_id_unique" UNIQUE CONSTRAINT, btree (repo_id, external_service_id)
    "external_service_repos_clone_url_idx" btree (clone_url)
    "external_service_repos_idx" btree (external_service_id, repo_id)
Foreign-key constraints:
    "external_service_repos_external_service_id_fkey" FOREIGN KEY (external_service_id) REFERENCES external_services(id) ON DELETE CASCADE DEFERRABLE
    "external_service_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.external_service_sync_jobs"
```
       Column        |           Type           | Collation | Nullable |                        Default                         
---------------------+--------------------------+-----------+----------+--------------------------------------------------------
 id                  | integer                  |           | not null | nextval('external_service_sync_jobs_id_seq'::regclass)
 state               | text                     |           | not null | 'queued'::text
 failure_message     | text                     |           |          | 
 started_at          | timestamp with time zone |           |          | 
 finished_at         | timestamp with time zone |           |          | 
 process_after       | timestamp with time zone |           |          | 
 num_resets          | integer                  |           | not null | 0
 external_service_id | bigint                   |           | not null | 
 num_failures        | integer                  |           | not null | 0
 log_contents        | text                     |           |          | 
 execution_logs      | json[]                   |           |          | 
 worker_hostname     | text                     |           | not null | ''::text
 last_heartbeat_at   | timestamp with time zone |           |          | 
 queued_at           | timestamp with time zone |           |          | now()
 cancel              | boolean                  |           | not null | false
 repos_synced        | integer                  |           | not null | 0
 repo_sync_errors    | integer                  |           | not null | 0
 repos_added         | integer                  |           | not null | 0
 repos_deleted       | integer                  |           | not null | 0
 repos_modified      | integer                  |           | not null | 0
 repos_unmodified    | integer                  |           | not null | 0
Indexes:
    "external_service_sync_jobs_state_external_service_id" btree (state, external_service_id) INCLUDE (finished_at)
Foreign-key constraints:
    "external_services_id_fk" FOREIGN KEY (external_service_id) REFERENCES external_services(id) ON DELETE CASCADE

```

**repo_sync_errors**: The number of times an error occurred syncing a repo during this sync job.

**repos_added**: The number of new repos discovered during this sync job.

**repos_deleted**: The number of repos deleted as a result of this sync job.

**repos_modified**: The number of existing repos whose metadata has changed during this sync job.

**repos_synced**: The number of repos synced during this sync job.

**repos_unmodified**: The number of existing repos whose metadata did not change during this sync job.

# Table "public.external_services"
```
      Column       |           Type           | Collation | Nullable |                    Default                    
-------------------+--------------------------+-----------+----------+-----------------------------------------------
 id                | bigint                   |           | not null | nextval('external_services_id_seq'::regclass)
 kind              | text                     |           | not null | 
 display_name      | text                     |           | not null | 
 config            | text                     |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 deleted_at        | timestamp with time zone |           |          | 
 last_sync_at      | timestamp with time zone |           |          | 
 next_sync_at      | timestamp with time zone |           |          | 
 unrestricted      | boolean                  |           | not null | false
 cloud_default     | boolean                  |           | not null | false
 encryption_key_id | text                     |           | not null | ''::text
 has_webhooks      | boolean                  |           |          | 
 token_expires_at  | timestamp with time zone |           |          | 
 code_host_id      | integer                  |           |          | 
 creator_id        | integer                  |           |          | 
 last_updater_id   | integer                  |           |          | 
Indexes:
    "external_services_pkey" PRIMARY KEY, btree (id)
    "kind_cloud_default" UNIQUE, btree (kind, cloud_default) WHERE cloud_default = true AND deleted_at IS NULL
    "external_services_has_webhooks_idx" btree (has_webhooks)
Check constraints:
    "check_non_empty_config" CHECK (btrim(config) <> ''::text)
Foreign-key constraints:
    "external_services_code_host_id_fkey" FOREIGN KEY (code_host_id) REFERENCES code_hosts(id) ON UPDATE CASCADE ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED
    "external_services_creator_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    "external_services_last_updater_id_fkey" FOREIGN KEY (last_updater_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
Referenced by:
    TABLE "external_service_repos" CONSTRAINT "external_service_repos_external_service_id_fkey" FOREIGN KEY (external_service_id) REFERENCES external_services(id) ON DELETE CASCADE DEFERRABLE
    TABLE "external_service_sync_jobs" CONSTRAINT "external_services_id_fk" FOREIGN KEY (external_service_id) REFERENCES external_services(id) ON DELETE CASCADE
    TABLE "webhook_logs" CONSTRAINT "webhook_logs_external_service_id_fkey" FOREIGN KEY (external_service_id) REFERENCES external_services(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.feature_flag_overrides"
```
      Column       |           Type           | Collation | Nullable | Default 
-------------------+--------------------------+-----------+----------+---------
 namespace_org_id  | integer                  |           |          | 
 namespace_user_id | integer                  |           |          | 
 flag_name         | text                     |           | not null | 
 flag_value        | boolean                  |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 deleted_at        | timestamp with time zone |           |          | 
Indexes:
    "feature_flag_overrides_unique_org_flag" UNIQUE CONSTRAINT, btree (namespace_org_id, flag_name)
    "feature_flag_overrides_unique_user_flag" UNIQUE CONSTRAINT, btree (namespace_user_id, flag_name)
    "feature_flag_overrides_org_id" btree (namespace_org_id) WHERE namespace_org_id IS NOT NULL
    "feature_flag_overrides_user_id" btree (namespace_user_id) WHERE namespace_user_id IS NOT NULL
Check constraints:
    "feature_flag_overrides_has_org_or_user_id" CHECK (namespace_org_id IS NOT NULL OR namespace_user_id IS NOT NULL)
Foreign-key constraints:
    "feature_flag_overrides_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    "feature_flag_overrides_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.feature_flags"
```
   Column   |           Type           | Collation | Nullable | Default 
------------+--------------------------+-----------+----------+---------
 flag_name  | text                     |           | not null | 
 flag_type  | feature_flag_type        |           | not null | 
 bool_value | boolean                  |           |          | 
 rollout    | integer                  |           |          | 
 created_at | timestamp with time zone |           | not null | now()
 updated_at | timestamp with time zone |           | not null | now()
 deleted_at | timestamp with time zone |           |          | 
Indexes:
    "feature_flags_pkey" PRIMARY KEY, btree (flag_name)
Check constraints:
    "feature_flags_rollout_check" CHECK (rollout >= 0 AND rollout <= 10000)
    "required_bool_fields" CHECK (1 =
CASE
    WHEN flag_type = 'bool'::feature_flag_type AND bool_value IS NULL THEN 0
    WHEN flag_type <> 'bool'::feature_flag_type AND bool_value IS NOT NULL THEN 0
    ELSE 1
END)
    "required_rollout_fields" CHECK (1 =
CASE
    WHEN flag_type = 'rollout'::feature_flag_type AND rollout IS NULL THEN 0
    WHEN flag_type <> 'rollout'::feature_flag_type AND rollout IS NOT NULL THEN 0
    ELSE 1
END)

```

**bool_value**: Bool value only defined when flag_type is bool

**rollout**: Rollout only defined when flag_type is rollout. Increments of 0.01%

# Table "public.github_app_installs"
```
       Column       |           Type           | Collation | Nullable |                     Default                     
--------------------+--------------------------+-----------+----------+-------------------------------------------------
 id                 | integer                  |           | not null | nextval('github_app_installs_id_seq'::regclass)
 app_id             | integer                  |           | not null | 
 installation_id    | integer                  |           | not null | 
 created_at         | timestamp with time zone |           | not null | now()
 url                | text                     |           |          | 
 account_login      | text                     |           |          | 
 account_avatar_url | text                     |           |          | 
 account_url        | text                     |           |          | 
 account_type       | text                     |           |          | 
 updated_at         | timestamp with time zone |           | not null | now()
Indexes:
    "github_app_installs_pkey" PRIMARY KEY, btree (id)
    "unique_app_install" UNIQUE CONSTRAINT, btree (app_id, installation_id)
    "app_id_idx" btree (app_id)
    "github_app_installs_account_login" btree (account_login)
    "installation_id_idx" btree (installation_id)
Foreign-key constraints:
    "github_app_installs_app_id_fkey" FOREIGN KEY (app_id) REFERENCES github_apps(id) ON DELETE CASCADE

```

# Table "public.github_apps"
```
      Column       |           Type           | Collation | Nullable |                 Default                 
-------------------+--------------------------+-----------+----------+-----------------------------------------
 id                | integer                  |           | not null | nextval('github_apps_id_seq'::regclass)
 app_id            | integer                  |           | not null | 
 name              | text                     |           | not null | 
 slug              | text                     |           | not null | 
 base_url          | text                     |           | not null | 
 client_id         | text                     |           | not null | 
 client_secret     | text                     |           | not null | 
 private_key       | text                     |           | not null | 
 encryption_key_id | text                     |           | not null | 
 logo              | text                     |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 app_url           | text                     |           | not null | ''::text
 webhook_id        | integer                  |           |          | 
 domain            | text                     |           | not null | 'repos'::text
 kind              | character varying(255)   |           | not null | 
Indexes:
    "github_apps_pkey" PRIMARY KEY, btree (id)
    "github_apps_app_id_slug_base_url_unique" UNIQUE, btree (app_id, slug, base_url)
Foreign-key constraints:
    "github_apps_webhook_id_fkey" FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE SET NULL
Referenced by:
    TABLE "batch_changes_site_credentials" CONSTRAINT "batch_changes_site_credentials_github_app_id_fkey" FOREIGN KEY (github_app_id) REFERENCES github_apps(id)
    TABLE "github_app_installs" CONSTRAINT "github_app_installs_app_id_fkey" FOREIGN KEY (app_id) REFERENCES github_apps(id) ON DELETE CASCADE
    TABLE "user_credentials" CONSTRAINT "user_credentials_github_app_id_fkey" FOREIGN KEY (github_app_id) REFERENCES github_apps(id)

```

# Table "public.gitserver_relocator_jobs"
```
      Column       |           Type           | Collation | Nullable |                       Default                        
-------------------+--------------------------+-----------+----------+------------------------------------------------------
 id                | integer                  |           | not null | nextval('gitserver_relocator_jobs_id_seq'::regclass)
 state             | text                     |           |          | 'queued'::text
 queued_at         | timestamp with time zone |           |          | now()
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 repo_id           | integer                  |           | not null | 
 source_hostname   | text                     |           | not null | 
 dest_hostname     | text                     |           | not null | 
 delete_source     | boolean                  |           | not null | false
 cancel            | boolean                  |           | not null | false
Indexes:
    "gitserver_relocator_jobs_pkey" PRIMARY KEY, btree (id)
    "gitserver_relocator_jobs_state" btree (state)

```

# Table "public.gitserver_repos"
```
      Column      |           Type           | Collation | Nullable |      Default       
------------------+--------------------------+-----------+----------+--------------------
 repo_id          | integer                  |           | not null | 
 clone_status     | text                     |           | not null | 'not_cloned'::text
 shard_id         | text                     |           | not null | 
 last_error       | text                     |           |          | 
 updated_at       | timestamp with time zone |           | not null | now()
 last_fetched     | timestamp with time zone |           | not null | now()
 last_changed     | timestamp with time zone |           | not null | now()
 repo_size_bytes  | bigint                   |           |          | 
 corrupted_at     | timestamp with time zone |           |          | 
 corruption_logs  | jsonb                    |           | not null | '[]'::jsonb
 cloning_progress | text                     |           |          | ''::text
Indexes:
    "gitserver_repos_pkey" PRIMARY KEY, btree (repo_id)
    "gitserver_repo_size_bytes" btree (repo_size_bytes)
    "gitserver_repos_cloned_status_idx" btree (repo_id) WHERE clone_status = 'cloned'::text
    "gitserver_repos_cloning_status_idx" btree (repo_id) WHERE clone_status = 'cloning'::text
    "gitserver_repos_last_changed_idx" btree (last_changed, repo_id)
    "gitserver_repos_last_error_idx" btree (repo_id) WHERE last_error IS NOT NULL
    "gitserver_repos_not_cloned_status_idx" btree (repo_id) WHERE clone_status = 'not_cloned'::text
    "gitserver_repos_not_explicitly_cloned_idx" btree (repo_id) WHERE clone_status <> 'cloned'::text
    "gitserver_repos_shard_id" btree (shard_id, repo_id)
Foreign-key constraints:
    "gitserver_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
Triggers:
    trig_recalc_gitserver_repos_statistics_on_delete AFTER DELETE ON gitserver_repos REFERENCING OLD TABLE AS oldtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_gitserver_repos_statistics_on_delete()
    trig_recalc_gitserver_repos_statistics_on_insert AFTER INSERT ON gitserver_repos REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_gitserver_repos_statistics_on_insert()
    trig_recalc_gitserver_repos_statistics_on_update AFTER UPDATE ON gitserver_repos REFERENCING OLD TABLE AS oldtab NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_gitserver_repos_statistics_on_update()

```

**corrupted_at**: Timestamp of when repo corruption was detected

**corruption_logs**: Log output of repo corruptions that have been detected - encoded as json

# Table "public.gitserver_repos_statistics"
```
    Column    |  Type  | Collation | Nullable | Default 
--------------+--------+-----------+----------+---------
 shard_id     | text   |           |          | 
 total        | bigint |           | not null | 0
 not_cloned   | bigint |           | not null | 0
 cloning      | bigint |           | not null | 0
 cloned       | bigint |           | not null | 0
 failed_fetch | bigint |           | not null | 0
 corrupted    | bigint |           | not null | 0
Indexes:
    "gitserver_repos_statistics_shard_id" btree (shard_id)

```

**cloned**: Number of repositories in gitserver_repos table on this shard that are cloned

**cloning**: Number of repositories in gitserver_repos table on this shard that cloning

**corrupted**: Number of repositories that are NOT soft-deleted and not blocked and have corrupted_at set in gitserver_repos table

**failed_fetch**: Number of repositories in gitserver_repos table on this shard where last_error is set

**not_cloned**: Number of repositories in gitserver_repos table on this shard that are not cloned yet

**shard_id**: ID of this gitserver shard. If an empty string then the repositories havent been assigned a shard.

**total**: Number of repositories in gitserver_repos table on this shard

# Table "public.gitserver_repos_sync_output"
```
   Column    |           Type           | Collation | Nullable | Default  
-------------+--------------------------+-----------+----------+----------
 repo_id     | integer                  |           | not null | 
 last_output | text                     |           | not null | ''::text
 updated_at  | timestamp with time zone |           | not null | now()
Indexes:
    "gitserver_repos_sync_output_pkey" PRIMARY KEY, btree (repo_id)
Foreign-key constraints:
    "gitserver_repos_sync_output_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE

```

Contains the most recent output from gitserver repository sync jobs.

# Table "public.global_state"
```
   Column    |  Type   | Collation | Nullable | Default 
-------------+---------+-----------+----------+---------
 site_id     | uuid    |           | not null | 
 initialized | boolean |           | not null | false
Indexes:
    "global_state_pkey" PRIMARY KEY, btree (site_id)

```

# Table "public.insights_query_runner_jobs"
```
      Column       |           Type           | Collation | Nullable |                        Default                         
-------------------+--------------------------+-----------+----------+--------------------------------------------------------
 id                | integer                  |           | not null | nextval('insights_query_runner_jobs_id_seq'::regclass)
 series_id         | text                     |           | not null | 
 search_query      | text                     |           | not null | 
 state             | text                     |           |          | 'queued'::text
 failure_message   | text                     |           |          | 
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 execution_logs    | json[]                   |           |          | 
 record_time       | timestamp with time zone |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 last_heartbeat_at | timestamp with time zone |           |          | 
 priority          | integer                  |           | not null | 1
 cost              | integer                  |           | not null | 500
 persist_mode      | persistmode              |           | not null | 'record'::persistmode
 queued_at         | timestamp with time zone |           |          | now()
 cancel            | boolean                  |           | not null | false
 trace_id          | text                     |           |          | 
Indexes:
    "insights_query_runner_jobs_pkey" PRIMARY KEY, btree (id)
    "finished_at_insights_query_runner_jobs_idx" btree (finished_at)
    "insights_query_runner_jobs_cost_idx" btree (cost)
    "insights_query_runner_jobs_priority_idx" btree (priority)
    "insights_query_runner_jobs_processable_priority_id" btree (priority, id) WHERE state = 'queued'::text OR state = 'errored'::text
    "insights_query_runner_jobs_series_id_state" btree (series_id, state)
    "insights_query_runner_jobs_state_btree" btree (state)
    "process_after_insights_query_runner_jobs_idx" btree (process_after)
Referenced by:
    TABLE "insights_query_runner_jobs_dependencies" CONSTRAINT "insights_query_runner_jobs_dependencies_fk_job_id" FOREIGN KEY (job_id) REFERENCES insights_query_runner_jobs(id) ON DELETE CASCADE

```

See [internal/insights/background/queryrunner/worker.go:Job](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:internal/insights/background/queryrunner/worker.go+type+Job&amp;patternType=literal)

**cost**: Integer representing a cost approximation of executing this search query.

**persist_mode**: The persistence level for this query. This value will determine the lifecycle of the resulting value.

**priority**: Integer representing a category of priority for this query. Priority in this context is ambiguously defined for consumers to decide an interpretation.

# Table "public.insights_query_runner_jobs_dependencies"
```
     Column     |            Type             | Collation | Nullable |                               Default                               
----------------+-----------------------------+-----------+----------+---------------------------------------------------------------------
 id             | integer                     |           | not null | nextval('insights_query_runner_jobs_dependencies_id_seq'::regclass)
 job_id         | integer                     |           | not null | 
 recording_time | timestamp without time zone |           | not null | 
Indexes:
    "insights_query_runner_jobs_dependencies_pkey" PRIMARY KEY, btree (id)
    "insights_query_runner_jobs_dependencies_job_id_fk_idx" btree (job_id)
Foreign-key constraints:
    "insights_query_runner_jobs_dependencies_fk_job_id" FOREIGN KEY (job_id) REFERENCES insights_query_runner_jobs(id) ON DELETE CASCADE

```

Stores data points for a code insight that do not need to be queried directly, but depend on the result of a query at a different point

**job_id**: Foreign key to the job that owns this record.

**recording_time**: The time for which this dependency should be recorded at using the parents value.

# Table "public.insights_settings_migration_jobs"
```
       Column        |            Type             | Collation | Nullable |                           Default                            
---------------------+-----------------------------+-----------+----------+--------------------------------------------------------------
 id                  | integer                     |           | not null | nextval('insights_settings_migration_jobs_id_seq'::regclass)
 user_id             | integer                     |           |          | 
 org_id              | integer                     |           |          | 
 global              | boolean                     |           |          | 
 settings_id         | integer                     |           | not null | 
 total_insights      | integer                     |           | not null | 0
 migrated_insights   | integer                     |           | not null | 0
 total_dashboards    | integer                     |           | not null | 0
 migrated_dashboards | integer                     |           | not null | 0
 runs                | integer                     |           | not null | 0
 completed_at        | timestamp without time zone |           |          | 

```

# Table "public.lsif_configuration_policies"
```
           Column            |           Type           | Collation | Nullable |                         Default                         
-----------------------------+--------------------------+-----------+----------+---------------------------------------------------------
 id                          | integer                  |           | not null | nextval('lsif_configuration_policies_id_seq'::regclass)
 repository_id               | integer                  |           |          | 
 name                        | text                     |           |          | 
 type                        | text                     |           | not null | 
 pattern                     | text                     |           | not null | 
 retention_enabled           | boolean                  |           | not null | 
 retention_duration_hours    | integer                  |           |          | 
 retain_intermediate_commits | boolean                  |           | not null | 
 indexing_enabled            | boolean                  |           | not null | 
 index_commit_max_age_hours  | integer                  |           |          | 
 index_intermediate_commits  | boolean                  |           | not null | 
 protected                   | boolean                  |           | not null | false
 repository_patterns         | text[]                   |           |          | 
 last_resolved_at            | timestamp with time zone |           |          | 
 embeddings_enabled          | boolean                  |           | not null | false
 syntactic_indexing_enabled  | boolean                  |           | not null | false
Indexes:
    "lsif_configuration_policies_pkey" PRIMARY KEY, btree (id)
    "lsif_configuration_policies_repository_id" btree (repository_id)
Triggers:
    trigger_configuration_policies_delete AFTER DELETE ON lsif_configuration_policies REFERENCING OLD TABLE AS old FOR EACH STATEMENT EXECUTE FUNCTION func_configuration_policies_delete()
    trigger_configuration_policies_insert AFTER INSERT ON lsif_configuration_policies FOR EACH ROW EXECUTE FUNCTION func_configuration_policies_insert()
    trigger_configuration_policies_update BEFORE UPDATE OF name, pattern, retention_enabled, retention_duration_hours, type, retain_intermediate_commits ON lsif_configuration_policies FOR EACH ROW EXECUTE FUNCTION func_configuration_policies_update()

```

**index_commit_max_age_hours**: The max age of commits indexed by this configuration policy. If null, the age is unbounded.

**index_intermediate_commits**: If the matching Git object is a branch, setting this value to true will also index all commits on the matching branches. Setting this value to false will only consider the tip of the branch.

**indexing_enabled**: Whether or not this configuration policy affects auto-indexing schedules.

**pattern**: A pattern used to match` names of the associated Git object type.

**protected**: Whether or not this configuration policy is protected from modification of its data retention behavior (except for duration).

**repository_id**: The identifier of the repository to which this configuration policy applies. If absent, this policy is applied globally.

**repository_patterns**: The name pattern matching repositories to which this configuration policy applies. If absent, all repositories are matched.

**retain_intermediate_commits**: If the matching Git object is a branch, setting this value to true will also retain all data used to resolve queries for any commit on the matching branches. Setting this value to false will only consider the tip of the branch.

**retention_duration_hours**: The max age of data retained by this configuration policy. If null, the age is unbounded.

**retention_enabled**: Whether or not this configuration policy affects data retention rules.

**type**: The type of Git object (e.g., COMMIT, BRANCH, TAG).

# Table "public.lsif_configuration_policies_repository_pattern_lookup"
```
  Column   |  Type   | Collation | Nullable | Default 
-----------+---------+-----------+----------+---------
 policy_id | integer |           | not null | 
 repo_id   | integer |           | not null | 
Indexes:
    "lsif_configuration_policies_repository_pattern_lookup_pkey" PRIMARY KEY, btree (policy_id, repo_id)

```

A lookup table to get all the repository patterns by repository id that apply to a configuration policy.

**policy_id**: The policy identifier associated with the repository.

**repo_id**: The repository identifier associated with the policy.

# Table "public.lsif_dependency_indexing_jobs"
```
        Column         |           Type           | Collation | Nullable |                          Default                           
-----------------------+--------------------------+-----------+----------+------------------------------------------------------------
 id                    | integer                  |           | not null | nextval('lsif_dependency_indexing_jobs_id_seq1'::regclass)
 state                 | text                     |           | not null | 'queued'::text
 failure_message       | text                     |           |          | 
 queued_at             | timestamp with time zone |           | not null | now()
 started_at            | timestamp with time zone |           |          | 
 finished_at           | timestamp with time zone |           |          | 
 process_after         | timestamp with time zone |           |          | 
 num_resets            | integer                  |           | not null | 0
 num_failures          | integer                  |           | not null | 0
 execution_logs        | json[]                   |           |          | 
 last_heartbeat_at     | timestamp with time zone |           |          | 
 worker_hostname       | text                     |           | not null | ''::text
 upload_id             | integer                  |           |          | 
 external_service_kind | text                     |           | not null | ''::text
 external_service_sync | timestamp with time zone |           |          | 
 cancel                | boolean                  |           | not null | false
Indexes:
    "lsif_dependency_indexing_jobs_pkey1" PRIMARY KEY, btree (id)
    "lsif_dependency_indexing_jobs_state" btree (state)
Foreign-key constraints:
    "lsif_dependency_indexing_jobs_upload_id_fkey1" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE

```

**external_service_kind**: Filter the external services for this kind to wait to have synced. If empty, external_service_sync is ignored and no external services are polled for their last sync time.

**external_service_sync**: The sync time after which external services of the given kind will have synced/created any repositories referenced by the LSIF upload that are resolvable.

# Table "public.lsif_dependency_repos"
```
     Column      |           Type           | Collation | Nullable |                      Default                      
-----------------+--------------------------+-----------+----------+---------------------------------------------------
 id              | bigint                   |           | not null | nextval('lsif_dependency_repos_id_seq'::regclass)
 name            | text                     |           | not null | 
 scheme          | text                     |           | not null | 
 blocked         | boolean                  |           | not null | false
 last_checked_at | timestamp with time zone |           |          | 
Indexes:
    "lsif_dependency_repos_pkey" PRIMARY KEY, btree (id)
    "lsif_dependency_repos_unique_scheme_name" UNIQUE, btree (scheme, name)
    "lsif_dependency_repos_blocked" btree (blocked)
    "lsif_dependency_repos_last_checked_at" btree (last_checked_at NULLS FIRST)
    "lsif_dependency_repos_name_gin" gin (name gin_trgm_ops)
    "lsif_dependency_repos_name_id" btree (name, id)
    "lsif_dependency_repos_scheme_id" btree (scheme, id)
Referenced by:
    TABLE "package_repo_versions" CONSTRAINT "package_id_fk" FOREIGN KEY (package_id) REFERENCES lsif_dependency_repos(id) ON DELETE CASCADE

```

# Table "public.lsif_dependency_syncing_jobs"
```
      Column       |           Type           | Collation | Nullable |                          Default                          
-------------------+--------------------------+-----------+----------+-----------------------------------------------------------
 id                | integer                  |           | not null | nextval('lsif_dependency_indexing_jobs_id_seq'::regclass)
 state             | text                     |           | not null | 'queued'::text
 failure_message   | text                     |           |          | 
 queued_at         | timestamp with time zone |           | not null | now()
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 execution_logs    | json[]                   |           |          | 
 upload_id         | integer                  |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 last_heartbeat_at | timestamp with time zone |           |          | 
 cancel            | boolean                  |           | not null | false
Indexes:
    "lsif_dependency_indexing_jobs_pkey" PRIMARY KEY, btree (id)
    "lsif_dependency_indexing_jobs_upload_id" btree (upload_id)
    "lsif_dependency_syncing_jobs_state" btree (state)
Foreign-key constraints:
    "lsif_dependency_indexing_jobs_upload_id_fkey" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE

```

Tracks jobs that scan imports of indexes to schedule auto-index jobs.

**upload_id**: The identifier of the triggering upload record.

# Table "public.lsif_dirty_repositories"
```
    Column     |           Type           | Collation | Nullable | Default 
---------------+--------------------------+-----------+----------+---------
 repository_id | integer                  |           | not null | 
 dirty_token   | integer                  |           | not null | 
 update_token  | integer                  |           | not null | 
 updated_at    | timestamp with time zone |           |          | 
 set_dirty_at  | timestamp with time zone |           | not null | now()
Indexes:
    "lsif_dirty_repositories_pkey" PRIMARY KEY, btree (repository_id)

```

Stores whether or not the nearest upload data for a repository is out of date (when update_token &gt; dirty_token).

**dirty_token**: Set to the value of update_token visible to the transaction that updates the commit graph. Updates of dirty_token during this time will cause a second update.

**update_token**: This value is incremented on each request to update the commit graph for the repository.

**updated_at**: The time the update_token value was last updated.

# Table "public.lsif_index_configuration"
```
      Column       |  Type   | Collation | Nullable |                       Default                        
-------------------+---------+-----------+----------+------------------------------------------------------
 id                | bigint  |           | not null | nextval('lsif_index_configuration_id_seq'::regclass)
 repository_id     | integer |           | not null | 
 data              | bytea   |           | not null | 
 autoindex_enabled | boolean |           | not null | true
Indexes:
    "lsif_index_configuration_pkey" PRIMARY KEY, btree (id)
    "lsif_index_configuration_repository_id_key" UNIQUE CONSTRAINT, btree (repository_id)
Foreign-key constraints:
    "lsif_index_configuration_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE

```

Stores the configuration used for code intel index jobs for a repository.

**autoindex_enabled**: Whether or not auto-indexing should be attempted on this repo. Index jobs may be inferred from the repository contents if data is empty.

**data**: The raw user-supplied [configuration](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@3.23/-/blob/enterprise/internal/codeintel/autoindex/config/types.go#L3:6) (encoded in JSONC).

# Table "public.lsif_indexes"
```
         Column         |           Type           | Collation | Nullable |                 Default                  
------------------------+--------------------------+-----------+----------+------------------------------------------
 id                     | bigint                   |           | not null | nextval('lsif_indexes_id_seq'::regclass)
 commit                 | text                     |           | not null | 
 queued_at              | timestamp with time zone |           | not null | now()
 state                  | text                     |           | not null | 'queued'::text
 failure_message        | text                     |           |          | 
 started_at             | timestamp with time zone |           |          | 
 finished_at            | timestamp with time zone |           |          | 
 repository_id          | integer                  |           | not null | 
 process_after          | timestamp with time zone |           |          | 
 num_resets             | integer                  |           | not null | 0
 num_failures           | integer                  |           | not null | 0
 docker_steps           | jsonb[]                  |           | not null | 
 root                   | text                     |           | not null | 
 indexer                | text                     |           | not null | 
 indexer_args           | text[]                   |           | not null | 
 outfile                | text                     |           | not null | 
 log_contents           | text                     |           |          | 
 execution_logs         | json[]                   |           |          | 
 local_steps            | text[]                   |           | not null | 
 commit_last_checked_at | timestamp with time zone |           |          | 
 worker_hostname        | text                     |           | not null | ''::text
 last_heartbeat_at      | timestamp with time zone |           |          | 
 cancel                 | boolean                  |           | not null | false
 should_reindex         | boolean                  |           | not null | false
 requested_envvars      | text[]                   |           |          | 
 enqueuer_user_id       | integer                  |           | not null | 0
Indexes:
    "lsif_indexes_pkey" PRIMARY KEY, btree (id)
    "lsif_indexes_commit_last_checked_at" btree (commit_last_checked_at) WHERE state <> 'deleted'::text
    "lsif_indexes_dequeue_order_idx" btree ((enqueuer_user_id > 0) DESC, queued_at DESC, id) WHERE state = 'queued'::text OR state = 'errored'::text
    "lsif_indexes_queued_at_id" btree (queued_at DESC, id)
    "lsif_indexes_repository_id_commit" btree (repository_id, commit)
    "lsif_indexes_repository_id_indexer" btree (repository_id, indexer)
    "lsif_indexes_state" btree (state)
Check constraints:
    "lsif_uploads_commit_valid_chars" CHECK (commit ~ '^[a-z0-9]{40}$'::text)

```

Stores metadata about a code intel index job.

**commit**: A 40-char revhash. Note that this commit may not be resolvable in the future.

**docker_steps**: An array of pre-index [steps](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@3.23/-/blob/enterprise/internal/codeintel/stores/dbstore/docker_step.go#L9:6) to run.

**execution_logs**: An array of [log entries](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@3.23/-/blob/internal/workerutil/store.go#L48:6) (encoded as JSON) from the most recent execution.

**indexer**: The docker image used to run the index command (e.g. sourcegraph/lsif-go).

**indexer_args**: The command run inside the indexer image to produce the index file (e.g. [&#39;lsif-node&#39;, &#39;-p&#39;, &#39;.&#39;])

**local_steps**: A list of commands to run inside the indexer image prior to running the indexer command.

**log_contents**: **Column deprecated in favor of execution_logs.**

**outfile**: The path to the index file produced by the index command relative to the working directory.

**root**: The working directory of the indexer image relative to the repository root.

# Table "public.lsif_last_index_scan"
```
       Column       |           Type           | Collation | Nullable | Default 
--------------------+--------------------------+-----------+----------+---------
 repository_id      | integer                  |           | not null | 
 last_index_scan_at | timestamp with time zone |           | not null | 
Indexes:
    "lsif_last_index_scan_pkey" PRIMARY KEY, btree (repository_id)

```

Tracks the last time repository was checked for auto-indexing job scheduling.

**last_index_scan_at**: The last time uploads of this repository were considered for auto-indexing job scheduling.

# Table "public.lsif_last_retention_scan"
```
         Column         |           Type           | Collation | Nullable | Default 
------------------------+--------------------------+-----------+----------+---------
 repository_id          | integer                  |           | not null | 
 last_retention_scan_at | timestamp with time zone |           | not null | 
Indexes:
    "lsif_last_retention_scan_pkey" PRIMARY KEY, btree (repository_id)

```

Tracks the last time uploads a repository were checked against data retention policies.

**last_retention_scan_at**: The last time uploads of this repository were checked against data retention policies.

# Table "public.lsif_nearest_uploads"
```
    Column     |  Type   | Collation | Nullable | Default 
---------------+---------+-----------+----------+---------
 repository_id | integer |           | not null | 
 commit_bytea  | bytea   |           | not null | 
 uploads       | jsonb   |           | not null | 
Indexes:
    "lsif_nearest_uploads_repository_id_commit_bytea" btree (repository_id, commit_bytea)
    "lsif_nearest_uploads_uploads" gin (uploads)

```

Associates commits with the complete set of uploads visible from that commit. Every commit with upload data is present in this table.

**commit_bytea**: A 40-char revhash. Note that this commit may not be resolvable in the future.

**uploads**: Encodes an {upload_id =&gt; distance} map that includes an entry for every upload visible from the commit. There is always at least one entry with a distance of zero.

# Table "public.lsif_nearest_uploads_links"
```
        Column         |  Type   | Collation | Nullable | Default 
-----------------------+---------+-----------+----------+---------
 repository_id         | integer |           | not null | 
 commit_bytea          | bytea   |           | not null | 
 ancestor_commit_bytea | bytea   |           | not null | 
 distance              | integer |           | not null | 
Indexes:
    "lsif_nearest_uploads_links_repository_id_ancestor_commit_bytea" btree (repository_id, ancestor_commit_bytea)
    "lsif_nearest_uploads_links_repository_id_commit_bytea" btree (repository_id, commit_bytea)

```

Associates commits with the closest ancestor commit with usable upload data. Together, this table and lsif_nearest_uploads cover all commits with resolvable code intelligence.

**ancestor_commit_bytea**: The 40-char revhash of the ancestor. Note that this commit may not be resolvable in the future.

**commit_bytea**: A 40-char revhash. Note that this commit may not be resolvable in the future.

**distance**: The distance bewteen the commits. Parent = 1, Grandparent = 2, etc.

# Table "public.lsif_packages"
```
 Column  |  Type   | Collation | Nullable |                  Default                  
---------+---------+-----------+----------+-------------------------------------------
 id      | integer |           | not null | nextval('lsif_packages_id_seq'::regclass)
 scheme  | text    |           | not null | 
 name    | text    |           | not null | 
 version | text    |           |          | 
 dump_id | integer |           | not null | 
 manager | text    |           | not null | ''::text
Indexes:
    "lsif_packages_pkey" PRIMARY KEY, btree (id)
    "lsif_packages_dump_id" btree (dump_id)
    "lsif_packages_scheme_name_version_dump_id" btree (scheme, name, version, dump_id)
Foreign-key constraints:
    "lsif_packages_dump_id_fkey" FOREIGN KEY (dump_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE

```

Associates an upload with the set of packages they provide within a given packages management scheme.

**dump_id**: The identifier of the upload that provides the package.

**manager**: The package manager name.

**name**: The package name.

**scheme**: The (export) moniker scheme.

**version**: The package version.

# Table "public.lsif_references"
```
 Column  |  Type   | Collation | Nullable |                   Default                   
---------+---------+-----------+----------+---------------------------------------------
 id      | integer |           | not null | nextval('lsif_references_id_seq'::regclass)
 scheme  | text    |           | not null | 
 name    | text    |           | not null | 
 version | text    |           |          | 
 dump_id | integer |           | not null | 
 manager | text    |           | not null | ''::text
Indexes:
    "lsif_references_pkey" PRIMARY KEY, btree (id)
    "lsif_references_dump_id" btree (dump_id)
    "lsif_references_scheme_name_version_dump_id" btree (scheme, name, version, dump_id)
Foreign-key constraints:
    "lsif_references_dump_id_fkey" FOREIGN KEY (dump_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE

```

Associates an upload with the set of packages they require within a given packages management scheme.

**dump_id**: The identifier of the upload that references the package.

**manager**: The package manager name.

**name**: The package name.

**scheme**: The (import) moniker scheme.

**version**: The package version.

# Table "public.lsif_retention_configuration"
```
                 Column                 |  Type   | Collation | Nullable |                         Default                          
----------------------------------------+---------+-----------+----------+----------------------------------------------------------
 id                                     | integer |           | not null | nextval('lsif_retention_configuration_id_seq'::regclass)
 repository_id                          | integer |           | not null | 
 max_age_for_non_stale_branches_seconds | integer |           | not null | 
 max_age_for_non_stale_tags_seconds     | integer |           | not null | 
Indexes:
    "lsif_retention_configuration_pkey" PRIMARY KEY, btree (id)
    "lsif_retention_configuration_repository_id_key" UNIQUE CONSTRAINT, btree (repository_id)
Foreign-key constraints:
    "lsif_retention_configuration_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE

```

Stores the retention policy of code intellience data for a repository.

**max_age_for_non_stale_branches_seconds**: The number of seconds since the last modification of a branch until it is considered stale.

**max_age_for_non_stale_tags_seconds**: The nujmber of seconds since the commit date of a tagged commit until it is considered stale.

# Table "public.lsif_uploads"
```
         Column          |           Type           | Collation | Nullable |                Default                 
-------------------------+--------------------------+-----------+----------+----------------------------------------
 id                      | integer                  |           | not null | nextval('lsif_dumps_id_seq'::regclass)
 commit                  | text                     |           | not null | 
 root                    | text                     |           | not null | ''::text
 uploaded_at             | timestamp with time zone |           | not null | now()
 state                   | text                     |           | not null | 'queued'::text
 failure_message         | text                     |           |          | 
 started_at              | timestamp with time zone |           |          | 
 finished_at             | timestamp with time zone |           |          | 
 repository_id           | integer                  |           | not null | 
 indexer                 | text                     |           | not null | 
 num_parts               | integer                  |           | not null | 
 uploaded_parts          | integer[]                |           | not null | 
 process_after           | timestamp with time zone |           |          | 
 num_resets              | integer                  |           | not null | 0
 upload_size             | bigint                   |           |          | 
 num_failures            | integer                  |           | not null | 0
 associated_index_id     | bigint                   |           |          | 
 committed_at            | timestamp with time zone |           |          | 
 commit_last_checked_at  | timestamp with time zone |           |          | 
 worker_hostname         | text                     |           | not null | ''::text
 last_heartbeat_at       | timestamp with time zone |           |          | 
 execution_logs          | json[]                   |           |          | 
 num_references          | integer                  |           |          | 
 expired                 | boolean                  |           | not null | false
 last_retention_scan_at  | timestamp with time zone |           |          | 
 reference_count         | integer                  |           |          | 
 indexer_version         | text                     |           |          | 
 queued_at               | timestamp with time zone |           |          | 
 cancel                  | boolean                  |           | not null | false
 uncompressed_size       | bigint                   |           |          | 
 last_referenced_scan_at | timestamp with time zone |           |          | 
 last_traversal_scan_at  | timestamp with time zone |           |          | 
 last_reconcile_at       | timestamp with time zone |           |          | 
 content_type            | text                     |           | not null | 'application/x-ndjson+lsif'::text
 should_reindex          | boolean                  |           | not null | false
Indexes:
    "lsif_uploads_pkey" PRIMARY KEY, btree (id)
    "lsif_uploads_repository_id_commit_root_indexer" UNIQUE, btree (repository_id, commit, root, indexer) WHERE state = 'completed'::text
    "lsif_uploads_associated_index_id" btree (associated_index_id)
    "lsif_uploads_commit_last_checked_at" btree (commit_last_checked_at) WHERE state <> 'deleted'::text
    "lsif_uploads_committed_at" btree (committed_at) WHERE state = 'completed'::text
    "lsif_uploads_last_reconcile_at" btree (last_reconcile_at, id) WHERE state = 'completed'::text
    "lsif_uploads_repository_id_commit" btree (repository_id, commit)
    "lsif_uploads_repository_id_indexer" btree (repository_id, indexer)
    "lsif_uploads_state" btree (state)
    "lsif_uploads_uploaded_at_id" btree (uploaded_at DESC, id) WHERE state <> 'deleted'::text
Check constraints:
    "lsif_uploads_commit_valid_chars" CHECK (commit ~ '^[a-z0-9]{40}$'::text)
Referenced by:
    TABLE "codeintel_ranking_exports" CONSTRAINT "codeintel_ranking_exports_upload_id_fkey" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE SET NULL
    TABLE "vulnerability_matches" CONSTRAINT "fk_upload" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    TABLE "lsif_uploads_vulnerability_scan" CONSTRAINT "fk_upload_id" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    TABLE "lsif_dependency_syncing_jobs" CONSTRAINT "lsif_dependency_indexing_jobs_upload_id_fkey" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    TABLE "lsif_dependency_indexing_jobs" CONSTRAINT "lsif_dependency_indexing_jobs_upload_id_fkey1" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    TABLE "lsif_packages" CONSTRAINT "lsif_packages_dump_id_fkey" FOREIGN KEY (dump_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    TABLE "lsif_references" CONSTRAINT "lsif_references_dump_id_fkey" FOREIGN KEY (dump_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    TABLE "lsif_uploads_reference_counts" CONSTRAINT "lsif_uploads_reference_counts_upload_id_fk" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
Triggers:
    trigger_lsif_uploads_delete AFTER DELETE ON lsif_uploads REFERENCING OLD TABLE AS old FOR EACH STATEMENT EXECUTE FUNCTION func_lsif_uploads_delete()
    trigger_lsif_uploads_insert AFTER INSERT ON lsif_uploads FOR EACH ROW EXECUTE FUNCTION func_lsif_uploads_insert()
    trigger_lsif_uploads_update BEFORE UPDATE OF state, num_resets, num_failures, worker_hostname, expired, committed_at ON lsif_uploads FOR EACH ROW EXECUTE FUNCTION func_lsif_uploads_update()

```

Stores metadata about an LSIF index uploaded by a user.

**commit**: A 40-char revhash. Note that this commit may not be resolvable in the future.

**content_type**: The content type of the upload record. For now, the default value is `application/x-ndjson+lsif` to backfill existing records. This will change as we remove LSIF support.

**expired**: Whether or not this upload data is no longer protected by any data retention policy.

**id**: Used as a logical foreign key with the (disjoint) codeintel database.

**indexer**: The name of the indexer that produced the index file. If not supplied by the user it will be pulled from the index metadata.

**indexer_version**: The version of the indexer that produced the index file. If not supplied by the user it will be pulled from the index metadata.

**last_referenced_scan_at**: The last time this upload was known to be referenced by another (possibly expired) index.

**last_retention_scan_at**: The last time this upload was checked against data retention policies.

**last_traversal_scan_at**: The last time this upload was known to be reachable by a non-expired index.

**num_parts**: The number of parts src-cli split the upload file into.

**num_references**: Deprecated in favor of reference_count.

**reference_count**: The number of references to this upload data from other upload records (via lsif_references).

**root**: The path for which the index can resolve code intelligence relative to the repository root.

**upload_size**: The size of the index file (in bytes).

**uploaded_parts**: The index of parts that have been successfully uploaded.

# Table "public.lsif_uploads_audit_logs"
```
       Column        |           Type           | Collation | Nullable |                     Default                      
---------------------+--------------------------+-----------+----------+--------------------------------------------------
 log_timestamp       | timestamp with time zone |           |          | now()
 record_deleted_at   | timestamp with time zone |           |          | 
 upload_id           | integer                  |           | not null | 
 commit              | text                     |           | not null | 
 root                | text                     |           | not null | 
 repository_id       | integer                  |           | not null | 
 uploaded_at         | timestamp with time zone |           | not null | 
 indexer             | text                     |           | not null | 
 indexer_version     | text                     |           |          | 
 upload_size         | bigint                   |           |          | 
 associated_index_id | integer                  |           |          | 
 transition_columns  | USER-DEFINED[]           |           |          | 
 reason              | text                     |           |          | ''::text
 sequence            | bigint                   |           | not null | nextval('lsif_uploads_audit_logs_seq'::regclass)
 operation           | audit_log_operation      |           | not null | 
 content_type        | text                     |           | not null | 'application/x-ndjson+lsif'::text
Indexes:
    "lsif_uploads_audit_logs_timestamp" brin (log_timestamp)
    "lsif_uploads_audit_logs_upload_id" btree (upload_id)

```

**log_timestamp**: Timestamp for this log entry.

**reason**: The reason/source for this entry.

**record_deleted_at**: Set once the upload this entry is associated with is deleted. Once NOW() - record_deleted_at is above a certain threshold, this log entry will be deleted.

**transition_columns**: Array of changes that occurred to the upload for this entry, in the form of {&#34;column&#34;=&gt;&#34;&lt;column name&gt;&#34;, &#34;old&#34;=&gt;&#34;&lt;previous value&gt;&#34;, &#34;new&#34;=&gt;&#34;&lt;new value&gt;&#34;}.

# Table "public.lsif_uploads_reference_counts"
```
     Column      |  Type   | Collation | Nullable | Default 
-----------------+---------+-----------+----------+---------
 upload_id       | integer |           | not null | 
 reference_count | integer |           | not null | 
Indexes:
    "lsif_uploads_reference_counts_upload_id_key" UNIQUE CONSTRAINT, btree (upload_id)
Foreign-key constraints:
    "lsif_uploads_reference_counts_upload_id_fk" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE

```

A less hot-path reference count for upload records.

**reference_count**: The number of references to the associated upload from other records (via lsif_references).

**upload_id**: The identifier of the referenced upload.

# Table "public.lsif_uploads_visible_at_tip"
```
       Column       |  Type   | Collation | Nullable | Default  
--------------------+---------+-----------+----------+----------
 repository_id      | integer |           | not null | 
 upload_id          | integer |           | not null | 
 branch_or_tag_name | text    |           | not null | ''::text
 is_default_branch  | boolean |           | not null | false
Indexes:
    "lsif_uploads_visible_at_tip_is_default_branch" btree (upload_id) WHERE is_default_branch
    "lsif_uploads_visible_at_tip_repository_id_upload_id" btree (repository_id, upload_id)

```

Associates a repository with the set of LSIF upload identifiers that can serve intelligence for the tip of the default branch.

**branch_or_tag_name**: The name of the branch or tag.

**is_default_branch**: Whether the specified branch is the default of the repository. Always false for tags.

**upload_id**: The identifier of the upload visible from the tip of the specified branch or tag.

# Table "public.lsif_uploads_vulnerability_scan"
```
     Column      |            Type             | Collation | Nullable |                           Default                           
-----------------+-----------------------------+-----------+----------+-------------------------------------------------------------
 id              | bigint                      |           | not null | nextval('lsif_uploads_vulnerability_scan_id_seq'::regclass)
 upload_id       | integer                     |           | not null | 
 last_scanned_at | timestamp without time zone |           | not null | now()
Indexes:
    "lsif_uploads_vulnerability_scan_pkey" PRIMARY KEY, btree (id)
    "lsif_uploads_vulnerability_scan_upload_id" UNIQUE, btree (upload_id)
Foreign-key constraints:
    "fk_upload_id" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE

```

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

# Table "public.names"
```
 Column  |  Type   | Collation | Nullable | Default 
---------+---------+-----------+----------+---------
 name    | citext  |           | not null | 
 user_id | integer |           |          | 
 org_id  | integer |           |          | 
 team_id | integer |           |          | 
Indexes:
    "names_pkey" PRIMARY KEY, btree (name)
Check constraints:
    "names_check" CHECK (user_id IS NOT NULL OR org_id IS NOT NULL OR team_id IS NOT NULL)
Foreign-key constraints:
    "names_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id) ON UPDATE CASCADE ON DELETE CASCADE
    "names_team_id_fkey" FOREIGN KEY (team_id) REFERENCES teams(id) ON UPDATE CASCADE ON DELETE CASCADE
    "names_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.namespace_permissions"
```
   Column    |           Type           | Collation | Nullable |                      Default                      
-------------+--------------------------+-----------+----------+---------------------------------------------------
 id          | integer                  |           | not null | nextval('namespace_permissions_id_seq'::regclass)
 namespace   | text                     |           | not null | 
 resource_id | integer                  |           | not null | 
 user_id     | integer                  |           | not null | 
 created_at  | timestamp with time zone |           | not null | now()
Indexes:
    "namespace_permissions_pkey" PRIMARY KEY, btree (id)
    "unique_resource_permission" UNIQUE, btree (namespace, resource_id, user_id)
Check constraints:
    "namespace_not_blank" CHECK (namespace <> ''::text)
Foreign-key constraints:
    "namespace_permissions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.notebook_stars"
```
   Column    |           Type           | Collation | Nullable | Default 
-------------+--------------------------+-----------+----------+---------
 notebook_id | integer                  |           | not null | 
 user_id     | integer                  |           | not null | 
 created_at  | timestamp with time zone |           | not null | now()
Indexes:
    "notebook_stars_pkey" PRIMARY KEY, btree (notebook_id, user_id)
    "notebook_stars_user_id_idx" btree (user_id)
Foreign-key constraints:
    "notebook_stars_notebook_id_fkey" FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE DEFERRABLE
    "notebook_stars_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.notebooks"
```
      Column       |           Type           | Collation | Nullable |                                              Default                                              
-------------------+--------------------------+-----------+----------+---------------------------------------------------------------------------------------------------
 id                | bigint                   |           | not null | nextval('notebooks_id_seq'::regclass)
 title             | text                     |           | not null | 
 blocks            | jsonb                    |           | not null | '[]'::jsonb
 public            | boolean                  |           | not null | 
 creator_user_id   | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 blocks_tsvector   | tsvector                 |           |          | generated always as (jsonb_to_tsvector('english'::regconfig, blocks, '["string"]'::jsonb)) stored
 namespace_user_id | integer                  |           |          | 
 namespace_org_id  | integer                  |           |          | 
 updater_user_id   | integer                  |           |          | 
Indexes:
    "notebooks_pkey" PRIMARY KEY, btree (id)
    "notebooks_blocks_tsvector_idx" gin (blocks_tsvector)
    "notebooks_namespace_org_id_idx" btree (namespace_org_id)
    "notebooks_namespace_user_id_idx" btree (namespace_user_id)
    "notebooks_title_trgm_idx" gin (title gin_trgm_ops)
Check constraints:
    "blocks_is_array" CHECK (jsonb_typeof(blocks) = 'array'::text)
    "notebooks_has_max_1_namespace" CHECK (namespace_user_id IS NULL AND namespace_org_id IS NULL OR (namespace_user_id IS NULL) <> (namespace_org_id IS NULL))
Foreign-key constraints:
    "notebooks_creator_user_id_fkey" FOREIGN KEY (creator_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    "notebooks_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE SET NULL DEFERRABLE
    "notebooks_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    "notebooks_updater_user_id_fkey" FOREIGN KEY (updater_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
Referenced by:
    TABLE "notebook_stars" CONSTRAINT "notebook_stars_notebook_id_fkey" FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.org_invitations"
```
      Column       |           Type           | Collation | Nullable |                   Default                   
-------------------+--------------------------+-----------+----------+---------------------------------------------
 id                | bigint                   |           | not null | nextval('org_invitations_id_seq'::regclass)
 org_id            | integer                  |           | not null | 
 sender_user_id    | integer                  |           | not null | 
 recipient_user_id | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 notified_at       | timestamp with time zone |           |          | 
 responded_at      | timestamp with time zone |           |          | 
 response_type     | boolean                  |           |          | 
 revoked_at        | timestamp with time zone |           |          | 
 deleted_at        | timestamp with time zone |           |          | 
 recipient_email   | citext                   |           |          | 
 expires_at        | timestamp with time zone |           |          | 
Indexes:
    "org_invitations_pkey" PRIMARY KEY, btree (id)
    "org_invitations_org_id" btree (org_id) WHERE deleted_at IS NULL
    "org_invitations_recipient_user_id" btree (recipient_user_id) WHERE deleted_at IS NULL
Check constraints:
    "check_atomic_response" CHECK ((responded_at IS NULL) = (response_type IS NULL))
    "check_single_use" CHECK (responded_at IS NULL AND response_type IS NULL OR revoked_at IS NULL)
    "either_user_id_or_email_defined" CHECK ((recipient_user_id IS NULL) <> (recipient_email IS NULL))
Foreign-key constraints:
    "org_invitations_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id)
    "org_invitations_recipient_user_id_fkey" FOREIGN KEY (recipient_user_id) REFERENCES users(id)
    "org_invitations_sender_user_id_fkey" FOREIGN KEY (sender_user_id) REFERENCES users(id)

```

# Table "public.org_members"
```
   Column   |           Type           | Collation | Nullable |                 Default                 
------------+--------------------------+-----------+----------+-----------------------------------------
 id         | integer                  |           | not null | nextval('org_members_id_seq'::regclass)
 org_id     | integer                  |           | not null | 
 created_at | timestamp with time zone |           | not null | now()
 updated_at | timestamp with time zone |           | not null | now()
 user_id    | integer                  |           | not null | 
Indexes:
    "org_members_pkey" PRIMARY KEY, btree (id)
    "org_members_org_id_user_id_key" UNIQUE CONSTRAINT, btree (org_id, user_id)
Foreign-key constraints:
    "org_members_references_orgs" FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE RESTRICT
    "org_members_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT

```

# Table "public.org_stats"
```
        Column        |           Type           | Collation | Nullable | Default 
----------------------+--------------------------+-----------+----------+---------
 org_id               | integer                  |           | not null | 
 code_host_repo_count | integer                  |           |          | 0
 updated_at           | timestamp with time zone |           | not null | now()
Indexes:
    "org_stats_pkey" PRIMARY KEY, btree (org_id)
Foreign-key constraints:
    "org_stats_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE DEFERRABLE

```

Business statistics for organizations

**code_host_repo_count**: Count of repositories accessible on all code hosts for this organization.

**org_id**: Org ID that the stats relate to.

# Table "public.orgs"
```
      Column       |           Type           | Collation | Nullable |             Default              
-------------------+--------------------------+-----------+----------+----------------------------------
 id                | integer                  |           | not null | nextval('orgs_id_seq'::regclass)
 name              | citext                   |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 display_name      | text                     |           |          | 
 slack_webhook_url | text                     |           |          | 
 deleted_at        | timestamp with time zone |           |          | 
Indexes:
    "orgs_pkey" PRIMARY KEY, btree (id)
    "orgs_name" UNIQUE, btree (name) WHERE deleted_at IS NULL
Check constraints:
    "orgs_display_name_max_length" CHECK (char_length(display_name) <= 255)
    "orgs_name_max_length" CHECK (char_length(name::text) <= 255)
    "orgs_name_valid_chars" CHECK (name ~ '^[a-zA-Z0-9](?:[a-zA-Z0-9]|[-.](?=[a-zA-Z0-9]))*-?$'::citext)
Referenced by:
    TABLE "batch_changes" CONSTRAINT "batch_changes_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE DEFERRABLE
    TABLE "cm_monitors" CONSTRAINT "cm_monitors_org_id_fk" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    TABLE "cm_recipients" CONSTRAINT "cm_recipients_org_id_fk" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    TABLE "executor_secrets" CONSTRAINT "executor_secrets_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    TABLE "feature_flag_overrides" CONSTRAINT "feature_flag_overrides_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    TABLE "names" CONSTRAINT "names_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id) ON UPDATE CASCADE ON DELETE CASCADE
    TABLE "notebooks" CONSTRAINT "notebooks_namespace_org_id_fkey" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE SET NULL DEFERRABLE
    TABLE "org_invitations" CONSTRAINT "org_invitations_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id)
    TABLE "org_members" CONSTRAINT "org_members_references_orgs" FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE RESTRICT
    TABLE "org_stats" CONSTRAINT "org_stats_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE DEFERRABLE
    TABLE "registry_extensions" CONSTRAINT "registry_extensions_publisher_org_id_fkey" FOREIGN KEY (publisher_org_id) REFERENCES orgs(id)
    TABLE "saved_searches" CONSTRAINT "saved_searches_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id)
    TABLE "search_contexts" CONSTRAINT "search_contexts_namespace_org_id_fk" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    TABLE "settings" CONSTRAINT "settings_references_orgs" FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE RESTRICT

```

# Table "public.orgs_open_beta_stats"
```
   Column   |           Type           | Collation | Nullable |      Default      
------------+--------------------------+-----------+----------+-------------------
 id         | uuid                     |           | not null | gen_random_uuid()
 user_id    | integer                  |           |          | 
 org_id     | integer                  |           |          | 
 created_at | timestamp with time zone |           |          | now()
 data       | jsonb                    |           | not null | '{}'::jsonb
Indexes:
    "orgs_open_beta_stats_pkey" PRIMARY KEY, btree (id)

```

# Table "public.out_of_band_migrations"
```
          Column          |           Type           | Collation | Nullable |                      Default                       
--------------------------+--------------------------+-----------+----------+----------------------------------------------------
 id                       | integer                  |           | not null | nextval('out_of_band_migrations_id_seq'::regclass)
 team                     | text                     |           | not null | 
 component                | text                     |           | not null | 
 description              | text                     |           | not null | 
 progress                 | double precision         |           | not null | 0
 created                  | timestamp with time zone |           | not null | 
 last_updated             | timestamp with time zone |           |          | 
 non_destructive          | boolean                  |           | not null | 
 apply_reverse            | boolean                  |           | not null | false
 is_enterprise            | boolean                  |           | not null | false
 introduced_version_major | integer                  |           | not null | 
 introduced_version_minor | integer                  |           | not null | 
 deprecated_version_major | integer                  |           |          | 
 deprecated_version_minor | integer                  |           |          | 
 metadata                 | jsonb                    |           | not null | '{}'::jsonb
Indexes:
    "out_of_band_migrations_pkey" PRIMARY KEY, btree (id)
Check constraints:
    "out_of_band_migrations_component_nonempty" CHECK (component <> ''::text)
    "out_of_band_migrations_description_nonempty" CHECK (description <> ''::text)
    "out_of_band_migrations_progress_range" CHECK (progress >= 0::double precision AND progress <= 1::double precision)
    "out_of_band_migrations_team_nonempty" CHECK (team <> ''::text)
Referenced by:
    TABLE "out_of_band_migrations_errors" CONSTRAINT "out_of_band_migrations_errors_migration_id_fkey" FOREIGN KEY (migration_id) REFERENCES out_of_band_migrations(id) ON DELETE CASCADE

```

Stores metadata and progress about an out-of-band migration routine.

**apply_reverse**: Whether this migration should run in the opposite direction (to support an upcoming downgrade).

**component**: The name of the component undergoing a migration.

**created**: The date and time the migration was inserted into the database (via an upgrade).

**deprecated_version_major**: The lowest Sourcegraph version (major component) that assumes the migration has completed.

**deprecated_version_minor**: The lowest Sourcegraph version (minor component) that assumes the migration has completed.

**description**: A brief description about the migration.

**id**: A globally unique primary key for this migration. The same key is used consistently across all Sourcegraph instances for the same migration.

**introduced_version_major**: The Sourcegraph version (major component) in which this migration was first introduced.

**introduced_version_minor**: The Sourcegraph version (minor component) in which this migration was first introduced.

**is_enterprise**: When true, these migrations are invisible to OSS mode.

**last_updated**: The date and time the migration was last updated.

**non_destructive**: Whether or not this migration alters data so it can no longer be read by the previous Sourcegraph instance.

**progress**: The percentage progress in the up direction (0=0%, 1=100%).

**team**: The name of the engineering team responsible for the migration.

# Table "public.out_of_band_migrations_errors"
```
    Column    |           Type           | Collation | Nullable |                          Default                          
--------------+--------------------------+-----------+----------+-----------------------------------------------------------
 id           | integer                  |           | not null | nextval('out_of_band_migrations_errors_id_seq'::regclass)
 migration_id | integer                  |           | not null | 
 message      | text                     |           | not null | 
 created      | timestamp with time zone |           | not null | now()
Indexes:
    "out_of_band_migrations_errors_pkey" PRIMARY KEY, btree (id)
Check constraints:
    "out_of_band_migrations_errors_message_nonempty" CHECK (message <> ''::text)
Foreign-key constraints:
    "out_of_band_migrations_errors_migration_id_fkey" FOREIGN KEY (migration_id) REFERENCES out_of_band_migrations(id) ON DELETE CASCADE

```

Stores errors that occurred while performing an out-of-band migration.

**created**: The date and time the error occurred.

**id**: A unique identifer.

**message**: The error message.

**migration_id**: The identifier of the migration.

# Table "public.outbound_webhook_event_types"
```
       Column        |  Type  | Collation | Nullable |                         Default                          
---------------------+--------+-----------+----------+----------------------------------------------------------
 id                  | bigint |           | not null | nextval('outbound_webhook_event_types_id_seq'::regclass)
 outbound_webhook_id | bigint |           | not null | 
 event_type          | text   |           | not null | 
 scope               | text   |           |          | 
Indexes:
    "outbound_webhook_event_types_pkey" PRIMARY KEY, btree (id)
    "outbound_webhook_event_types_event_type_idx" btree (event_type, scope)
Foreign-key constraints:
    "outbound_webhook_event_types_outbound_webhook_id_fkey" FOREIGN KEY (outbound_webhook_id) REFERENCES outbound_webhooks(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.outbound_webhook_jobs"
```
      Column       |           Type           | Collation | Nullable |                      Default                      
-------------------+--------------------------+-----------+----------+---------------------------------------------------
 id                | bigint                   |           | not null | nextval('outbound_webhook_jobs_id_seq'::regclass)
 event_type        | text                     |           | not null | 
 scope             | text                     |           |          | 
 encryption_key_id | text                     |           |          | 
 payload           | bytea                    |           | not null | 
 state             | text                     |           | not null | 'queued'::text
 failure_message   | text                     |           |          | 
 queued_at         | timestamp with time zone |           | not null | now()
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 cancel            | boolean                  |           | not null | false
Indexes:
    "outbound_webhook_jobs_pkey" PRIMARY KEY, btree (id)
    "outbound_webhook_jobs_state_idx" btree (state)
    "outbound_webhook_payload_process_after_idx" btree (process_after)
Referenced by:
    TABLE "outbound_webhook_logs" CONSTRAINT "outbound_webhook_logs_job_id_fkey" FOREIGN KEY (job_id) REFERENCES outbound_webhook_jobs(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.outbound_webhook_logs"
```
       Column        |           Type           | Collation | Nullable |                      Default                      
---------------------+--------------------------+-----------+----------+---------------------------------------------------
 id                  | bigint                   |           | not null | nextval('outbound_webhook_logs_id_seq'::regclass)
 job_id              | bigint                   |           | not null | 
 outbound_webhook_id | bigint                   |           | not null | 
 sent_at             | timestamp with time zone |           | not null | now()
 status_code         | integer                  |           | not null | 
 encryption_key_id   | text                     |           |          | 
 request             | bytea                    |           | not null | 
 response            | bytea                    |           | not null | 
 error               | bytea                    |           | not null | 
Indexes:
    "outbound_webhook_logs_pkey" PRIMARY KEY, btree (id)
    "outbound_webhook_logs_outbound_webhook_id_idx" btree (outbound_webhook_id)
    "outbound_webhooks_logs_status_code_idx" btree (status_code)
Foreign-key constraints:
    "outbound_webhook_logs_job_id_fkey" FOREIGN KEY (job_id) REFERENCES outbound_webhook_jobs(id) ON UPDATE CASCADE ON DELETE CASCADE
    "outbound_webhook_logs_outbound_webhook_id_fkey" FOREIGN KEY (outbound_webhook_id) REFERENCES outbound_webhooks(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.outbound_webhooks"
```
      Column       |           Type           | Collation | Nullable |                    Default                    
-------------------+--------------------------+-----------+----------+-----------------------------------------------
 id                | bigint                   |           | not null | nextval('outbound_webhooks_id_seq'::regclass)
 created_by        | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_by        | integer                  |           |          | 
 updated_at        | timestamp with time zone |           | not null | now()
 encryption_key_id | text                     |           |          | 
 url               | bytea                    |           | not null | 
 secret            | bytea                    |           | not null | 
Indexes:
    "outbound_webhooks_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "outbound_webhooks_created_by_fkey" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
    "outbound_webhooks_updated_by_fkey" FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
Referenced by:
    TABLE "outbound_webhook_event_types" CONSTRAINT "outbound_webhook_event_types_outbound_webhook_id_fkey" FOREIGN KEY (outbound_webhook_id) REFERENCES outbound_webhooks(id) ON UPDATE CASCADE ON DELETE CASCADE
    TABLE "outbound_webhook_logs" CONSTRAINT "outbound_webhook_logs_outbound_webhook_id_fkey" FOREIGN KEY (outbound_webhook_id) REFERENCES outbound_webhooks(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.own_aggregate_recent_contribution"
```
        Column        |  Type   | Collation | Nullable |                            Default                            
----------------------+---------+-----------+----------+---------------------------------------------------------------
 id                   | integer |           | not null | nextval('own_aggregate_recent_contribution_id_seq'::regclass)
 commit_author_id     | integer |           | not null | 
 changed_file_path_id | integer |           | not null | 
 contributions_count  | integer |           |          | 0
Indexes:
    "own_aggregate_recent_contribution_pkey" PRIMARY KEY, btree (id)
    "own_aggregate_recent_contribution_file_author" UNIQUE, btree (changed_file_path_id, commit_author_id)
Foreign-key constraints:
    "own_aggregate_recent_contribution_changed_file_path_id_fkey" FOREIGN KEY (changed_file_path_id) REFERENCES repo_paths(id)
    "own_aggregate_recent_contribution_commit_author_id_fkey" FOREIGN KEY (commit_author_id) REFERENCES commit_authors(id)

```

# Table "public.own_aggregate_recent_view"
```
       Column        |  Type   | Collation | Nullable |                        Default                        
---------------------+---------+-----------+----------+-------------------------------------------------------
 id                  | integer |           | not null | nextval('own_aggregate_recent_view_id_seq'::regclass)
 viewer_id           | integer |           | not null | 
 viewed_file_path_id | integer |           | not null | 
 views_count         | integer |           |          | 0
Indexes:
    "own_aggregate_recent_view_pkey" PRIMARY KEY, btree (id)
    "own_aggregate_recent_view_viewer" UNIQUE, btree (viewed_file_path_id, viewer_id)
Foreign-key constraints:
    "own_aggregate_recent_view_viewed_file_path_id_fkey" FOREIGN KEY (viewed_file_path_id) REFERENCES repo_paths(id)
    "own_aggregate_recent_view_viewer_id_fkey" FOREIGN KEY (viewer_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

One entry contains a number of views of a single file by a given viewer.

# Table "public.own_background_jobs"
```
      Column       |           Type           | Collation | Nullable |                     Default                     
-------------------+--------------------------+-----------+----------+-------------------------------------------------
 id                | integer                  |           | not null | nextval('own_background_jobs_id_seq'::regclass)
 state             | text                     |           |          | 'queued'::text
 failure_message   | text                     |           |          | 
 queued_at         | timestamp with time zone |           |          | now()
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 cancel            | boolean                  |           | not null | false
 repo_id           | integer                  |           | not null | 
 job_type          | integer                  |           | not null | 
Indexes:
    "own_background_jobs_pkey" PRIMARY KEY, btree (id)
    "own_background_jobs_repo_id_idx" btree (repo_id)
    "own_background_jobs_state_idx" btree (state)

```

# Table "public.own_signal_configurations"
```
         Column         |  Type   | Collation | Nullable |                        Default                        
------------------------+---------+-----------+----------+-------------------------------------------------------
 id                     | integer |           | not null | nextval('own_signal_configurations_id_seq'::regclass)
 name                   | text    |           | not null | 
 description            | text    |           | not null | ''::text
 excluded_repo_patterns | text[]  |           |          | 
 enabled                | boolean |           | not null | false
Indexes:
    "own_signal_configurations_pkey" PRIMARY KEY, btree (id)
    "own_signal_configurations_name_uidx" UNIQUE, btree (name)

```

# Table "public.own_signal_recent_contribution"
```
        Column        |            Type             | Collation | Nullable |                          Default                           
----------------------+-----------------------------+-----------+----------+------------------------------------------------------------
 id                   | integer                     |           | not null | nextval('own_signal_recent_contribution_id_seq'::regclass)
 commit_author_id     | integer                     |           | not null | 
 changed_file_path_id | integer                     |           | not null | 
 commit_timestamp     | timestamp without time zone |           | not null | 
 commit_id            | bytea                       |           | not null | 
Indexes:
    "own_signal_recent_contribution_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "own_signal_recent_contribution_changed_file_path_id_fkey" FOREIGN KEY (changed_file_path_id) REFERENCES repo_paths(id)
    "own_signal_recent_contribution_commit_author_id_fkey" FOREIGN KEY (commit_author_id) REFERENCES commit_authors(id)
Triggers:
    update_own_aggregate_recent_contribution AFTER INSERT ON own_signal_recent_contribution FOR EACH ROW EXECUTE FUNCTION update_own_aggregate_recent_contribution()

```

One entry per file changed in every commit that classifies as a contribution signal.

# Table "public.ownership_path_stats"
```
               Column                |            Type             | Collation | Nullable | Default 
-------------------------------------+-----------------------------+-----------+----------+---------
 file_path_id                        | integer                     |           | not null | 
 tree_codeowned_files_count          | integer                     |           |          | 
 last_updated_at                     | timestamp without time zone |           | not null | 
 tree_assigned_ownership_files_count | integer                     |           |          | 
 tree_any_ownership_files_count      | integer                     |           |          | 
Indexes:
    "ownership_path_stats_pkey" PRIMARY KEY, btree (file_path_id)
Foreign-key constraints:
    "ownership_path_stats_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)

```

Data on how many files in given tree are owned by anyone.

We choose to have a table for `ownership_path_stats` - more general than for CODEOWNERS,
with a specific tree_codeowned_files_count CODEOWNERS column. The reason for that
is that we aim at expanding path stats by including total owned files (via CODEOWNERS
or assigned ownership), and perhaps files count by assigned ownership only.

**last_updated_at**: When the last background job updating counts run.

# Table "public.package_repo_filters"
```
   Column   |           Type           | Collation | Nullable |                     Default                      
------------+--------------------------+-----------+----------+--------------------------------------------------
 id         | integer                  |           | not null | nextval('package_repo_filters_id_seq'::regclass)
 behaviour  | text                     |           | not null | 
 scheme     | text                     |           | not null | 
 matcher    | jsonb                    |           | not null | 
 deleted_at | timestamp with time zone |           |          | 
 updated_at | timestamp with time zone |           | not null | statement_timestamp()
Indexes:
    "package_repo_filters_pkey" PRIMARY KEY, btree (id)
    "package_repo_filters_unique_matcher_per_scheme" UNIQUE, btree (scheme, matcher)
Check constraints:
    "package_repo_filters_behaviour_is_allow_or_block" CHECK (behaviour = ANY ('{BLOCK,ALLOW}'::text[]))
    "package_repo_filters_is_pkgrepo_scheme" CHECK (scheme = ANY ('{semanticdb,npm,go,python,rust-analyzer,scip-ruby}'::text[]))
    "package_repo_filters_valid_oneof_glob" CHECK (matcher ? 'VersionGlob'::text AND (matcher ->> 'VersionGlob'::text) <> ''::text AND (matcher ->> 'PackageName'::text) <> ''::text AND NOT matcher ? 'PackageGlob'::text OR matcher ? 'PackageGlob'::text AND (matcher ->> 'PackageGlob'::text) <> ''::text AND NOT matcher ? 'VersionGlob'::text)
Triggers:
    trigger_package_repo_filters_updated_at BEFORE UPDATE ON package_repo_filters FOR EACH ROW WHEN (old.* IS DISTINCT FROM new.*) EXECUTE FUNCTION func_package_repo_filters_updated_at()

```

# Table "public.package_repo_versions"
```
     Column      |           Type           | Collation | Nullable |                      Default                      
-----------------+--------------------------+-----------+----------+---------------------------------------------------
 id              | bigint                   |           | not null | nextval('package_repo_versions_id_seq'::regclass)
 package_id      | bigint                   |           | not null | 
 version         | text                     |           | not null | 
 blocked         | boolean                  |           | not null | false
 last_checked_at | timestamp with time zone |           |          | 
Indexes:
    "package_repo_versions_pkey" PRIMARY KEY, btree (id)
    "package_repo_versions_unique_version_per_package" UNIQUE, btree (package_id, version)
    "package_repo_versions_blocked" btree (blocked)
    "package_repo_versions_last_checked_at" btree (last_checked_at NULLS FIRST)
Foreign-key constraints:
    "package_id_fk" FOREIGN KEY (package_id) REFERENCES lsif_dependency_repos(id) ON DELETE CASCADE

```

# Table "public.permission_sync_jobs"
```
        Column        |           Type           | Collation | Nullable |                     Default                      
----------------------+--------------------------+-----------+----------+--------------------------------------------------
 id                   | integer                  |           | not null | nextval('permission_sync_jobs_id_seq'::regclass)
 state                | text                     |           |          | 'queued'::text
 reason               | text                     |           | not null | 
 failure_message      | text                     |           |          | 
 queued_at            | timestamp with time zone |           |          | now()
 started_at           | timestamp with time zone |           |          | 
 finished_at          | timestamp with time zone |           |          | 
 process_after        | timestamp with time zone |           |          | 
 num_resets           | integer                  |           | not null | 0
 num_failures         | integer                  |           | not null | 0
 last_heartbeat_at    | timestamp with time zone |           |          | 
 execution_logs       | json[]                   |           |          | 
 worker_hostname      | text                     |           | not null | ''::text
 cancel               | boolean                  |           | not null | false
 repository_id        | integer                  |           |          | 
 user_id              | integer                  |           |          | 
 triggered_by_user_id | integer                  |           |          | 
 priority             | integer                  |           | not null | 0
 invalidate_caches    | boolean                  |           | not null | false
 cancellation_reason  | text                     |           |          | 
 no_perms             | boolean                  |           | not null | false
 permissions_added    | integer                  |           | not null | 0
 permissions_removed  | integer                  |           | not null | 0
 permissions_found    | integer                  |           | not null | 0
 code_host_states     | json[]                   |           |          | 
 is_partial_success   | boolean                  |           |          | false
Indexes:
    "permission_sync_jobs_pkey" PRIMARY KEY, btree (id)
    "permission_sync_jobs_unique" UNIQUE, btree (priority, user_id, repository_id, cancel, process_after) WHERE state = 'queued'::text
    "permission_sync_jobs_process_after" btree (process_after)
    "permission_sync_jobs_repository_id" btree (repository_id)
    "permission_sync_jobs_state" btree (state)
    "permission_sync_jobs_user_id" btree (user_id)
Check constraints:
    "permission_sync_jobs_for_repo_or_user" CHECK ((user_id IS NULL) <> (repository_id IS NULL))
Foreign-key constraints:
    "permission_sync_jobs_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE
    "permission_sync_jobs_triggered_by_user_id_fkey" FOREIGN KEY (triggered_by_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    "permission_sync_jobs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

**cancellation_reason**: Specifies why permissions sync job was cancelled.

**priority**: Specifies numeric priority for the permissions sync job.

**reason**: Specifies why permissions sync job was triggered.

**triggered_by_user_id**: Specifies an ID of a user who triggered a sync.

# Table "public.permissions"
```
   Column   |           Type           | Collation | Nullable |                 Default                 
------------+--------------------------+-----------+----------+-----------------------------------------
 id         | integer                  |           | not null | nextval('permissions_id_seq'::regclass)
 namespace  | text                     |           | not null | 
 action     | text                     |           | not null | 
 created_at | timestamp with time zone |           | not null | now()
Indexes:
    "permissions_pkey" PRIMARY KEY, btree (id)
    "permissions_unique_namespace_action" UNIQUE, btree (namespace, action)
Check constraints:
    "action_not_blank" CHECK (action <> ''::text)
    "namespace_not_blank" CHECK (namespace <> ''::text)
Referenced by:
    TABLE "role_permissions" CONSTRAINT "role_permissions_permission_id_fkey" FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.phabricator_repos"
```
   Column   |           Type           | Collation | Nullable |                    Default                    
------------+--------------------------+-----------+----------+-----------------------------------------------
 id         | integer                  |           | not null | nextval('phabricator_repos_id_seq'::regclass)
 callsign   | citext                   |           | not null | 
 repo_name  | citext                   |           | not null | 
 created_at | timestamp with time zone |           | not null | now()
 updated_at | timestamp with time zone |           | not null | now()
 deleted_at | timestamp with time zone |           |          | 
 url        | text                     |           | not null | ''::text
Indexes:
    "phabricator_repos_pkey" PRIMARY KEY, btree (id)
    "phabricator_repos_repo_name_key" UNIQUE CONSTRAINT, btree (repo_name)

```

# Table "public.product_licenses"
```
         Column          |           Type           | Collation | Nullable | Default 
-------------------------+--------------------------+-----------+----------+---------
 id                      | uuid                     |           | not null | 
 product_subscription_id | uuid                     |           | not null | 
 license_key             | text                     |           | not null | 
 created_at              | timestamp with time zone |           | not null | now()
 license_version         | integer                  |           |          | 
 license_tags            | text[]                   |           |          | 
 license_user_count      | integer                  |           |          | 
 license_expires_at      | timestamp with time zone |           |          | 
 access_token_enabled    | boolean                  |           | not null | true
 site_id                 | uuid                     |           |          | 
 license_check_token     | bytea                    |           |          | 
 revoked_at              | timestamp with time zone |           |          | 
 salesforce_sub_id       | text                     |           |          | 
 salesforce_opp_id       | text                     |           |          | 
 revoke_reason           | text                     |           |          | 
Indexes:
    "product_licenses_pkey" PRIMARY KEY, btree (id)
    "product_licenses_license_check_token_idx" UNIQUE, btree (license_check_token)
Foreign-key constraints:
    "product_licenses_product_subscription_id_fkey" FOREIGN KEY (product_subscription_id) REFERENCES product_subscriptions(id)

```

**access_token_enabled**: Whether this license key can be used as an access token to authenticate API requests

# Table "public.product_subscriptions"
```
                      Column                       |           Type           | Collation | Nullable | Default 
---------------------------------------------------+--------------------------+-----------+----------+---------
 id                                                | uuid                     |           | not null | 
 user_id                                           | integer                  |           | not null | 
 billing_subscription_id                           | text                     |           |          | 
 created_at                                        | timestamp with time zone |           | not null | now()
 updated_at                                        | timestamp with time zone |           | not null | now()
 archived_at                                       | timestamp with time zone |           |          | 
 account_number                                    | text                     |           |          | 
 cody_gateway_enabled                              | boolean                  |           | not null | false
 cody_gateway_chat_rate_limit                      | bigint                   |           |          | 
 cody_gateway_chat_rate_interval_seconds           | integer                  |           |          | 
 cody_gateway_embeddings_api_rate_limit            | bigint                   |           |          | 
 cody_gateway_embeddings_api_rate_interval_seconds | integer                  |           |          | 
 cody_gateway_embeddings_api_allowed_models        | text[]                   |           |          | 
 cody_gateway_chat_rate_limit_allowed_models       | text[]                   |           |          | 
 cody_gateway_code_rate_limit                      | bigint                   |           |          | 
 cody_gateway_code_rate_interval_seconds           | integer                  |           |          | 
 cody_gateway_code_rate_limit_allowed_models       | text[]                   |           |          | 
Indexes:
    "product_subscriptions_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "product_subscriptions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
Referenced by:
    TABLE "product_licenses" CONSTRAINT "product_licenses_product_subscription_id_fkey" FOREIGN KEY (product_subscription_id) REFERENCES product_subscriptions(id)

```

**cody_gateway_embeddings_api_allowed_models**: Custom override for the set of models allowed for embedding

**cody_gateway_embeddings_api_rate_interval_seconds**: Custom time interval over which the embeddings rate limit is applied

**cody_gateway_embeddings_api_rate_limit**: Custom requests per time interval allowed for embeddings

# Table "public.query_runner_state"
```
      Column      |           Type           | Collation | Nullable | Default 
------------------+--------------------------+-----------+----------+---------
 query            | text                     |           |          | 
 last_executed    | timestamp with time zone |           |          | 
 latest_result    | timestamp with time zone |           |          | 
 exec_duration_ns | bigint                   |           |          | 

```

# Table "public.redis_key_value"
```
  Column   | Type  | Collation | Nullable | Default 
-----------+-------+-----------+----------+---------
 namespace | text  |           | not null | 
 key       | text  |           | not null | 
 value     | bytea |           | not null | 
Indexes:
    "redis_key_value_pkey" PRIMARY KEY, btree (namespace, key) INCLUDE (value)

```

# Table "public.registry_extension_releases"
```
        Column         |           Type           | Collation | Nullable |                         Default                         
-----------------------+--------------------------+-----------+----------+---------------------------------------------------------
 id                    | bigint                   |           | not null | nextval('registry_extension_releases_id_seq'::regclass)
 registry_extension_id | integer                  |           | not null | 
 creator_user_id       | integer                  |           | not null | 
 release_version       | citext                   |           |          | 
 release_tag           | citext                   |           | not null | 
 manifest              | jsonb                    |           | not null | 
 bundle                | text                     |           |          | 
 created_at            | timestamp with time zone |           | not null | now()
 deleted_at            | timestamp with time zone |           |          | 
 source_map            | text                     |           |          | 
Indexes:
    "registry_extension_releases_pkey" PRIMARY KEY, btree (id)
    "registry_extension_releases_version" UNIQUE, btree (registry_extension_id, release_version) WHERE release_version IS NOT NULL
    "registry_extension_releases_registry_extension_id" btree (registry_extension_id, release_tag, created_at DESC) WHERE deleted_at IS NULL
    "registry_extension_releases_registry_extension_id_created_at" btree (registry_extension_id, created_at) WHERE deleted_at IS NULL
Foreign-key constraints:
    "registry_extension_releases_creator_user_id_fkey" FOREIGN KEY (creator_user_id) REFERENCES users(id)
    "registry_extension_releases_registry_extension_id_fkey" FOREIGN KEY (registry_extension_id) REFERENCES registry_extensions(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.registry_extensions"
```
      Column       |           Type           | Collation | Nullable |                     Default                     
-------------------+--------------------------+-----------+----------+-------------------------------------------------
 id                | integer                  |           | not null | nextval('registry_extensions_id_seq'::regclass)
 uuid              | uuid                     |           | not null | 
 publisher_user_id | integer                  |           |          | 
 publisher_org_id  | integer                  |           |          | 
 name              | citext                   |           | not null | 
 manifest          | text                     |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 deleted_at        | timestamp with time zone |           |          | 
Indexes:
    "registry_extensions_pkey" PRIMARY KEY, btree (id)
    "registry_extensions_publisher_name" UNIQUE, btree (COALESCE(publisher_user_id, 0), COALESCE(publisher_org_id, 0), name) WHERE deleted_at IS NULL
    "registry_extensions_uuid" UNIQUE, btree (uuid)
Check constraints:
    "registry_extensions_name_length" CHECK (char_length(name::text) > 0 AND char_length(name::text) <= 128)
    "registry_extensions_name_valid_chars" CHECK (name ~ '^[a-zA-Z0-9](?:[a-zA-Z0-9]|[_.-](?=[a-zA-Z0-9]))*$'::citext)
    "registry_extensions_single_publisher" CHECK ((publisher_user_id IS NULL) <> (publisher_org_id IS NULL))
Foreign-key constraints:
    "registry_extensions_publisher_org_id_fkey" FOREIGN KEY (publisher_org_id) REFERENCES orgs(id)
    "registry_extensions_publisher_user_id_fkey" FOREIGN KEY (publisher_user_id) REFERENCES users(id)
Referenced by:
    TABLE "registry_extension_releases" CONSTRAINT "registry_extension_releases_registry_extension_id_fkey" FOREIGN KEY (registry_extension_id) REFERENCES registry_extensions(id) ON UPDATE CASCADE ON DELETE CASCADE

```

# Table "public.repo"
```
        Column         |           Type           | Collation | Nullable |                                          Default                                           
-----------------------+--------------------------+-----------+----------+--------------------------------------------------------------------------------------------
 id                    | integer                  |           | not null | nextval('repo_id_seq'::regclass)
 name                  | citext                   |           | not null | 
 description           | text                     |           |          | 
 fork                  | boolean                  |           | not null | false
 created_at            | timestamp with time zone |           | not null | now()
 updated_at            | timestamp with time zone |           |          | 
 external_id           | text                     |           |          | 
 external_service_type | text                     |           |          | 
 external_service_id   | text                     |           |          | 
 archived              | boolean                  |           | not null | false
 uri                   | citext                   |           |          | 
 deleted_at            | timestamp with time zone |           |          | 
 metadata              | jsonb                    |           | not null | '{}'::jsonb
 private               | boolean                  |           | not null | false
 stars                 | integer                  |           | not null | 0
 blocked               | jsonb                    |           |          | 
 topics                | text[]                   |           |          | generated always as (extract_topics_from_metadata(external_service_type, metadata)) stored
Indexes:
    "repo_pkey" PRIMARY KEY, btree (id)
    "repo_external_unique_idx" UNIQUE, btree (external_service_type, external_service_id, external_id)
    "repo_name_unique" UNIQUE CONSTRAINT, btree (name) DEFERRABLE
    "idx_repo_topics" gin (topics)
    "repo_archived" btree (archived)
    "repo_blocked_idx" btree ((blocked IS NOT NULL))
    "repo_created_at" btree (created_at)
    "repo_description_trgm_idx" gin (lower(description) gin_trgm_ops)
    "repo_dotcom_indexable_repos_idx" btree (stars DESC NULLS LAST) INCLUDE (id, name) WHERE deleted_at IS NULL AND blocked IS NULL AND (stars >= 5 AND NOT COALESCE(fork, false) AND NOT archived OR lower(name::text) ~ '^(src\.fedoraproject\.org|maven|npm|jdk)'::text)
    "repo_fork" btree (fork)
    "repo_hashed_name_idx" btree (sha256(lower(name::text)::bytea)) WHERE deleted_at IS NULL
    "repo_is_not_blocked_idx" btree ((blocked IS NULL))
    "repo_metadata_gin_idx" gin (metadata)
    "repo_name_case_sensitive_trgm_idx" gin ((name::text) gin_trgm_ops)
    "repo_name_idx" btree (lower(name::text) COLLATE "C")
    "repo_name_trgm" gin (lower(name::text) gin_trgm_ops)
    "repo_non_deleted_id_name_idx" btree (id, name) WHERE deleted_at IS NULL
    "repo_private" btree (private)
    "repo_stars_desc_id_desc_idx" btree (stars DESC NULLS LAST, id DESC) WHERE deleted_at IS NULL AND blocked IS NULL
    "repo_stars_idx" btree (stars DESC NULLS LAST)
    "repo_uri_idx" btree (uri)
Check constraints:
    "check_name_nonempty" CHECK (name <> ''::citext)
    "repo_metadata_check" CHECK (jsonb_typeof(metadata) = 'object'::text)
Referenced by:
    TABLE "batch_spec_workspaces" CONSTRAINT "batch_spec_workspaces_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) DEFERRABLE
    TABLE "changeset_specs" CONSTRAINT "changeset_specs_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) DEFERRABLE
    TABLE "changesets" CONSTRAINT "changesets_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE
    TABLE "cm_last_searched" CONSTRAINT "cm_last_searched_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "codeintel_autoindexing_exceptions" CONSTRAINT "codeintel_autoindexing_exceptions_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "codeowners" CONSTRAINT "codeowners_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "discussion_threads_target_repo" CONSTRAINT "discussion_threads_target_repo_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "exhaustive_search_repo_jobs" CONSTRAINT "exhaustive_search_repo_jobs_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "external_service_repos" CONSTRAINT "external_service_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE
    TABLE "gitserver_repos" CONSTRAINT "gitserver_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "gitserver_repos_sync_output" CONSTRAINT "gitserver_repos_sync_output_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "lsif_index_configuration" CONSTRAINT "lsif_index_configuration_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "lsif_retention_configuration" CONSTRAINT "lsif_retention_configuration_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "permission_sync_jobs" CONSTRAINT "permission_sync_jobs_repository_id_fkey" FOREIGN KEY (repository_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "repo_commits_changelists" CONSTRAINT "repo_commits_changelists_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE
    TABLE "repo_kvps" CONSTRAINT "repo_kvps_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "repo_paths" CONSTRAINT "repo_paths_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE
    TABLE "search_context_repos" CONSTRAINT "search_context_repos_repo_id_fk" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "sub_repo_permissions" CONSTRAINT "sub_repo_permissions_repo_id_fk" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "user_public_repos" CONSTRAINT "user_public_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "user_repo_permissions" CONSTRAINT "user_repo_permissions_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    TABLE "zoekt_repos" CONSTRAINT "zoekt_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
Triggers:
    trig_create_zoekt_repo_on_repo_insert AFTER INSERT ON repo FOR EACH ROW EXECUTE FUNCTION func_insert_zoekt_repo()
    trig_delete_repo_ref_on_external_service_repos AFTER UPDATE OF deleted_at ON repo FOR EACH ROW EXECUTE FUNCTION delete_repo_ref_on_external_service_repos()
    trig_delete_user_repo_permissions_on_repo_soft_delete AFTER UPDATE ON repo FOR EACH ROW EXECUTE FUNCTION delete_user_repo_permissions_on_repo_soft_delete()
    trig_recalc_repo_statistics_on_repo_delete AFTER DELETE ON repo REFERENCING OLD TABLE AS oldtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_repo_statistics_on_repo_delete()
    trig_recalc_repo_statistics_on_repo_insert AFTER INSERT ON repo REFERENCING NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_repo_statistics_on_repo_insert()
    trig_recalc_repo_statistics_on_repo_update AFTER UPDATE ON repo REFERENCING OLD TABLE AS oldtab NEW TABLE AS newtab FOR EACH STATEMENT EXECUTE FUNCTION recalc_repo_statistics_on_repo_update()
    trigger_gitserver_repo_insert AFTER INSERT ON repo FOR EACH ROW EXECUTE FUNCTION func_insert_gitserver_repo()

```

# Table "public.repo_commits_changelists"
```
         Column         |           Type           | Collation | Nullable |                       Default                        
------------------------+--------------------------+-----------+----------+------------------------------------------------------
 id                     | integer                  |           | not null | nextval('repo_commits_changelists_id_seq'::regclass)
 repo_id                | integer                  |           | not null | 
 commit_sha             | bytea                    |           | not null | 
 perforce_changelist_id | integer                  |           | not null | 
 created_at             | timestamp with time zone |           | not null | now()
Indexes:
    "repo_commits_changelists_pkey" PRIMARY KEY, btree (id)
    "repo_id_perforce_changelist_id_unique" UNIQUE, btree (repo_id, perforce_changelist_id)
Foreign-key constraints:
    "repo_commits_changelists_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.repo_embedding_job_stats"
```
        Column        |  Type   | Collation | Nullable |   Default   
----------------------+---------+-----------+----------+-------------
 job_id               | integer |           | not null | 
 is_incremental       | boolean |           | not null | false
 code_files_total     | integer |           | not null | 0
 code_files_embedded  | integer |           | not null | 0
 code_chunks_embedded | integer |           | not null | 0
 code_files_skipped   | jsonb   |           | not null | '{}'::jsonb
 code_bytes_embedded  | bigint  |           | not null | 0
 text_files_total     | integer |           | not null | 0
 text_files_embedded  | integer |           | not null | 0
 text_chunks_embedded | integer |           | not null | 0
 text_files_skipped   | jsonb   |           | not null | '{}'::jsonb
 text_bytes_embedded  | bigint  |           | not null | 0
 code_chunks_excluded | integer |           | not null | 0
 text_chunks_excluded | integer |           | not null | 0
Indexes:
    "repo_embedding_job_stats_pkey" PRIMARY KEY, btree (job_id)
Foreign-key constraints:
    "repo_embedding_job_stats_job_id_fkey" FOREIGN KEY (job_id) REFERENCES repo_embedding_jobs(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.repo_embedding_jobs"
```
      Column       |           Type           | Collation | Nullable |                     Default                     
-------------------+--------------------------+-----------+----------+-------------------------------------------------
 id                | integer                  |           | not null | nextval('repo_embedding_jobs_id_seq'::regclass)
 state             | text                     |           |          | 'queued'::text
 failure_message   | text                     |           |          | 
 queued_at         | timestamp with time zone |           |          | now()
 started_at        | timestamp with time zone |           |          | 
 finished_at       | timestamp with time zone |           |          | 
 process_after     | timestamp with time zone |           |          | 
 num_resets        | integer                  |           | not null | 0
 num_failures      | integer                  |           | not null | 0
 last_heartbeat_at | timestamp with time zone |           |          | 
 execution_logs    | json[]                   |           |          | 
 worker_hostname   | text                     |           | not null | ''::text
 cancel            | boolean                  |           | not null | false
 repo_id           | integer                  |           | not null | 
 revision          | text                     |           | not null | 
Indexes:
    "repo_embedding_jobs_pkey" PRIMARY KEY, btree (id)
    "repo_embedding_jobs_repo" btree (repo_id, revision)
Referenced by:
    TABLE "repo_embedding_job_stats" CONSTRAINT "repo_embedding_job_stats_job_id_fkey" FOREIGN KEY (job_id) REFERENCES repo_embedding_jobs(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.repo_kvps"
```
 Column  |  Type   | Collation | Nullable | Default 
---------+---------+-----------+----------+---------
 repo_id | integer |           | not null | 
 key     | text    |           | not null | 
 value   | text    |           |          | 
Indexes:
    "repo_kvps_pkey" PRIMARY KEY, btree (repo_id, key) INCLUDE (value)
Foreign-key constraints:
    "repo_kvps_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE

```

# Table "public.repo_paths"
```
            Column            |            Type             | Collation | Nullable |                Default                 
------------------------------+-----------------------------+-----------+----------+----------------------------------------
 id                           | integer                     |           | not null | nextval('repo_paths_id_seq'::regclass)
 repo_id                      | integer                     |           | not null | 
 absolute_path                | text                        |           | not null | 
 parent_id                    | integer                     |           |          | 
 tree_files_count             | integer                     |           |          | 
 tree_files_counts_updated_at | timestamp without time zone |           |          | 
Indexes:
    "repo_paths_pkey" PRIMARY KEY, btree (id)
    "repo_paths_index_absolute_path" UNIQUE, btree (repo_id, absolute_path)
Foreign-key constraints:
    "repo_paths_parent_id_fkey" FOREIGN KEY (parent_id) REFERENCES repo_paths(id)
    "repo_paths_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE DEFERRABLE
Referenced by:
    TABLE "assigned_owners" CONSTRAINT "assigned_owners_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    TABLE "assigned_teams" CONSTRAINT "assigned_teams_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    TABLE "codeowners_individual_stats" CONSTRAINT "codeowners_individual_stats_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    TABLE "own_aggregate_recent_contribution" CONSTRAINT "own_aggregate_recent_contribution_changed_file_path_id_fkey" FOREIGN KEY (changed_file_path_id) REFERENCES repo_paths(id)
    TABLE "own_aggregate_recent_view" CONSTRAINT "own_aggregate_recent_view_viewed_file_path_id_fkey" FOREIGN KEY (viewed_file_path_id) REFERENCES repo_paths(id)
    TABLE "own_signal_recent_contribution" CONSTRAINT "own_signal_recent_contribution_changed_file_path_id_fkey" FOREIGN KEY (changed_file_path_id) REFERENCES repo_paths(id)
    TABLE "ownership_path_stats" CONSTRAINT "ownership_path_stats_file_path_id_fkey" FOREIGN KEY (file_path_id) REFERENCES repo_paths(id)
    TABLE "repo_paths" CONSTRAINT "repo_paths_parent_id_fkey" FOREIGN KEY (parent_id) REFERENCES repo_paths(id)

```

**absolute_path**: Absolute path does not start or end with forward slash. Example: &#34;a/b/c&#34;. Root directory is empty path &#34;&#34;.

**tree_files_count**: Total count of files in the file tree rooted at the path. 1 for files.

**tree_files_counts_updated_at**: Timestamp of the job that updated the file counts

# Table "public.repo_pending_permissions"
```
    Column     |           Type           | Collation | Nullable |     Default     
---------------+--------------------------+-----------+----------+-----------------
 repo_id       | integer                  |           | not null | 
 permission    | text                     |           | not null | 
 updated_at    | timestamp with time zone |           | not null | 
 user_ids_ints | bigint[]                 |           | not null | '{}'::integer[]
Indexes:
    "repo_pending_permissions_perm_unique" UNIQUE CONSTRAINT, btree (repo_id, permission)

```

# Table "public.repo_permissions"
```
    Column     |           Type           | Collation | Nullable |     Default     
---------------+--------------------------+-----------+----------+-----------------
 repo_id       | integer                  |           | not null | 
 permission    | text                     |           | not null | 
 updated_at    | timestamp with time zone |           | not null | 
 synced_at     | timestamp with time zone |           |          | 
 user_ids_ints | integer[]                |           | not null | '{}'::integer[]
 unrestricted  | boolean                  |           | not null | false
Indexes:
    "repo_permissions_perm_unique" UNIQUE CONSTRAINT, btree (repo_id, permission)
    "repo_permissions_unrestricted_true_idx" btree (unrestricted) WHERE unrestricted

```

# Table "public.repo_statistics"
```
    Column    |  Type  | Collation | Nullable | Default 
--------------+--------+-----------+----------+---------
 total        | bigint |           | not null | 0
 soft_deleted | bigint |           | not null | 0
 not_cloned   | bigint |           | not null | 0
 cloning      | bigint |           | not null | 0
 cloned       | bigint |           | not null | 0
 failed_fetch | bigint |           | not null | 0
 corrupted    | bigint |           | not null | 0

```

**cloned**: Number of repositories that are NOT soft-deleted and not blocked and cloned by gitserver

**cloning**: Number of repositories that are NOT soft-deleted and not blocked and currently being cloned by gitserver

**corrupted**: Number of repositories that are NOT soft-deleted and not blocked and have corrupted_at set in gitserver_repos table

**failed_fetch**: Number of repositories that are NOT soft-deleted and not blocked and have last_error set in gitserver_repos table

**not_cloned**: Number of repositories that are NOT soft-deleted and not blocked and not cloned by gitserver

**soft_deleted**: Number of repositories that are soft-deleted and not blocked

**total**: Number of repositories that are not soft-deleted and not blocked

# Table "public.role_permissions"
```
    Column     |           Type           | Collation | Nullable | Default 
---------------+--------------------------+-----------+----------+---------
 role_id       | integer                  |           | not null | 
 permission_id | integer                  |           | not null | 
 created_at    | timestamp with time zone |           | not null | now()
Indexes:
    "role_permissions_pkey" PRIMARY KEY, btree (permission_id, role_id)
Foreign-key constraints:
    "role_permissions_permission_id_fkey" FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE DEFERRABLE
    "role_permissions_role_id_fkey" FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.roles"
```
   Column   |           Type           | Collation | Nullable |              Default              
------------+--------------------------+-----------+----------+-----------------------------------
 id         | integer                  |           | not null | nextval('roles_id_seq'::regclass)
 created_at | timestamp with time zone |           | not null | now()
 system     | boolean                  |           | not null | false
 name       | citext                   |           | not null | 
Indexes:
    "roles_pkey" PRIMARY KEY, btree (id)
    "unique_role_name" UNIQUE, btree (name)
Referenced by:
    TABLE "role_permissions" CONSTRAINT "role_permissions_role_id_fkey" FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE DEFERRABLE
    TABLE "user_roles" CONSTRAINT "user_roles_role_id_fkey" FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE DEFERRABLE

```

**system**: This is used to indicate whether a role is read-only or can be modified.

# Table "public.saved_searches"
```
      Column       |           Type           | Collation | Nullable |                  Default                   
-------------------+--------------------------+-----------+----------+--------------------------------------------
 id                | integer                  |           | not null | nextval('saved_searches_id_seq'::regclass)
 description       | text                     |           | not null | 
 query             | text                     |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 notify_owner      | boolean                  |           | not null | 
 notify_slack      | boolean                  |           | not null | 
 user_id           | integer                  |           |          | 
 org_id            | integer                  |           |          | 
 slack_webhook_url | text                     |           |          | 
Indexes:
    "saved_searches_pkey" PRIMARY KEY, btree (id)
Check constraints:
    "saved_searches_notifications_disabled" CHECK (notify_owner = false AND notify_slack = false)
    "user_or_org_id_not_null" CHECK (user_id IS NOT NULL AND org_id IS NULL OR org_id IS NOT NULL AND user_id IS NULL)
Foreign-key constraints:
    "saved_searches_org_id_fkey" FOREIGN KEY (org_id) REFERENCES orgs(id)
    "saved_searches_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)

```

# Table "public.search_context_default"
```
      Column       |  Type   | Collation | Nullable | Default 
-------------------+---------+-----------+----------+---------
 user_id           | integer |           | not null | 
 search_context_id | bigint  |           | not null | 
Indexes:
    "search_context_default_pkey" PRIMARY KEY, btree (user_id)
Foreign-key constraints:
    "search_context_default_search_context_id_fkey" FOREIGN KEY (search_context_id) REFERENCES search_contexts(id) ON DELETE CASCADE DEFERRABLE
    "search_context_default_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

When a user sets a search context as default, a row is inserted into this table. A user can only have one default search context. If the user has not set their default search context, it will fall back to `global`.

# Table "public.search_context_repos"
```
      Column       |  Type   | Collation | Nullable | Default 
-------------------+---------+-----------+----------+---------
 search_context_id | bigint  |           | not null | 
 repo_id           | integer |           | not null | 
 revision          | text    |           | not null | 
Indexes:
    "search_context_repos_unique" UNIQUE CONSTRAINT, btree (repo_id, search_context_id, revision)
Foreign-key constraints:
    "search_context_repos_repo_id_fk" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    "search_context_repos_search_context_id_fk" FOREIGN KEY (search_context_id) REFERENCES search_contexts(id) ON DELETE CASCADE

```

# Table "public.search_context_stars"
```
      Column       |           Type           | Collation | Nullable | Default 
-------------------+--------------------------+-----------+----------+---------
 search_context_id | bigint                   |           | not null | 
 user_id           | integer                  |           | not null | 
 created_at        | timestamp with time zone |           | not null | now()
Indexes:
    "search_context_stars_pkey" PRIMARY KEY, btree (search_context_id, user_id)
Foreign-key constraints:
    "search_context_stars_search_context_id_fkey" FOREIGN KEY (search_context_id) REFERENCES search_contexts(id) ON DELETE CASCADE DEFERRABLE
    "search_context_stars_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

When a user stars a search context, a row is inserted into this table. If the user unstars the search context, the row is deleted. The global context is not in the database, and therefore cannot be starred.

# Table "public.search_contexts"
```
      Column       |           Type           | Collation | Nullable |                   Default                   
-------------------+--------------------------+-----------+----------+---------------------------------------------
 id                | bigint                   |           | not null | nextval('search_contexts_id_seq'::regclass)
 name              | citext                   |           | not null | 
 description       | text                     |           | not null | 
 public            | boolean                  |           | not null | 
 namespace_user_id | integer                  |           |          | 
 namespace_org_id  | integer                  |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 deleted_at        | timestamp with time zone |           |          | 
 query             | text                     |           |          | 
Indexes:
    "search_contexts_pkey" PRIMARY KEY, btree (id)
    "search_contexts_name_namespace_org_id_unique" UNIQUE, btree (name, namespace_org_id) WHERE namespace_org_id IS NOT NULL
    "search_contexts_name_namespace_user_id_unique" UNIQUE, btree (name, namespace_user_id) WHERE namespace_user_id IS NOT NULL
    "search_contexts_name_without_namespace_unique" UNIQUE, btree (name) WHERE namespace_user_id IS NULL AND namespace_org_id IS NULL
    "search_contexts_query_idx" btree (query)
Check constraints:
    "search_contexts_has_one_or_no_namespace" CHECK (namespace_user_id IS NULL OR namespace_org_id IS NULL)
Foreign-key constraints:
    "search_contexts_namespace_org_id_fk" FOREIGN KEY (namespace_org_id) REFERENCES orgs(id) ON DELETE CASCADE
    "search_contexts_namespace_user_id_fk" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
Referenced by:
    TABLE "search_context_default" CONSTRAINT "search_context_default_search_context_id_fkey" FOREIGN KEY (search_context_id) REFERENCES search_contexts(id) ON DELETE CASCADE DEFERRABLE
    TABLE "search_context_repos" CONSTRAINT "search_context_repos_search_context_id_fk" FOREIGN KEY (search_context_id) REFERENCES search_contexts(id) ON DELETE CASCADE
    TABLE "search_context_stars" CONSTRAINT "search_context_stars_search_context_id_fkey" FOREIGN KEY (search_context_id) REFERENCES search_contexts(id) ON DELETE CASCADE DEFERRABLE

```

**deleted_at**: This column is unused as of Sourcegraph 3.34. Do not refer to it anymore. It will be dropped in a future version.

# Table "public.security_event_logs"
```
      Column       |           Type           | Collation | Nullable |                     Default                     
-------------------+--------------------------+-----------+----------+-------------------------------------------------
 id                | bigint                   |           | not null | nextval('security_event_logs_id_seq'::regclass)
 name              | text                     |           | not null | 
 url               | text                     |           | not null | 
 user_id           | integer                  |           | not null | 
 anonymous_user_id | text                     |           | not null | 
 source            | text                     |           | not null | 
 argument          | jsonb                    |           | not null | 
 version           | text                     |           | not null | 
 timestamp         | timestamp with time zone |           | not null | 
Indexes:
    "security_event_logs_pkey" PRIMARY KEY, btree (id)
    "security_event_logs_timestamp" btree ("timestamp")
Check constraints:
    "security_event_logs_check_has_user" CHECK (user_id = 0 AND anonymous_user_id <> ''::text OR user_id <> 0 AND anonymous_user_id = ''::text OR user_id <> 0 AND anonymous_user_id <> ''::text)
    "security_event_logs_check_name_not_empty" CHECK (name <> ''::text)
    "security_event_logs_check_source_not_empty" CHECK (source <> ''::text)
    "security_event_logs_check_version_not_empty" CHECK (version <> ''::text)

```

Contains security-relevant events with a long time horizon for storage.

**anonymous_user_id**: The UUID of the actor associated with the event.

**argument**: An arbitrary JSON blob containing event data.

**name**: The event name as a CAPITALIZED_SNAKE_CASE string.

**source**: The site section (WEB, BACKEND, etc.) that generated the event.

**url**: The URL within the Sourcegraph app which generated the event.

**user_id**: The ID of the actor associated with the event.

**version**: The version of Sourcegraph which generated the event.

# Table "public.settings"
```
     Column     |           Type           | Collation | Nullable |               Default                
----------------+--------------------------+-----------+----------+--------------------------------------
 id             | integer                  |           | not null | nextval('settings_id_seq'::regclass)
 org_id         | integer                  |           |          | 
 contents       | text                     |           | not null | '{}'::text
 created_at     | timestamp with time zone |           | not null | now()
 user_id        | integer                  |           |          | 
 author_user_id | integer                  |           |          | 
Indexes:
    "settings_pkey" PRIMARY KEY, btree (id)
    "settings_global_id" btree (id DESC) WHERE user_id IS NULL AND org_id IS NULL
    "settings_org_id_idx" btree (org_id)
    "settings_user_id_idx" btree (user_id)
Check constraints:
    "settings_no_empty_contents" CHECK (contents <> ''::text)
Foreign-key constraints:
    "settings_author_user_id_fkey" FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT
    "settings_references_orgs" FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE RESTRICT
    "settings_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT

```

# Table "public.sub_repo_permissions"
```
   Column   |           Type           | Collation | Nullable | Default 
------------+--------------------------+-----------+----------+---------
 repo_id    | integer                  |           | not null | 
 user_id    | integer                  |           | not null | 
 version    | integer                  |           | not null | 1
 updated_at | timestamp with time zone |           | not null | now()
 paths      | text[]                   |           |          | 
Indexes:
    "sub_repo_permissions_repo_id_user_id_version_uindex" UNIQUE, btree (repo_id, user_id, version)
    "sub_repo_perms_user_id" btree (user_id)
Foreign-key constraints:
    "sub_repo_permissions_repo_id_fk" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    "sub_repo_permissions_users_id_fk" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

Responsible for storing permissions at a finer granularity than repo

**paths**: Paths that begin with a minus sign (-) are exclusion paths.

# Table "public.survey_responses"
```
     Column     |           Type           | Collation | Nullable |                   Default                    
----------------+--------------------------+-----------+----------+----------------------------------------------
 id             | bigint                   |           | not null | nextval('survey_responses_id_seq'::regclass)
 user_id        | integer                  |           |          | 
 email          | text                     |           |          | 
 score          | integer                  |           | not null | 
 reason         | text                     |           |          | 
 better         | text                     |           |          | 
 created_at     | timestamp with time zone |           | not null | now()
 use_cases      | text[]                   |           |          | 
 other_use_case | text                     |           |          | 
Indexes:
    "survey_responses_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "survey_responses_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)

```

# Table "public.syntactic_scip_indexing_jobs"
```
         Column         |           Type           | Collation | Nullable |                         Default                          
------------------------+--------------------------+-----------+----------+----------------------------------------------------------
 id                     | bigint                   |           | not null | nextval('syntactic_scip_indexing_jobs_id_seq'::regclass)
 commit                 | text                     |           | not null | 
 queued_at              | timestamp with time zone |           | not null | now()
 state                  | text                     |           | not null | 'queued'::text
 failure_message        | text                     |           |          | 
 started_at             | timestamp with time zone |           |          | 
 finished_at            | timestamp with time zone |           |          | 
 repository_id          | integer                  |           | not null | 
 process_after          | timestamp with time zone |           |          | 
 num_resets             | integer                  |           | not null | 0
 num_failures           | integer                  |           | not null | 0
 execution_logs         | json[]                   |           |          | 
 commit_last_checked_at | timestamp with time zone |           |          | 
 worker_hostname        | text                     |           | not null | ''::text
 last_heartbeat_at      | timestamp with time zone |           |          | 
 cancel                 | boolean                  |           | not null | false
 should_reindex         | boolean                  |           | not null | false
 enqueuer_user_id       | integer                  |           | not null | 0
Indexes:
    "syntactic_scip_indexing_jobs_pkey" PRIMARY KEY, btree (id)
    "syntactic_scip_indexing_jobs_dequeue_order_idx" btree ((enqueuer_user_id > 0) DESC, queued_at DESC, id) WHERE state = 'queued'::text OR state = 'errored'::text
    "syntactic_scip_indexing_jobs_queued_at_id" btree (queued_at DESC, id)
    "syntactic_scip_indexing_jobs_repository_id_commit" btree (repository_id, commit)
    "syntactic_scip_indexing_jobs_state" btree (state)
Check constraints:
    "syntactic_scip_indexing_jobs_commit_valid_chars" CHECK (commit ~ '^[a-f0-9]{40}$'::text)

```

Stores metadata about a code intel syntactic index job.

**commit**: A 40-char revhash. Note that this commit may not be resolvable in the future.

**enqueuer_user_id**: ID of the user who scheduled this index. Records with a non-NULL user ID are prioritised over the rest

**execution_logs**: An array of [log entries](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@3.23/-/blob/internal/workerutil/store.go#L48:6) (encoded as JSON) from the most recent execution.

# Table "public.syntactic_scip_last_index_scan"
```
       Column       |           Type           | Collation | Nullable | Default 
--------------------+--------------------------+-----------+----------+---------
 repository_id      | integer                  |           | not null | 
 last_index_scan_at | timestamp with time zone |           | not null | 
Indexes:
    "syntactic_scip_last_index_scan_pkey" PRIMARY KEY, btree (repository_id)

```

Tracks the last time repository was checked for syntactic indexing job scheduling.

**last_index_scan_at**: The last time uploads of this repository were considered for syntactic indexing job scheduling.

# Table "public.team_members"
```
   Column   |           Type           | Collation | Nullable | Default 
------------+--------------------------+-----------+----------+---------
 team_id    | integer                  |           | not null | 
 user_id    | integer                  |           | not null | 
 created_at | timestamp with time zone |           | not null | now()
 updated_at | timestamp with time zone |           | not null | now()
Indexes:
    "team_members_team_id_user_id_key" PRIMARY KEY, btree (team_id, user_id)
Foreign-key constraints:
    "team_members_team_id_fkey" FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
    "team_members_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.teams"
```
     Column     |           Type           | Collation | Nullable |              Default              
----------------+--------------------------+-----------+----------+-----------------------------------
 id             | integer                  |           | not null | nextval('teams_id_seq'::regclass)
 name           | citext                   |           | not null | 
 display_name   | text                     |           |          | 
 readonly       | boolean                  |           | not null | false
 parent_team_id | integer                  |           |          | 
 creator_id     | integer                  |           |          | 
 created_at     | timestamp with time zone |           | not null | now()
 updated_at     | timestamp with time zone |           | not null | now()
Indexes:
    "teams_pkey" PRIMARY KEY, btree (id)
    "teams_name" UNIQUE, btree (name)
Check constraints:
    "teams_display_name_max_length" CHECK (char_length(display_name) <= 255)
    "teams_name_max_length" CHECK (char_length(name::text) <= 255)
    "teams_name_valid_chars" CHECK (name ~ '^[a-zA-Z0-9](?:[a-zA-Z0-9]|[-.](?=[a-zA-Z0-9]))*-?$'::citext)
Foreign-key constraints:
    "teams_creator_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL
    "teams_parent_team_id_fkey" FOREIGN KEY (parent_team_id) REFERENCES teams(id) ON DELETE CASCADE
Referenced by:
    TABLE "assigned_teams" CONSTRAINT "assigned_teams_owner_team_id_fkey" FOREIGN KEY (owner_team_id) REFERENCES teams(id) ON DELETE CASCADE DEFERRABLE
    TABLE "names" CONSTRAINT "names_team_id_fkey" FOREIGN KEY (team_id) REFERENCES teams(id) ON UPDATE CASCADE ON DELETE CASCADE
    TABLE "team_members" CONSTRAINT "team_members_team_id_fkey" FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
    TABLE "teams" CONSTRAINT "teams_parent_team_id_fkey" FOREIGN KEY (parent_team_id) REFERENCES teams(id) ON DELETE CASCADE

```

# Table "public.telemetry_events_export_queue"
```
   Column    |           Type           | Collation | Nullable | Default 
-------------+--------------------------+-----------+----------+---------
 id          | text                     |           | not null | 
 timestamp   | timestamp with time zone |           | not null | 
 payload_pb  | bytea                    |           | not null | 
 exported_at | timestamp with time zone |           |          | 
Indexes:
    "telemetry_events_export_queue_pkey" PRIMARY KEY, btree (id)

```

# Table "public.temporary_settings"
```
   Column   |           Type           | Collation | Nullable |                    Default                     
------------+--------------------------+-----------+----------+------------------------------------------------
 id         | integer                  |           | not null | nextval('temporary_settings_id_seq'::regclass)
 user_id    | integer                  |           | not null | 
 contents   | jsonb                    |           |          | 
 created_at | timestamp with time zone |           | not null | now()
 updated_at | timestamp with time zone |           | not null | now()
Indexes:
    "temporary_settings_pkey" PRIMARY KEY, btree (id)
    "temporary_settings_user_id_key" UNIQUE CONSTRAINT, btree (user_id)
Foreign-key constraints:
    "temporary_settings_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

Stores per-user temporary settings used in the UI, for example, which modals have been dimissed or what theme is preferred.

**contents**: JSON-encoded temporary settings.

**user_id**: The ID of the user the settings will be saved for.

# Table "public.user_credentials"
```
        Column         |           Type           | Collation | Nullable |                   Default                    
-----------------------+--------------------------+-----------+----------+----------------------------------------------
 id                    | bigint                   |           | not null | nextval('user_credentials_id_seq'::regclass)
 domain                | text                     |           | not null | 
 user_id               | integer                  |           | not null | 
 external_service_type | text                     |           | not null | 
 external_service_id   | text                     |           | not null | 
 created_at            | timestamp with time zone |           | not null | now()
 updated_at            | timestamp with time zone |           | not null | now()
 credential            | bytea                    |           | not null | 
 ssh_migration_applied | boolean                  |           | not null | false
 encryption_key_id     | text                     |           | not null | ''::text
 github_app_id         | integer                  |           |          | 
Indexes:
    "user_credentials_pkey" PRIMARY KEY, btree (id)
    "user_credentials_domain_user_id_external_service_type_exter_key" UNIQUE CONSTRAINT, btree (domain, user_id, external_service_type, external_service_id)
    "user_credentials_credential_idx" btree ((encryption_key_id = ANY (ARRAY[''::text, 'previously-migrated'::text])))
Check constraints:
    "check_github_app_id_and_external_service_type" CHECK (github_app_id IS NULL OR external_service_type = 'github'::text)
Foreign-key constraints:
    "user_credentials_github_app_id_fkey" FOREIGN KEY (github_app_id) REFERENCES github_apps(id)
    "user_credentials_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.user_emails"
```
          Column           |           Type           | Collation | Nullable | Default 
---------------------------+--------------------------+-----------+----------+---------
 user_id                   | integer                  |           | not null | 
 email                     | citext                   |           | not null | 
 created_at                | timestamp with time zone |           | not null | now()
 verification_code         | text                     |           |          | 
 verified_at               | timestamp with time zone |           |          | 
 last_verification_sent_at | timestamp with time zone |           |          | 
 is_primary                | boolean                  |           | not null | false
Indexes:
    "user_emails_no_duplicates_per_user" UNIQUE CONSTRAINT, btree (user_id, email)
    "user_emails_user_id_is_primary_idx" UNIQUE, btree (user_id, is_primary) WHERE is_primary = true
    "user_emails_unique_verified_email" EXCLUDE USING btree (email) WHERE verified_at IS NOT NULL
Foreign-key constraints:
    "user_emails_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)

```

# Table "public.user_external_accounts"
```
      Column       |           Type           | Collation | Nullable |                      Default                       
-------------------+--------------------------+-----------+----------+----------------------------------------------------
 id                | integer                  |           | not null | nextval('user_external_accounts_id_seq'::regclass)
 user_id           | integer                  |           | not null | 
 service_type      | text                     |           | not null | 
 service_id        | text                     |           | not null | 
 account_id        | text                     |           | not null | 
 auth_data         | text                     |           |          | 
 account_data      | text                     |           |          | 
 created_at        | timestamp with time zone |           | not null | now()
 updated_at        | timestamp with time zone |           | not null | now()
 deleted_at        | timestamp with time zone |           |          | 
 client_id         | text                     |           | not null | 
 expired_at        | timestamp with time zone |           |          | 
 last_valid_at     | timestamp with time zone |           |          | 
 encryption_key_id | text                     |           | not null | ''::text
Indexes:
    "user_external_accounts_pkey" PRIMARY KEY, btree (id)
    "user_external_accounts_account" UNIQUE, btree (service_type, service_id, client_id, account_id) WHERE deleted_at IS NULL
    "user_external_accounts_user_id_scim_service_type" UNIQUE, btree (user_id, service_type) WHERE service_type = 'scim'::text AND deleted_at IS NULL
    "user_external_accounts_user_id" btree (user_id) WHERE deleted_at IS NULL
Foreign-key constraints:
    "user_external_accounts_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
Referenced by:
    TABLE "user_repo_permissions" CONSTRAINT "user_repo_permissions_user_external_account_id_fkey" FOREIGN KEY (user_external_account_id) REFERENCES user_external_accounts(id) ON DELETE CASCADE
Triggers:
    trig_delete_user_repo_permissions_on_external_account_soft_dele AFTER UPDATE ON user_external_accounts FOR EACH ROW EXECUTE FUNCTION delete_user_repo_permissions_on_external_account_soft_delete()

```

# Table "public.user_onboarding_tour"
```
   Column   |            Type             | Collation | Nullable |                     Default                      
------------+-----------------------------+-----------+----------+--------------------------------------------------
 id         | integer                     |           | not null | nextval('user_onboarding_tour_id_seq'::regclass)
 raw_json   | text                        |           | not null | 
 created_at | timestamp without time zone |           | not null | now()
 updated_by | integer                     |           |          | 
Indexes:
    "user_onboarding_tour_pkey" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "user_onboarding_tour_users_fk" FOREIGN KEY (updated_by) REFERENCES users(id)

```

# Table "public.user_pending_permissions"
```
     Column      |           Type           | Collation | Nullable |                       Default                        
-----------------+--------------------------+-----------+----------+------------------------------------------------------
 id              | bigint                   |           | not null | nextval('user_pending_permissions_id_seq'::regclass)
 bind_id         | text                     |           | not null | 
 permission      | text                     |           | not null | 
 object_type     | text                     |           | not null | 
 updated_at      | timestamp with time zone |           | not null | 
 service_type    | text                     |           | not null | 
 service_id      | text                     |           | not null | 
 object_ids_ints | integer[]                |           | not null | '{}'::integer[]
Indexes:
    "user_pending_permissions_service_perm_object_unique" UNIQUE CONSTRAINT, btree (service_type, service_id, permission, object_type, bind_id)

```

# Table "public.user_permissions"
```
     Column      |           Type           | Collation | Nullable |     Default     
-----------------+--------------------------+-----------+----------+-----------------
 user_id         | integer                  |           | not null | 
 permission      | text                     |           | not null | 
 object_type     | text                     |           | not null | 
 updated_at      | timestamp with time zone |           | not null | 
 synced_at       | timestamp with time zone |           |          | 
 object_ids_ints | integer[]                |           | not null | '{}'::integer[]
 migrated        | boolean                  |           |          | true
Indexes:
    "user_permissions_perm_object_unique" UNIQUE CONSTRAINT, btree (user_id, permission, object_type)

```

# Table "public.user_public_repos"
```
  Column  |  Type   | Collation | Nullable | Default 
----------+---------+-----------+----------+---------
 user_id  | integer |           | not null | 
 repo_uri | text    |           | not null | 
 repo_id  | integer |           | not null | 
Indexes:
    "user_public_repos_user_id_repo_id_key" UNIQUE CONSTRAINT, btree (user_id, repo_id)
Foreign-key constraints:
    "user_public_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    "user_public_repos_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.user_repo_permissions"
```
          Column          |           Type           | Collation | Nullable |                      Default                      
--------------------------+--------------------------+-----------+----------+---------------------------------------------------
 id                       | bigint                   |           | not null | nextval('user_repo_permissions_id_seq'::regclass)
 user_id                  | integer                  |           |          | 
 repo_id                  | integer                  |           | not null | 
 user_external_account_id | integer                  |           |          | 
 created_at               | timestamp with time zone |           | not null | now()
 updated_at               | timestamp with time zone |           | not null | now()
 source                   | text                     |           | not null | 'sync'::text
Indexes:
    "user_repo_permissions_pkey" PRIMARY KEY, btree (id)
    "user_repo_permissions_perms_unique_idx" UNIQUE, btree (user_id, user_external_account_id, repo_id)
    "user_repo_permissions_repo_id_idx" btree (repo_id)
    "user_repo_permissions_source_idx" btree (source)
    "user_repo_permissions_updated_at_idx" btree (updated_at)
    "user_repo_permissions_user_external_account_id_idx" btree (user_external_account_id)
Foreign-key constraints:
    "user_repo_permissions_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE
    "user_repo_permissions_user_external_account_id_fkey" FOREIGN KEY (user_external_account_id) REFERENCES user_external_accounts(id) ON DELETE CASCADE
    "user_repo_permissions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

```

# Table "public.user_roles"
```
   Column   |           Type           | Collation | Nullable | Default 
------------+--------------------------+-----------+----------+---------
 user_id    | integer                  |           | not null | 
 role_id    | integer                  |           | not null | 
 created_at | timestamp with time zone |           | not null | now()
Indexes:
    "user_roles_pkey" PRIMARY KEY, btree (user_id, role_id)
Foreign-key constraints:
    "user_roles_role_id_fkey" FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE DEFERRABLE
    "user_roles_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE

```

# Table "public.users"
```
         Column          |           Type           | Collation | Nullable |              Default              
-------------------------+--------------------------+-----------+----------+-----------------------------------
 id                      | integer                  |           | not null | nextval('users_id_seq'::regclass)
 username                | citext                   |           | not null | 
 display_name            | text                     |           |          | 
 avatar_url              | text                     |           |          | 
 created_at              | timestamp with time zone |           | not null | now()
 updated_at              | timestamp with time zone |           | not null | now()
 deleted_at              | timestamp with time zone |           |          | 
 invite_quota            | integer                  |           | not null | 100
 passwd                  | text                     |           |          | 
 passwd_reset_code       | text                     |           |          | 
 passwd_reset_time       | timestamp with time zone |           |          | 
 site_admin              | boolean                  |           | not null | false
 page_views              | integer                  |           | not null | 0
 search_queries          | integer                  |           | not null | 0
 billing_customer_id     | text                     |           |          | 
 invalidated_sessions_at | timestamp with time zone |           | not null | now()
 tos_accepted            | boolean                  |           | not null | false
 completions_quota       | integer                  |           |          | 
 code_completions_quota  | integer                  |           |          | 
 completed_post_signup   | boolean                  |           | not null | false
 cody_pro_enabled_at     | timestamp with time zone |           |          | 
Indexes:
    "users_pkey" PRIMARY KEY, btree (id)
    "users_billing_customer_id" UNIQUE, btree (billing_customer_id) WHERE deleted_at IS NULL
    "users_username" UNIQUE, btree (username) WHERE deleted_at IS NULL
    "users_created_at_idx" btree (created_at)
Check constraints:
    "users_display_name_max_length" CHECK (char_length(display_name) <= 255)
    "users_username_max_length" CHECK (char_length(username::text) <= 255)
    "users_username_valid_chars" CHECK (username ~ '^\w(?:\w|[-.](?=\w))*-?$'::citext)
Referenced by:
    TABLE "access_requests" CONSTRAINT "access_requests_decision_by_user_id_fkey" FOREIGN KEY (decision_by_user_id) REFERENCES users(id) ON DELETE SET NULL
    TABLE "access_tokens" CONSTRAINT "access_tokens_creator_user_id_fkey" FOREIGN KEY (creator_user_id) REFERENCES users(id)
    TABLE "access_tokens" CONSTRAINT "access_tokens_subject_user_id_fkey" FOREIGN KEY (subject_user_id) REFERENCES users(id)
    TABLE "aggregated_user_statistics" CONSTRAINT "aggregated_user_statistics_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "assigned_owners" CONSTRAINT "assigned_owners_owner_user_id_fkey" FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "assigned_owners" CONSTRAINT "assigned_owners_who_assigned_user_id_fkey" FOREIGN KEY (who_assigned_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "assigned_teams" CONSTRAINT "assigned_teams_who_assigned_team_id_fkey" FOREIGN KEY (who_assigned_team_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "batch_changes" CONSTRAINT "batch_changes_initial_applier_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "batch_changes" CONSTRAINT "batch_changes_last_applier_id_fkey" FOREIGN KEY (last_applier_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "batch_changes" CONSTRAINT "batch_changes_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "batch_spec_execution_cache_entries" CONSTRAINT "batch_spec_execution_cache_entries_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "batch_spec_resolution_jobs" CONSTRAINT "batch_spec_resolution_jobs_initiator_id_fkey" FOREIGN KEY (initiator_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE
    TABLE "batch_spec_workspace_execution_last_dequeues" CONSTRAINT "batch_spec_workspace_execution_last_dequeues_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
    TABLE "batch_specs" CONSTRAINT "batch_specs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "changeset_jobs" CONSTRAINT "changeset_jobs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "changeset_specs" CONSTRAINT "changeset_specs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "cm_emails" CONSTRAINT "cm_emails_changed_by_fk" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_emails" CONSTRAINT "cm_emails_created_by_fk" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_monitors" CONSTRAINT "cm_monitors_changed_by_fk" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_monitors" CONSTRAINT "cm_monitors_created_by_fk" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_monitors" CONSTRAINT "cm_monitors_user_id_fk" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_recipients" CONSTRAINT "cm_recipients_user_id_fk" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_slack_webhooks" CONSTRAINT "cm_slack_webhooks_changed_by_fkey" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_slack_webhooks" CONSTRAINT "cm_slack_webhooks_created_by_fkey" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_queries" CONSTRAINT "cm_triggers_changed_by_fk" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_queries" CONSTRAINT "cm_triggers_created_by_fk" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_webhooks" CONSTRAINT "cm_webhooks_changed_by_fkey" FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "cm_webhooks" CONSTRAINT "cm_webhooks_created_by_fkey" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
    TABLE "discussion_comments" CONSTRAINT "discussion_comments_author_user_id_fkey" FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT
    TABLE "discussion_mail_reply_tokens" CONSTRAINT "discussion_mail_reply_tokens_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
    TABLE "discussion_threads" CONSTRAINT "discussion_threads_author_user_id_fkey" FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT
    TABLE "executor_secret_access_logs" CONSTRAINT "executor_secret_access_logs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "executor_secrets" CONSTRAINT "executor_secrets_creator_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL
    TABLE "executor_secrets" CONSTRAINT "executor_secrets_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "exhaustive_search_jobs" CONSTRAINT "exhaustive_search_jobs_initiator_id_fkey" FOREIGN KEY (initiator_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE DEFERRABLE
    TABLE "external_services" CONSTRAINT "external_services_creator_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "external_services" CONSTRAINT "external_services_last_updater_id_fkey" FOREIGN KEY (last_updater_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "feature_flag_overrides" CONSTRAINT "feature_flag_overrides_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "names" CONSTRAINT "names_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE
    TABLE "namespace_permissions" CONSTRAINT "namespace_permissions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "notebook_stars" CONSTRAINT "notebook_stars_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "notebooks" CONSTRAINT "notebooks_creator_user_id_fkey" FOREIGN KEY (creator_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "notebooks" CONSTRAINT "notebooks_namespace_user_id_fkey" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "notebooks" CONSTRAINT "notebooks_updater_user_id_fkey" FOREIGN KEY (updater_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "org_invitations" CONSTRAINT "org_invitations_recipient_user_id_fkey" FOREIGN KEY (recipient_user_id) REFERENCES users(id)
    TABLE "org_invitations" CONSTRAINT "org_invitations_sender_user_id_fkey" FOREIGN KEY (sender_user_id) REFERENCES users(id)
    TABLE "org_members" CONSTRAINT "org_members_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
    TABLE "outbound_webhooks" CONSTRAINT "outbound_webhooks_created_by_fkey" FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
    TABLE "outbound_webhooks" CONSTRAINT "outbound_webhooks_updated_by_fkey" FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
    TABLE "own_aggregate_recent_view" CONSTRAINT "own_aggregate_recent_view_viewer_id_fkey" FOREIGN KEY (viewer_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "permission_sync_jobs" CONSTRAINT "permission_sync_jobs_triggered_by_user_id_fkey" FOREIGN KEY (triggered_by_user_id) REFERENCES users(id) ON DELETE SET NULL DEFERRABLE
    TABLE "permission_sync_jobs" CONSTRAINT "permission_sync_jobs_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "product_subscriptions" CONSTRAINT "product_subscriptions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
    TABLE "registry_extension_releases" CONSTRAINT "registry_extension_releases_creator_user_id_fkey" FOREIGN KEY (creator_user_id) REFERENCES users(id)
    TABLE "registry_extensions" CONSTRAINT "registry_extensions_publisher_user_id_fkey" FOREIGN KEY (publisher_user_id) REFERENCES users(id)
    TABLE "saved_searches" CONSTRAINT "saved_searches_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
    TABLE "search_context_default" CONSTRAINT "search_context_default_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "search_context_stars" CONSTRAINT "search_context_stars_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "search_contexts" CONSTRAINT "search_contexts_namespace_user_id_fk" FOREIGN KEY (namespace_user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "settings" CONSTRAINT "settings_author_user_id_fkey" FOREIGN KEY (author_user_id) REFERENCES users(id) ON DELETE RESTRICT
    TABLE "settings" CONSTRAINT "settings_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
    TABLE "sub_repo_permissions" CONSTRAINT "sub_repo_permissions_users_id_fk" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "survey_responses" CONSTRAINT "survey_responses_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
    TABLE "team_members" CONSTRAINT "team_members_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "teams" CONSTRAINT "teams_creator_id_fkey" FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE SET NULL
    TABLE "temporary_settings" CONSTRAINT "temporary_settings_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "user_credentials" CONSTRAINT "user_credentials_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "user_emails" CONSTRAINT "user_emails_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
    TABLE "user_external_accounts" CONSTRAINT "user_external_accounts_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id)
    TABLE "user_onboarding_tour" CONSTRAINT "user_onboarding_tour_users_fk" FOREIGN KEY (updated_by) REFERENCES users(id)
    TABLE "user_public_repos" CONSTRAINT "user_public_repos_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "user_repo_permissions" CONSTRAINT "user_repo_permissions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    TABLE "user_roles" CONSTRAINT "user_roles_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE DEFERRABLE
    TABLE "webhooks" CONSTRAINT "webhooks_created_by_user_id_fkey" FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL
    TABLE "webhooks" CONSTRAINT "webhooks_updated_by_user_id_fkey" FOREIGN KEY (updated_by_user_id) REFERENCES users(id) ON DELETE SET NULL
Triggers:
    trig_delete_user_repo_permissions_on_user_soft_delete AFTER UPDATE ON users FOR EACH ROW EXECUTE FUNCTION delete_user_repo_permissions_on_user_soft_delete()
    trig_invalidate_session_on_password_change BEFORE UPDATE OF passwd ON users FOR EACH ROW EXECUTE FUNCTION invalidate_session_for_userid_on_password_change()

```

# Table "public.versions"
```
    Column     |           Type           | Collation | Nullable | Default 
---------------+--------------------------+-----------+----------+---------
 service       | text                     |           | not null | 
 version       | text                     |           | not null | 
 updated_at    | timestamp with time zone |           | not null | now()
 first_version | text                     |           | not null | 
 auto_upgrade  | boolean                  |           | not null | false
Indexes:
    "versions_pkey" PRIMARY KEY, btree (service)
Triggers:
    versions_insert BEFORE INSERT ON versions FOR EACH ROW EXECUTE FUNCTION versions_insert_row_trigger()

```

# Table "public.vulnerabilities"
```
    Column    |           Type           | Collation | Nullable |                   Default                   
--------------+--------------------------+-----------+----------+---------------------------------------------
 id           | integer                  |           | not null | nextval('vulnerabilities_id_seq'::regclass)
 source_id    | text                     |           | not null | 
 summary      | text                     |           | not null | 
 details      | text                     |           | not null | 
 cpes         | text[]                   |           | not null | 
 cwes         | text[]                   |           | not null | 
 aliases      | text[]                   |           | not null | 
 related      | text[]                   |           | not null | 
 data_source  | text                     |           | not null | 
 urls         | text[]                   |           | not null | 
 severity     | text                     |           | not null | 
 cvss_vector  | text                     |           | not null | 
 cvss_score   | text                     |           | not null | 
 published_at | timestamp with time zone |           | not null | 
 modified_at  | timestamp with time zone |           |          | 
 withdrawn_at | timestamp with time zone |           |          | 
Indexes:
    "vulnerabilities_pkey" PRIMARY KEY, btree (id)
    "vulnerabilities_source_id" UNIQUE, btree (source_id)
Referenced by:
    TABLE "vulnerability_affected_packages" CONSTRAINT "fk_vulnerabilities" FOREIGN KEY (vulnerability_id) REFERENCES vulnerabilities(id) ON DELETE CASCADE

```

# Table "public.vulnerability_affected_packages"
```
       Column       |  Type   | Collation | Nullable |                           Default                           
--------------------+---------+-----------+----------+-------------------------------------------------------------
 id                 | integer |           | not null | nextval('vulnerability_affected_packages_id_seq'::regclass)
 vulnerability_id   | integer |           | not null | 
 package_name       | text    |           | not null | 
 language           | text    |           | not null | 
 namespace          | text    |           | not null | 
 version_constraint | text[]  |           | not null | 
 fixed              | boolean |           | not null | 
 fixed_in           | text    |           |          | 
Indexes:
    "vulnerability_affected_packages_pkey" PRIMARY KEY, btree (id)
    "vulnerability_affected_packages_vulnerability_id_package_name" UNIQUE, btree (vulnerability_id, package_name)
Foreign-key constraints:
    "fk_vulnerabilities" FOREIGN KEY (vulnerability_id) REFERENCES vulnerabilities(id) ON DELETE CASCADE
Referenced by:
    TABLE "vulnerability_affected_symbols" CONSTRAINT "fk_vulnerability_affected_packages" FOREIGN KEY (vulnerability_affected_package_id) REFERENCES vulnerability_affected_packages(id) ON DELETE CASCADE
    TABLE "vulnerability_matches" CONSTRAINT "fk_vulnerability_affected_packages" FOREIGN KEY (vulnerability_affected_package_id) REFERENCES vulnerability_affected_packages(id) ON DELETE CASCADE

```

# Table "public.vulnerability_affected_symbols"
```
              Column               |  Type   | Collation | Nullable |                          Default                           
-----------------------------------+---------+-----------+----------+------------------------------------------------------------
 id                                | integer |           | not null | nextval('vulnerability_affected_symbols_id_seq'::regclass)
 vulnerability_affected_package_id | integer |           | not null | 
 path                              | text    |           | not null | 
 symbols                           | text[]  |           | not null | 
Indexes:
    "vulnerability_affected_symbols_pkey" PRIMARY KEY, btree (id)
    "vulnerability_affected_symbols_vulnerability_affected_package_i" UNIQUE, btree (vulnerability_affected_package_id, path)
Foreign-key constraints:
    "fk_vulnerability_affected_packages" FOREIGN KEY (vulnerability_affected_package_id) REFERENCES vulnerability_affected_packages(id) ON DELETE CASCADE

```

# Table "public.vulnerability_matches"
```
              Column               |  Type   | Collation | Nullable |                      Default                      
-----------------------------------+---------+-----------+----------+---------------------------------------------------
 id                                | integer |           | not null | nextval('vulnerability_matches_id_seq'::regclass)
 upload_id                         | integer |           | not null | 
 vulnerability_affected_package_id | integer |           | not null | 
Indexes:
    "vulnerability_matches_pkey" PRIMARY KEY, btree (id)
    "vulnerability_matches_upload_id_vulnerability_affected_package_" UNIQUE, btree (upload_id, vulnerability_affected_package_id)
    "vulnerability_matches_vulnerability_affected_package_id" btree (vulnerability_affected_package_id)
Foreign-key constraints:
    "fk_upload" FOREIGN KEY (upload_id) REFERENCES lsif_uploads(id) ON DELETE CASCADE
    "fk_vulnerability_affected_packages" FOREIGN KEY (vulnerability_affected_package_id) REFERENCES vulnerability_affected_packages(id) ON DELETE CASCADE

```

# Table "public.webhook_logs"
```
       Column        |           Type           | Collation | Nullable |                 Default                  
---------------------+--------------------------+-----------+----------+------------------------------------------
 id                  | bigint                   |           | not null | nextval('webhook_logs_id_seq'::regclass)
 received_at         | timestamp with time zone |           | not null | now()
 external_service_id | integer                  |           |          | 
 status_code         | integer                  |           | not null | 
 request             | bytea                    |           | not null | 
 response            | bytea                    |           | not null | 
 encryption_key_id   | text                     |           | not null | 
 webhook_id          | integer                  |           |          | 
Indexes:
    "webhook_logs_pkey" PRIMARY KEY, btree (id)
    "webhook_logs_external_service_id_idx" btree (external_service_id)
    "webhook_logs_received_at_idx" btree (received_at)
    "webhook_logs_status_code_idx" btree (status_code)
Foreign-key constraints:
    "webhook_logs_external_service_id_fkey" FOREIGN KEY (external_service_id) REFERENCES external_services(id) ON UPDATE CASCADE ON DELETE CASCADE
    "webhook_logs_webhook_id_fkey" FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE

```

# Table "public.webhooks"
```
       Column       |           Type           | Collation | Nullable |               Default                
--------------------+--------------------------+-----------+----------+--------------------------------------
 id                 | integer                  |           | not null | nextval('webhooks_id_seq'::regclass)
 code_host_kind     | text                     |           | not null | 
 code_host_urn      | text                     |           | not null | 
 secret             | text                     |           |          | 
 created_at         | timestamp with time zone |           | not null | now()
 updated_at         | timestamp with time zone |           | not null | now()
 encryption_key_id  | text                     |           |          | 
 uuid               | uuid                     |           | not null | gen_random_uuid()
 created_by_user_id | integer                  |           |          | 
 updated_by_user_id | integer                  |           |          | 
 name               | text                     |           | not null | 
Indexes:
    "webhooks_pkey" PRIMARY KEY, btree (id)
    "webhooks_uuid_key" UNIQUE CONSTRAINT, btree (uuid)
Foreign-key constraints:
    "webhooks_created_by_user_id_fkey" FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL
    "webhooks_updated_by_user_id_fkey" FOREIGN KEY (updated_by_user_id) REFERENCES users(id) ON DELETE SET NULL
Referenced by:
    TABLE "github_apps" CONSTRAINT "github_apps_webhook_id_fkey" FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE SET NULL
    TABLE "webhook_logs" CONSTRAINT "webhook_logs_webhook_id_fkey" FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE

```

Webhooks registered in Sourcegraph instance.

**code_host_kind**: Kind of an external service for which webhooks are registered.

**code_host_urn**: URN of a code host. This column maps to external_service_id column of repo table.

**created_by_user_id**: ID of a user, who created the webhook. If NULL, then the user does not exist (never existed or was deleted).

**name**: Descriptive name of a webhook.

**secret**: Secret used to decrypt webhook payload (if supported by the code host).

**updated_by_user_id**: ID of a user, who updated the webhook. If NULL, then the user does not exist (never existed or was deleted).

# Table "public.zoekt_repos"
```
     Column      |           Type           | Collation | Nullable |       Default       
-----------------+--------------------------+-----------+----------+---------------------
 repo_id         | integer                  |           | not null | 
 branches        | jsonb                    |           | not null | '[]'::jsonb
 index_status    | text                     |           | not null | 'not_indexed'::text
 updated_at      | timestamp with time zone |           | not null | now()
 created_at      | timestamp with time zone |           | not null | now()
 last_indexed_at | timestamp with time zone |           |          | 
Indexes:
    "zoekt_repos_pkey" PRIMARY KEY, btree (repo_id)
    "zoekt_repos_index_status" btree (index_status)
Foreign-key constraints:
    "zoekt_repos_repo_id_fkey" FOREIGN KEY (repo_id) REFERENCES repo(id) ON DELETE CASCADE

```

# View "public.batch_spec_workspace_execution_jobs_with_rank"

## View query:

```sql
 SELECT j.id,
    j.batch_spec_workspace_id,
    j.state,
    j.failure_message,
    j.started_at,
    j.finished_at,
    j.process_after,
    j.num_resets,
    j.num_failures,
    j.execution_logs,
    j.worker_hostname,
    j.last_heartbeat_at,
    j.created_at,
    j.updated_at,
    j.cancel,
    j.queued_at,
    j.user_id,
    j.version,
    q.place_in_global_queue,
    q.place_in_user_queue
   FROM (batch_spec_workspace_execution_jobs j
     LEFT JOIN batch_spec_workspace_execution_queue q ON ((j.id = q.id)));
```

# View "public.batch_spec_workspace_execution_queue"

## View query:

```sql
 WITH queue_candidates AS (
         SELECT exec.id,
            rank() OVER (PARTITION BY queue.user_id ORDER BY exec.created_at, exec.id) AS place_in_user_queue
           FROM (batch_spec_workspace_execution_jobs exec
             JOIN batch_spec_workspace_execution_last_dequeues queue ON ((queue.user_id = exec.user_id)))
          WHERE (exec.state = 'queued'::text)
          ORDER BY (rank() OVER (PARTITION BY queue.user_id ORDER BY exec.created_at, exec.id)), queue.latest_dequeue NULLS FIRST
        )
 SELECT queue_candidates.id,
    row_number() OVER () AS place_in_global_queue,
    queue_candidates.place_in_user_queue
   FROM queue_candidates;
```

# View "public.branch_changeset_specs_and_changesets"

## View query:

```sql
 SELECT changeset_specs.id AS changeset_spec_id,
    COALESCE(changesets.id, (0)::bigint) AS changeset_id,
    changeset_specs.repo_id,
    changeset_specs.batch_spec_id,
    changesets.owned_by_batch_change_id AS owner_batch_change_id,
    repo.name AS repo_name,
    changeset_specs.title AS changeset_name,
    changesets.external_state,
    changesets.publication_state,
    changesets.reconciler_state,
    changesets.computed_state
   FROM ((changeset_specs
     LEFT JOIN changesets ON (((changesets.repo_id = changeset_specs.repo_id) AND (changesets.current_spec_id IS NOT NULL) AND (EXISTS ( SELECT 1
           FROM changeset_specs changeset_specs_1
          WHERE ((changeset_specs_1.id = changesets.current_spec_id) AND (changeset_specs_1.head_ref = changeset_specs.head_ref)))))))
     JOIN repo ON ((changeset_specs.repo_id = repo.id)))
  WHERE ((changeset_specs.external_id IS NULL) AND (repo.deleted_at IS NULL));
```

# View "public.codeintel_configuration_policies"

## View query:

```sql
 SELECT lsif_configuration_policies.id,
    lsif_configuration_policies.repository_id,
    lsif_configuration_policies.name,
    lsif_configuration_policies.type,
    lsif_configuration_policies.pattern,
    lsif_configuration_policies.retention_enabled,
    lsif_configuration_policies.retention_duration_hours,
    lsif_configuration_policies.retain_intermediate_commits,
    lsif_configuration_policies.indexing_enabled,
    lsif_configuration_policies.index_commit_max_age_hours,
    lsif_configuration_policies.index_intermediate_commits,
    lsif_configuration_policies.protected,
    lsif_configuration_policies.repository_patterns,
    lsif_configuration_policies.last_resolved_at,
    lsif_configuration_policies.embeddings_enabled
   FROM lsif_configuration_policies;
```

# View "public.codeintel_configuration_policies_repository_pattern_lookup"

## View query:

```sql
 SELECT lsif_configuration_policies_repository_pattern_lookup.policy_id,
    lsif_configuration_policies_repository_pattern_lookup.repo_id
   FROM lsif_configuration_policies_repository_pattern_lookup;
```

# View "public.external_service_sync_jobs_with_next_sync_at"

## View query:

```sql
 SELECT j.id,
    j.state,
    j.failure_message,
    j.queued_at,
    j.started_at,
    j.finished_at,
    j.process_after,
    j.num_resets,
    j.num_failures,
    j.execution_logs,
    j.external_service_id,
    e.next_sync_at
   FROM (external_services e
     JOIN external_service_sync_jobs j ON ((e.id = j.external_service_id)));
```

# View "public.gitserver_relocator_jobs_with_repo_name"

## View query:

```sql
 SELECT glj.id,
    glj.state,
    glj.queued_at,
    glj.failure_message,
    glj.started_at,
    glj.finished_at,
    glj.process_after,
    glj.num_resets,
    glj.num_failures,
    glj.last_heartbeat_at,
    glj.execution_logs,
    glj.worker_hostname,
    glj.repo_id,
    glj.source_hostname,
    glj.dest_hostname,
    glj.delete_source,
    r.name AS repo_name
   FROM (gitserver_relocator_jobs glj
     JOIN repo r ON ((r.id = glj.repo_id)));
```

# View "public.lsif_dumps"

## View query:

```sql
 SELECT u.id,
    u.commit,
    u.root,
    u.queued_at,
    u.uploaded_at,
    u.state,
    u.failure_message,
    u.started_at,
    u.finished_at,
    u.repository_id,
    u.indexer,
    u.indexer_version,
    u.num_parts,
    u.uploaded_parts,
    u.process_after,
    u.num_resets,
    u.upload_size,
    u.num_failures,
    u.associated_index_id,
    u.expired,
    u.last_retention_scan_at,
    u.finished_at AS processed_at
   FROM lsif_uploads u
  WHERE ((u.state = 'completed'::text) OR (u.state = 'deleting'::text));
```

# View "public.lsif_dumps_with_repository_name"

## View query:

```sql
 SELECT u.id,
    u.commit,
    u.root,
    u.queued_at,
    u.uploaded_at,
    u.state,
    u.failure_message,
    u.started_at,
    u.finished_at,
    u.repository_id,
    u.indexer,
    u.indexer_version,
    u.num_parts,
    u.uploaded_parts,
    u.process_after,
    u.num_resets,
    u.upload_size,
    u.num_failures,
    u.associated_index_id,
    u.expired,
    u.last_retention_scan_at,
    u.processed_at,
    r.name AS repository_name
   FROM (lsif_dumps u
     JOIN repo r ON ((r.id = u.repository_id)))
  WHERE (r.deleted_at IS NULL);
```

# View "public.lsif_indexes_with_repository_name"

## View query:

```sql
 SELECT u.id,
    u.commit,
    u.queued_at,
    u.state,
    u.failure_message,
    u.started_at,
    u.finished_at,
    u.repository_id,
    u.process_after,
    u.num_resets,
    u.num_failures,
    u.docker_steps,
    u.root,
    u.indexer,
    u.indexer_args,
    u.outfile,
    u.log_contents,
    u.execution_logs,
    u.local_steps,
    u.should_reindex,
    u.requested_envvars,
    r.name AS repository_name,
    u.enqueuer_user_id
   FROM (lsif_indexes u
     JOIN repo r ON ((r.id = u.repository_id)))
  WHERE (r.deleted_at IS NULL);
```

# View "public.lsif_uploads_with_repository_name"

## View query:

```sql
 SELECT u.id,
    u.commit,
    u.root,
    u.queued_at,
    u.uploaded_at,
    u.state,
    u.failure_message,
    u.started_at,
    u.finished_at,
    u.repository_id,
    u.indexer,
    u.indexer_version,
    u.num_parts,
    u.uploaded_parts,
    u.process_after,
    u.num_resets,
    u.upload_size,
    u.num_failures,
    u.associated_index_id,
    u.content_type,
    u.should_reindex,
    u.expired,
    u.last_retention_scan_at,
    r.name AS repository_name,
    u.uncompressed_size
   FROM (lsif_uploads u
     JOIN repo r ON ((r.id = u.repository_id)))
  WHERE (r.deleted_at IS NULL);
```

# View "public.outbound_webhooks_with_event_types"

## View query:

```sql
 SELECT outbound_webhooks.id,
    outbound_webhooks.created_by,
    outbound_webhooks.created_at,
    outbound_webhooks.updated_by,
    outbound_webhooks.updated_at,
    outbound_webhooks.encryption_key_id,
    outbound_webhooks.url,
    outbound_webhooks.secret,
    array_to_json(ARRAY( SELECT json_build_object('id', outbound_webhook_event_types.id, 'outbound_webhook_id', outbound_webhook_event_types.outbound_webhook_id, 'event_type', outbound_webhook_event_types.event_type, 'scope', outbound_webhook_event_types.scope) AS json_build_object
           FROM outbound_webhook_event_types
          WHERE (outbound_webhook_event_types.outbound_webhook_id = outbound_webhooks.id))) AS event_types
   FROM outbound_webhooks;
```

# View "public.own_background_jobs_config_aware"

## View query:

```sql
 SELECT obj.id,
    obj.state,
    obj.failure_message,
    obj.queued_at,
    obj.started_at,
    obj.finished_at,
    obj.process_after,
    obj.num_resets,
    obj.num_failures,
    obj.last_heartbeat_at,
    obj.execution_logs,
    obj.worker_hostname,
    obj.cancel,
    obj.repo_id,
    obj.job_type,
    osc.name AS config_name
   FROM (own_background_jobs obj
     JOIN own_signal_configurations osc ON ((obj.job_type = osc.id)))
  WHERE (osc.enabled IS TRUE);
```

# View "public.reconciler_changesets"

## View query:

```sql
 SELECT c.id,
    c.batch_change_ids,
    c.repo_id,
    c.queued_at,
    c.created_at,
    c.updated_at,
    c.metadata,
    c.external_id,
    c.external_service_type,
    c.external_deleted_at,
    c.external_branch,
    c.external_updated_at,
    c.external_state,
    c.external_review_state,
    c.external_check_state,
    c.commit_verification,
    c.diff_stat_added,
    c.diff_stat_deleted,
    c.sync_state,
    c.current_spec_id,
    c.previous_spec_id,
    c.publication_state,
    c.owned_by_batch_change_id,
    c.reconciler_state,
    c.computed_state,
    c.failure_message,
    c.started_at,
    c.finished_at,
    c.process_after,
    c.num_resets,
    c.closing,
    c.num_failures,
    c.log_contents,
    c.execution_logs,
    c.syncer_error,
    c.external_title,
    c.worker_hostname,
    c.ui_publication_state,
    c.last_heartbeat_at,
    c.external_fork_name,
    c.external_fork_namespace,
    c.detached_at,
    c.previous_failure_message
   FROM (changesets c
     JOIN repo r ON ((r.id = c.repo_id)))
  WHERE ((r.deleted_at IS NULL) AND (EXISTS ( SELECT 1
           FROM ((batch_changes
             LEFT JOIN users namespace_user ON ((batch_changes.namespace_user_id = namespace_user.id)))
             LEFT JOIN orgs namespace_org ON ((batch_changes.namespace_org_id = namespace_org.id)))
          WHERE ((c.batch_change_ids ? (batch_changes.id)::text) AND (namespace_user.deleted_at IS NULL) AND (namespace_org.deleted_at IS NULL)))));
```

# View "public.site_config"

## View query:

```sql
 SELECT global_state.site_id,
    global_state.initialized
   FROM global_state;
```

# View "public.syntactic_scip_indexing_jobs_with_repository_name"

## View query:

```sql
 SELECT u.id,
    u.commit,
    u.queued_at,
    u.state,
    u.failure_message,
    u.started_at,
    u.finished_at,
    u.repository_id,
    u.process_after,
    u.num_resets,
    u.num_failures,
    u.execution_logs,
    u.should_reindex,
    u.enqueuer_user_id,
    r.name AS repository_name
   FROM (syntactic_scip_indexing_jobs u
     JOIN repo r ON ((r.id = u.repository_id)))
  WHERE (r.deleted_at IS NULL);
```

# View "public.tracking_changeset_specs_and_changesets"

## View query:

```sql
 SELECT changeset_specs.id AS changeset_spec_id,
    COALESCE(changesets.id, (0)::bigint) AS changeset_id,
    changeset_specs.repo_id,
    changeset_specs.batch_spec_id,
    repo.name AS repo_name,
    COALESCE((changesets.metadata ->> 'Title'::text), (changesets.metadata ->> 'title'::text)) AS changeset_name,
    changesets.external_state,
    changesets.publication_state,
    changesets.reconciler_state,
    changesets.computed_state
   FROM ((changeset_specs
     LEFT JOIN changesets ON (((changesets.repo_id = changeset_specs.repo_id) AND (changesets.external_id = changeset_specs.external_id))))
     JOIN repo ON ((changeset_specs.repo_id = repo.id)))
  WHERE ((changeset_specs.external_id IS NOT NULL) AND (repo.deleted_at IS NULL));
```

# Type audit_log_operation

- create
- modify
- delete

# Type batch_changes_changeset_ui_publication_state

- UNPUBLISHED
- DRAFT
- PUBLISHED

# Type cm_email_priority

- NORMAL
- CRITICAL

# Type critical_or_site

- critical
- site

# Type feature_flag_type

- bool
- rollout

# Type persistmode

- record
- snapshot
