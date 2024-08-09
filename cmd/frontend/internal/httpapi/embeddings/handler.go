package embeddings

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/life4/genesis/slices"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/context/chunkers"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

func newEmbeddingsChunkingHandler(
	logger log.Logger,
	db database.DB,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Method != "POST" {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		// First check that Cody is enabled for this Sourcegraph instance.
		if isEnabled, reason := cody.IsCodyEnabled(ctx, db); !isEnabled {
			errResponse := fmt.Sprintf("cody is not enabled: %s", reason)
			http.Error(w, errResponse, http.StatusUnauthorized)
			return
		}
		var req ChunkingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Warn("malformed ChunkingRequest", log.Error(err))
			http.Error(w, "could not decode request body", http.StatusBadRequest)
			return
		}
		var cc chunkers.Chunker
		if req.UseClassic {
			cc = chunkers.NewClassicChunker(nil)
		} else {
			cc = chunkers.NewTreeSitterChunker(nil)
		}
		chunks := cc.Chunk(req.Content, req.FileName)
		res := ChunkingResponse{
			Chunks: slices.Map(chunks, func(c context.EmbeddableChunk) Chunk {
				return Chunk{
					FileName:  c.FileName,
					StartLine: c.StartLine,
					EndLine:   c.EndLine,
					Content:   c.Content,
				}
			}),
		}
		chunkBytes, err := json.Marshal(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(chunkBytes)
	})
}

type ChunkingRequest struct {
	Content    string `json:"content"`
	FileName   string `json:"fileName"`
	UseClassic bool   `json:"useClassic"`
}

type Chunk struct {
	FileName  string `json:"fileName"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Content   string `json:"content"`
}

type ChunkingResponse struct {
	Chunks []Chunk `json:"chunks"`
}
