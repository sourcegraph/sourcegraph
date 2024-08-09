package background

import (
	"context"
	"strconv"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/background"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/own/types"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	tableName = "own_background_jobs"
	viewName  = "own_background_jobs_config_aware"
)

type Job struct {
	ID              int
	State           string
	FailureMessage  *string
	QueuedAt        time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ProcessAfter    *time.Time
	NumResets       int
	NumFailures     int
	LastHeartbeatAt time.Time
	ExecutionLogs   []executor.ExecutionLogEntry
	WorkerHostname  string
	Cancel          bool
	RepoId          int
	JobType         int
	ConfigName      string
}

func (b *Job) RecordID() int {
	return b.ID
}

func (b *Job) RecordUID() string {
	return strconv.Itoa(b.ID)
}

var jobColumns = []*sqlf.Query{
	sqlf.Sprintf("id"),
	sqlf.Sprintf("state"),
	sqlf.Sprintf("failure_message"),
	sqlf.Sprintf("queued_at"),
	sqlf.Sprintf("started_at"),
	sqlf.Sprintf("finished_at"),
	sqlf.Sprintf("process_after"),
	sqlf.Sprintf("num_resets"),
	sqlf.Sprintf("num_failures"),
	sqlf.Sprintf("last_heartbeat_at"),
	sqlf.Sprintf("execution_logs"),
	sqlf.Sprintf("worker_hostname"),
	sqlf.Sprintf("cancel"),
	sqlf.Sprintf("repo_id"),
	sqlf.Sprintf("config_name"),
}

func scanJob(s dbutil.Scanner) (*Job, error) {
	var job Job
	var executionLogs []executor.ExecutionLogEntry

	if err := s.Scan(
		&job.ID,
		&job.State,
		&job.FailureMessage,
		&job.QueuedAt,
		&job.StartedAt,
		&job.FinishedAt,
		&job.ProcessAfter,
		&job.NumResets,
		&job.NumFailures,
		&job.LastHeartbeatAt,
		pq.Array(&executionLogs),
		&job.WorkerHostname,
		&job.Cancel,
		&job.RepoId,
		&job.ConfigName,
	); err != nil {
		return nil, err
	}
	job.ExecutionLogs = append(job.ExecutionLogs, executionLogs...)
	return &job, nil
}

func NewOwnBackgroundWorker(ctx context.Context, db database.DB, observationCtx *observation.Context) []goroutine.BackgroundRoutine {
	worker, resetter, _ := makeWorker(ctx, db, observationCtx)
	janitor := background.NewJanitorJob(ctx, background.JanitorOptions{
		Name:        "own-background-jobs-janitor",
		Description: "Janitor for own-background-jobs queue",
		Interval:    time.Minute * 5,
		Metrics:     background.NewJanitorMetrics(observationCtx, "own-background-jobs-janitor"),
		CleanupFunc: janitorFunc(db, time.Hour*24*7),
	})
	return []goroutine.BackgroundRoutine{worker, resetter, janitor}
}

func makeWorkerStore(db database.DB, observationCtx *observation.Context) dbworkerstore.Store[*Job] {
	return dbworkerstore.New(observationCtx, db.Handle(), dbworkerstore.Options[*Job]{
		Name:              "own_background_worker_store",
		TableName:         tableName,
		ViewName:          viewName,
		ColumnExpressions: jobColumns,
		Scan:              dbworkerstore.BuildWorkerScan(scanJob),
		OrderByExpression: sqlf.Sprintf("id"), // processes oldest records first
		MaxNumResets:      10,
		StalledMaxAge:     time.Second * 30,
		RetryAfter:        time.Second * 30,
		MaxNumRetries:     3,
	})
}

func makeWorker(ctx context.Context, db database.DB, observationCtx *observation.Context) (*workerutil.Worker[*Job], *dbworker.Resetter[*Job], dbworkerstore.Store[*Job]) {
	workerStore := makeWorkerStore(db, observationCtx)

	limit, burst := getRateLimitConfig()
	limiter := rate.NewLimiter(limit, burst)
	indexLimiter := ratelimit.NewInstrumentedLimiter("OwnRepoIndexWorker", limiter)
	conf.Watch(func() {
		setRateLimitConfig(limiter)
	})

	task := handler{
		workerStore: workerStore,
		limiter:     indexLimiter,
		db:          db,
	}

	worker := dbworker.NewWorker(ctx, workerStore, workerutil.Handler[*Job](&task), workerutil.WorkerOptions{
		Name:              "own_background_worker",
		Description:       "Code ownership background processing partitioned by repository",
		NumHandlers:       getConcurrencyConfig(),
		Interval:          10 * time.Second,
		HeartbeatInterval: 20 * time.Second,
		Metrics:           workerutil.NewMetrics(observationCtx, "own_background_worker_processor"),
	})

	resetter := dbworker.NewResetter(ctx, log.Scoped("OwnBackgroundResetter"), workerStore, dbworker.ResetterOptions{
		Name:     "own_background_worker_resetter",
		Interval: time.Second * 20,
		Metrics:  dbworker.NewResetterMetrics(observationCtx, "own_background_worker"),
	})

	return worker, resetter, workerStore
}

type handler struct {
	db          database.DB
	workerStore dbworkerstore.Store[*Job]
	limiter     *ratelimit.InstrumentedLimiter
}

func (h *handler) Handle(ctx context.Context, lgr log.Logger, record *Job) error {
	err := h.limiter.Wait(ctx)
	if err != nil {
		return errors.Wrap(err, "limiter.Wait")
	}

	var delegate signalIndexFunc
	switch record.ConfigName {
	case types.SignalRecentContributors:
		delegate = handleRecentContributors
	case types.Analytics:
		delegate = handleAnalytics
	default:
		return errcode.MakeNonRetryable(errors.New("unsupported own index job type"))
	}

	return delegate(ctx, lgr, api.RepoID(record.RepoId), h.db)
}

type signalIndexFunc func(ctx context.Context, lgr log.Logger, repoId api.RepoID, db database.DB) error

// janitorQuery is split into 2 parts. The first half is records that are finished (either completed or failed), the second half is records for jobs that are not enabled.
func janitorQuery(deleteSince time.Time) *sqlf.Query {
	return sqlf.Sprintf("DELETE FROM %s WHERE (state NOT IN ('queued', 'processing', 'errored') AND finished_at < %s) OR (id NOT IN (select id from %s))", sqlf.Sprintf(tableName), deleteSince, sqlf.Sprintf(viewName))
}

func janitorFunc(db database.DB, retention time.Duration) func(ctx context.Context) (numRecordsScanned, numRecordsAltered int, err error) {
	return func(ctx context.Context) (numRecordsScanned, numRecordsAltered int, err error) {
		ts := time.Now().Add(-1 * retention)
		result, err := basestore.NewWithHandle(db.Handle()).ExecResult(ctx, janitorQuery(ts))
		if err != nil {
			return 0, 0, err
		}
		affected, _ := result.RowsAffected()
		return 0, int(affected), nil
	}
}

const (
	DefaultRateLimit      = 20
	DefaultRateBurstLimit = 5
	DefaultMaxConcurrency = 5
)

func getConcurrencyConfig() int {
	val := conf.Get().SiteConfiguration.OwnBackgroundRepoIndexConcurrencyLimit
	if val == 0 {
		val = DefaultMaxConcurrency
	}
	return val
}

func getRateLimitConfig() (rate.Limit, int) {
	limit := conf.Get().SiteConfiguration.OwnBackgroundRepoIndexRateLimit
	if limit == 0 {
		limit = DefaultRateLimit
	}
	burst := conf.Get().SiteConfiguration.OwnBackgroundRepoIndexRateBurstLimit
	if burst == 0 {
		burst = DefaultRateBurstLimit
	}
	return rate.Limit(limit), burst
}

func setRateLimitConfig(limiter *rate.Limiter) {
	limit, burst := getRateLimitConfig()
	limiter.SetLimit(limit)
	limiter.SetBurst(burst)
}
