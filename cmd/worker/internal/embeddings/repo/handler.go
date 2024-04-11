package repo

import (
	"context"
	"io"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/diff"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	codeintelContext "github.com/sourcegraph/sourcegraph/internal/codeintel/context"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/ranking"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/embeddings"
	bgrepo "github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/db"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/paths"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type handler struct {
	db                     database.DB
	uploadStore            uploadstore.Store
	gitserverClient        gitserver.Client
	getQdrantInserter      func() (db.VectorInserter, error)
	contextService         embed.ContextService
	repoEmbeddingJobsStore bgrepo.RepoEmbeddingJobsStore
	rankingService         *ranking.Service
}

var _ workerutil.Handler[*bgrepo.RepoEmbeddingJob] = &handler{}

// The threshold to embed the entire file is slightly larger than the chunk threshold to
// avoid splitting small files unnecessarily.
const (
	embedEntireFileTokensThreshold          = 384
	embeddingChunkTokensThreshold           = 256
	embeddingChunkEarlySplitTokensThreshold = embeddingChunkTokensThreshold - 32
	embeddingsTolerableFailureRatio         = 0.10
)

var embeddingsBatchSize = env.MustGetInt("SRC_EMBEDDINGS_BATCH_SIZE", 512, "Number of chunks to embed at a time.")

var splitOptions = codeintelContext.SplitOptions{
	NoSplitTokensThreshold:         embedEntireFileTokensThreshold,
	ChunkTokensThreshold:           embeddingChunkTokensThreshold,
	ChunkEarlySplitTokensThreshold: embeddingChunkEarlySplitTokensThreshold,
}

func (h *handler) Handle(ctx context.Context, logger log.Logger, record *bgrepo.RepoEmbeddingJob) (err error) {
	embeddingsConfig := conf.GetEmbeddingsConfig(conf.Get().SiteConfig())
	if embeddingsConfig == nil {
		logger.Warn("embeddings are not configured or disabled")
		return nil
	}

	ctx = featureflag.WithFlags(ctx, h.db.FeatureFlags())

	repo, err := h.db.Repos().Get(ctx, record.RepoID)
	if err != nil {
		return err
	}

	logger = logger.With(
		log.String("repoName", string(repo.Name)),
		log.Int32("repoID", int32(repo.ID)),
	)

	fetcher := &revisionFetcher{
		repo:      repo.ID,
		repoName:  repo.Name,
		revision:  record.Revision,
		gitserver: h.gitserverClient,
	}

	err = fetcher.validateRevision(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			return
		}

		// If we return with err=nil, then we have created a new index with a
		// name based on the repo ID. It might be that the previous index had a
		// name based on the repo name (deprecated), which we can delete now on
		// a best-effort basis.
		indexNameDeprecated := string(embeddings.GetRepoEmbeddingIndexNameDeprecated(repo.Name))
		_ = h.uploadStore.Delete(ctx, indexNameDeprecated)
	}()

	embeddingsClient, err := embed.NewEmbeddingsClient(embeddingsConfig)
	if err != nil {
		return err
	}

	modelID := embeddingsClient.GetModelIdentifier()
	modelDims, err := embeddingsClient.GetDimensions()
	if err != nil {
		return err
	}

	qdrantInserter, err := h.getQdrantInserter()
	if err != nil {
		return err
	}

	err = qdrantInserter.PrepareUpdate(ctx, modelID, uint64(modelDims))
	if err != nil {
		return err
	}

	var previousIndex *embeddings.RepoEmbeddingIndex
	if embeddingsConfig.Incremental {
		previousIndex, err = embeddings.DownloadRepoEmbeddingIndex(ctx, h.uploadStore, repo.ID, repo.Name)
		if err != nil {
			logger.Info("no previous embeddings index found. Performing a full index", log.Error(err))
		} else if !previousIndex.IsModelCompatible(embeddingsClient.GetModelIdentifier()) {
			logger.Info("Embeddings model has changed in config. Performing a full index")
			previousIndex = nil
		}
	}

	includedFiles, excludedFiles := getFileFilterPathPatterns(embeddingsConfig)
	opts := embed.EmbedRepoOpts{
		RepoName: repo.Name,
		Revision: record.Revision,
		FileFilters: embed.FileFilters{
			ExcludePatterns:  excludedFiles,
			IncludePatterns:  includedFiles,
			MaxFileSizeBytes: embeddingsConfig.FileFilters.MaxFileSizeBytes,
		},
		SplitOptions:          splitOptions,
		MaxCodeEmbeddings:     embeddingsConfig.MaxCodeEmbeddingsPerRepo,
		MaxTextEmbeddings:     embeddingsConfig.MaxTextEmbeddingsPerRepo,
		BatchSize:             embeddingsBatchSize,
		ExcludeChunks:         embeddingsConfig.ExcludeChunkOnError,
		TolerableFailureRatio: embeddingsTolerableFailureRatio,
	}

	if previousIndex != nil {
		logger.Info("found previous embeddings index. Attempting incremental update", log.String("old_revision", string(previousIndex.Revision)))
		opts.IndexedRevision = previousIndex.Revision

		hasPreviousIndex, err := qdrantInserter.HasIndex(ctx, modelID, repo.ID, previousIndex.Revision)
		if err != nil {
			return err
		}

		if !hasPreviousIndex {
			err = uploadPreviousIndex(ctx, modelID, qdrantInserter, repo.ID, previousIndex)
			if err != nil {
				return err
			}
		}
	}

	ranks, err := h.rankingService.GetDocumentRanks(ctx, repo.Name)
	if err != nil {
		return err
	}

	reportStats := func(stats *bgrepo.EmbedRepoStats) {
		if err := h.repoEmbeddingJobsStore.UpdateRepoEmbeddingJobStats(ctx, record.ID, stats); err != nil {
			logger.Error("failed to update embedding stats", log.Error(err))
		}
	}

	repoEmbeddingIndex, toRemove, stats, err := embed.EmbedRepo(
		ctx,
		embeddingsClient,
		qdrantInserter,
		h.contextService,
		fetcher,
		repo.IDName(),
		ranks,
		opts,
		logger,
		reportStats,
	)
	if err != nil {
		return err
	}

	err = qdrantInserter.FinalizeUpdate(ctx, db.FinalizeUpdateParams{
		ModelID:       modelID,
		RepoID:        repo.ID,
		Revision:      record.Revision,
		FilesToRemove: toRemove,
	})
	if err != nil {
		return err
	}

	reportStats(stats) // final, complete report

	logger.Info(
		"finished generating repo embeddings",
		log.String("revision", string(record.Revision)),
		log.Object("stats", stats.ToFields()...),
	)

	indexName := string(embeddings.GetRepoEmbeddingIndexName(repo.ID))
	if stats.IsIncremental {
		return embeddings.UpdateRepoEmbeddingIndex(ctx, h.uploadStore, indexName, previousIndex, repoEmbeddingIndex, toRemove, ranks)
	} else {
		return embeddings.UploadRepoEmbeddingIndex(ctx, h.uploadStore, indexName, repoEmbeddingIndex)
	}
}

func getFileFilterPathPatterns(embeddingsConfig *conftypes.EmbeddingsConfig) (includedFiles, excludedFiles []*paths.GlobPattern) {
	var includedGlobPatterns, excludedGlobPatterns []*paths.GlobPattern
	if embeddingsConfig != nil {
		if len(embeddingsConfig.FileFilters.ExcludedFilePathPatterns) != 0 {
			excludedGlobPatterns = embed.CompileGlobPatterns(embeddingsConfig.FileFilters.ExcludedFilePathPatterns)
		}
		if len(embeddingsConfig.FileFilters.IncludedFilePathPatterns) != 0 {
			includedGlobPatterns = embed.CompileGlobPatterns(embeddingsConfig.FileFilters.IncludedFilePathPatterns)
		}
	}
	if len(excludedGlobPatterns) == 0 {
		excludedGlobPatterns = embed.GetDefaultExcludedFilePathPatterns()
	}
	return includedGlobPatterns, excludedGlobPatterns
}

type revisionFetcher struct {
	repo      api.RepoID
	repoName  api.RepoName
	revision  api.CommitID
	gitserver gitserver.Client
}

func (r *revisionFetcher) Read(ctx context.Context, fileName string) ([]byte, error) {
	fr, err := r.gitserver.NewFileReader(ctx, r.repo, r.revision, fileName)
	if err != nil {
		return nil, err
	}
	defer fr.Close()
	return io.ReadAll(fr)
}

func (r *revisionFetcher) List(ctx context.Context) ([]embed.FileEntry, error) {
	fileInfos, err := r.gitserver.ReadDir(ctx, r.repo, r.revision, "", true)
	if err != nil {
		return nil, err
	}

	entries := make([]embed.FileEntry, 0, len(fileInfos))
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			entries = append(entries, embed.FileEntry{
				Name: fileInfo.Name(),
				Size: fileInfo.Size(),
			})
		}
	}
	return entries, nil
}

func (r *revisionFetcher) Diff(ctx context.Context, oldCommit api.CommitID) (
	toIndex []embed.FileEntry,
	toRemove []string,
	err error,
) {
	ctx = actor.WithInternalActor(ctx)
	b, err := r.gitserver.DiffSymbols(ctx, r.repo, oldCommit, r.revision)
	if err != nil {
		return nil, nil, err
	}

	toRemove, changedNew, err := diff.ParseGitDiffNameStatus(b)
	if err != nil {
		return nil, nil, err
	}

	// toRemove only contains file names, but we also need the file sizes. We could
	// ask gitserver for the file size of each file, however my intuition tells me
	// it is cheaper to call r.List(ctx) once. As a downside we have to loop over
	// allFiles.
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	changedNewSet := make(map[string]struct{})
	for _, file := range changedNew {
		changedNewSet[file] = struct{}{}
	}

	for _, file := range allFiles {
		if _, ok := changedNewSet[file.Name]; ok {
			toIndex = append(toIndex, file)
		}
	}

	return
}

// validateRevision returns an error if the revision provided to this job is empty.
// This can happen when GetDefaultBranch's response is error or empty at the time this job was scheduled.
// Only the handler should provide the error to mark a failed/errored job, therefore handler requires a revision check.
func (r *revisionFetcher) validateRevision(ctx context.Context) error {
	// if the revision is empty then fetch from gitserver to determine this job's failure message
	if r.revision == "" {
		_, _, err := r.gitserver.GetDefaultBranch(ctx, r.repo, false)

		if err != nil {
			return err
		}

		// We likely had an empty repo at the time of scheduling this job.
		// The repo can be processed once it's resubmitted with a non-empty revision.
		return errors.Newf("could not get latest commit for repo %s", r.repoName)
	}
	return nil
}

func uploadPreviousIndex(ctx context.Context, modelID string, inserter db.VectorInserter, repoID api.RepoID, previousIndex *embeddings.RepoEmbeddingIndex) error {
	const batchSize = 128
	batch := make([]db.ChunkPoint, batchSize)

	for indexNum, index := range []embeddings.EmbeddingIndex{previousIndex.CodeIndex, previousIndex.TextIndex} {
		isCode := indexNum == 0

		// returns the ith row in the index as a ChunkPoint
		getChunkPoint := func(i int) db.ChunkPoint {
			payload := db.ChunkPayload{
				RepoName:  previousIndex.RepoName,
				RepoID:    repoID,
				Revision:  previousIndex.Revision,
				FilePath:  index.RowMetadata[i].FileName,
				StartLine: uint32(index.RowMetadata[i].StartLine),
				EndLine:   uint32(index.RowMetadata[i].EndLine),
				IsCode:    isCode,
			}
			return db.NewChunkPoint(payload, embeddings.Dequantize(index.Row(i)))
		}

		for batchStart := 0; batchStart < len(index.RowMetadata); batchStart += batchSize {
			// Build a batch
			batch = batch[:0] // reset batch
			for i := batchStart; i < batchStart+batchSize && i < len(index.RowMetadata); i++ {
				batch = append(batch, getChunkPoint(i))
			}

			// Insert the batch
			err := inserter.InsertChunks(ctx, db.InsertParams{
				ModelID:     modelID,
				ChunkPoints: batch,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
