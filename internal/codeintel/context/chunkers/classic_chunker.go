package chunkers

import (
	"github.com/sourcegraph/sourcegraph/internal/codeintel/context"
)

type ClassicChunker struct {
	chunkOptions *ChunkOptions
}

func NewClassicChunker(chunkOptions *ChunkOptions) *ClassicChunker {
	if chunkOptions == nil {
		chunkOptions = &defaultChunkOptions
	}
	return &ClassicChunker{chunkOptions: chunkOptions}
}

func (cc *ClassicChunker) Chunk(content, filename string) ([]context.EmbeddableChunk, error) {
	return context.SplitIntoEmbeddableChunks(content, filename, context.SplitOptions{
		NoSplitTokensThreshold:         cc.chunkOptions.NoSplitTokensThreshold,
		ChunkTokensThreshold:           cc.chunkOptions.ChunkTokensThreshold,
		ChunkEarlySplitTokensThreshold: cc.chunkOptions.ChunkEarlySplitTokensThreshold,
	}), nil
}
