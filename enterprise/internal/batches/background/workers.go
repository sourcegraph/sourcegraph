package background

import (
	"context"
	"database/sql"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/reconciler"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

// reconcilerMaxNumRetries is the maximum number of attempts the reconciler
// makes to process a changeset when it fails.
const reconcilerMaxNumRetries = 60

// reconcilerMaxNumResets is the maximum number of attempts the reconciler
// makes to process a changeset when it stalls (process crashes, etc.).
const reconcilerMaxNumResets = 60

// newWorker creates a dbworker.newWorker that fetches enqueued changesets
// from the database and passes them to the changeset reconciler for
// processing.
func newWorker(
	ctx context.Context,
	s *store.Store,
	gitClient reconciler.GitserverClient,
	sourcer repos.Sourcer,
	metrics batchChangesMetrics,
) *workerutil.Worker {
	r := &reconciler.Reconciler{GitserverClient: gitClient, Sourcer: sourcer, Store: s}

	options := workerutil.WorkerOptions{
		Name:        "batches_reconciler_worker",
		NumHandlers: 5,
		Interval:    5 * time.Second,
		Metrics:     metrics.workerMetrics,
	}

	workerStore := createDBWorkerStore(s)

	worker := dbworker.NewWorker(ctx, workerStore, r.HandlerFunc(), options)
	return worker
}

func newWorkerResetter(s *store.Store, metrics batchChangesMetrics) *dbworker.Resetter {
	workerStore := createDBWorkerStore(s)

	options := dbworker.ResetterOptions{
		Name:     "batches_reconciler_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics: dbworker.ResetterMetrics{
			Errors:              metrics.errors,
			RecordResetFailures: metrics.resetFailures,
			RecordResets:        metrics.resets,
		},
	}

	resetter := dbworker.NewResetter(workerStore, options)
	return resetter
}

func scanFirstChangesetRecord(rows *sql.Rows, err error) (workerutil.Record, bool, error) {
	return store.ScanFirstChangeset(rows, err)
}

func createDBWorkerStore(s *store.Store) dbworkerstore.Store {
	return dbworkerstore.New(s.Handle(), dbworkerstore.Options{
		Name:                 "batches_reconciler_worker_store",
		TableName:            "changesets",
		ViewName:             "reconciler_changesets changesets",
		AlternateColumnNames: map[string]string{"state": "reconciler_state"},
		ColumnExpressions:    store.ChangesetColumns,
		Scan:                 scanFirstChangesetRecord,

		// Order changesets by state, so that freshly enqueued changesets have
		// higher priority.
		// If state is equal, prefer the newer ones.
		OrderByExpression: sqlf.Sprintf("changesets.reconciler_state = 'errored', changesets.updated_at DESC"),

		StalledMaxAge: 60 * time.Second,
		MaxNumResets:  reconcilerMaxNumResets,

		RetryAfter:    5 * time.Second,
		MaxNumRetries: reconcilerMaxNumRetries,
	})
}
