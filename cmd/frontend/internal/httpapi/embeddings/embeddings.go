package embeddings

import (
	"net/http"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

func NewEmbeddingsChunkingHandler(logger log.Logger, db database.DB) http.Handler {
	return newEmbeddingsChunkingHandler(logger, db)
}
