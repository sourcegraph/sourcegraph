# Code navigation environment variables

All of the following environment variables are prefixed with `PRECISE_CODE_INTEL`.

## frontend

The following are variables are read from the `frontend` service to control code navigation behavior exposed via the GraphQL API.

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_HUNK_CACHE_SIZE` | `1000` | The capacity of the git diff hunk cache. |
| `_DIAGNOSTICS_COUNT_MIGRATION_BATCH_SIZE` | `1000` | The maximum number of document records to migrate at a time. |
| `_DIAGNOSTICS_COUNT_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |
| `_DEFINITIONS_COUNT_MIGRATION_BATCH_SIZE` | `1000` | The maximum number of definition records to migrate at once. |
| `_DEFINITIONS_COUNT_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |
| `_REFERENCES_COUNT_MIGRATION_BATCH_SIZE` | `1000` | The maximum number of reference records to migrate at a time. |
| `_REFERENCES_COUNT_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |
| `_DOCUMENT_COLUMN_SPLIT_MIGRATION_BATCH_SIZE` | `100` | The maximum number of document records to migrate at a time. |
| `_DOCUMENT_COLUMN_SPLIT_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |
| `_API_DOCS_SEARCH_MIGRATION_BATCH_SIZE` | `1` | The maximum number of bundles to migrate at a time. |
| `_API_DOCS_SEARCH_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |
| `_COMMITTED_AT_MIGRATION_BATCH_SIZE` | `100` | The maximum number of upload records to migrate at a time. |
| `_COMMITTED_AT_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |
| `_REFERENCE_COUNT_MIGRATION_BATCH_SIZE` | `100` | The maximum number of upload records to migrate at a time. |
| `_REFERENCE_COUNT_MIGRATION_BATCH_INTERVAL` | `1s` | The timeout between processing migration batches. |

The following settings should be the same for the [`precise-code-intel-worker`](#precise-code-intel-worker) service as well.

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_UPLOAD_BACKEND` | `Blobstore` | The target file service for code graph uploads. S3, GCS, and Blobstore are supported. In older versions of Sourcegraph (before v3.4.2) `Minio` was also a valid value. |
| `_UPLOAD_MANAGE_BUCKET` | `false` | Whether or not the client should manage the target bucket configuration |
| `_UPLOAD_BUCKET` | `lsif-uploads` | The name of the bucket to store LSIF uploads in |
| `_UPLOAD_TTL` | `168h` | The maximum age of an upload before deletion |

The following settings should be the same for the [`codeintel-auto-indexing`](#codeintel-auto-indexing) worker task as well.

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_AUTO_INDEX_MAXIMUM_REPOSITORIES_INSPECTED_PER_SECOND` | `0` | The maximum number of repositories inspected for auto-indexing per second. Set to zero to disable limit. |
| `_AUTO_INDEX_MAXIMUM_REPOSITORIES_UPDATED_PER_SECOND` | `0` | The maximum number of repositories cloned or fetched for auto-indexing per second. Set to zero to disable limit. |
| `_AUTO_INDEX_MAXIMUM_INDEX_JOBS_PER_INFERRED_CONFIGURATION` | `25` | Repositories with a number of inferred auto-index jobs exceeding this threshold will be auto-indexed |

## worker

The following are variables are read from the `worker` service to control code graph data behavior run in asynchronous background tasks.

### `codeintel-commitgraph`

The following variables influence the behavior of the [`codeintel-commitgraph` worker task](../../admin/workers.md#codeintel-commitgraph).

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_MAX_AGE_FOR_NON_STALE_BRANCHES` | `2160h` (about 3 months) | The age after which a branch should be considered stale. Code graph indexes will be evicted from stale branches. |
| `_MAX_AGE_FOR_NON_STALE_TAGS` | `8760h` (about 1 year) | The age after which a tagged commit should be considered stale. Code graph indexes will be evicted from stale tagged commits. |
| `_COMMIT_GRAPH_UPDATE_TASK_INTERVAL` | `10s` | The frequency with which to run periodic codeintel commit graph update tasks. |

### `codeintel-auto-indexing`

The following variables influence the behavior of the [`codeintel-auto-indexing` worker task](../../admin/workers.md#codeintel-auto-indexing).

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_AUTO_INDEXING_TASK_INTERVAL` | `10m` | The frequency with which to run periodic codeintel auto-indexing tasks. |
| `_AUTO_INDEXING_REPOSITORY_PROCESS_DELAY` | `24h` | The minimum frequency that the same repository can be considered for auto-index scheduling. |
| `_AUTO_INDEXING_REPOSITORY_BATCH_SIZE` | `100` | The number of repositories to consider for auto-indexing scheduling at a time. |
| `_AUTO_INDEXING_POLICY_BATCH_SIZE` | `100` | The number of policies to consider for auto-indexing scheduling at a time. |
| `_DEPENDENCY_INDEXER_SCHEDULER_POLL_INTERVAL` | `1s` | Interval between queries to the dependency indexing job queue. |
| `_DEPENDENCY_INDEXER_SCHEDULER_CONCURRENCY` | `1` | The maximum number of dependency graphs that can be processed concurrently. |


The following settings should be the same for the [`frontend`](#frontend) service as well.

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_AUTO_INDEX_MAXIMUM_REPOSITORIES_INSPECTED_PER_SECOND` | `0` | The maximum number of repositories inspected for auto-indexing per second. Set to zero to disable limit. |
| `_AUTO_INDEX_MAXIMUM_REPOSITORIES_UPDATED_PER_SECOND` | `0` | The maximum number of repositories cloned or fetched for auto-indexing per second. Set to zero to disable limit. |
| `_AUTO_INDEX_MAXIMUM_INDEX_JOBS_PER_INFERRED_CONFIGURATION` | `25` | Repositories with a number of inferred auto-index jobs exceeding this threshold will be auto-indexed |

### `codeintel-janitor`

The following variables influence the behavior of the [`codeintel-janitor` worker task](../../admin/workers.md#codeintel-janitor).

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_UPLOAD_TIMEOUT` | `24h` | The maximum time an upload can be in the 'uploading' state. |`
| `_CLEANUP_TASK_INTERVAL` | `1m` | The frequency with which to run periodic codeintel cleanup tasks. |`
| `_COMMIT_RESOLVER_TASK_INTERVAL` | `10s` | The frequency with which to run the periodic commit resolver task. |`
| `_COMMIT_RESOLVER_MINIMUM_TIME_SINCE_LAST_CHECK` | `24h` | The minimum time the commit resolver will re-check an upload or index record. |`
| `_COMMIT_RESOLVER_BATCH_SIZE` | `100` | The maximum number of unique commits to resolve at a time. |`
| `_COMMIT_RESOLVER_MAXIMUM_COMMIT_LAG` | `0s` | The maximum acceptable delay between accepting an upload and its commit becoming resolvable. Be cautious about setting this to a large value, as uploads for unresolvable commits will be retried periodically during this interval. |`
| `_RETENTION_REPOSITORY_PROCESS_DELAY` | `24h` | The minimum frequency that the same repository's uploads can be considered for expiration. |`
| `_RETENTION_REPOSITORY_BATCH_SIZE` | `100` | The number of repositories to consider for expiration at a time. |`
| `_RETENTION_UPLOAD_PROCESS_DELAY` | `24h` | The minimum frequency that the same upload record can be considered for expiration. |`
| `_RETENTION_UPLOAD_BATCH_SIZE` | `100` | The number of uploads to consider for expiration at a time. |`
| `_RETENTION_POLICY_BATCH_SIZE` | `100` | The number of policies to consider for expiration at a time. |`
| `_RETENTION_COMMIT_BATCH_SIZE` | `100` | The number of commits to process per upload at a time. |`
| `_RETENTION_BRANCHES_CACHE_MAX_KEYS` | `10000` | The number of maximum keys used to cache the set of branches visible from a commit. |`
| `_CONFIGURATION_POLICY_MEMBERSHIP_BATCH_SIZE` | `100` | The maximum number of policy configurations to update repository membership for at a time. |`
| `_DOCUMENTATION_SEARCH_CURRENT_MINIMUM_TIME_SINCE_LAST_CHECK` | `24h` | The minimum time the documentation search current janitor will re-check records for a unique search key. |`
| `_DOCUMENTATION_SEARCH_CURRENT_BATCH_SIZE` | `100` | The maximum number of unique search keys to clean up at a time. |`

## precise-code-intel-worker

The following are variables are read from the `precise-code-intel-worker` service to control code graph data upload processing behavior.

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_WORKER_POLL_INTERVAL` | `1s` | Interval between queries to the upload queue. |
| `_WORKER_CONCURRENCY` | `1` | The maximum number of indexes that can be processed concurrently. |
| `_WORKER_BUDGET` | `0` | The amount of compressed input data (in bytes) a worker can process concurrently. Zero acts as an infinite budget. |

The following settings should be the same for the [`frontend`](#frontend) service as well.

| Name | Default | Description |
| ---- | ------- | ----------- |
| `_UPLOAD_BACKEND` | `Blobstore` | The target file service for code graph data uploads. S3, GCS, and Blobstore are supported. In older versions of Sourcegraph (before v3.4.2) `Minio` was also a valid value. |
| `_UPLOAD_MANAGE_BUCKET` | `false` | Whether or not the client should manage the target bucket configuration |
| `_UPLOAD_BUCKET` | `lsif-uploads` | The name of the bucket to store LSIF uploads in |
| `_UPLOAD_TTL` | `168h` | The maximum age of an upload before deletion |
