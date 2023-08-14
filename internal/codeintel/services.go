package codeintel

import (
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	ossdependencies "github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/ranking"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/sentinel"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gosyntect"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Services struct {
	AutoIndexingService *autoindexing.Service
	CodenavService      *codenav.Service
	DependenciesService *ossdependencies.Service
	PoliciesService     *policies.Service
	RankingService      *ranking.Service
	UploadsService      *uploads.Service
	SentinelService     *sentinel.Service
	ContextService      *context.Service
	GitserverClient     gitserver.Client
}

type ServiceDependencies struct {
	DB             database.DB
	CodeIntelDB    codeintelshared.CodeIntelDB
	ObservationCtx *observation.Context
}

func NewServices(deps ServiceDependencies) (Services, error) {
	db, codeIntelDB := deps.DB, deps.CodeIntelDB
	repoDB := deps.DB.Repos()
	gitserverClient := gitserver.NewClient(db)
	syntectClient := gosyntect.GetSyntectClient()

	uploadsSvc := uploads.NewService(deps.ObservationCtx, db, codeIntelDB, gitserverClient)
	dependenciesSvc := dependencies.NewService(deps.ObservationCtx, db)
	policiesSvc := policies.NewService(deps.ObservationCtx, db, uploadsSvc, gitserverClient)
	autoIndexingSvc := autoindexing.NewService(deps.ObservationCtx, db, dependenciesSvc, policiesSvc, gitserverClient)
	codenavSvc := codenav.NewService(deps.ObservationCtx, db, codeIntelDB, uploadsSvc, gitserverClient)
	rankingSvc := ranking.NewService(deps.ObservationCtx, db, codeIntelDB)
	sentinelService := sentinel.NewService(deps.ObservationCtx, db)
	contextService := context.NewService(deps.ObservationCtx, codeIntelDB, repoDB, codenavSvc, syntectClient, gitserverClient)

	return Services{
		AutoIndexingService: autoIndexingSvc,
		CodenavService:      codenavSvc,
		DependenciesService: dependenciesSvc,
		PoliciesService:     policiesSvc,
		RankingService:      rankingSvc,
		UploadsService:      uploadsSvc,
		SentinelService:     sentinelService,
		ContextService:      contextService,
		GitserverClient:     gitserverClient,
	}, nil
}
