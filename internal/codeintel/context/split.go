package context

import (
	"math"
	"strings"
)

var splittableLinePrefixes = []string{
	"//",
	"#",
	"/*",
	"func",
	"var",
	"const",
	"fn",
	"public",
	"private",
	"type",
}

func isSplittableLine(line string) bool {
	trimmedLine := strings.TrimSpace(line)
	if len(trimmedLine) == 0 {
		return true
	}
	for _, prefix := range splittableLinePrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type SplitOptions struct {
	NoSplitTokensThreshold         int
	ChunkTokensThreshold           int
	ChunkEarlySplitTokensThreshold int
}

type EmbeddableChunk struct {
	FileName  string
	StartLine int
	EndLine   int
	Content   string
}

const CHARS_PER_TOKEN = 4

func EstimateTokens(text string) int {
	return int(math.Ceil(float64(len(text)) / float64(CHARS_PER_TOKEN)))
}

// SplitIntoEmbeddableChunks splits the given text into embeddable chunks.
//
// The text is split on newline characters into lines. The lines are then grouped into chunks based on the split options.
// When the token sum of lines in a chunk exceeds the chunk token threshold or an early split token threshold is met
// and the current line is splittable (empty line, or starts with a comment or declaration), a chunk is ended and added to the results.
func SplitIntoEmbeddableChunks(text string, fileName string, splitOptions SplitOptions) []EmbeddableChunk {
	// If the text is short enough, embed the entire file rather than splitting it into chunks.
	if EstimateTokens(text) < splitOptions.NoSplitTokensThreshold {
		return []EmbeddableChunk{{FileName: fileName, StartLine: 0, EndLine: strings.Count(text, "\n") + 1, Content: text}}
	}

	chunks := []EmbeddableChunk{}
	startLine, tokensSum := 0, 0
	lines := strings.Split(text, "\n")

	addChunk := func(endLine int) {
		content := strings.Join(lines[startLine:endLine], "\n")
		if len(content) > 0 {
			chunks = append(chunks, EmbeddableChunk{FileName: fileName, StartLine: startLine, EndLine: endLine, Content: content})
		}
		startLine, tokensSum = endLine, 0
	}

	for i := range len(lines) {
		if tokensSum > splitOptions.ChunkTokensThreshold || (tokensSum > splitOptions.ChunkEarlySplitTokensThreshold && isSplittableLine(lines[i])) {
			addChunk(i)
		}
		tokensSum += EstimateTokens(lines[i])
	}

	if tokensSum > 0 {
		addChunk(len(lines))
	}

	return chunks
}
