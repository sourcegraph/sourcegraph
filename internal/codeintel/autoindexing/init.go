package autoindexing

import (
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/background"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/background/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/background/scheduler"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/background/summary"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/inference"
	autoindexingstore "github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/reposcheduler"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

var (
	IndexWorkerStoreOptions                 = background.IndexWorkerStoreOptions
	DependencySyncingJobWorkerStoreOptions  = background.DependencySyncingJobWorkerStoreOptions
	DependencyIndexingJobWorkerStoreOptions = background.DependencyIndexingJobWorkerStoreOptions
)

func NewService(
	observationCtx *observation.Context,
	db database.DB,
	depsSvc DependenciesService,
	policiesSvc PoliciesService,
	gitserverClient gitserver.Client,
) *Service {
	store := autoindexingstore.New(scopedContext("store", observationCtx), db)
	inferenceSvc := inference.NewService(db)

	return newService(
		scopedContext("service", observationCtx),
		store,
		inferenceSvc,
		db.Repos(),
		gitserverClient,
	)
}

var (
	DependenciesConfigInst = &dependencies.Config{}
	SchedulerConfigInst    = &scheduler.Config{}
	SummaryConfigInst      = &summary.Config{}
)

func NewIndexSchedulers(
	observationCtx *observation.Context,
	uploadSvc UploadService,
	policiesSvc PoliciesService,
	policyMatcher PolicyMatcher,
	repoSchedulingSvc reposcheduler.RepositorySchedulingService,
	autoindexingSvc Service,
	repoStore database.RepoStore,
) []goroutine.BackgroundRoutine {
	return background.NewIndexSchedulers(
		scopedContext("scheduler", observationCtx),
		policiesSvc,
		policyMatcher,
		repoSchedulingSvc,
		autoindexingSvc.indexEnqueuer,
		repoStore,
		autoindexingSvc.store,
		SchedulerConfigInst,
	)
}

func NewDependencyIndexSchedulers(
	observationCtx *observation.Context,
	db database.DB,
	uploadSvc UploadService,
	depsSvc DependenciesService,
	autoindexingSvc *Service,
	repoUpdater RepoUpdaterClient,
) []goroutine.BackgroundRoutine {
	return background.NewDependencyIndexSchedulers(
		scopedContext("dependencies", observationCtx),
		db,
		uploadSvc,
		depsSvc,
		autoindexingSvc.store,
		autoindexingSvc.indexEnqueuer,
		repoUpdater,
		DependenciesConfigInst,
	)
}

func NewSummaryBuilder(
	observationCtx *observation.Context,
	autoindexingSvc *Service,
	uploadSvc UploadService,
) []goroutine.BackgroundRoutine {
	return background.NewSummaryBuilder(
		scopedContext("summary", observationCtx),
		autoindexingSvc.store,
		autoindexingSvc.jobSelector,
		uploadSvc,
		SummaryConfigInst,
	)
}

func scopedContext(component string, observationCtx *observation.Context) *observation.Context {
	return observation.ScopedContext("codeintel", "autoindexing", component, observationCtx)
}
