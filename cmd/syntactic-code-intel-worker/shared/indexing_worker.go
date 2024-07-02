package shared

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/lsifuploadstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/jobstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/uploadhandler"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewIndexingWorker(ctx context.Context,
	observationCtx *observation.Context,
	jobStore jobstore.SyntacticIndexingJobStore,
	config IndexingWorkerConfig,
	db database.DB,
	codeintelDB codeintelshared.CodeIntelDB,
	gitserverClient gitserver.Client,
) (*workerutil.Worker[*jobstore.SyntacticIndexingJob], error) {

	name := "syntactic_code_intel_indexing_worker"

	uploadStore, err := lsifuploadstore.New(ctx, observationCtx, config.LSIFUploadStoreConfig)

	if err != nil {
		return nil, err
	}

	handler, err := NewIndexingHandler(ctx, observationCtx, jobStore, config, db, codeintelDB, gitserverClient, uploadStore)
	if err != nil {
		return nil, err
	}

	return dbworker.NewWorker(ctx, jobStore.DBWorkerStore(), handler, workerutil.WorkerOptions{
		Name:                 name,
		Interval:             config.PollInterval,
		HeartbeatInterval:    10 * time.Second, // TODO: extract into config as well?
		Metrics:              workerutil.NewMetrics(observationCtx, name),
		NumHandlers:          config.Concurrency,
		MaximumRuntimePerJob: config.MaximumRuntimePerJob,
	}), nil

}

func NewIndexingHandler(ctx context.Context,
	observationCtx *observation.Context,
	jobStore jobstore.SyntacticIndexingJobStore,
	config IndexingWorkerConfig,
	db database.DB,
	codeintelDB codeintelshared.CodeIntelDB,
	gitserverClient gitserver.Client,
	uploadStore uploadstore.Store,
) (*indexingHandler, error) {

	uploadsService := uploads.NewService(observationCtx, db, codeintelDB, gitserverClient)
	uploadsDBStore := uploadsService.UploadHandlerStore()

	uploadEnqueuer := uploadhandler.NewUploadEnqueuer(observationCtx, uploadsDBStore, uploadStore)

	return &indexingHandler{
		Config:          config,
		GitServerClient: gitserverClient,
		UploadEnqueuer:  uploadEnqueuer,
	}, nil

}

type indexingHandler struct {
	Config          IndexingWorkerConfig
	GitServerClient gitserver.Client
	UploadEnqueuer  uploadhandler.UploadEnqueuer[uploads.UploadMetadata]
}

var _ workerutil.Handler[*jobstore.SyntacticIndexingJob] = &indexingHandler{}

func (i indexingHandler) Handle(ctx context.Context, logger log.Logger, record *jobstore.SyntacticIndexingJob) error {
	logger.Debug("Syntactic indexing worker handling record",
		log.Int("id", record.ID),
		log.String("repository name", record.RepositoryName),
		log.String("commit", string(record.Commit)))

	tarStream, err := i.GitServerClient.ArchiveReader(
		ctx,
		api.RepoName(record.RepositoryName),
		gitserver.ArchiveOptions{Treeish: string(record.Commit), Format: gitserver.ArchiveFormatTar},
	)
	if err != nil {
		return err
	}

	tempLocation := path.Join(os.TempDir(), fmt.Sprintf("syntactic-index-job_%d-repo_%d-commit_%s.scip", record.ID, record.RepositoryID, record.Commit))
	defer func() {
		os.Remove(tempLocation)
	}()

	command := exec.Command(i.Config.CliPath, "index", "tar", "-", "--language", "java", "--out", tempLocation)

	cmdStdinPipe, err := command.StdinPipe()
	if err != nil {
		return err
	}

	if err = command.Start(); err != nil {
		return err
	}

	if _, err := io.Copy(cmdStdinPipe, tarStream); err != nil {
		return err
	}

	if err = command.Wait(); err != nil {
		return err
	}

	// TODO: once the CLI can output metrics, we should log them here
	logger.Debug("Syntactic indexing finished",
		log.Int("repository_id", int(record.RepositoryID)),
		log.String("commit", string(record.Commit)),
		log.String("output", tempLocation))

	f, err := os.Open(tempLocation)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
	}()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	fileSize := fi.Size()

	uploadResult, err := i.UploadEnqueuer.EnqueueSinglePayload(
		ctx,
		createUploadMetadata(record.RepositoryID, record.Commit),
		&fileSize,
		gzipReader(bufio.NewReader(f)),
	)

	if err != nil {
		return err
	}

	logger.Info("Successfully queued upload",
		log.Int("uploadID", uploadResult.UploadID),
		log.Int64("uncompressed size", fileSize),
		log.Int("repository_id", int(record.RepositoryID)),
		log.String("commit", string(record.Commit)),
	)

	return nil
}

func createUploadMetadata(repositoryId api.RepoID, commit api.CommitID) uploads.UploadMetadata {
	return uploads.UploadMetadata{
		RepositoryID:   int(repositoryId),
		Commit:         string(commit),
		Root:           "",
		Indexer:        "scip-syntax",
		IndexerVersion: "1.0.0",
		ContentType:    "application/x-protobuf+scip",
	}
}

// gzipReader decorates a source reader by gzip compressing its contents.
func gzipReader(source io.Reader) io.Reader {
	r, w := io.Pipe()
	go func() {
		// propagate gzip write errors into new reader
		w.CloseWithError(gzipPipe(source, w))
	}()
	return r
}

// gzipPipe reads uncompressed data from r and writes compressed data to w.
func gzipPipe(r io.Reader, w io.Writer) (err error) {
	gzipWriter := gzip.NewWriter(w)
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil {
			err = errors.Append(err, err)
		}
	}()

	_, err = io.Copy(gzipWriter, r)
	return err
}
