package codeintel

import (
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	ossdependencies "github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/ranking"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/reposcheduler"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/sentinel"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Services struct {
	AutoIndexingService          *autoindexing.Service
	PreciseRepoSchedulingService reposcheduler.RepositorySchedulingService
	CodenavService               *codenav.Service
	DependenciesService          *ossdependencies.Service
	PoliciesService              *policies.Service
	RankingService               *ranking.Service
	UploadsService               *uploads.Service
	SentinelService              *sentinel.Service
	ContextService               *context.Service
	GitserverClient              gitserver.Client
}

type ServiceDependencies struct {
	DB             database.DB
	CodeIntelDB    codeintelshared.CodeIntelDB
	ObservationCtx *observation.Context
}

func NewServices(deps ServiceDependencies) (Services, error) {
	db, codeIntelDB := deps.DB, deps.CodeIntelDB
	gitserverClient := gitserver.NewClient("codeintel")

	uploadsSvc := uploads.NewService(deps.ObservationCtx, db, codeIntelDB, gitserverClient.Scoped("uploads"))
	dependenciesSvc := dependencies.NewService(deps.ObservationCtx, db)
	policiesSvc := policies.NewService(deps.ObservationCtx, db, uploadsSvc, gitserverClient.Scoped("policies"))
	autoIndexingSvc := autoindexing.NewService(deps.ObservationCtx, db, dependenciesSvc, policiesSvc, gitserverClient.Scoped("autoindexing"))
	codenavSvc := codenav.NewService(deps.ObservationCtx, db, codeIntelDB, uploadsSvc, gitserverClient.Scoped("codenav"))
	rankingSvc := ranking.NewService(deps.ObservationCtx, db, codeIntelDB)
	sentinelService := sentinel.NewService(deps.ObservationCtx, db)
	contextService := context.NewService(deps.ObservationCtx, db)
	reposchedulingService := reposcheduler.NewService(reposcheduler.NewPreciseStore(deps.ObservationCtx, db))

	return Services{
		AutoIndexingService:          autoIndexingSvc,
		PreciseRepoSchedulingService: reposchedulingService,
		CodenavService:               codenavSvc,
		DependenciesService:          dependenciesSvc,
		PoliciesService:              policiesSvc,
		RankingService:               rankingSvc,
		UploadsService:               uploadsSvc,
		SentinelService:              sentinelService,
		ContextService:               contextService,
		GitserverClient:              gitserverClient,
	}, nil
}
