package repos

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type SyncWorkerOptions struct {
	NumHandlers            int           // defaults to 3
	WorkerInterval         time.Duration // defaults to 10s
	CleanupOldJobs         bool          // run a background process to cleanup old jobs
	CleanupOldJobsInterval time.Duration // defaults to 1h
}

// NewSyncWorker creates a new external service sync worker, resetter, and janitor
// to clean up old job records.
func NewSyncWorker(ctx context.Context, observationCtx *observation.Context, dbHandle basestore.TransactableHandle, handler workerutil.Handler[*SyncJob], opts SyncWorkerOptions) (*workerutil.Worker[*SyncJob], *dbworker.Resetter[*SyncJob], goroutine.BackgroundRoutine) {
	if opts.NumHandlers == 0 {
		opts.NumHandlers = 3
	}
	if opts.WorkerInterval == 0 {
		opts.WorkerInterval = 10 * time.Second
	}
	if opts.CleanupOldJobsInterval == 0 {
		opts.CleanupOldJobsInterval = time.Hour
	}

	syncJobColumns := []*sqlf.Query{
		sqlf.Sprintf("id"),
		sqlf.Sprintf("state"),
		sqlf.Sprintf("failure_message"),
		sqlf.Sprintf("started_at"),
		sqlf.Sprintf("finished_at"),
		sqlf.Sprintf("process_after"),
		sqlf.Sprintf("num_resets"),
		sqlf.Sprintf("num_failures"),
		sqlf.Sprintf("execution_logs"),
		sqlf.Sprintf("external_service_id"),
		sqlf.Sprintf("next_sync_at"),
	}

	observationCtx = observation.ContextWithLogger(observationCtx.Logger.Scoped("repo.sync.workerstore.Store"), observationCtx)

	store := dbworkerstore.New(observationCtx, dbHandle, dbworkerstore.Options[*SyncJob]{
		Name:              "repo_sync_worker_store",
		TableName:         "external_service_sync_jobs",
		ViewName:          "external_service_sync_jobs_with_next_sync_at",
		Scan:              dbworkerstore.BuildWorkerScan(scanJob),
		OrderByExpression: sqlf.Sprintf("next_sync_at"),
		ColumnExpressions: syncJobColumns,
		StalledMaxAge:     30 * time.Second,
		MaxNumResets:      5,
		MaxNumRetries:     0,
	})

	worker := dbworker.NewWorker(ctx, store, handler, workerutil.WorkerOptions{
		Name:              "repo_sync_worker",
		Description:       "syncs repos in a streaming fashion",
		NumHandlers:       opts.NumHandlers,
		Interval:          opts.WorkerInterval,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           newWorkerMetrics(observationCtx),
	})

	resetter := dbworker.NewResetter(ctx, observationCtx.Logger.Scoped("repo.sync.worker.Resetter"), store, dbworker.ResetterOptions{
		Name:     "repo_sync_worker_resetter",
		Interval: 5 * time.Minute,
		Metrics:  newResetterMetrics(observationCtx),
	})

	var janitor goroutine.BackgroundRoutine
	if opts.CleanupOldJobs {
		janitor = newJobCleanerRoutine(ctx, database.NewDBWith(observationCtx.Logger, basestore.NewWithHandle(dbHandle)), opts.CleanupOldJobsInterval)
	} else {
		janitor = goroutine.NoopRoutine("noop sync worker")
	}

	return worker, resetter, janitor
}

func newWorkerMetrics(observationCtx *observation.Context) workerutil.WorkerObservability {
	observationCtx = observation.ContextWithLogger(log.Scoped("sync_worker"), observationCtx)

	return workerutil.NewMetrics(observationCtx, "repo_updater_external_service_syncer")
}

func newResetterMetrics(observationCtx *observation.Context) dbworker.ResetterMetrics {
	return dbworker.ResetterMetrics{
		RecordResets: promauto.With(observationCtx.Registerer).NewCounter(prometheus.CounterOpts{
			Name: "src_external_service_queue_resets_total",
			Help: "Total number of external services put back into queued state",
		}),
		RecordResetFailures: promauto.With(observationCtx.Registerer).NewCounter(prometheus.CounterOpts{
			Name: "src_external_service_queue_max_resets_total",
			Help: "Total number of external services that exceed the max number of resets",
		}),
		Errors: promauto.With(observationCtx.Registerer).NewCounter(prometheus.CounterOpts{
			Name: "src_external_service_queue_reset_errors_total",
			Help: "Total number of errors when running the external service resetter",
		}),
	}
}

func newJobCleanerRoutine(ctx context.Context, db database.DB, interval time.Duration) goroutine.BackgroundRoutine {
	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(ctx),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			err := db.ExternalServices().CleanupSyncJobs(ctx, database.ExternalServicesCleanupSyncJobsOptions{
				OlderThan:             30 * 24 * time.Hour, // 30 days
				MaxPerExternalService: 100,
			})
			return errors.Wrap(err, "error while running job cleaner")
		}),
		goroutine.WithName("repo-updater.sync-job-cleaner"),
		goroutine.WithDescription("periodically cleans old sync jobs from the database"),
		goroutine.WithInterval(interval),
	)
}

// SyncJob represents an external service that needs to be synced
type SyncJob struct {
	ID                int
	State             string
	FailureMessage    sql.NullString
	StartedAt         sql.NullTime
	FinishedAt        sql.NullTime
	ProcessAfter      sql.NullTime
	NumResets         int
	NumFailures       int
	ExternalServiceID int64
	NextSyncAt        sql.NullTime
}

// RecordID implements workerutil.Record and indicates the queued item id
func (s *SyncJob) RecordID() int {
	return s.ID
}

func (s *SyncJob) RecordUID() string {
	return strconv.Itoa(s.ID)
}
