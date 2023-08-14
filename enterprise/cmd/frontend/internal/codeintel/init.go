package codeintel

import (
	"context"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	autoindexinggraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/transport/graphql"
	codenavgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/transport/graphql"
	contextgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/context/transport/graphql"
	policiesgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/transport/graphql"
	rankinggraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/transport/graphql"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	sentinelgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/sentinel/transport/graphql"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/lsifuploadstore"
	sharedresolvers "github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
	uploadgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/transport/graphql"
	uploadshttp "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/transport/http"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func LoadConfig() {
	ConfigInst.Load()
}

func Init(
	ctx context.Context,
	observationCtx *observation.Context,
	db database.DB,
	codeIntelServices codeintel.Services,
	conf conftypes.UnifiedWatchable,
	enterpriseServices *enterprise.Services,
) error {
	if err := ConfigInst.Validate(); err != nil {
		return err
	}

	uploadStore, err := lsifuploadstore.New(context.Background(), observationCtx, ConfigInst.LSIFUploadStoreConfig)
	if err != nil {
		return err
	}

	newUploadHandler := func(withCodeHostAuth bool) http.Handler {
		return uploadshttp.GetHandler(codeIntelServices.UploadsService, db, codeIntelServices.GitserverClient, uploadStore, withCodeHostAuth)
	}

	repoStore := db.Repos()
	siteAdminChecker := sharedresolvers.NewSiteAdminChecker(db)
	locationResolverFactory := gitresolvers.NewCachedLocationResolverFactory(repoStore, codeIntelServices.GitserverClient)
	uploadLoaderFactory := uploadgraphql.NewUploadLoaderFactory(codeIntelServices.UploadsService)
	indexLoaderFactory := uploadgraphql.NewIndexLoaderFactory(codeIntelServices.UploadsService)
	preciseIndexResolverFactory := uploadgraphql.NewPreciseIndexResolverFactory(
		codeIntelServices.UploadsService,
		codeIntelServices.PoliciesService,
		codeIntelServices.GitserverClient,
		siteAdminChecker,
		repoStore,
	)

	autoindexingRootResolver := autoindexinggraphql.NewRootResolver(
		scopedContext("autoindexing"),
		codeIntelServices.AutoIndexingService,
		siteAdminChecker,
		uploadLoaderFactory,
		indexLoaderFactory,
		locationResolverFactory,
		preciseIndexResolverFactory,
	)

	codenavRootResolver, err := codenavgraphql.NewRootResolver(
		scopedContext("codenav"),
		codeIntelServices.CodenavService,
		codeIntelServices.AutoIndexingService,
		codeIntelServices.GitserverClient,
		siteAdminChecker,
		repoStore,
		uploadLoaderFactory,
		indexLoaderFactory,
		preciseIndexResolverFactory,
		locationResolverFactory,
		ConfigInst.HunkCacheSize,
		ConfigInst.MaximumIndexesPerMonikerSearch,
	)
	if err != nil {
		return err
	}

	policyRootResolver := policiesgraphql.NewRootResolver(
		scopedContext("policies"),
		codeIntelServices.PoliciesService,
		repoStore,
		siteAdminChecker,
	)

	uploadRootResolver := uploadgraphql.NewRootResolver(
		scopedContext("upload"),
		codeIntelServices.UploadsService,
		codeIntelServices.AutoIndexingService,
		siteAdminChecker,
		uploadLoaderFactory,
		indexLoaderFactory,
		locationResolverFactory,
		preciseIndexResolverFactory,
	)

	sentinelRootResolver := sentinelgraphql.NewRootResolver(
		scopedContext("sentinel"),
		codeIntelServices.SentinelService,
		uploadLoaderFactory,
		indexLoaderFactory,
		locationResolverFactory,
		preciseIndexResolverFactory,
	)

	contextRootResolver := contextgraphql.NewRootResolver(
		scopedContext("context"),
		codeIntelServices.ContextService,
		locationResolverFactory,
	)

	rankingRootResolver := rankinggraphql.NewRootResolver(
		scopedContext("ranking"),
		codeIntelServices.RankingService,
		siteAdminChecker,
	)

	enterpriseServices.CodeIntelResolver = graphqlbackend.NewCodeIntelResolver(resolvers.NewCodeIntelResolver(
		autoindexingRootResolver,
		codenavRootResolver,
		policyRootResolver,
		uploadRootResolver,
		sentinelRootResolver,
		contextRootResolver,
		rankingRootResolver,
	))
	enterpriseServices.NewCodeIntelUploadHandler = newUploadHandler
	enterpriseServices.RankingService = codeIntelServices.RankingService
	return nil
}

func scopedContext(name string) *observation.Context {
	return observation.NewContext(log.Scoped(name+".transport.graphql", "codeintel "+name+" graphql transport"))
}
