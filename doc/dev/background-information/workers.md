# Developing a worker

Workers are the consumer side of a producer/consumer relationship.

Examples:

- [Precise code-intel worker that handles LSIF uploads](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b946a20362ee7dfedb3b1fbc7f8bb002135d7283/-/blob/enterprise/cmd/precise-code-intel-worker/internal/worker/worker.go)
- [Insights query runner worker](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b946a20362ee7dfedb3b1fbc7f8bb002135d7283/-/blob/enterprise/internal/insights/background/queryrunner/worker.go?subtree=true#L29)
- [Batch Changes background worker that reconciles changesets](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b946a20362ee7dfedb3b1fbc7f8bb002135d7283/-/blob/enterprise/internal/batches/background/workers.go?subtree=true#L26)

## Overview

A **worker** is an generic process configured with a _store_ and a _handler_. In short, the store describes how to interact with where jobs are persisted; the handler (supplied by the user) describes how to process each job. Both of these components will be discussed in more detail below.

The **store** is responsible for selecting the next available job from the backing persistence layer and suitably _locking_ it from other consumers as well as updating the job records as they make progress in the handler. Generally, this will be an instance of [dbworker/store.Store](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.25.0/-/blob/internal/workerutil/dbworker/store/store.go#L204:6), although there are [other implementations](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.25.0/-/blob/enterprise/cmd/executor/internal/apiclient/client.go#L51:6).

The **handler** is responsible for handling a single job once dequeued from the associated store. Handlers can be fairly minimal, but there are a number of hooks which can be overridden to customize the behavior of the worker.

#### Hook 1: Pre-dequeue (optional)

Before the worker dequeues the next job, the _pre dequeue_ hook (if defined) is invoked. The hook has the following signature:

```
func (h *myHandler) PreDequeue(context.Context) (dequeueable bool, extraDequeueArguments interface{}, err error) {
  // configure conditional job selection
  return true, nil, nil
}
```

If the hook returns with `dequeueable = false`, the worker will continue to wait before the next attempt to dequeue an available job. If the hook returns with `extraDequeueArguments`, then it will be passed (in an implementation-specific manner) to the store while dequeueing a job. For the database-backed store, the `extraDequeueArguments` take the form of `*sqlf.Query` expressions, which are added to the conditional clause when selecting a candidate job record.

The main use of these return values are to aid in implementation of a worker _budget_. If the worker is processing multiple jobs, it must be careful that the maximum number of concurrent jobs do not exceed the resource capacity of the worker process. Adding additional conditions to the dequeue method in this manner allows us to skip over jobs that would require more resources than our current capacity. (This applies to jobs for which the resource usage can be fairly accurately estimated.)

#### Hook 2: PreHandle (optional)

After the worker dequeues a record to process, but before it's processed, the _pre handle_ hook (if defined) is invoked. The hook has the following signature:

```
func (h *myHandler) PreHandle(ctx context.Context, record Record) {
  // do something before
}
```

The record value is what was dequeued from the backing store. Its type is a nearly useless interface, thus the value must be cast to the expected type of job of concern to this handler before doing anything useful with it.

Along with the `PostHandle` hook described below, these hooks can effectively maintain the worker budget discussed above: before processing each job we atomically decrease our worker's current _headroom_, and restore the headroom once the job has completed.

#### Hook 3: Handle (required)

To process a record, the worker invokes the _handle_ hook, which is the only required hook. The hook has the following signature:

```
func (h *myHandler) Handle(ctx context.Context, store Store, record Record) error {
  // process record
  return nil
}
```

The record value is what was dequeued from the backing store. It's type is a nearly useless interface, thus the value must be cast to the expected type of job of concern to this handler before doing anything useful with it.

The store passed along with the record may be refined version of the store configured with the worker. For the database-backed store, it is a version of the configured store, but has been modified to execute all statements within the transaction that locked the record.

After processing a job, the worker will update a job's state (via the store) according to the handle hook's return value. A nil error will result in a _complete_ job; a retryable error (according to [this function](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.25.0/-/blob/internal/errcode/code.go#L174:6)) will result in an _errored_ job (which may be retried); any other error will result in a _failed_ job (which are not retried).

#### Hook 4: PostHandle (optional)

After the worker processes a record (successfully _or_ unsuccessfully), the _post handle_ hook (if defined) is invoked. The hook has the following signature:

```
func (h *myHandler) PostHandle(ctx context.Context, record Record) {
  // do something after
}
```

The record value is what was just processed. It's type is a nearly useless interface, thus the value must be cast to the expected type of job of concern to this handler before doing anything useful with it.

### Worker configuration

The worker's throughput behavior can be modified by adjusting additional options on the worker instance. The `Interval` option specifies the delay between job dequeue attempts. The `NumHandlers` option specifies the number of jobs that can be processed currently.

## Database-backed stores

The most common way to use a worker is to use the database-backed store. When using the [dbworker/store.Store](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.25.0/-/blob/internal/workerutil/dbworker/store/store.go#L204:6), you must also use the [dbworker/Worker](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.25.0/-/blob/internal/workerutil/dbworker/worker.go#L10:6), which slightly refines the type of the handler's `store` parameter. This type refinement allows database-backed handlers to operate in the same transactional context that dequeued the job record.

The store relies on a jobs table _specific to your worker_ to exist with the following columns. For a live example, see the [lsif_uploads table](https://github.com/sourcegraph/sourcegraph/blob/3.25/internal/database/schema.md#table-publiclsif_uploads).

| Name              | Type                     | Description |
| ----------------- | ------------------------ | ----------- |
| `id`              | integer                  | The job's primary key |
| `state`           | text                     | The job's current status (one of `queued`, `processing`, `errored`, or `failed`) |
| `failure_message` | text                     | Updated with the text of the error returned from the handle hook |
| `started_at`      | timestamp with time zone | Updated when the job is dequeued for processing |
| `finished_at`     | timestamp with time zone | Updated when the handler finishes processing the job (successfully or unsuccessfully) |
| `process_after`   | timestamp with time zone | Controls the time after which the job is visible for processing |
| `num_resets`      | integer                  | Updated when the job is moved back from `failed` to `queued` |
| `num_failures`    | integer                  | Updated when the job enters the `failed` state |
| `execution_logs`  | json[]                   | A list of log entries from the most recent processing attempt |

The target jobs table may have additional columns as the store only selects and updates records. Again, inserting/enqueueing job records is a task that is **not** handled by the worker, thus columns with non-null constraints are safe to add here as well.

The shape of the target table is configured via options on the database-backed store instance. The `TableName` option specifies the name of the table used in `UPDATE` and `SELECT [FOR UPDATE]` statements. The `ViewName` option, if supplied, specifies the view used in `SELECT` statements (the data of which is ultimately passed to the handler hook). This can be useful when the job record has foreign keys to other relations that should be eagerly selected.

The `ColumnExpressions` option is a list of `*sqlf.Query` values to select from the configured table or view. The `Scan` option specifies a function to call to read a job record from a `*sql.Rows` object. The values in the rows object are precisely the values selected via `ColumnExpressions`.

The `OrderByExpression` option specifies a `*sql.Query` expression which is used to order the records by priority. A dequeue operation will select the first record which is not currently being processed by another worker.

If the table has different column names than described above, they can be remapped via the `AlternateColumnNames` option. For example, the mapping `{"state": "status"}` will cause the store to use `status` in place of `state` in all queries.

### Retries

If the handle hook returns a retryable error, the the worker will update the job's state _errored_ and not _failed_ if the same job can be reprocessed in the future.

Retries are disabled by default, and can be enabled by setting the `MaxNumRetries` and `RetryAfter` options on the database-backed store. These options control the number of secondary processing attempts and the delay between attempts, respectively. Once a record hits the maximum number of retries, the worker will (permanently) move it to the state _failed_ on the next unsuccessful attempt.

### Dequeueing and resetting jobs

The database-backed store will dequeue a record from the target table using the following algorithm:

1. Outside of a transaction (so changes are visible to all readers), do:
    1. Select a record with the state _queued_ (or the has the state _errored_ and `now() >= process_after`)
    1. Update that record's state to _processing_ outside of a transaction so it's visible to all readers
1. Within a fresh transaction, do:
    1. `SELECT FOR UPDATE` the same record within the transaction
    1. Process the record and update the record's state within the transaction
    1. Commit to make state available to all readers

It may be the case that a job can be _orphaned_ between selecting/updating a record as _processing_ and actually processing the record. Because the first state change happens outside of a transaction, there is no automatic rollback when a worker crashes.

To handle this case, register a [resetter]() instance to periodically run in the background of the instance. This will select all records with the state _processing_ that have not been row-locked by some transaction and move them back to the _queued_ state.

This behavior can be controlled by setting the `StalledMaxAge` and `MaxNumResets` options on the database-backed store instance, which control the maximum grace period setting a record to _processing_ and locking it and number of times a record can be reset (to avoid poison messages from indefinitely crashing workers), respectively. Once a record hits the maximum number of resets, the resetter will move it from state _processing_ to _failed_ with a canned failure message.

## Adding a new worker

This guide will show you how to add a new database-backed worker instance.

#### Step 1: Create a jobs table

First, we create a table containing _at least_ the fields described above. We're also going to add a reference to a repository (by identifier). We're also define a view that additionally grabs the name of the associated repository from the `repo` table.

Defining this view is **optional** and is done here to showcase the flexibility in configuration. The rest of the tutorial would remain the same using the table name directly where the view is used (except, of course, references to fields defined only on the view).

```sql
BEGIN;

CREATE TABLE example_jobs (
  id              SERIAL PRIMARY KEY,
  state           text DEFAULT 'queued',
  failure_message text,
  started_at      timestamp with time zone,
  finished_at     timestamp with time zone,
  process_after   timestamp with time zone,
  num_resets      integer not null default 0,
  num_failures    integer not null default 0,

  repository_id integer not null
);

CREATE VIEW example_jobs_with_repository_name AS
  SELECT ej.*, r.name
  FROM example_jobs ej
  JOIN repo r ON r.id = ej.repository_id;

COMMIT;
```

We assume that the repository name is be necessary to process the record, meaning it would be best to grab it while dequeueing the job rather than making a second unconditional request.

#### Step 2: Write the model definition and scan function

Next, we define the struct instance `ExampleJob` that mirrors the interesting fields of the `example_jobs_with_repository_name` view.

We will additionally define an array of SQL column expressions that correspond to each field of the struct. For these expressions to be valid, we assume they will be embeddded in a query where `j` corresponds to a row of the `example_jobs_with_repository_name` table. Note that these expressions can be arbitrarily complex (conditional, sub-select expressions, etc).

```go
import (
	"time"

	"github.com/keegancsmith/sqlf"
)

type ExampleJob struct {
	ID             int
	State          string
	FailureMessage *string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	ProcessAfter   *time.Time
	NumResets      int
	NumFailures    int

	RepositoryID   int
	RepositoryName string
}

var exampleJobColumns = []*sqlf.Query{
	sqlf.Sprintf("j.id"),
	sqlf.Sprintf("j.state"),
	sqlf.Sprintf("j.failure_message"),
	sqlf.Sprintf("j.started_at"),
	sqlf.Sprintf("j.finished_at"),
	sqlf.Sprintf("j.process_after"),
	sqlf.Sprintf("j.num_resets"),
	sqlf.Sprintf("j.num_failures"),
	sqlf.Sprintf("j.repository_id"),
	sqlf.Sprintf("j.repository_name"),
}
```

Now, we define a function `scanFirstExampleJob` that consumes a `*sql.Rows` object and returns an `ExampleJob` struct value (hidden behind the abstract `workerutil.Record` type) and a boolean flag indicating whether the result rows were non-empty. We write this method to work specifically with the SQL expressions from `exampleJobColumns`, above.

```go
import (
	"database/sql"

	"github.com/sourcegraph/sourcegraph/internal/workerutil"
)

// scanFirstExampleJob scans a single job from the return value of `*Store.query`.
func scanFirstExampleJob(rows *sql.Rows, queryErr error) (_ workerutil.Record, exists bool, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	if rows.Next() {
		var job ExampleJob

		if err := rows.Scan(
			&job.ID,
			&job.State,
			&job.FailureMessage,
			&job.StartedAt,
			&job.FinishedAt,
			&job.ProcessAfter,
			&job.NumResets,
			&job.NumFailures,
			&job.RepositoryID,
			&job.RepositoryName,
		); err != nil {
			return nil, false, err
		}

		return job, true, nil
	}

	return ExampleJob{}, false, nil
}
```

This scanning function is a [basestore](./basestore.md) idiom which allows us to call it directly from the result of `*store.Query`:

```go
job, exists, err := scanFirstExampleJob(store.Query(
	"SELECT %s FROM example_jobs_with_repository_name LIMIT 1",
	sqlf.Join(expressions, ", "),
))
```

#### Step 3: Configure the store

Given our table definition and new scanning function, we can configure a database-backed worker store, as follows. This configuration will row-lock records in the `example_jobs` table in a transaction (specifically, the first unlocked record with the lowest `(repository_id, id)` value) and select the same record selected from the `example_jobs_with_repository_name` view.

```go
import (
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

func makeStore(db dbutil.DB) store.Store {
	return store.New(db, store.Options{
		Name:              "example_job_worker_store",
		TableName:         "example_jobs j",
		ViewName:          "example_jobs_with_repository_name j",
		ColumnExpressions: exampleJobColumns,
		Scan:              scanFirstExampleJob,
		OrderByExpression: sqlf.Sprintf("j.repository_id, j.id"),
		MaxNumResets:      5,
		StalledMaxAge:     time.Second * 5,
	})
}
```

Notice here that we provided a table and view name with an _alias_, which we can use to unambiguously refer to columns in the expressions listed in `exampleJobColumns`.

#### Step 4: Write the handler

We now have a way to dequeue jobs but no way to process them. We define our _handler_ logic, which is implemented to **specifically** for the `ExampleJob` record. We will ensure by construction of the worker process (in the next step) that our handler is only passed data that it knows how to process.

```go
import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

type handler struct {
	myOwnStore MyOwnStore
}

var _ dbworker.Handler = &handler{}

func (h *handler) Handle(ctx context.Context, tx store.Store, rawRecord workerutil.Record) error {
	// We're going to use ths transaction context given to us by the dbworker
	// for all of the stuff we're going to touch in the database while processing
	// this job. This ensures that no unsuccessful job attempt will make any
	// externally observable change in the (same) database.
	store := h.myOwnStore.With(tx)

	// Due to us registering our own Scan functions with the dbstore (see next step),
	// we can guarantee that the value of rawRecord will always be of a particular
	// processable type.
	record := rawRecord.(MyRecord)

	// Do the actual processing
	return store.Process(record)
}
```

#### Step 4: Configure the worker and resetter

Now that we have all of our constituent parts ready, we can finally construct our root objects that orchestrate the consumer behavior. Here, we make constructor functions for a worker instance as well as a resetter instance.

```go
import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

func makeWorker(ctx context.Context, workerStore store.Store, myOwnStore MyOwnStore) {
	handler := &handler{
		myOwnStore: myOwnStore,
	}

	return dbworker.NewWorker(ctx, store, handler, workerutil.WorkerOptions{
		Name:        "example_job_worker",
		Interval:    time.Second, // Poll for a job once per second
		NumHandlers: 1,           // Process only one job at a time (per instance)
	})
}

func makeResetter(workerStore store.Store) {
	return dbworker.NewResetter(workerStore, dbworker.ResetterOptions{
		Name:     "example_job_worker_resetter",
		Interval: time.Second * 30, // Check for orphaned jobs every 30 seconds
	})
}
```

#### Step 5: Register the worker and resetter

The results of `makeWorker` and `makeResetter` can then be passed to `goroutine.MonitorBackgroundRoutines`.

The worker and resetter may or or may execute in the same process. For example, we run all code intelligence background routines in the frontend, except for our LSIF conversion worker, which runs in a separate process for resource isolation and independent scaling.
