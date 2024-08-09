package chunkers

import (
	"fmt"
	ctxt "github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	"io"
	"os"
	"strings"
)

func (ts *TreeSitterChunkerSuite) testClassicChunker(filename string, expectedChunks []ctxt.EmbeddableChunk, chunkOptions *ChunkOptions) {
	tsc := NewClassicChunker(chunkOptions)

	file, err := os.Open(filename)
	ts.NoError(err)
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	ts.NoError(err)
	content := string(fileBytes)

	chunks := tsc.Chunk(content, filename)
	reconstructedContent := ""
	for _, chunk := range chunks {
		// ts.Equal(expectedChunks[i], chunk)
		reconstructedContent += chunk.Content
		fmt.Printf("chunk (len %d, start line %d, end line %d): ```\n%s```\n", len(chunk.Content), chunk.StartLine, chunk.EndLine, chunk.Content)
	}

	ts.Equal(strings.TrimSpace(content), strings.TrimSpace(reconstructedContent))
}
