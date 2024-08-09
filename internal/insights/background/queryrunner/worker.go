package queryrunner

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/insights/compression"
	"github.com/sourcegraph/sourcegraph/internal/insights/discovery"
	"github.com/sourcegraph/sourcegraph/internal/insights/priority"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
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
func NewWorker(ctx context.Context, logger log.Logger, workerStore *workerStoreExtra, insightsStore *store.Store, repoStore discovery.RepoStore, metrics workerutil.WorkerObservability, limiter *ratelimit.InstrumentedLimiter) *workerutil.Worker[*Job] {
	numHandlers := conf.Get().InsightsQueryWorkerConcurrency
	if numHandlers <= 0 {
		// Default concurrency is set to 5.
		numHandlers = 5
	}

	options := workerutil.WorkerOptions{
		Name:              "insights_query_runner_worker",
		Description:       "runs code insights queries for daily snapshots and new recordings",
		NumHandlers:       numHandlers,
		Interval:          5 * time.Second,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           metrics,
	}

	sharedCache := make(map[string]*types.InsightSeries)

	prometheus.DefaultRegisterer.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_query_runner_worker_total",
		Help: "Total number of jobs in the queued state.",
	}, func() float64 {
		count, err := workerStore.CountByState(context.Background(), dbworkerstore.StateQueued|dbworkerstore.StateErrored)
		if err != nil {
			logger.Error("Failed to get queued job count", log.Error(err))
		}

		return float64(count)
	}))

	return dbworker.NewWorker[*Job](ctx, workerStore, &workHandler{
		baseWorkerStore: workerStore,
		insightsStore:   insightsStore,
		repoStore:       repoStore,
		limiter:         limiter,
		metadadataStore: store.NewInsightStoreWith(insightsStore),
		seriesCache:     sharedCache,
		searchHandlers:  GetSearchHandlers(),
		logger:          log.Scoped("insights.queryRunner.Handler"),
	}, options)
}

// NewResetter returns a resetter that will reset pending query runner jobs if they take too long
// to complete.
func NewResetter(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*Job], metrics dbworker.ResetterMetrics) *dbworker.Resetter[*Job] {
	options := dbworker.ResetterOptions{
		Name:     "insights_query_runner_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics,
	}
	return dbworker.NewResetter(ctx, logger, workerStore, options)
}

// CreateDBWorkerStore creates the dbworker store for the query runner worker.
//
// See internal/workerutil/dbworker for more information about dbworkers.
func CreateDBWorkerStore(observationContext *observation.Context, s *basestore.Store) *workerStoreExtra {
	options := dbworkerstore.Options[*Job]{
		Name:              "insights_query_runner_jobs_store",
		TableName:         "insights_query_runner_jobs",
		ColumnExpressions: jobsColumns,
		Scan:              dbworkerstore.BuildWorkerScan(scanJob),

		// If you change this, be sure to adjust the interval that work is enqueued in
		// internal/insights/background:newInsightEnqueuer.
		StalledMaxAge:     60 * time.Second,
		RetryAfter:        30 * time.Minute,
		MaxNumRetries:     10,
		MaxNumResets:      10,
		OrderByExpression: sqlf.Sprintf("priority, id"),
	}
	inner := dbworkerstore.New(observationContext, s.Handle(), options)
	return &workerStoreExtra{Store: inner, options: options}
}

type workerStoreExtra struct {
	dbworkerstore.Store[*Job]
	options dbworkerstore.Options[*Job]
}

// WillRetry will return true if the next iteration of this job is valid (would
// retry) or false if this is the last iteration.
func (w *workerStoreExtra) WillRetry(job *Job) bool {
	return int(job.NumFailures)+1 < w.options.MaxNumRetries
}

func getDependencies(ctx context.Context, workerBaseStore *basestore.Store, jobID int) (_ []time.Time, err error) {
	q := sqlf.Sprintf(getJobDependencies, jobID)
	return scanDependencies(workerBaseStore.Query(ctx, q))
}

func scanDependencies(rows *sql.Rows, queryErr error) (_ []time.Time, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	results := make([]time.Time, 0)
	for rows.Next() {
		var temp time.Time
		if err := rows.Scan(&temp); err != nil {
			return nil, err
		}
		results = append(results, temp)
	}
	return results, nil
}

func insertDependencies(ctx context.Context, workerBaseStore *basestore.Store, job *Job) error {
	vals := make([]*sqlf.Query, 0, len(job.DependentFrames))
	for _, frame := range job.DependentFrames {
		vals = append(vals, sqlf.Sprintf("(%s, %s)", job.ID, frame))
	}
	if len(vals) == 0 {
		return nil
	}
	q := sqlf.Sprintf(insertJobDependencies, sqlf.Join(vals, ","))
	if err := workerBaseStore.Exec(ctx, q); err != nil {
		return err
	}
	return nil
}

const getJobDependencies = `
select recording_time from insights_query_runner_jobs_dependencies where job_id = %s;
`

const insertJobDependencies = `
INSERT INTO insights_query_runner_jobs_dependencies (job_id, recording_time) VALUES %s;`

// EnqueueJob enqueues a job for the query runner worker to execute later.
func EnqueueJob(ctx context.Context, workerBaseStore *basestore.Store, job *Job) (id int, err error) {
	tx, err := workerBaseStore.Transact(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { err = tx.Done(err) }()

	id, _, err = basestore.ScanFirstInt(tx.Query(
		ctx,
		sqlf.Sprintf(
			enqueueJobFmtStr,
			job.SeriesID,
			job.SearchQuery,
			job.RecordTime,
			job.State,
			job.ProcessAfter,
			job.Cost,
			job.Priority,
			job.PersistMode,
		),
	))
	if err != nil {
		return 0, err
	}
	job.ID = id
	if err := insertDependencies(ctx, tx, job); err != nil {
		return 0, nil
	}
	return id, nil
}

const enqueueJobFmtStr = `
INSERT INTO insights_query_runner_jobs (
	series_id,
	search_query,
	record_time,
	state,
	process_after,
	cost,
	priority,
	persist_mode
) VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
RETURNING id
`

// PurgeJobsForSeries removes all jobs for a seriesID.
func PurgeJobsForSeries(ctx context.Context, workerBaseStore *basestore.Store, seriesID string) (err error) {
	tx, err := workerBaseStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	err = tx.Exec(ctx, sqlf.Sprintf(purgeJobsForSeriesFmtStr, seriesID))
	return err
}

const purgeJobsForSeriesFmtStr = `
DELETE FROM insights_query_runner_jobs
WHERE series_id = %s
`

func dequeueJob(ctx context.Context, workerBaseStore *basestore.Store, recordID int) (_ *Job, err error) {
	tx, err := workerBaseStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	rows, err := tx.Query(ctx, sqlf.Sprintf(dequeueJobFmtStr, recordID))
	if err != nil {
		return nil, err
	}
	jobs, err := scanJobs(rows, nil)
	if err != nil {
		return nil, err
	}
	if len(jobs) != 1 {
		return nil, errors.Errorf("expected 1 job to dequeue, found %v", len(jobs))
	}

	deps, err := getDependencies(ctx, tx, recordID)
	if err != nil {
		return nil, err
	}
	job := jobs[0]
	job.DependentFrames = deps

	return job, nil
}

const dequeueJobFmtStr = `
SELECT
	series_id,
	search_query,
	record_time,
	cost,
	priority,
	persist_mode,
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

type JobsStatus struct {
	Queued, Processing uint64
	Completed          uint64
	Errored, Failed    uint64
}

// QueryJobsStatus queries the current status of jobs for the specified series.
func QueryJobsStatus(ctx context.Context, workerBaseStore *basestore.Store, seriesID string) (_ *JobsStatus, err error) {
	var status JobsStatus

	rows, err := workerBaseStore.Query(ctx, sqlf.Sprintf(queryJobsStatusSql, seriesID))
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		var state string
		var value int
		if err := rows.Scan(&state, &value); err != nil {
			return nil, err
		}
		switch state {
		case "queued":
			status.Queued = uint64(value)
		case "processing":
			status.Processing = uint64(value)
		case "completed":
			status.Completed = uint64(value)
		case "errored":
			status.Errored = uint64(value)
		case "failed":
			status.Failed = uint64(value)
		}
	}
	return &status, nil
}

const queryJobsStatusSql = `
SELECT state, COUNT(*) FROM insights_query_runner_jobs WHERE series_id=%s GROUP BY state
`

func QueryAllSeriesStatus(ctx context.Context, workerBaseStore *basestore.Store) (_ []types.InsightSeriesStatus, err error) {
	q := sqlf.Sprintf(queryAllSeriesStatusSql, true)
	query, err := workerBaseStore.Query(ctx, q)
	return scanSeriesStatusRows(query, err)
}

func QuerySeriesStatus(ctx context.Context, workerBaseStore *basestore.Store, seriesIDs []string) (_ []types.InsightSeriesStatus, err error) {
	q := sqlf.Sprintf(queryAllSeriesStatusSql, sqlf.Sprintf("series_id = ANY(%s)", pq.Array(seriesIDs)))
	query, err := workerBaseStore.Query(ctx, q)
	return scanSeriesStatusRows(query, err)
}

func scanSeriesStatusRows(rows *sql.Rows, queryErr error) (_ []types.InsightSeriesStatus, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var results []types.InsightSeriesStatus
	for rows.Next() {
		var temp types.InsightSeriesStatus
		if err := rows.Scan(
			&temp.SeriesId,
			&temp.Errored,
			&temp.Processing,
			&temp.Failed,
			&temp.Completed,
			&temp.Queued,
		); err != nil {
			return []types.InsightSeriesStatus{}, err
		}
		results = append(results, temp)
	}
	return results, nil
}

const queryAllSeriesStatusSql = `
select
       series_id,
       sum(case when state = 'errored' then 1 else 0 end) as errored,
       sum(case when state = 'processing' then 1 else 0 end) as processing,
       sum(case when state = 'failed' then 1 else 0 end) as failed,
       sum(case when state = 'completed' then 1 else 0 end) as completed,
       sum(case when state = 'queued' then 1 else 0 end) as queued
from insights_query_runner_jobs
WHERE %s
group by series_id
order by series_id;
`

func QuerySeriesSearchFailures(ctx context.Context, workerBaseStore *basestore.Store, seriesID string, limit int) (_ []types.InsightSearchFailure, err error) {
	errorStates := []string{"errored", "failed"}
	switch {
	case limit <= 0:
		limit = 50
	case limit > 500:
		limit = 500
	}

	q := sqlf.Sprintf(`
						SELECT
							search_query,
							queued_at,
							failure_message,
							state,
							record_time,
							persist_mode
					FROM insights_query_runner_jobs
					WHERE series_id = %s AND state = ANY (%s)
					ORDER BY queued_at desc
					LIMIT %d;`,
		seriesID, pq.Array(&errorStates), limit)
	query, err := workerBaseStore.Query(ctx, q)
	return scanSearchFailureRows(query, err)
}

func scanSearchFailureRows(rows *sql.Rows, queryErr error) (_ []types.InsightSearchFailure, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var results []types.InsightSearchFailure
	for rows.Next() {
		var temp types.InsightSearchFailure
		if err := rows.Scan(
			&temp.Query,
			&temp.QueuedAt,
			&temp.FailureMessage,
			&temp.State,
			&temp.RecordTime,
			&temp.PersistMode,
		); err != nil {
			return []types.InsightSearchFailure{}, err
		}
		results = append(results, temp)
	}
	return results, nil
}

// Job represents a single job for the query runner worker to perform. When enqueued, it is stored
// in the insights_query_runner_jobs table - then the worker dequeues it by reading it from that
// table.
//
// See internal/workerutil/dbworker for more information about dbworkers.

type SearchJob struct {
	SeriesID        string
	SearchQuery     string
	RecordTime      *time.Time
	PersistMode     string
	DependentFrames []time.Time
}

type Job struct {
	// Query runner fields.
	SearchJob

	Cost     int
	Priority int
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
	ExecutionLogs  []executor.ExecutionLogEntry
}

// Implements the internal/workerutil.Record interface, used by the work handler to locate the job
// once executing (see work_handler.go:Handle).
func (j *Job) RecordID() int {
	return j.ID
}

func (j *Job) RecordUID() string {
	return strconv.Itoa(j.ID)
}

func scanJobs(rows *sql.Rows, err error) ([]*Job, error) {
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()
	var jobs []*Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
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

func scanJob(sc dbutil.Scanner) (*Job, error) {
	j := &Job{}
	if err := sc.Scan(
		// Query runner fields.
		&j.SeriesID,
		&j.SearchQuery,
		&j.RecordTime,
		&j.Cost,
		&j.Priority,
		&j.PersistMode,

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

	return j, nil
}

var jobsColumns = []*sqlf.Query{
	sqlf.Sprintf("insights_query_runner_jobs.series_id"),
	sqlf.Sprintf("insights_query_runner_jobs.search_query"),
	sqlf.Sprintf("insights_query_runner_jobs.record_time"),
	sqlf.Sprintf("insights_query_runner_jobs.cost"),
	sqlf.Sprintf("insights_query_runner_jobs.priority"),
	sqlf.Sprintf("insights_query_runner_jobs.persist_mode"),
	sqlf.Sprintf("id"),
	sqlf.Sprintf("state"),
	sqlf.Sprintf("failure_message"),
	sqlf.Sprintf("started_at"),
	sqlf.Sprintf("finished_at"),
	sqlf.Sprintf("process_after"),
	sqlf.Sprintf("num_resets"),
	sqlf.Sprintf("num_failures"),
	sqlf.Sprintf("execution_logs"),
}

// ToQueueJob converts the query execution into a queueable job with its relevant dependent times.
func ToQueueJob(q compression.QueryExecution, seriesID string, query string, cost priority.Cost, jobPriority priority.Priority) *Job {
	return &Job{
		SearchJob: SearchJob{
			SeriesID:        seriesID,
			SearchQuery:     query,
			RecordTime:      &q.RecordingTime,
			PersistMode:     string(store.RecordMode),
			DependentFrames: q.SharedRecordings,
		},
		Cost:     int(cost),
		Priority: int(jobPriority),
		State:    "queued",
	}
}
