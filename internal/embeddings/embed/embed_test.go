package embed

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sourcegraph/log"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/paths"
	"github.com/sourcegraph/sourcegraph/internal/types"

	"github.com/sourcegraph/sourcegraph/internal/api"
	codeintelContext "github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	citypes "github.com/sourcegraph/sourcegraph/internal/codeintel/types"
	"github.com/sourcegraph/sourcegraph/internal/embeddings"
	bgrepo "github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/db"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func mockFile(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n"))
}

func defaultSplitter(ctx context.Context, text, fileName string, splitOptions codeintelContext.SplitOptions) ([]codeintelContext.EmbeddableChunk, error) {
	return codeintelContext.SplitIntoEmbeddableChunks(text, fileName, splitOptions), nil
}

func TestEmbedRepo(t *testing.T) {
	ctx := context.Background()
	repoName := api.RepoName("repo/name")
	repoIDName := types.RepoIDName{
		ID:   0,
		Name: repoName,
	}
	revision := api.CommitID("deadbeef")
	embeddingsClient := NewMockEmbeddingsClient()
	inserter := db.NewNoopDB()
	contextService := NewMockContextService()
	contextService.SplitIntoEmbeddableChunksFunc.SetDefaultHook(defaultSplitter)
	splitOptions := codeintelContext.SplitOptions{ChunkTokensThreshold: 8}
	mockFiles := map[string][]byte{
		// 2 embedding chunks (based on split options above)
		"a.go": mockFile(
			strings.Repeat("a", 32),
			"",
			strings.Repeat("b", 32),
		),
		// 2 embedding chunks
		"b.md": mockFile(
			"# "+strings.Repeat("a", 32),
			"",
			"## "+strings.Repeat("b", 32),
		),
		// 3 embedding chunks
		"c.java": mockFile(
			strings.Repeat("a", 32),
			"",
			strings.Repeat("b", 32),
			"",
			strings.Repeat("c", 32),
		),
		// Should be excluded
		"autogen.py": mockFile(
			"# "+strings.Repeat("a", 32),
			"// Do not edit",
		),
		// Should be excluded
		"lines_too_long.c": mockFile(
			strings.Repeat("a", 2049),
			strings.Repeat("b", 2049),
			strings.Repeat("c", 2049),
		),
		"not_included.jl": mockFile(
			strings.Repeat("a", 32),
			"",
			strings.Repeat("b", 32),
		),
		// Should be excluded
		"empty.rb": mockFile(""),
		// Should be excluded (binary file),
		"binary.bin": {0xFF, 0xF, 0xF, 0xF, 0xFF, 0xF, 0xF, 0xA},
	}

	mockRanks := map[string]float64{
		"a.go":             0.1,
		"b.md":             0.2,
		"c.java":           0.3,
		"autogen.py":       0.4,
		"lines_too_long.c": 0.5,
		"empty.rb":         0.6,
		"binary.bin":       0.7,
	}

	mockRepoPathRanks := citypes.RepoPathRanks{
		MeanRank: 0,
		Paths:    mockRanks,
	}

	reader := funcReader(func(_ context.Context, fileName string) ([]byte, error) {
		content, ok := mockFiles[fileName]
		if !ok {
			return nil, errors.Newf("file %s not found", fileName)
		}
		return content, nil
	})

	newReadLister := func(fileNames ...string) FileReadLister {
		fileEntries := make([]FileEntry, len(fileNames))
		for i, fileName := range fileNames {
			fileEntries[i] = FileEntry{Name: fileName, Size: 350}
		}
		return listReader{
			FileReader: reader,
			FileLister: staticLister(fileEntries),
		}
	}

	excludedGlobPatterns := GetDefaultExcludedFilePathPatterns()
	// include all but .jl files
	includePatterns := []string{"*.go", "*.md", "*.java", "*.py", "*.c", "*.rb", "*.bin"}
	includeGlobs := make([]*paths.GlobPattern, len(includePatterns))
	for idx, ip := range includePatterns {
		g, err := paths.Compile(ip)
		require.Nil(t, err)
		includeGlobs[idx] = g
	}

	opts := EmbedRepoOpts{
		RepoName: repoName,
		Revision: revision,
		FileFilters: FileFilters{
			ExcludePatterns:  excludedGlobPatterns,
			IncludePatterns:  includeGlobs,
			MaxFileSizeBytes: 100000,
		},
		SplitOptions:      splitOptions,
		MaxCodeEmbeddings: 100000,
		MaxTextEmbeddings: 100000,
		BatchSize:         512,
		// initially this was the default behavior, before this flag was added.
		ExcludeChunks: false,
	}

	logger := log.NoOp()
	noopReport := func(*bgrepo.EmbedRepoStats) {}

	t.Run("no files", func(t *testing.T) {
		index, _, stats, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, newReadLister(), repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.NoError(t, err)
		require.Len(t, index.CodeIndex.Embeddings, 0)
		require.Len(t, index.TextIndex.Embeddings, 0)

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesSkipped: map[string]int{},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesSkipped: map[string]int{},
			},
		}
		require.Equal(t, expectedStats, stats)
	})

	t.Run("code files only", func(t *testing.T) {
		index, _, stats, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, newReadLister("a.go"), repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.NoError(t, err)
		require.Len(t, index.TextIndex.Embeddings, 0)
		require.Len(t, index.CodeIndex.Embeddings, 6)
		require.Len(t, index.CodeIndex.RowMetadata, 2)
		require.Len(t, index.CodeIndex.Ranks, 2)

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 2,
				BytesEmbedded:  65,
				FilesSkipped:   map[string]int{},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesSkipped: map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})

	t.Run("text files only", func(t *testing.T) {
		index, _, stats, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, newReadLister("b.md"), repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.NoError(t, err)
		require.Len(t, index.CodeIndex.Embeddings, 0)
		require.Len(t, index.TextIndex.Embeddings, 6)
		require.Len(t, index.TextIndex.RowMetadata, 2)
		require.Len(t, index.TextIndex.Ranks, 2)

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesSkipped: map[string]int{},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 2,
				BytesEmbedded:  70,
				FilesSkipped:   map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})

	t.Run("mixed code and text files", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java", "autogen.py", "empty.rb", "lines_too_long.c", "binary.bin")
		index, _, stats, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.NoError(t, err)
		require.Len(t, index.CodeIndex.Embeddings, 15)
		require.Len(t, index.CodeIndex.RowMetadata, 5)
		require.Len(t, index.CodeIndex.Ranks, 5)
		require.Len(t, index.TextIndex.Embeddings, 6)
		require.Len(t, index.TextIndex.RowMetadata, 2)
		require.Len(t, index.TextIndex.Ranks, 2)

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 6,
				FilesEmbedded:  2,
				ChunksEmbedded: 5,
				BytesEmbedded:  163,
				FilesSkipped: map[string]int{
					"autogenerated": 1,
					"binary":        1,
					"longLine":      1,
					"small":         1,
				},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 2,
				BytesEmbedded:  70,
				FilesSkipped:   map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})

	t.Run("not included files", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java", "autogen.py", "empty.rb", "lines_too_long.c", "binary.bin", "not_included.jl")
		index, _, stats, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.NoError(t, err)
		require.Len(t, index.CodeIndex.Embeddings, 15)
		require.Len(t, index.CodeIndex.RowMetadata, 5)
		require.Len(t, index.CodeIndex.Ranks, 5)
		require.Len(t, index.TextIndex.Embeddings, 6)
		require.Len(t, index.TextIndex.RowMetadata, 2)
		require.Len(t, index.TextIndex.Ranks, 2)

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 7,
				FilesEmbedded:  2,
				ChunksEmbedded: 5,
				BytesEmbedded:  163,
				FilesSkipped: map[string]int{
					"autogenerated": 1,
					"binary":        1,
					"longLine":      1,
					"small":         1,
					"notIncluded":   1,
				},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 2,
				BytesEmbedded:  70,
				FilesSkipped:   map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})

	t.Run("mixed code and text files with skips", func(t *testing.T) {
		// 3 will be embedded, 4 will be skipped
		fileNames := []string{"a.go", "b.md", "c.java", "autogen.py", "empty.rb", "lines_too_long.c", "binary.bin"}
		opts := opts
		opts.TolerableFailureRatio = 0.1
		rl := newReadLister(fileNames...)
		statReports := 0
		countingReporter := func(*bgrepo.EmbedRepoStats) {
			statReports++
		}
		_, _, _, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, countingReporter)
		require.NoError(t, err)
		require.Equal(t, 2, statReports, `
			Expected one update for flush. This is subject to change if the
			test changes, so a failure should be considered a notification of a
			change rather than a signal that something is wrong.
		`)
	})

	t.Run("embeddings limited", func(t *testing.T) {
		optsCopy := opts
		optsCopy.MaxCodeEmbeddings = 3
		optsCopy.MaxTextEmbeddings = 1

		rl := newReadLister("a.go", "b.md", "c.java", "autogen.py", "empty.rb", "lines_too_long.c", "binary.bin")
		index, _, _, err := EmbedRepo(ctx, embeddingsClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)
		require.NoError(t, err)

		// a.md has 2 chunks, c.java has 3 chunks
		require.Len(t, index.CodeIndex.Embeddings, index.CodeIndex.ColumnDimension*5)
		// b.md has 2 chunks
		require.Len(t, index.TextIndex.Embeddings, index.CodeIndex.ColumnDimension*2)
	})

	t.Run("misbehaving embeddings service", func(t *testing.T) {
		// We should not trust the embeddings service to return the correct number of dimensions.
		// We've had multiple issues in the past where the embeddings call succeeds, but returns
		// the wrong number of dimensions either because the model changed or because there was
		// some sort of uncaught error.
		optsCopy := opts
		optsCopy.MaxCodeEmbeddings = 3
		optsCopy.MaxTextEmbeddings = 1
		rl := newReadLister("a.go", "b.md", "c.java", "autogen.py", "empty.rb", "lines_too_long.c", "binary.bin")

		misbehavingClient := &misbehavingEmbeddingsClient{embeddingsClient, 32} // too many dimensions
		_, _, _, err := EmbedRepo(ctx, misbehavingClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)
		require.ErrorContains(t, err, "expected embeddings for batch to have length")

		misbehavingClient = &misbehavingEmbeddingsClient{embeddingsClient, 32} // too few dimensions
		_, _, _, err = EmbedRepo(ctx, misbehavingClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)
		require.ErrorContains(t, err, "expected embeddings for batch to have length")

		misbehavingClient = &misbehavingEmbeddingsClient{embeddingsClient, 0} // empty return
		_, _, _, err = EmbedRepo(ctx, misbehavingClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)
		require.ErrorContains(t, err, "expected embeddings for batch to have length")

		erroringClient := &erroringEmbeddingsClient{embeddingsClient, errors.New("whoops")} // normal error
		_, _, _, err = EmbedRepo(ctx, erroringClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)
		require.ErrorContains(t, err, "whoops")
	})

	t.Run("Fail a single chunk from code index", func(t *testing.T) {
		optsCopy := opts
		optsCopy.BatchSize = 512
		rl := newReadLister("a.go", "b.md")
		failed := make(map[int]struct{})

		// fail on second chunk of the first code file
		failed[1] = struct{}{}

		partialFailureClient := &partialFailureEmbeddingsClient{embeddingsClient, 0, failed}
		_, _, _, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)

		require.ErrorContains(t, err, "batch failed on file")
		require.ErrorContains(t, err, "a.go", "for a chunk error, the error message should contain the file name")
	})

	t.Run("Fail a single chunk from code index", func(t *testing.T) {
		optsCopy := opts
		optsCopy.BatchSize = 512
		rl := newReadLister("a.go", "b.md")
		failed := make(map[int]struct{})

		// fail on second chunk of the first text file
		failed[3] = struct{}{}

		partialFailureClient := &partialFailureEmbeddingsClient{embeddingsClient, 0, failed}
		_, _, _, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)

		require.ErrorContains(t, err, "batch failed on file")
		require.ErrorContains(t, err, "b.md", "for a chunk error, the error message should contain the file name")
	})
}

func TestEmbedRepo_ExcludeChunkOnError(t *testing.T) {
	ctx := context.Background()
	repoName := api.RepoName("repo/name")
	revision := api.CommitID("deadbeef")
	repoIDName := types.RepoIDName{Name: repoName}
	embeddingsClient := NewMockEmbeddingsClient()
	contextService := NewMockContextService()
	inserter := db.NewNoopDB()
	contextService.SplitIntoEmbeddableChunksFunc.SetDefaultHook(defaultSplitter)
	splitOptions := codeintelContext.SplitOptions{ChunkTokensThreshold: 8}
	mockFiles := map[string][]byte{
		// 3 embedding chunks (based on split options above)
		"a.go": mockFile(
			strings.Repeat("a", 32),
			"",
			strings.Repeat("b", 32),
			"",
			strings.Repeat("c", 32),
		),
		// 2 embedding chunks
		"b.md": mockFile(
			"# "+strings.Repeat("a", 32),
			"",
			"## "+strings.Repeat("b", 32),
			"",
			"## "+strings.Repeat("c", 32),
		),
		// 3 embedding chunks
		"c.java": mockFile(
			strings.Repeat("a", 32),
			"",
			strings.Repeat("b", 32),
			"",
			strings.Repeat("c", 32),
		),
		// many embedding chunks
		"big.java": mockFile(
			strings.Repeat("abc\n", 1000000),
		),
	}

	mockRanks := map[string]float64{
		"a.go":   0.1,
		"b.java": 0.3,
	}
	mockRepoPathRanks := citypes.RepoPathRanks{
		MeanRank: 0,
		Paths:    mockRanks,
	}

	reader := funcReader(func(_ context.Context, fileName string) ([]byte, error) {
		content, ok := mockFiles[fileName]
		if !ok {
			return nil, errors.Newf("file %s not found", fileName)
		}
		return content, nil
	})
	newReadLister := func(fileNames ...string) FileReadLister {
		fileEntries := make([]FileEntry, len(fileNames))
		for i, fileName := range fileNames {
			fileEntries[i] = FileEntry{Name: fileName, Size: 350}
		}
		return listReader{
			FileReader: reader,
			FileLister: staticLister(fileEntries),
		}
	}

	opts := EmbedRepoOpts{
		RepoName: repoName,
		Revision: revision,
		FileFilters: FileFilters{
			ExcludePatterns:  nil,
			IncludePatterns:  nil,
			MaxFileSizeBytes: 100000,
		},
		SplitOptions:          splitOptions,
		MaxCodeEmbeddings:     100000,
		MaxTextEmbeddings:     100000,
		BatchSize:             512,
		ExcludeChunks:         true,
		TolerableFailureRatio: 1.0,
	}

	logger := log.NoOp()
	noopReport := func(*bgrepo.EmbedRepoStats) {}

	t.Run("fail single full request", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java", "big.java")
		opts := opts
		opts.TolerableFailureRatio = 0.1

		partialFailureClient := &flakyEmbeddingsClient{
			EmbeddingsClient:  embeddingsClient,
			remainingFailures: 1,
			err:               errors.New("FAIL"),
		}
		_, _, stats, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.NoError(t, err)
		require.True(t, stats.CodeIndexStats.ChunksEmbedded > 0)
	})

	t.Run("fail many full requests", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java", "big.java")
		opts := opts
		opts.TolerableFailureRatio = 0.1

		partialFailureClient := &flakyEmbeddingsClient{
			EmbeddingsClient:  embeddingsClient,
			remainingFailures: 100,
			err:               errors.New("FAIL"),
		}
		_, _, _, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.Error(t, err)
	})

	t.Run("immediately fail if rate limit hit", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java", "big.java")
		opts := opts
		opts.TolerableFailureRatio = 0.1

		partialFailureClient := &flakyEmbeddingsClient{
			EmbeddingsClient:  embeddingsClient,
			remainingFailures: 1,
			err:               client.NewRateLimitExceededError(time.Now().Add(time.Minute)),
		}
		_, _, _, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)
		require.Error(t, err)
	})

	t.Run("Exclude single chunk from each index", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java")
		failed := make(map[int]struct{})

		// fail on second chunk of the first code file
		failed[1] = struct{}{}

		// fail on second chunk of the first text file
		failed[7] = struct{}{}

		partialFailureClient := &partialFailureEmbeddingsClient{embeddingsClient, 0, failed}
		index, _, stats, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)

		require.NoError(t, err)

		require.Len(t, index.TextIndex.Embeddings, 6)
		require.Len(t, index.TextIndex.RowMetadata, 2)
		require.Len(t, index.TextIndex.Ranks, 2)

		require.Len(t, index.CodeIndex.Embeddings, 15)
		require.Len(t, index.CodeIndex.RowMetadata, 5)
		require.Len(t, index.CodeIndex.Ranks, 5)

		require.True(t, validateEmbeddings(index))

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 2,
				FilesEmbedded:  2,
				ChunksEmbedded: 5,
				ChunksExcluded: 1,
				BytesEmbedded:  163,
				FilesSkipped:   map[string]int{},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 2,
				ChunksExcluded: 1,
				BytesEmbedded:  70,
				FilesSkipped:   map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})
	t.Run("Exclude chunks multiple files", func(t *testing.T) {
		rl := newReadLister("a.go", "b.md", "c.java")
		failed := make(map[int]struct{})

		// fail on second chunk of the first code file
		failed[1] = struct{}{}

		// fail on second chunk of the second code file
		failed[4] = struct{}{}

		// fail on second and third chunks of the first text file
		failed[7] = struct{}{}
		failed[8] = struct{}{}

		partialFailureClient := &partialFailureEmbeddingsClient{embeddingsClient, 0, failed}
		index, _, stats, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, opts, logger, noopReport)

		require.NoError(t, err)

		require.Len(t, index.TextIndex.Embeddings, 3)
		require.Len(t, index.TextIndex.RowMetadata, 1)
		require.Len(t, index.TextIndex.Ranks, 1)

		require.Len(t, index.CodeIndex.Embeddings, 12)
		require.Len(t, index.CodeIndex.RowMetadata, 4)
		require.Len(t, index.CodeIndex.Ranks, 4)

		require.True(t, validateEmbeddings(index))

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 2,
				FilesEmbedded:  2,
				ChunksEmbedded: 4,
				ChunksExcluded: 2,
				BytesEmbedded:  130,
				FilesSkipped:   map[string]int{},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 1,
				ChunksExcluded: 2,
				BytesEmbedded:  34,
				FilesSkipped:   map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})
	t.Run("Exclude chunks multiple files and multiple batches", func(t *testing.T) {
		optsCopy := opts
		optsCopy.BatchSize = 2
		rl := newReadLister("a.go", "b.md", "c.java")
		failed := make(map[int]struct{})

		// fail on second chunk of the first code file
		failed[1] = struct{}{}

		// fail on second chunk of the second code file
		failed[4] = struct{}{}

		// fail on second and third chunks of the first text file
		failed[7] = struct{}{}
		failed[8] = struct{}{}

		partialFailureClient := &partialFailureEmbeddingsClient{embeddingsClient, 0, failed}
		index, _, stats, err := EmbedRepo(ctx, partialFailureClient, inserter, contextService, rl, repoIDName, mockRepoPathRanks, optsCopy, logger, noopReport)

		require.NoError(t, err)

		require.Len(t, index.TextIndex.Embeddings, 3)
		require.Len(t, index.TextIndex.RowMetadata, 1)
		require.Len(t, index.TextIndex.Ranks, 1)

		require.Len(t, index.CodeIndex.Embeddings, 12)
		require.Len(t, index.CodeIndex.RowMetadata, 4)
		require.Len(t, index.CodeIndex.Ranks, 4)

		require.True(t, validateEmbeddings(index))

		expectedStats := &bgrepo.EmbedRepoStats{
			CodeIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 2,
				FilesEmbedded:  2,
				ChunksEmbedded: 4,
				ChunksExcluded: 2,
				BytesEmbedded:  130,
				FilesSkipped:   map[string]int{},
			},
			TextIndexStats: bgrepo.EmbedFilesStats{
				FilesScheduled: 1,
				FilesEmbedded:  1,
				ChunksEmbedded: 1,
				ChunksExcluded: 2,
				BytesEmbedded:  34,
				FilesSkipped:   map[string]int{},
			},
		}
		// ignore durations
		require.Equal(t, expectedStats, stats)
	})
}

type erroringEmbeddingsClient struct {
	client.EmbeddingsClient
	err error
}

func (c *erroringEmbeddingsClient) GetQueryEmbedding(_ context.Context, text string) (*client.EmbeddingsResults, error) {
	return nil, c.err
}

func (c *erroringEmbeddingsClient) GetDocumentEmbeddings(_ context.Context, texts []string) (*client.EmbeddingsResults, error) {
	return nil, c.err
}

type misbehavingEmbeddingsClient struct {
	client.EmbeddingsClient
	returnedDimsPerInput int
}

func (c *misbehavingEmbeddingsClient) GetQueryEmbedding(ctx context.Context, query string) (*client.EmbeddingsResults, error) {
	return &client.EmbeddingsResults{Embeddings: make([]float32, c.returnedDimsPerInput), Dimensions: c.returnedDimsPerInput}, nil
}

func (c *misbehavingEmbeddingsClient) GetDocumentEmbeddings(ctx context.Context, documents []string) (*client.EmbeddingsResults, error) {
	return &client.EmbeddingsResults{Embeddings: make([]float32, len(documents)*c.returnedDimsPerInput), Dimensions: c.returnedDimsPerInput}, nil
}

func NewMockEmbeddingsClient() client.EmbeddingsClient {
	return &mockEmbeddingsClient{}
}

type mockEmbeddingsClient struct{}

func (c *mockEmbeddingsClient) GetDimensions() (int, error) {
	return 3, nil
}

func (c *mockEmbeddingsClient) GetModelIdentifier() string {
	return "mock/some-model"
}

func (c *mockEmbeddingsClient) GetQueryEmbedding(_ context.Context, query string) (*client.EmbeddingsResults, error) {
	dimensions, err := c.GetDimensions()
	if err != nil {
		return nil, err
	}
	return &client.EmbeddingsResults{Embeddings: make([]float32, dimensions), Dimensions: dimensions}, nil
}

func (c *mockEmbeddingsClient) GetDocumentEmbeddings(_ context.Context, texts []string) (*client.EmbeddingsResults, error) {
	dimensions, err := c.GetDimensions()
	if err != nil {
		return nil, err
	}
	return &client.EmbeddingsResults{Embeddings: make([]float32, len(texts)*dimensions), Dimensions: dimensions}, nil
}

type flakyEmbeddingsClient struct {
	client.EmbeddingsClient
	remainingFailures int
	err               error
}

func (c *flakyEmbeddingsClient) GetQueryEmbedding(ctx context.Context, query string) (*client.EmbeddingsResults, error) {
	if c.remainingFailures > 0 {
		c.remainingFailures -= 1
		return nil, c.err
	}
	return c.EmbeddingsClient.GetQueryEmbedding(ctx, query)
}

func (c *flakyEmbeddingsClient) GetDocumentEmbeddings(ctx context.Context, documents []string) (*client.EmbeddingsResults, error) {
	if c.remainingFailures > 0 {
		c.remainingFailures -= 1
		return nil, c.err
	}
	return c.EmbeddingsClient.GetDocumentEmbeddings(ctx, documents)
}

type partialFailureEmbeddingsClient struct {
	client.EmbeddingsClient
	counter        int
	failedAttempts map[int]struct{}
}

func (c *partialFailureEmbeddingsClient) GetQueryEmbedding(ctx context.Context, query string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, []string{query})
}

func (c *partialFailureEmbeddingsClient) GetDocumentEmbeddings(ctx context.Context, documents []string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, documents)
}

func (c *partialFailureEmbeddingsClient) getEmbeddings(_ context.Context, texts []string) (*client.EmbeddingsResults, error) {
	dimensions, err := c.GetDimensions()
	if err != nil {
		return nil, err
	}

	failed := make([]int, 0, len(texts)*dimensions)
	embeddings := make([]float32, len(texts)*dimensions)
	for i := range len(texts) {
		sign := 1

		if _, ok := c.failedAttempts[c.counter]; ok {
			sign = -1 // later we'll assert that negatives are not indexed
			failed = append(failed, i)
		}

		for j := range dimensions {
			idx := (i * dimensions) + j
			embeddings[idx] = float32(sign)
		}
		c.counter++
	}

	return &client.EmbeddingsResults{Embeddings: embeddings, Failed: failed, Dimensions: dimensions}, nil
}

func validateEmbeddings(index *embeddings.RepoEmbeddingIndex) bool {
	for _, quantity := range index.CodeIndex.Embeddings {
		if quantity < 0 {
			return false
		}
	}
	for _, quantity := range index.TextIndex.Embeddings {
		if quantity < 0 {
			return false
		}
	}
	return true
}

type funcReader func(ctx context.Context, fileName string) ([]byte, error)

func (f funcReader) Read(ctx context.Context, fileName string) ([]byte, error) {
	return f(ctx, fileName)
}

type staticLister []FileEntry

func (l staticLister) List(_ context.Context) ([]FileEntry, error) {
	return l, nil
}

type listReader struct {
	FileReader
	FileLister
	FileDiffer
}
