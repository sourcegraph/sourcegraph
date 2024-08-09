package codycontext

import (
	"context"
	"net/http"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/completions"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/httpapi/embeddings"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func Init(
	_ context.Context,
	observationCtx *observation.Context,
	db database.DB,
	_ codeintel.Services,
	conf conftypes.UnifiedWatchable,
	enterpriseServices *enterprise.Services,
) error {
	logger := log.Scoped("embeddings")

	enterpriseServices.NewEmbeddingsHandler = func() http.Handler {
		completionsHandler := embeddings.NewEmbeddingsChunkingHandler(logger, db)
		return completions.RequireVerifiedEmailMiddleware(db, observationCtx.Logger, completionsHandler)
	}

	return nil
}
