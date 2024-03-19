package embed

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	codeintelContext "github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	citypes "github.com/sourcegraph/sourcegraph/internal/codeintel/types"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/embeddings"
	bgrepo "github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/db"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client/azureopenai"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client/openai"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client/sourcegraph"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/paths"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewEmbeddingsClient(config *conftypes.EmbeddingsConfig) (client.EmbeddingsClient, error) {
	switch config.Provider {
	case conftypes.EmbeddingsProviderNameSourcegraph:
		return sourcegraph.NewClient(httpcli.UncachedExternalClient, config), nil
	case conftypes.EmbeddingsProviderNameOpenAI:
		return openai.NewClient(httpcli.UncachedExternalClient, config), nil
	case conftypes.EmbeddingsProviderNameAzureOpenAI:
		return azureopenai.NewClient(azureopenai.GetAPIClient, config)
	default:
		return nil, errors.Newf("invalid provider %q", config.Provider)
	}
}

// EmbedRepo embeds file contents from the given file names for a repository.
// It separates the file names into code files and text files and embeds them separately.
// It returns a RepoEmbeddingIndex containing the embeddings and metadata.
func EmbedRepo(
	ctx context.Context,
	client client.EmbeddingsClient,
	inserter db.VectorInserter,
	contextService ContextService,
	readLister FileReadLister,
	repo types.RepoIDName,
	ranks citypes.RepoPathRanks,
	opts EmbedRepoOpts,
	logger log.Logger,
	reportProgress func(*bgrepo.EmbedRepoStats),
) (*embeddings.RepoEmbeddingIndex, []string, *bgrepo.EmbedRepoStats, error) {
	var toIndex []FileEntry
	var toRemove []string
	var err error

	isIncremental := opts.IndexedRevision != ""

	if isIncremental {
		toIndex, toRemove, err = readLister.Diff(ctx, opts.IndexedRevision)
		if err != nil {
			logger.Error(
				"failed to get diff. Falling back to full index",
				log.String("RepoName", string(opts.RepoName)),
				log.String("revision", string(opts.Revision)),
				log.String("old revision", string(opts.IndexedRevision)),
				log.Error(err),
			)
			toRemove = nil
			isIncremental = false
		}
	}

	if !isIncremental { // full index
		toIndex, err = readLister.List(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var codeFileNames, textFileNames []FileEntry
	for _, file := range toIndex {
		if IsValidTextFile(file.Name) {
			textFileNames = append(textFileNames, file)
		} else {
			codeFileNames = append(codeFileNames, file)
		}
	}

	dimensions, err := client.GetDimensions()
	if err != nil {
		return nil, nil, nil, err
	}
	newIndex := func(numFiles int) embeddings.EmbeddingIndex {
		return embeddings.EmbeddingIndex{
			Embeddings:      make([]int8, 0, numFiles*dimensions/2),
			RowMetadata:     make([]embeddings.RepoEmbeddingRowMetadata, 0, numFiles/2),
			ColumnDimension: dimensions,
			Ranks:           make([]float32, 0, numFiles/2),
		}
	}

	stats := bgrepo.EmbedRepoStats{
		CodeIndexStats: bgrepo.NewEmbedFilesStats(len(codeFileNames)),
		TextIndexStats: bgrepo.NewEmbedFilesStats(len(textFileNames)),
		IsIncremental:  isIncremental,
	}

	insertDB := func(batch []embeddings.RepoEmbeddingRowMetadata, embeddings []float32, isCode bool) error {
		return inserter.InsertChunks(ctx, db.InsertParams{
			ModelID:     client.GetModelIdentifier(),
			ChunkPoints: batchToChunkPoints(repo, opts.Revision, batch, embeddings, isCode),
		})
	}

	insertIndex := func(index *embeddings.EmbeddingIndex, metadata []embeddings.RepoEmbeddingRowMetadata, vectors []float32) {
		index.RowMetadata = append(index.RowMetadata, metadata...)
		index.Embeddings = append(index.Embeddings, embeddings.Quantize(vectors, nil)...)
		// Unknown documents have rank 0. Zoekt is a bit smarter about this, assigning 0
		// to "unimportant" files and the average for unknown files. We should probably
		// add this here, too.
		for _, md := range metadata {
			index.Ranks = append(index.Ranks, float32(ranks.Paths[md.FileName]))
		}
	}

	codeIndex := newIndex(len(codeFileNames))
	insertCode := func(md []embeddings.RepoEmbeddingRowMetadata, embeddings []float32) error {
		insertIndex(&codeIndex, md, embeddings)
		return insertDB(md, embeddings, true)
	}

	reportCodeProgress := func(codeIndexStats bgrepo.EmbedFilesStats) {
		stats.CodeIndexStats = codeIndexStats
		reportProgress(&stats)
	}

	codeIndexStats, err := embedFiles(ctx, logger, codeFileNames, client, contextService, opts.FileFilters, opts.SplitOptions, readLister, opts.MaxCodeEmbeddings, opts.BatchSize, opts.ExcludeChunks, insertCode, reportCodeProgress)
	if err != nil {
		return nil, nil, nil, err
	}

	if codeIndexStats.ChunksExcluded > 0 {
		logger.Warn("error getting embeddings for chunks",
			log.Int("count", codeIndexStats.ChunksExcluded),
			log.String("file_type", "code"),
		)
	}

	stats.CodeIndexStats = codeIndexStats

	textIndex := newIndex(len(textFileNames))
	insertText := func(md []embeddings.RepoEmbeddingRowMetadata, embeddings []float32) error {
		insertIndex(&textIndex, md, embeddings)
		return insertDB(md, embeddings, false)
	}

	reportTextProgress := func(textIndexStats bgrepo.EmbedFilesStats) {
		stats.TextIndexStats = textIndexStats
		reportProgress(&stats)
	}

	textIndexStats, err := embedFiles(ctx, logger, textFileNames, client, contextService, opts.FileFilters, opts.SplitOptions, readLister, opts.MaxTextEmbeddings, opts.BatchSize, opts.ExcludeChunks, insertText, reportTextProgress)
	if err != nil {
		return nil, nil, nil, err
	}

	if textIndexStats.ChunksExcluded > 0 {
		logger.Warn("error getting embeddings for chunks",
			log.Int("count", textIndexStats.ChunksExcluded),
			log.String("file_type", "text"),
		)
	}

	stats.TextIndexStats = textIndexStats

	excluded := stats.CodeIndexStats.ChunksExcluded + stats.TextIndexStats.ChunksExcluded
	embedded := stats.CodeIndexStats.ChunksEmbedded + stats.TextIndexStats.ChunksEmbedded
	failureRatio := float64(excluded) / float64(excluded+embedded)
	if failureRatio > opts.TolerableFailureRatio {
		return nil, nil, nil, errors.Newf("embeddings failed at a rate (%0.2f) higher than acceptable (%0.2f)", failureRatio, opts.TolerableFailureRatio)
	}

	embeddingsModel := client.GetModelIdentifier()
	index := &embeddings.RepoEmbeddingIndex{
		RepoName:        opts.RepoName,
		Revision:        opts.Revision,
		EmbeddingsModel: embeddingsModel,
		CodeIndex:       codeIndex,
		TextIndex:       textIndex,
	}

	return index, toRemove, &stats, nil
}

type EmbedRepoOpts struct {
	RepoName              api.RepoName
	Revision              api.CommitID
	FileFilters           FileFilters
	SplitOptions          codeintelContext.SplitOptions
	MaxCodeEmbeddings     int
	MaxTextEmbeddings     int
	BatchSize             int
	ExcludeChunks         bool
	TolerableFailureRatio float64

	// If set, we already have an index for a previous commit.
	IndexedRevision api.CommitID
}

type FileFilters struct {
	ExcludePatterns  []*paths.GlobPattern
	IncludePatterns  []*paths.GlobPattern
	MaxFileSizeBytes int
}

type batchInserter func(metadata []embeddings.RepoEmbeddingRowMetadata, embeddings []float32) error

type FlushResults struct {
	size  int
	count int
}

// embedFiles embeds file contents from the given file names. Since embedding models can only handle a certain amount of text (tokens) we cannot embed
// entire files. So we split the file contents into chunks and get embeddings for the chunks in batches. Functions returns an EmbeddingIndex containing
// the embeddings and metadata about the chunks the embeddings correspond to.
func embedFiles(
	ctx context.Context,
	logger log.Logger,
	files []FileEntry,
	embeddingsClient client.EmbeddingsClient,
	contextService ContextService,
	fileFilters FileFilters,
	splitOptions codeintelContext.SplitOptions,
	reader FileReader,
	maxEmbeddingVectors int,
	batchSize int,
	excludeChunksOnError bool,
	insert batchInserter,
	reportProgress func(bgrepo.EmbedFilesStats),
) (bgrepo.EmbedFilesStats, error) {
	dimensions, err := embeddingsClient.GetDimensions()
	if err != nil {
		return bgrepo.EmbedFilesStats{}, err
	}

	stats := bgrepo.NewEmbedFilesStats(len(files))

	var batch []codeintelContext.EmbeddableChunk

	flush := func() (*FlushResults, error) {
		if len(batch) == 0 {
			return nil, nil
		}

		batchChunks := make([]string, len(batch))
		for idx, chunk := range batch {
			batchChunks[idx] = chunk.Content
		}

		batchEmbeddings, err := embeddingsClient.GetDocumentEmbeddings(ctx, batchChunks)
		if err != nil {
			if !excludeChunksOnError || errors.Is(err, &client.RateLimitExceededError{}) {
				// Fail immediately if we hit a rate limit, so we don't continually retry and fail on every chunk.
				return nil, errors.Wrap(err, "error while getting embeddings")
			} else {
				// To avoid failing large jobs on a flaky API, just mark all files
				// as failed and continue. This means we may have some missing
				// files, but they will be logged as such below and some embeddings
				// are better than no embeddings.
				logger.Warn("error while getting embeddings", log.Error(err))
				failed := make([]int, len(batchChunks))
				for i := range len(batchChunks) {
					failed[i] = i
				}
				batchEmbeddings = &client.EmbeddingsResults{
					Embeddings: make([]float32, len(batchChunks)*dimensions),
					Failed:     failed,
					Dimensions: dimensions,
				}
			}
		}

		if expected := len(batchChunks) * dimensions; len(batchEmbeddings.Embeddings) != expected {
			return nil, errors.Newf("expected embeddings for batch to have length %d, got %d", expected, len(batchEmbeddings.Embeddings))
		}

		if !excludeChunksOnError && len(batchEmbeddings.Failed) > 0 {
			// if at least one chunk failed then return an error instead of completing the embedding indexing
			return nil, errors.Newf("batch failed on file %q", batch[batchEmbeddings.Failed[0]].FileName)
		}

		// When excluding failed chunks we
		// (1) report total chunks failed at the end and
		// (2) log filenames that have failed chunks
		excludedBatches := make(map[int]struct{}, len(batchEmbeddings.Failed))
		filesFailedChunks := make(map[string]int, len(batchEmbeddings.Failed))
		for _, batchIdx := range batchEmbeddings.Failed {

			if batchIdx < 0 || batchIdx >= len(batch) {
				continue
			}
			excludedBatches[batchIdx] = struct{}{}

			if chunks, ok := filesFailedChunks[batch[batchIdx].FileName]; ok {
				filesFailedChunks[batch[batchIdx].FileName] = chunks + 1
			} else {
				filesFailedChunks[batch[batchIdx].FileName] = 1
			}
		}

		// log filenames at most once per flush
		for fileName, count := range filesFailedChunks {
			logger.Warn("failed to generate one or more chunks for file",
				log.String("file", fileName),
				log.Int("count", count),
			)
		}

		rowsCount := len(batch) - len(batchEmbeddings.Failed)
		metadata := make([]embeddings.RepoEmbeddingRowMetadata, 0, rowsCount)
		var size int
		cursor := 0
		for idx, chunk := range batch {
			if _, ok := excludedBatches[idx]; ok {
				continue
			}
			copy(batchEmbeddings.Row(cursor), batchEmbeddings.Row(idx))
			metadata = append(metadata, embeddings.RepoEmbeddingRowMetadata{
				FileName:  chunk.FileName,
				StartLine: chunk.StartLine,
				EndLine:   chunk.EndLine,
			})
			size += len(chunk.Content)
			cursor++
		}

		if err := insert(metadata, batchEmbeddings.Embeddings[:cursor*dimensions]); err != nil {
			return nil, err
		}

		batch = batch[:0] // reset batch
		reportProgress(stats)
		return &FlushResults{size, rowsCount}, nil
	}

	addToBatch := func(chunk codeintelContext.EmbeddableChunk) (*FlushResults, error) {
		batch = append(batch, chunk)
		if len(batch) >= batchSize {
			// Flush if we've hit batch size
			return flush()
		}
		return nil, nil
	}

	for _, file := range files {
		if ctx.Err() != nil {
			return bgrepo.EmbedFilesStats{}, ctx.Err()
		}

		// This is a fail-safe measure to prevent producing an extremely large index for large repositories.
		if stats.ChunksEmbedded >= maxEmbeddingVectors {
			stats.Skip(SkipReasonMaxEmbeddings, int(file.Size))
			continue
		}

		if file.Size > int64(fileFilters.MaxFileSizeBytes) {
			stats.Skip(SkipReasonLarge, int(file.Size))
			continue
		}

		if isExcludedFilePathMatch(file.Name, fileFilters.ExcludePatterns) {
			stats.Skip(SkipReasonExcluded, int(file.Size))
			continue
		}

		if !isIncludedFilePathMatch(file.Name, fileFilters.IncludePatterns) {
			stats.Skip(SkipReasonNotIncluded, int(file.Size))
			continue
		}

		contentBytes, err := reader.Read(ctx, file.Name)
		if err != nil {
			return bgrepo.EmbedFilesStats{}, errors.Wrap(err, "error while reading a file")
		}

		if embeddable, skipReason := isEmbeddableFileContent(contentBytes); !embeddable {
			stats.Skip(skipReason, len(contentBytes))
			continue
		}

		// At this point, we have determined that we want to embed this file.
		chunks, err := contextService.SplitIntoEmbeddableChunks(ctx, string(contentBytes), file.Name, splitOptions)
		if err != nil {
			return bgrepo.EmbedFilesStats{}, errors.Wrap(err, "error while splitting file")
		}

		for _, chunk := range chunks {
			if results, err := addToBatch(chunk); err != nil {
				return bgrepo.EmbedFilesStats{}, err
			} else if results != nil {
				stats.AddChunks(results.count, results.size)
				stats.ExcludeChunks(batchSize - results.count)
			}
		}
		stats.AddFile()
	}

	// Always do a final flush
	currentBatch := len(batch)
	if results, err := flush(); err != nil {
		return bgrepo.EmbedFilesStats{}, err
	} else if results != nil {
		stats.AddChunks(results.count, results.size)
		stats.ExcludeChunks(currentBatch - results.count)
	}

	return stats, nil
}

func batchToChunkPoints(repo types.RepoIDName, revision api.CommitID, batch []embeddings.RepoEmbeddingRowMetadata, embeddings []float32, isCode bool) []db.ChunkPoint {
	if len(batch) == 0 {
		return nil
	}

	dimensions := len(embeddings) / len(batch)
	points := make([]db.ChunkPoint, 0, len(batch))
	for i, chunk := range batch {
		payload := db.ChunkPayload{
			RepoName:  repo.Name,
			RepoID:    repo.ID,
			Revision:  revision,
			FilePath:  chunk.FileName,
			StartLine: uint32(chunk.StartLine),
			EndLine:   uint32(chunk.EndLine),
			IsCode:    isCode,
		}
		point := db.NewChunkPoint(payload, embeddings[i*dimensions:(i+1)*dimensions])
		points = append(points, point)
	}
	return points
}

type FileReadLister interface {
	FileReader
	FileLister
	FileDiffer
}

type FileEntry struct {
	Name string
	Size int64
}

type FileLister interface {
	List(context.Context) ([]FileEntry, error)
}

type FileReader interface {
	Read(context.Context, string) ([]byte, error)
}

type FileDiffer interface {
	Diff(context.Context, api.CommitID) ([]FileEntry, []string, error)
}
