# Table "public.archived_insight_series_recording_times"
```
      Column       |           Type           | Collation | Nullable | Default 
-------------------+--------------------------+-----------+----------+---------
 insight_series_id | integer                  |           | not null | 
 recording_time    | timestamp with time zone |           | not null | 
 snapshot          | boolean                  |           | not null | 
Indexes:
    "archived_insight_series_recor_insight_series_id_recording_t_key" UNIQUE CONSTRAINT, btree (insight_series_id, recording_time)
Foreign-key constraints:
    "insight_series_id_fkey" FOREIGN KEY (insight_series_id) REFERENCES insight_series(id) ON DELETE CASCADE

```

# Table "public.archived_series_points"
```
        Column         |           Type           | Collation | Nullable | Default 
-----------------------+--------------------------+-----------+----------+---------
 series_id             | text                     |           | not null | 
 time                  | timestamp with time zone |           | not null | 
 value                 | double precision         |           | not null | 
 repo_id               | integer                  |           |          | 
 repo_name_id          | integer                  |           |          | 
 original_repo_name_id | integer                  |           |          | 
 capture               | text                     |           |          | 
Check constraints:
    "check_repo_fields_specifity" CHECK (repo_id IS NULL AND repo_name_id IS NULL AND original_repo_name_id IS NULL OR repo_id IS NOT NULL AND repo_name_id IS NOT NULL AND original_repo_name_id IS NOT NULL)
Foreign-key constraints:
    "insight_series_series_id_fkey" FOREIGN KEY (series_id) REFERENCES insight_series(series_id) ON DELETE CASCADE

```

# Table "public.dashboard"
```
       Column       |            Type             | Collation | Nullable |                Default                
--------------------+-----------------------------+-----------+----------+---------------------------------------
 id                 | integer                     |           | not null | nextval('dashboard_id_seq'::regclass)
 title              | text                        |           |          | 
 created_at         | timestamp without time zone |           | not null | now()
 created_by_user_id | integer                     |           |          | 
 last_updated_at    | timestamp without time zone |           | not null | now()
 deleted_at         | timestamp without time zone |           |          | 
 save               | boolean                     |           | not null | false
 type               | text                        |           | not null | 'standard'::text
Indexes:
    "dashboard_pk" PRIMARY KEY, btree (id)
Referenced by:
    TABLE "dashboard_grants" CONSTRAINT "dashboard_grants_dashboard_id_fk" FOREIGN KEY (dashboard_id) REFERENCES dashboard(id) ON DELETE CASCADE
    TABLE "dashboard_insight_view" CONSTRAINT "dashboard_insight_view_dashboard_id_fk" FOREIGN KEY (dashboard_id) REFERENCES dashboard(id) ON DELETE CASCADE

```

Metadata for dashboards of insights

**created_at**: Timestamp the dashboard was initially created.

**created_by_user_id**: User that created the dashboard, if available.

**deleted_at**: Set to the time the dashboard was soft deleted.

**last_updated_at**: Time the dashboard was last updated, either metadata or insights.

**save**: TEMPORARY Do not delete this dashboard when migrating settings.

**title**: Title of the dashboard

# Table "public.dashboard_grants"
```
    Column    |  Type   | Collation | Nullable |                   Default                    
--------------+---------+-----------+----------+----------------------------------------------
 id           | integer |           | not null | nextval('dashboard_grants_id_seq'::regclass)
 dashboard_id | integer |           | not null | 
 user_id      | integer |           |          | 
 org_id       | integer |           |          | 
 global       | boolean |           |          | 
Indexes:
    "dashboard_grants_pk" PRIMARY KEY, btree (id)
    "dashboard_grants_dashboard_id_index" btree (dashboard_id)
    "dashboard_grants_global_idx" btree (global) WHERE global IS TRUE
    "dashboard_grants_org_id_idx" btree (org_id)
    "dashboard_grants_user_id_idx" btree (user_id)
Foreign-key constraints:
    "dashboard_grants_dashboard_id_fk" FOREIGN KEY (dashboard_id) REFERENCES dashboard(id) ON DELETE CASCADE

```

Permission grants for dashboards. Each row should represent a unique principal (user, org, etc).

**global**: Grant that does not belong to any specific principal and is granted to all users.

**org_id**: Org ID that that receives this grant.

**user_id**: User ID that that receives this grant.

# Table "public.dashboard_insight_view"
```
     Column      |  Type   | Collation | Nullable |                      Default                       
-----------------+---------+-----------+----------+----------------------------------------------------
 id              | integer |           | not null | nextval('dashboard_insight_view_id_seq'::regclass)
 dashboard_id    | integer |           | not null | 
 insight_view_id | integer |           | not null | 
Indexes:
    "dashboard_insight_view_pk" PRIMARY KEY, btree (id)
    "unique_dashboard_id_insight_view_id" UNIQUE CONSTRAINT, btree (dashboard_id, insight_view_id)
    "dashboard_insight_view_dashboard_id_fk_idx" btree (dashboard_id)
    "dashboard_insight_view_insight_view_id_fk_idx" btree (insight_view_id)
Foreign-key constraints:
    "dashboard_insight_view_dashboard_id_fk" FOREIGN KEY (dashboard_id) REFERENCES dashboard(id) ON DELETE CASCADE
    "dashboard_insight_view_insight_view_id_fk" FOREIGN KEY (insight_view_id) REFERENCES insight_view(id) ON DELETE CASCADE

```

# Table "public.insight_series"
```
            Column             |            Type             | Collation | Nullable |                  Default                   
-------------------------------+-----------------------------+-----------+----------+--------------------------------------------
 id                            | integer                     |           | not null | nextval('insight_series_id_seq'::regclass)
 series_id                     | text                        |           | not null | 
 query                         | text                        |           | not null | 
 created_at                    | timestamp without time zone |           | not null | CURRENT_TIMESTAMP
 oldest_historical_at          | timestamp without time zone |           | not null | (CURRENT_TIMESTAMP - '1 year'::interval)
 last_recorded_at              | timestamp without time zone |           | not null | (CURRENT_TIMESTAMP - '10 years'::interval)
 next_recording_after          | timestamp without time zone |           | not null | CURRENT_TIMESTAMP
 deleted_at                    | timestamp without time zone |           |          | 
 backfill_queued_at            | timestamp without time zone |           |          | 
 last_snapshot_at              | timestamp without time zone |           |          | (CURRENT_TIMESTAMP - '10 years'::interval)
 next_snapshot_after           | timestamp without time zone |           |          | CURRENT_TIMESTAMP
 repositories                  | text[]                      |           |          | 
 sample_interval_unit          | time_unit                   |           | not null | 'MONTH'::time_unit
 sample_interval_value         | integer                     |           | not null | 1
 generated_from_capture_groups | boolean                     |           | not null | false
 generation_method             | text                        |           | not null | 
 just_in_time                  | boolean                     |           | not null | false
 group_by                      | text                        |           |          | 
 backfill_attempts             | integer                     |           | not null | 0
 needs_migration               | boolean                     |           |          | 
 backfill_completed_at         | timestamp without time zone |           |          | 
 supports_augmentation         | boolean                     |           | not null | true
 repository_criteria           | text                        |           |          | 
 query_old                     | text                        |           |          | 
Indexes:
    "insight_series_pkey" PRIMARY KEY, btree (id)
    "insight_series_series_id_unique_idx" UNIQUE, btree (series_id)
    "insight_series_deleted_at_idx" btree (deleted_at)
    "insight_series_next_recording_after_idx" btree (next_recording_after)
Referenced by:
    TABLE "insight_series_backfill" CONSTRAINT "insight_series_backfill_series_id_fk" FOREIGN KEY (series_id) REFERENCES insight_series(id) ON DELETE CASCADE
    TABLE "archived_insight_series_recording_times" CONSTRAINT "insight_series_id_fkey" FOREIGN KEY (insight_series_id) REFERENCES insight_series(id) ON DELETE CASCADE
    TABLE "insight_series_recording_times" CONSTRAINT "insight_series_id_fkey" FOREIGN KEY (insight_series_id) REFERENCES insight_series(id) ON DELETE CASCADE
    TABLE "insight_series_incomplete_points" CONSTRAINT "insight_series_incomplete_points_series_id_fk" FOREIGN KEY (series_id) REFERENCES insight_series(id) ON DELETE CASCADE
    TABLE "archived_series_points" CONSTRAINT "insight_series_series_id_fkey" FOREIGN KEY (series_id) REFERENCES insight_series(series_id) ON DELETE CASCADE
    TABLE "insight_view_series" CONSTRAINT "insight_view_series_insight_series_id_fkey" FOREIGN KEY (insight_series_id) REFERENCES insight_series(id)

```

Data series that comprise code insights.

**created_at**: Timestamp when this series was created

**deleted_at**: Timestamp of a soft-delete of this row.

**generation_method**: Specifies the execution method for how this series is generated. This helps the system understand how to generate the time series data.

**id**: Primary key ID of this series

**just_in_time**: Specifies if the series should be resolved just in time at query time, or recorded in background processing.

**last_recorded_at**: Timestamp when this series was last recorded (non-historical).

**next_recording_after**: Timestamp when this series should next record (non-historical).

**oldest_historical_at**: Timestamp representing the oldest point of which this series is backfilled.

**query**: Query string that generates this series

**query_old**: Backup for migration. Remove with release 5.6 or later.

**repository_criteria**: The search criteria used to determine the repositories that are included in this series.

**series_id**: Timestamp that this series completed a full repository iteration for backfill. This flag has limited semantic value, and only means it tried to queue up queries for each repository. It does not guarantee success on those queries.

# Table "public.insight_series_backfill"
```
      Column      |       Type       | Collation | Nullable |                       Default                       
------------------+------------------+-----------+----------+-----------------------------------------------------
 id               | integer          |           | not null | nextval('insight_series_backfill_id_seq'::regclass)
 series_id        | integer          |           | not null | 
 repo_iterator_id | integer          |           |          | 
 estimated_cost   | double precision |           |          | 
 state            | text             |           | not null | 'new'::text
Indexes:
    "insight_series_backfill_pk" PRIMARY KEY, btree (id)
Foreign-key constraints:
    "insight_series_backfill_series_id_fk" FOREIGN KEY (series_id) REFERENCES insight_series(id) ON DELETE CASCADE
Referenced by:
    TABLE "insights_background_jobs" CONSTRAINT "insights_background_jobs_backfill_id_fkey" FOREIGN KEY (backfill_id) REFERENCES insight_series_backfill(id) ON DELETE CASCADE

```

# Table "public.insight_series_incomplete_points"
```
  Column   |            Type             | Collation | Nullable |                           Default                            
-----------+-----------------------------+-----------+----------+--------------------------------------------------------------
 id        | integer                     |           | not null | nextval('insight_series_incomplete_points_id_seq'::regclass)
 series_id | integer                     |           | not null | 
 reason    | text                        |           | not null | 
 time      | timestamp without time zone |           | not null | 
 repo_id   | integer                     |           |          | 
Indexes:
    "insight_series_incomplete_points_pk" PRIMARY KEY, btree (id)
    "insight_series_incomplete_points_unique_idx" UNIQUE, btree (series_id, reason, "time", repo_id)
Foreign-key constraints:
    "insight_series_incomplete_points_series_id_fk" FOREIGN KEY (series_id) REFERENCES insight_series(id) ON DELETE CASCADE

```

# Table "public.insight_series_recording_times"
```
      Column       |           Type           | Collation | Nullable | Default 
-------------------+--------------------------+-----------+----------+---------
 insight_series_id | integer                  |           |          | 
 recording_time    | timestamp with time zone |           |          | 
 snapshot          | boolean                  |           |          | 
Indexes:
    "insight_series_recording_time_insight_series_id_recording_t_key" UNIQUE CONSTRAINT, btree (insight_series_id, recording_time)
Foreign-key constraints:
    "insight_series_id_fkey" FOREIGN KEY (insight_series_id) REFERENCES insight_series(id) ON DELETE CASCADE

```

# Table "public.insight_view"
```
              Column               |            Type            | Collation | Nullable |                 Default                  
-----------------------------------+----------------------------+-----------+----------+------------------------------------------
 id                                | integer                    |           | not null | nextval('insight_view_id_seq'::regclass)
 title                             | text                       |           |          | 
 description                       | text                       |           |          | 
 unique_id                         | text                       |           | not null | 
 default_filter_include_repo_regex | text                       |           |          | 
 default_filter_exclude_repo_regex | text                       |           |          | 
 other_threshold                   | double precision           |           |          | 
 presentation_type                 | presentation_type_enum     |           | not null | 'LINE'::presentation_type_enum
 is_frozen                         | boolean                    |           | not null | false
 default_filter_search_contexts    | text[]                     |           |          | 
 series_sort_mode                  | series_sort_mode_enum      |           |          | 
 series_sort_direction             | series_sort_direction_enum |           |          | 
 series_limit                      | integer                    |           |          | 
 series_num_samples                | integer                    |           |          | 
Indexes:
    "insight_view_pkey" PRIMARY KEY, btree (id)
    "insight_view_unique_id_unique_idx" UNIQUE, btree (unique_id)
Referenced by:
    TABLE "dashboard_insight_view" CONSTRAINT "dashboard_insight_view_insight_view_id_fk" FOREIGN KEY (insight_view_id) REFERENCES insight_view(id) ON DELETE CASCADE
    TABLE "insight_view_grants" CONSTRAINT "insight_view_grants_insight_view_id_fk" FOREIGN KEY (insight_view_id) REFERENCES insight_view(id) ON DELETE CASCADE
    TABLE "insight_view_series" CONSTRAINT "insight_view_series_insight_view_id_fkey" FOREIGN KEY (insight_view_id) REFERENCES insight_view(id) ON DELETE CASCADE

```

Views for insight data series. An insight view is an abstraction on top of an insight data series that allows for lightweight modifications to filters or metadata without regenerating the underlying series.

**description**: Description of the view. This may render in a chart depending on the view type.

**id**: Primary key ID for this view

**other_threshold**: Percent threshold for grouping series under &#34;other&#34;

**presentation_type**: The basic presentation type for the insight view. (e.g Line, Pie, etc.)

**title**: Title of the view. This may render in a chart depending on the view type.

**unique_id**: Globally unique identifier for this view that is externally referencable.

# Table "public.insight_view_grants"
```
     Column      |  Type   | Collation | Nullable |                     Default                     
-----------------+---------+-----------+----------+-------------------------------------------------
 id              | integer |           | not null | nextval('insight_view_grants_id_seq'::regclass)
 insight_view_id | integer |           | not null | 
 user_id         | integer |           |          | 
 org_id          | integer |           |          | 
 global          | boolean |           |          | 
Indexes:
    "insight_view_grants_pk" PRIMARY KEY, btree (id)
    "insight_view_grants_global_idx" btree (global) WHERE global IS TRUE
    "insight_view_grants_insight_view_id_index" btree (insight_view_id)
    "insight_view_grants_org_id_idx" btree (org_id)
    "insight_view_grants_user_id_idx" btree (user_id)
Foreign-key constraints:
    "insight_view_grants_insight_view_id_fk" FOREIGN KEY (insight_view_id) REFERENCES insight_view(id) ON DELETE CASCADE

```

Permission grants for insight views. Each row should represent a unique principal (user, org, etc).

**global**: Grant that does not belong to any specific principal and is granted to all users.

**org_id**: Org ID that that receives this grant.

**user_id**: User ID that that receives this grant.

# Table "public.insight_view_series"
```
      Column       |  Type   | Collation | Nullable | Default 
-------------------+---------+-----------+----------+---------
 insight_view_id   | integer |           | not null | 
 insight_series_id | integer |           | not null | 
 label             | text    |           |          | 
 stroke            | text    |           |          | 
Indexes:
    "insight_view_series_pkey" PRIMARY KEY, btree (insight_view_id, insight_series_id)
Foreign-key constraints:
    "insight_view_series_insight_series_id_fkey" FOREIGN KEY (insight_series_id) REFERENCES insight_series(id)
    "insight_view_series_insight_view_id_fkey" FOREIGN KEY (insight_view_id) REFERENCES insight_view(id) ON DELETE CASCADE

```

Join table to correlate data series with insight views

**insight_series_id**: Foreign key to insight data series.

**insight_view_id**: Foreign key to insight view.

**label**: Label text for this data series. This may render in a chart depending on the view type.

**stroke**: Stroke color metadata for this data series. This may render in a chart depending on the view type.

# Table "public.insights_background_jobs"
```
      Column       |           Type           | Collation | Nullable |                       Default                        
-------------------+--------------------------+-----------+----------+------------------------------------------------------
 id                | integer                  |           | not null | nextval('insights_background_jobs_id_seq'::regclass)
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
 backfill_id       | integer                  |           |          | 
Indexes:
    "insights_background_jobs_pkey" PRIMARY KEY, btree (id)
    "insights_jobs_state_idx" btree (state)
Foreign-key constraints:
    "insights_background_jobs_backfill_id_fkey" FOREIGN KEY (backfill_id) REFERENCES insight_series_backfill(id) ON DELETE CASCADE

```

# Table "public.insights_data_retention_jobs"
```
      Column       |           Type           | Collation | Nullable |                         Default                          
-------------------+--------------------------+-----------+----------+----------------------------------------------------------
 id                | integer                  |           | not null | nextval('insights_data_retention_jobs_id_seq'::regclass)
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
 series_id         | integer                  |           | not null | 
 series_id_string  | text                     |           | not null | ''::text
Indexes:
    "insights_data_retention_jobs_pkey" PRIMARY KEY, btree (id)

```

# Table "public.metadata"
```
  Column  |  Type  | Collation | Nullable |               Default                
----------+--------+-----------+----------+--------------------------------------
 id       | bigint |           | not null | nextval('metadata_id_seq'::regclass)
 metadata | jsonb  |           | not null | 
Indexes:
    "metadata_pkey" PRIMARY KEY, btree (id)
    "metadata_metadata_unique_idx" UNIQUE, btree (metadata)
    "metadata_metadata_gin" gin (metadata)
Referenced by:
    TABLE "series_points" CONSTRAINT "series_points_metadata_id_fkey" FOREIGN KEY (metadata_id) REFERENCES metadata(id) ON DELETE CASCADE DEFERRABLE

```

Records arbitrary metadata about events. Stored in a separate table as it is often repeated for multiple events.

**id**: The metadata ID.

**metadata**: Metadata about some event, this can be any arbitrary JSON emtadata which will be returned when querying events, and can be filtered on and grouped using jsonb operators ?, ?&amp;, ?|, and @&gt;. This should be small data only.

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

# Table "public.repo_iterator"
```
      Column      |            Type             | Collation | Nullable |                  Default                  
------------------+-----------------------------+-----------+----------+-------------------------------------------
 id               | integer                     |           | not null | nextval('repo_iterator_id_seq'::regclass)
 created_at       | timestamp without time zone |           |          | now()
 started_at       | timestamp without time zone |           |          | 
 completed_at     | timestamp without time zone |           |          | 
 last_updated_at  | timestamp without time zone |           | not null | now()
 runtime_duration | bigint                      |           | not null | 0
 percent_complete | double precision            |           | not null | 0
 total_count      | integer                     |           | not null | 0
 success_count    | integer                     |           | not null | 0
 repos            | integer[]                   |           |          | 
 repo_cursor      | integer                     |           |          | 0
Indexes:
    "repo_iterator_pk" PRIMARY KEY, btree (id)
Referenced by:
    TABLE "repo_iterator_errors" CONSTRAINT "repo_iterator_fk" FOREIGN KEY (repo_iterator_id) REFERENCES repo_iterator(id)

```

# Table "public.repo_iterator_errors"
```
      Column      |  Type   | Collation | Nullable |                     Default                      
------------------+---------+-----------+----------+--------------------------------------------------
 id               | integer |           | not null | nextval('repo_iterator_errors_id_seq'::regclass)
 repo_iterator_id | integer |           | not null | 
 repo_id          | integer |           | not null | 
 error_message    | text[]  |           | not null | 
 failure_count    | integer |           |          | 1
Indexes:
    "repo_iterator_errors_pk" PRIMARY KEY, btree (id)
    "repo_iterator_errors_fk_idx" btree (repo_iterator_id)
Foreign-key constraints:
    "repo_iterator_fk" FOREIGN KEY (repo_iterator_id) REFERENCES repo_iterator(id)

```

# Table "public.repo_names"
```
 Column |  Type  | Collation | Nullable |                Default                 
--------+--------+-----------+----------+----------------------------------------
 id     | bigint |           | not null | nextval('repo_names_id_seq'::regclass)
 name   | citext |           | not null | 
Indexes:
    "repo_names_pkey" PRIMARY KEY, btree (id)
    "repo_names_name_unique_idx" UNIQUE, btree (name)
    "repo_names_name_trgm" gin (lower(name::text) gin_trgm_ops)
Check constraints:
    "check_name_nonempty" CHECK (name <> ''::citext)
Referenced by:
    TABLE "series_points" CONSTRAINT "series_points_original_repo_name_id_fkey" FOREIGN KEY (original_repo_name_id) REFERENCES repo_names(id) ON DELETE CASCADE DEFERRABLE
    TABLE "series_points" CONSTRAINT "series_points_repo_name_id_fkey" FOREIGN KEY (repo_name_id) REFERENCES repo_names(id) ON DELETE CASCADE DEFERRABLE

```

Records repository names, both historical and present, using a unique repository _name_ ID (unrelated to the repository ID.)

**id**: The repository _name_ ID.

**name**: The repository name string, with unique constraint for table entry deduplication and trigram index for e.g. regex filtering.

# Table "public.series_points"
```
        Column         |           Type           | Collation | Nullable | Default 
-----------------------+--------------------------+-----------+----------+---------
 series_id             | text                     |           | not null | 
 time                  | timestamp with time zone |           | not null | 
 value                 | double precision         |           | not null | 
 metadata_id           | integer                  |           |          | 
 repo_id               | integer                  |           |          | 
 repo_name_id          | integer                  |           |          | 
 original_repo_name_id | integer                  |           |          | 
 capture               | text                     |           |          | 
Indexes:
    "series_points_original_repo_name_id_btree" btree (original_repo_name_id)
    "series_points_repo_id_btree" btree (repo_id)
    "series_points_repo_name_id_btree" btree (repo_name_id)
    "series_points_series_id_btree" btree (series_id)
    "series_points_series_id_repo_id_time_idx" btree (series_id, repo_id, "time")
Check constraints:
    "check_repo_fields_specifity" CHECK (repo_id IS NULL AND repo_name_id IS NULL AND original_repo_name_id IS NULL OR repo_id IS NOT NULL AND repo_name_id IS NOT NULL AND original_repo_name_id IS NOT NULL)
Foreign-key constraints:
    "series_points_metadata_id_fkey" FOREIGN KEY (metadata_id) REFERENCES metadata(id) ON DELETE CASCADE DEFERRABLE
    "series_points_original_repo_name_id_fkey" FOREIGN KEY (original_repo_name_id) REFERENCES repo_names(id) ON DELETE CASCADE DEFERRABLE
    "series_points_repo_name_id_fkey" FOREIGN KEY (repo_name_id) REFERENCES repo_names(id) ON DELETE CASCADE DEFERRABLE

```

Records events over time associated with a repository (or none, i.e. globally) where a single numerical value is going arbitrarily up and down.  Repository association is based on both repository ID and name. The ID can be used to refer toa specific repository, or lookup the current name of a repository after it has been e.g. renamed. The name can be used to refer to the name of the repository at the time of the events creation, for example to trace the change in a gauge back to a repository being renamed.

**metadata_id**: Associated metadata for this event, if any.

**original_repo_name_id**: The repository name as it was known at the time the event was created. It may have been renamed since.

**repo_id**: The repository ID (from the main application DB) at the time the event was created. Note that the repository may no longer exist / be valid at query time, however.

**repo_name_id**: The most recently known name for the repository, updated periodically to account for e.g. repository renames. If the repository was deleted, this is still the most recently known name.  null if the event was not for a single repository (i.e. a global gauge).

**series_id**: A unique identifier for the series of data being recorded. This is not an ID from another table, but rather just a unique identifier.

**time**: The timestamp of the recorded event.

**value**: The floating point value at the time of the event.

# Table "public.series_points_snapshots"
```
        Column         |           Type           | Collation | Nullable | Default 
-----------------------+--------------------------+-----------+----------+---------
 series_id             | text                     |           | not null | 
 time                  | timestamp with time zone |           | not null | 
 value                 | double precision         |           | not null | 
 metadata_id           | integer                  |           |          | 
 repo_id               | integer                  |           |          | 
 repo_name_id          | integer                  |           |          | 
 original_repo_name_id | integer                  |           |          | 
 capture               | text                     |           |          | 
Indexes:
    "series_points_snapshots_original_repo_name_id_idx" btree (original_repo_name_id)
    "series_points_snapshots_repo_id_idx" btree (repo_id)
    "series_points_snapshots_repo_name_id_idx" btree (repo_name_id)
    "series_points_snapshots_series_id_idx" btree (series_id)
    "series_points_snapshots_series_id_repo_id_time_idx" btree (series_id, repo_id, "time")
Check constraints:
    "check_repo_fields_specifity" CHECK (repo_id IS NULL AND repo_name_id IS NULL AND original_repo_name_id IS NULL OR repo_id IS NOT NULL AND repo_name_id IS NOT NULL AND original_repo_name_id IS NOT NULL)

```

Stores ephemeral snapshot data of insight recordings.

# View "public.insights_jobs_backfill_in_progress"

## View query:

```sql
 SELECT jobs.id,
    jobs.state,
    jobs.failure_message,
    jobs.queued_at,
    jobs.started_at,
    jobs.finished_at,
    jobs.process_after,
    jobs.num_resets,
    jobs.num_failures,
    jobs.last_heartbeat_at,
    jobs.execution_logs,
    jobs.worker_hostname,
    jobs.cancel,
    jobs.backfill_id,
    isb.state AS backfill_state,
    isb.estimated_cost,
    width_bucket(isb.estimated_cost, (0)::double precision, max((isb.estimated_cost + (1)::double precision)) OVER (), 4) AS cost_bucket
   FROM (insights_background_jobs jobs
     JOIN insight_series_backfill isb ON ((jobs.backfill_id = isb.id)))
  WHERE (isb.state = 'processing'::text);
```

# View "public.insights_jobs_backfill_new"

## View query:

```sql
 SELECT jobs.id,
    jobs.state,
    jobs.failure_message,
    jobs.queued_at,
    jobs.started_at,
    jobs.finished_at,
    jobs.process_after,
    jobs.num_resets,
    jobs.num_failures,
    jobs.last_heartbeat_at,
    jobs.execution_logs,
    jobs.worker_hostname,
    jobs.cancel,
    jobs.backfill_id,
    isb.state AS backfill_state,
    isb.estimated_cost
   FROM (insights_background_jobs jobs
     JOIN insight_series_backfill isb ON ((jobs.backfill_id = isb.id)))
  WHERE (isb.state = 'new'::text);
```

# Type presentation_type_enum

- LINE
- PIE

# Type series_sort_direction_enum

- ASC
- DESC

# Type series_sort_mode_enum

- RESULT_COUNT
- LEXICOGRAPHICAL
- DATE_ADDED

# Type time_unit

- HOUR
- DAY
- WEEK
- MONTH
- YEAR
