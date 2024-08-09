package background

import (
	"context"
	"os"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codegraph"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background/backfiller"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background/commitgraph"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background/expirer"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background/janitor"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background/processor"
	uploadsstore "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/object"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

func NewUploadProcessorJob(
	ctx context.Context,
	observationCtx *observation.Context,
	store uploadsstore.Store,
	dataStore codegraph.DataStore,
	repoStore processor.RepoStore,
	gitserverClient gitserver.Client,
	db database.DB,
	uploadStore object.Storage,
	config *processor.Config,
) []goroutine.BackgroundRoutine {
	metrics := processor.NewResetterMetrics(observationCtx)
	uploadsProcessorStore := dbworkerstore.New(observationCtx, db.Handle(), uploadsstore.UploadWorkerStoreOptions)
	uploadsResetterStore := dbworkerstore.New(observationCtx.Clone(observation.Honeycomb(nil)), db.Handle(), uploadsstore.UploadWorkerStoreOptions)
	dbworker.InitPrometheusMetric(observationCtx, uploadsProcessorStore, "codeintel", "upload", nil)

	return []goroutine.BackgroundRoutine{
		processor.NewUploadProcessorWorker(
			observationCtx,
			store,
			dataStore,
			gitserverClient,
			repoStore,
			uploadsProcessorStore,
			uploadStore,
			config,
		),
		processor.NewUploadResetter(ctx, observationCtx.Logger, uploadsResetterStore, metrics),
	}
}

func NewCommittedAtBackfillerJob(
	store uploadsstore.Store,
	gitserverClient gitserver.Client,
	config *backfiller.Config,
) []goroutine.BackgroundRoutine {
	return []goroutine.BackgroundRoutine{
		backfiller.NewCommittedAtBackfiller(
			store,
			gitserverClient,
			config,
		),
	}
}

func NewJanitor(
	observationCtx *observation.Context,
	store uploadsstore.Store,
	dataStore codegraph.DataStore,
	gitserverClient gitserver.Client,
	config *janitor.Config,
) []goroutine.BackgroundRoutine {
	jobsByName := map[string]goroutine.BackgroundRoutine{
		"DeletedRepositoryJanitor":           janitor.NewDeletedRepositoryJanitor(store, config, observationCtx),
		"UnknownCommitJanitor":               janitor.NewUnknownCommitJanitor(store, gitserverClient, config, observationCtx),
		"AbandonedUploadJanitor":             janitor.NewAbandonedUploadJanitor(store, config, observationCtx),
		"ExpiredUploadJanitor":               janitor.NewExpiredUploadJanitor(store, config, observationCtx),
		"ExpiredUploadTraversalJanitor":      janitor.NewExpiredUploadTraversalJanitor(store, config, observationCtx),
		"HardDeleter":                        janitor.NewHardDeleter(store, dataStore, config, observationCtx),
		"AuditLogJanitor":                    janitor.NewAuditLogJanitor(store, config, observationCtx),
		"SCIPExpirationTask":                 janitor.NewSCIPExpirationTask(dataStore, config, observationCtx),
		"AbandonedSchemaVersionsRecordsTask": janitor.NewAbandonedSchemaVersionsRecordsTask(dataStore, config, observationCtx),
		"UnknownRepositoryJanitor":           janitor.NewUnknownRepositoryJanitor(store, config, observationCtx),
		"UnknownCommitJanitor2":              janitor.NewUnknownCommitJanitor2(store, gitserverClient, config, observationCtx),
		"ExpiredRecordJanitor":               janitor.NewExpiredRecordJanitor(store, config, observationCtx),
		"FrontendDBReconciler":               janitor.NewFrontendDBReconciler(store, dataStore, config, observationCtx),
		"CodeIntelDBReconciler":              janitor.NewCodeIntelDBReconciler(store, dataStore, config, observationCtx),
	}

	disabledJobs := map[string]struct{}{}
	for _, name := range strings.Split(os.Getenv("CODEINTEL_UPLOAD_JANITOR_DISABLED_SUB_JOBS"), ",") {
		disabledJobs[name] = struct{}{}
	}

	jobs := []goroutine.BackgroundRoutine{}
	for name, v := range jobsByName {
		if _, ok := disabledJobs[name]; ok {
			observationCtx.Logger.Warn("DISABLING CODE INTEL UPLOAD JANITOR SUB-JOB", log.String("name", name))
		} else {
			jobs = append(jobs, v)
		}
	}

	return jobs
}

func NewCommitGraphUpdater(
	store uploadsstore.Store,
	gitserverClient gitserver.Client,
	config *commitgraph.Config,
) []goroutine.BackgroundRoutine {
	return []goroutine.BackgroundRoutine{
		commitgraph.NewCommitGraphUpdater(
			store,
			gitserverClient,
			config,
		),
	}
}

func NewExpirationTasks(
	observationCtx *observation.Context,
	store uploadsstore.Store,
	policySvc expirer.PolicyService,
	gitserverClient gitserver.Client,
	repoStore database.RepoStore,
	config *expirer.Config,
) []goroutine.BackgroundRoutine {
	return []goroutine.BackgroundRoutine{
		expirer.NewUploadExpirer(
			observationCtx,
			store,
			repoStore,
			policySvc,
			gitserverClient,
			config,
		),
	}
}
