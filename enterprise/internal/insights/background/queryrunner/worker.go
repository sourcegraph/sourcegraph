package queryrunner

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

// This file contains all the methods required to:
//
// 1. Create the query runner worker
// 2. Enqueue jobs for the query runner to execute.
// 3. Dequeue jobs from the query runner.
// 4. Serialize jobs for the query runner into the DB.
//

// NewWorker returns a worker that will execute search queries and insert information about the
// results into the code insights database.
func NewWorker(ctx context.Context, workerBaseStore *basestore.Store, insightsStore *store.Store, metrics workerutil.WorkerMetrics) *workerutil.Worker {
	workerStore := createDBWorkerStore(workerBaseStore)
	options := workerutil.WorkerOptions{
		Name:        "insights_query_runner_worker",
		NumHandlers: 1,
		Interval:    5 * time.Second,
		Metrics:     metrics,
	}
	return dbworker.NewWorker(ctx, workerStore, &workHandler{
		workerBaseStore: workerBaseStore,
		insightsStore:   insightsStore,
	}, options)
}

// NewResetter returns a resetter that will reset pending query runner jobs if they take too long
// to complete.
func NewResetter(ctx context.Context, workerBaseStore *basestore.Store, metrics dbworker.ResetterMetrics) *dbworker.Resetter {
	workerStore := createDBWorkerStore(workerBaseStore)
	options := dbworker.ResetterOptions{
		Name:     "insights_query_runner_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics,
	}
	return dbworker.NewResetter(workerStore, options)
}

// createDBWorkerStore creates the dbworker store for the query runner worker.
//
// See internal/workerutil/dbworker for more information about dbworkers.
func createDBWorkerStore(s *basestore.Store) dbworkerstore.Store {
	return dbworkerstore.New(s.Handle(), dbworkerstore.Options{
		Name:              "insights_query_runner_jobs_store",
		TableName:         "insights_query_runner_jobs",
		ColumnExpressions: jobsColumns,
		Scan:              scanJobs,

		// We will let a search query or webhook run for up to 60s. After that, it times out and
		// retries in 10s. If 3 timeouts occur, it is not retried.
		//
		// If you change this, be sure to adjust the interval that work is enqueued in
		// enterprise/internal/insights/background:newInsightEnqueuer.
		StalledMaxAge:     60 * time.Second,
		RetryAfter:        10 * time.Second,
		MaxNumRetries:     3,
		OrderByExpression: sqlf.Sprintf("id"),
	})
}

// EnqueueJob enqueues a job for the query runner worker to execute later.
func EnqueueJob(ctx context.Context, workerBaseStore *basestore.Store, job *Job) (id int, err error) {
	id, _, err = basestore.ScanFirstInt(workerBaseStore.Query(
		ctx,
		sqlf.Sprintf(
			enqueueJobFmtStr,
			job.SeriesID,
			job.SearchQuery,
			job.RecordTime,
			job.State,
			job.ProcessAfter,
		),
	))
	return
}

const enqueueJobFmtStr = `
-- source: enterprise/internal/insights/background/queryrunner/worker.go:EnqueueJob
INSERT INTO insights_query_runner_jobs (
	series_id,
	search_query,
	record_time,
	state,
	process_after
) VALUES (%s, %s, %s, %s, %s)
RETURNING id
`

func dequeueJob(ctx context.Context, workerBaseStore *basestore.Store, recordID int) (*Job, error) {
	rows, err := workerBaseStore.Query(ctx, sqlf.Sprintf(dequeueJobFmtStr, recordID))
	if err != nil {
		return nil, err
	}
	jobs, err := doScanJobs(rows, nil)
	if err != nil {
		return nil, err
	}
	if len(jobs) != 1 {
		return nil, fmt.Errorf("expected 1 job to dequeue, found %v", len(jobs))
	}
	return jobs[0], nil
}

const dequeueJobFmtStr = `
-- source: enterprise/internal/insights/background/queryrunner/worker.go:dequeueJob
SELECT
	series_id,
	search_query,
	record_time,
	id,
	state,
	failure_message,
	started_at,
	finished_at,
	process_after,
	num_resets,
	num_failures,
	execution_logs
FROM insights_query_runner_jobs
WHERE id = %s;
`

// Job represents a single job for the query runner worker to perform. When enqueued, it is stored
// in the insights_query_runner_jobs table - then the worker dequeues it by reading it from that
// table.
//
// See internal/workerutil/dbworker for more information about dbworkers.
type Job struct {
	// Query runner fields.
	SeriesID    string
	SearchQuery string
	RecordTime  *time.Time // If non-nil, record results at this time instead of the time at which search results were found.

	// Standard/required dbworker fields. If enqueuing a job, these may all be zero values except State.
	//
	// See https://sourcegraph.com/github.com/sourcegraph/sourcegraph@cd0b3904c674ee3568eb2ef5d7953395b6432d20/-/blob/internal/workerutil/dbworker/store/store.go#L114-134
	ID             int
	State          string // If enqueing a job, set to "queued"
	FailureMessage *string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	ProcessAfter   *time.Time
	NumResets      int32
	NumFailures    int32
	ExecutionLogs  []workerutil.ExecutionLogEntry
}

// Implements the internal/workerutil.Record interface, used by the work handler to locate the job
// once executing (see work_handler.go:Handle).
func (j *Job) RecordID() int {
	return j.ID
}

func scanJobs(rows *sql.Rows, err error) (workerutil.Record, bool, error) {
	records, err := doScanJobs(rows, err)
	if err != nil {
		return &Job{}, false, err
	}
	return records[0], true, nil
}

func doScanJobs(rows *sql.Rows, err error) ([]*Job, error) {
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()
	var jobs []*Job
	for rows.Next() {
		j := &Job{}
		if err := rows.Scan(
			// Query runner fields.
			&j.SeriesID,
			&j.SearchQuery,
			&j.RecordTime,

			// Standard/required dbworker fields.
			&j.ID,
			&j.State,
			&j.FailureMessage,
			&j.StartedAt,
			&j.FinishedAt,
			&j.ProcessAfter,
			&j.NumResets,
			&j.NumFailures,
			pq.Array(&j.ExecutionLogs),
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	if err != nil {
		return nil, err
	}
	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

var jobsColumns = append([]*sqlf.Query{
	sqlf.Sprintf("insights_query_runner_jobs.series_id"),
	sqlf.Sprintf("insights_query_runner_jobs.search_query"),
	sqlf.Sprintf("insights_query_runner_jobs.record_time"),
}, dbworkerstore.DefaultColumnExpressions()...)
