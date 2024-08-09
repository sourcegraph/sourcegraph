package chunkers

import "github.com/sourcegraph/sourcegraph/internal/codeintel/context"

type Chunker interface {
	Chunk(content, filename string) ([]context.EmbeddableChunk, error)
}

type ChunkOptions struct {
	ChunkTokensThreshold           int
	NoSplitTokensThreshold         int
	ChunkEarlySplitTokensThreshold int
	CoalesceThreshold              int
}

var defaultChunkOptions = ChunkOptions{
	ChunkTokensThreshold:           256,
	NoSplitTokensThreshold:         384,
	ChunkEarlySplitTokensThreshold: 224,
	CoalesceThreshold:              50,
}
