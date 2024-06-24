package context

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codycontext"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/context/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
)

func Init(
	_ context.Context,
	observationCtx *observation.Context,
	db database.DB,
	services codeintel.Services,
	_ conftypes.UnifiedWatchable,
	enterpriseServices *enterprise.Services,
) error {
	observationCtx = observationCtx.Clone()
	observationCtx.Logger = observationCtx.Logger.Scoped("codycontext")

	searchClient := client.New(observationCtx.Logger, db, gitserver.NewClient("graphql.context.search"))

	contextClient := codycontext.NewCodyContextClient(
		observationCtx,
		db,
		searchClient,
		services.GitserverClient.Scoped("codycontext.client"),
	)
	enterpriseServices.CodyContextResolver = resolvers.NewResolver(
		db,
		services.GitserverClient,
		contextClient,
		observationCtx.Logger,
	)

	return nil
}
