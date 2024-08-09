package uploads

import (
	"context"
	"time"

	lsifstore "github.com/sourcegraph/sourcegraph/internal/codeintel/codegraph"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background"
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
)

func NewService(
	observationCtx *observation.Context,
	db database.DB,
	codeIntelDB codeintelshared.CodeIntelDB,
	gitserverClient gitserver.Client,
) *Service {
	store := uploadsstore.New(scopedContext("uploadsstore", observationCtx), db)
	lsifStore := lsifstore.New(scopedContext("lsifstore", observationCtx), codeIntelDB)

	svc := newService(
		scopedContext("service", observationCtx),
		store,
		db.Repos(),
		lsifStore,
		gitserverClient,
	)

	return svc
}

var (
	BackfillerConfigInst  = &backfiller.Config{}
	CommitGraphConfigInst = &commitgraph.Config{}
	ExpirerConfigInst     = &expirer.Config{}
	JanitorConfigInst     = &janitor.Config{}
	ProcessorConfigInst   = &processor.Config{}
)

func NewUploadProcessorJob(
	ctx context.Context,
	observationCtx *observation.Context,
	uploadSvc *Service,
	db database.DB,
	uploadStore object.Storage,
	workerConcurrency int,
	workerBudget int64,
	workerPollInterval time.Duration,
	maximumRuntimePerJob time.Duration,
) []goroutine.BackgroundRoutine {
	ProcessorConfigInst.WorkerConcurrency = workerConcurrency
	ProcessorConfigInst.WorkerBudget = workerBudget
	ProcessorConfigInst.WorkerPollInterval = workerPollInterval
	ProcessorConfigInst.MaximumRuntimePerJob = maximumRuntimePerJob

	return background.NewUploadProcessorJob(
		ctx,
		scopedContext("processor", observationCtx),
		uploadSvc.store,
		uploadSvc.codeGraphDataStore,
		uploadSvc.repoStore,
		uploadSvc.gitserverClient,
		db,
		uploadStore,
		ProcessorConfigInst,
	)
}

func NewCommittedAtBackfillerJob(
	uploadSvc *Service,
	gitserverClient gitserver.Client,
) []goroutine.BackgroundRoutine {
	return background.NewCommittedAtBackfillerJob(
		// TODO - context
		uploadSvc.store,
		gitserverClient,
		BackfillerConfigInst,
	)
}

func NewJanitor(
	observationCtx *observation.Context,
	uploadSvc *Service,
	gitserverClient gitserver.Client,
) []goroutine.BackgroundRoutine {
	return background.NewJanitor(
		scopedContext("janitor", observationCtx),
		uploadSvc.store,
		uploadSvc.codeGraphDataStore,
		gitserverClient,
		JanitorConfigInst,
	)
}

func NewCommitGraphUpdater(
	uploadSvc *Service,
	gitserverClient gitserver.Client,
) []goroutine.BackgroundRoutine {
	return background.NewCommitGraphUpdater(
		// TODO - context
		uploadSvc.store,
		gitserverClient,
		CommitGraphConfigInst,
	)
}

func NewExpirationTasks(
	observationCtx *observation.Context,
	uploadSvc *Service,
	policySvc expirer.PolicyService,
	repoStore database.RepoStore,
) []goroutine.BackgroundRoutine {
	return background.NewExpirationTasks(
		scopedContext("expiration", observationCtx),
		uploadSvc.store,
		policySvc,
		uploadSvc.gitserverClient,
		repoStore,
		ExpirerConfigInst,
	)
}

func scopedContext(component string, parent *observation.Context) *observation.Context {
	return observation.ScopedContext("codeintel", "uploads", component, parent)
}
