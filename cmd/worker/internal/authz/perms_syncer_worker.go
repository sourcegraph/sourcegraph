package authz

import (
	"context"
	"fmt"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type syncType int

const (
	syncTypeRepo syncType = iota
	syncTypeUser
)

func makePermsSyncerWorker(observationCtx *observation.Context, syncer permsSyncer, syncType syncType, jobsStore database.PermissionSyncJobStore) *permsSyncerWorker {
	logger := observationCtx.Logger.Scoped("RepoPermsSyncerWorkerRepo")
	if syncType == syncTypeUser {
		logger = observationCtx.Logger.Scoped("UserPermsSyncerWorker")
	}
	return &permsSyncerWorker{
		logger:    logger,
		syncer:    syncer,
		syncType:  syncType,
		jobsStore: jobsStore,
	}
}

type permsSyncerWorker struct {
	logger    log.Logger
	syncer    permsSyncer
	syncType  syncType
	jobsStore database.PermissionSyncJobStore
}

// PreDequeue in our case does a nice trick of adding a predicate (WHERE clause)
// to worker dequeue SQL query. Depending on a type of worker, it will only
// dequeue corresponding jobs from the table.
func (h *permsSyncerWorker) PreDequeue(_ context.Context, _ log.Logger) (bool, any, error) {
	query := "repository_id IS NOT NULL"
	if h.syncType == syncTypeUser {
		query = "user_id IS NOT NULL"
	}
	return true, []*sqlf.Query{sqlf.Sprintf(query)}, nil
}

func (h *permsSyncerWorker) Handle(ctx context.Context, _ log.Logger, record *database.PermissionSyncJob) error {
	reqType := requestTypeUser
	reqID := int32(record.UserID)
	if record.RepositoryID != 0 {
		reqType = requestTypeRepo
		reqID = int32(record.RepositoryID)
	}

	h.logger.Debug(
		"Handling permissions sync job",
		log.String("type", reqType.String()),
		log.Int32("id", reqID),
		log.Int("priority", int(record.Priority)),
	)

	return h.handlePermsSync(ctx, reqType, reqID, record.ID, record.NoPerms, record.InvalidateCaches)
}

// handlePermsSync is effectively a sync version of `perms_syncer.syncPerms`
// which calls `perms_syncer.syncUserPerms` or `perms_syncer.syncRepoPerms`
// depending on a request type and logs/adds metrics of sync statistics
// afterwards.
func (h *permsSyncerWorker) handlePermsSync(ctx context.Context, reqType requestType, reqID int32, recordID int, noPerms, invalidateCaches bool) error {
	var err error
	var result *database.SetPermissionsResult
	var providerStates database.CodeHostStatusesSet

	switch reqType {
	case requestTypeUser:
		result, providerStates, err = h.syncer.syncUserPerms(ctx, reqID, noPerms, authz.FetchPermsOptions{InvalidateCaches: invalidateCaches})
	case requestTypeRepo:
		result, providerStates, err = h.syncer.syncRepoPerms(ctx, api.RepoID(reqID), noPerms, authz.FetchPermsOptions{InvalidateCaches: invalidateCaches})
	default:
		return errors.Newf("unexpected request type: %q", reqType)
	}

	// Adding an extra check in case of all providers errored out, but sync has been
	// completed successfully. This can happen e.g. if we only got HTTP401 responses.
	if err == nil {
		total, _, failed := providerStates.CountStatuses()
		if failed == total && total > 0 {
			err = errors.New("All providers failed to sync permissions.")
		}
	}

	if err != nil {
		h.logger.Error("failed to sync permissions", providerStates.SummaryField(), log.Error(err))
	} else {
		h.logger.Debug("succeeded in syncing permissions", providerStates.SummaryField())
	}

	// NOTE(naman): here we are saving permissions added, removed and found results
	// as well as the code host sync status to the job record.
	if saveErr := h.jobsStore.SaveSyncResult(ctx, recordID, err == nil, result, providerStates); saveErr != nil {
		err = errors.Append(err, saveErr)
		h.logger.Error(fmt.Sprintf("failed to save permissions sync job(%d) results", recordID), log.Error(saveErr))
	}

	return err
}

func makeStore(observationCtx *observation.Context, dbHandle basestore.TransactableHandle, syncType syncType) dbworkerstore.Store[*database.PermissionSyncJob] {
	name := "repo_permissions_sync_job_worker_store"
	if syncType == syncTypeUser {
		name = "user_permissions_sync_job_worker_store"
	}

	return dbworkerstore.New(observationCtx, dbHandle, dbworkerstore.Options[*database.PermissionSyncJob]{
		Name:              name,
		TableName:         "permission_sync_jobs",
		ColumnExpressions: database.PermissionSyncJobColumns,
		Scan:              dbworkerstore.BuildWorkerScan(database.ScanPermissionSyncJob),
		// NOTE(naman): the priority order to process the queue is as follows:
		// 1. priority: 10(high) > 5(medium) > 0(low)
		// 2. process_after: null(scheduled for immediate processing) > 1 > 2(scheduled for processing at a later time than 1)
		// 3. job_id: 1(old) > 2(enqueued after 1)
		OrderByExpression: sqlf.Sprintf("permission_sync_jobs.priority DESC, permission_sync_jobs.process_after ASC NULLS FIRST, permission_sync_jobs.id ASC"),
		MaxNumResets:      5,
		StalledMaxAge:     time.Second * 30,
	})
}

func makeWorker(ctx context.Context, observationCtx *observation.Context, workerStore dbworkerstore.Store[*database.PermissionSyncJob], permsSyncer *permsSyncerImpl, syncType syncType, jobsStore database.PermissionSyncJobStore) *workerutil.Worker[*database.PermissionSyncJob] {
	handler := makePermsSyncerWorker(observationCtx, permsSyncer, syncType, jobsStore)
	// Number of handlers depends on a type of perms sync jobs this worker processes.
	numHandlers := 1
	name := "repo_permissions_sync_job_worker"
	if syncType == syncTypeUser {
		name = "user_permissions_sync_job_worker"
		numHandlers = syncUsersMaxConcurrency()
	}

	return dbworker.NewWorker[*database.PermissionSyncJob](ctx, workerStore, handler, workerutil.WorkerOptions{
		Name:              name,
		Interval:          time.Second, // Poll for a job once per second
		HeartbeatInterval: 10 * time.Second,
		Metrics:           workerutil.NewMetrics(observationCtx, name),
		NumHandlers:       numHandlers,
	})
}

func makeResetter(ctx context.Context, observationCtx *observation.Context, workerStore dbworkerstore.Store[*database.PermissionSyncJob]) *dbworker.Resetter[*database.PermissionSyncJob] {
	return dbworker.NewResetter(ctx, observationCtx.Logger, workerStore, dbworker.ResetterOptions{
		Name:     "permissions_sync_job_worker_resetter",
		Interval: time.Second * 30, // Check for orphaned jobs every 30 seconds
		Metrics:  dbworker.NewResetterMetrics(observationCtx, "permissions_sync_job_worker"),
	})
}

func syncUsersMaxConcurrency() int {
	n := conf.Get().PermissionsSyncUsersMaxConcurrency
	if n <= 0 {
		return 1
	}
	return n
}
