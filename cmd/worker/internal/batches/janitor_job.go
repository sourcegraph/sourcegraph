package batches

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/batches/janitor"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/executorqueue"
	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type janitorJob struct{}

func NewJanitorJob() job.Job {
	return &janitorJob{}
}

func (j *janitorJob) Description() string {
	return ""
}

func (j *janitorJob) Config() []env.Config {
	return []env.Config{janitorConfigInst}
}

func (j *janitorJob) Routines(_ context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	if !batches.IsEnabled() {
		return nil, nil
	}
	observationCtx = observation.NewContext(observationCtx.Logger.Scoped("routines"))
	workCtx := actor.WithInternalActor(context.Background())

	bstore, err := InitStore()
	if err != nil {
		return nil, err
	}

	janitorMetrics := janitor.NewMetrics(observationCtx)

	reconcilerStore, err := InitReconcilerWorkerStore()
	if err != nil {
		return nil, err
	}
	bulkOperationStore, err := InitBulkOperationWorkerStore()
	if err != nil {
		return nil, err
	}
	workspaceExecutionStore, err := InitBatchSpecWorkspaceExecutionWorkerStore()
	if err != nil {
		return nil, err
	}
	workspaceResolutionStore, err := InitBatchSpecResolutionWorkerStore()
	if err != nil {
		return nil, err
	}

	executorMetricsReporter, err := executorqueue.NewMetricReporter(observationCtx, "batches", workspaceExecutionStore, janitorConfigInst.MetricsConfig)
	if err != nil {
		return nil, err
	}

	routines := []goroutine.BackgroundRoutine{
		executorMetricsReporter,

		janitor.NewReconcilerWorkerResetter(
			workCtx,
			observationCtx.Logger.Scoped("ReconcilerWorkerResetter"),
			reconcilerStore,
			janitorMetrics,
		),
		janitor.NewBulkOperationWorkerResetter(
			workCtx,
			observationCtx.Logger.Scoped("BulkOperationWorkerResetter"),
			bulkOperationStore,
			janitorMetrics,
		),
		janitor.NewBatchSpecWorkspaceExecutionWorkerResetter(
			workCtx,
			observationCtx.Logger.Scoped("BatchSpecWorkspaceExecutionWorkerResetter"),
			workspaceExecutionStore,
			janitorMetrics,
		),
		janitor.NewBatchSpecWorkspaceResolutionWorkerResetter(
			workCtx,
			observationCtx.Logger.Scoped("BatchSpecWorkspaceResolutionWorkerResetter"),
			workspaceResolutionStore,
			janitorMetrics,
		),

		janitor.NewSpecExpirer(workCtx, bstore),
		janitor.NewCacheEntryCleaner(workCtx, bstore),
		janitor.NewChangesetDetachedCleaner(workCtx, bstore),
	}

	return routines, nil
}
