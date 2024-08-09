package janitor

import (
	"context"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

func NewReconcilerWorkerResetter(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*types.Changeset], metrics *metrics) *dbworker.Resetter[*types.Changeset] {
	options := dbworker.ResetterOptions{
		Name:     "batches_reconciler_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics.reconcilerWorkerResetterMetrics,
	}

	resetter := dbworker.NewResetter(ctx, logger, workerStore, options)
	return resetter
}

// NewBulkOperationWorkerResetter creates a dbworker.Resetter that reenqueues lost jobs
// for processing.
func NewBulkOperationWorkerResetter(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*types.ChangesetJob], metrics *metrics) *dbworker.Resetter[*types.ChangesetJob] {
	options := dbworker.ResetterOptions{
		Name:     "batches_bulk_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics.bulkProcessorWorkerResetterMetrics,
	}

	resetter := dbworker.NewResetter(ctx, logger, workerStore, options)
	return resetter
}

// NewBatchSpecWorkspaceExecutionWorkerResetter creates a dbworker.Resetter that re-enqueues
// lost batch_spec_workspace_execution_jobs for processing.
func NewBatchSpecWorkspaceExecutionWorkerResetter(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*types.BatchSpecWorkspaceExecutionJob], metrics *metrics) *dbworker.Resetter[*types.BatchSpecWorkspaceExecutionJob] {
	options := dbworker.ResetterOptions{
		Name:     "batch_spec_workspace_execution_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics.batchSpecWorkspaceExecutionWorkerResetterMetrics,
	}

	resetter := dbworker.NewResetter(ctx, logger, workerStore, options)
	return resetter
}

func NewBatchSpecWorkspaceResolutionWorkerResetter(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*types.BatchSpecResolutionJob], metrics *metrics) *dbworker.Resetter[*types.BatchSpecResolutionJob] {
	options := dbworker.ResetterOptions{
		Name:     "batch_changes_batch_spec_resolution_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics.batchSpecResolutionWorkerResetterMetrics,
	}

	resetter := dbworker.NewResetter(ctx, logger, workerStore, options)
	return resetter
}
