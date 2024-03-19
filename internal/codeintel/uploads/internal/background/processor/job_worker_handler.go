package processor

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/jackc/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/lsifstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/store"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func NewUploadProcessorWorker(
	observationCtx *observation.Context,
	store store.Store,
	lsifStore lsifstore.Store,
	gitserverClient gitserver.Client,
	repoStore RepoStore,
	workerStore dbworkerstore.Store[uploadsshared.Upload],
	uploadStore uploadstore.Store,
	config *Config,
) *workerutil.Worker[uploadsshared.Upload] {
	rootContext := actor.WithInternalActor(context.Background())

	handler := NewUploadProcessorHandler(
		observationCtx,
		store,
		lsifStore,
		gitserverClient,
		repoStore,
		workerStore,
		uploadStore,
		config.WorkerBudget,
	)

	metrics := workerutil.NewMetrics(observationCtx, "codeintel_upload_processor", workerutil.WithSampler(func(job workerutil.Record) bool { return true }))

	return dbworker.NewWorker(rootContext, workerStore, handler, workerutil.WorkerOptions{
		Name:                 "precise_code_intel_upload_worker",
		Description:          "processes precise code-intel uploads",
		NumHandlers:          config.WorkerConcurrency,
		Interval:             config.WorkerPollInterval,
		HeartbeatInterval:    time.Second,
		Metrics:              metrics,
		MaximumRuntimePerJob: config.MaximumRuntimePerJob,
	})
}

type handler struct {
	store           store.Store
	lsifStore       lsifstore.Store
	gitserverClient gitserver.Client
	repoStore       RepoStore
	workerStore     dbworkerstore.Store[uploadsshared.Upload]
	uploadStore     uploadstore.Store
	handleOp        *observation.Operation
	budgetRemaining int64
	enableBudget    bool
	uploadSizeGauge prometheus.Gauge
}

var (
	_ workerutil.Handler[uploadsshared.Upload]   = &handler{}
	_ workerutil.WithPreDequeue                  = &handler{}
	_ workerutil.WithHooks[uploadsshared.Upload] = &handler{}
)

func NewUploadProcessorHandler(
	observationCtx *observation.Context,
	store store.Store,
	lsifStore lsifstore.Store,
	gitserverClient gitserver.Client,
	repoStore RepoStore,
	workerStore dbworkerstore.Store[uploadsshared.Upload],
	uploadStore uploadstore.Store,
	budgetMax int64,
) workerutil.Handler[uploadsshared.Upload] {
	operations := newWorkerOperations(observationCtx)

	return &handler{
		store:           store,
		lsifStore:       lsifStore,
		gitserverClient: gitserverClient,
		repoStore:       repoStore,
		workerStore:     workerStore,
		uploadStore:     uploadStore,
		handleOp:        operations.uploadProcessor,
		budgetRemaining: budgetMax,
		enableBudget:    budgetMax > 0,
		uploadSizeGauge: operations.uploadSizeGauge,
	}
}

func (h *handler) Handle(ctx context.Context, logger log.Logger, upload uploadsshared.Upload) (err error) {
	var requeued bool

	ctx, tr, endObservation := h.handleOp.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: append(
			createLogFields(upload),
			attribute.Bool("requeued", requeued),
		)})
	}()

	requeued, err = h.HandleRawUpload(ctx, logger, upload, h.uploadStore, tr)

	return err
}

func (h *handler) PreDequeue(_ context.Context, _ log.Logger) (bool, any, error) {
	if !h.enableBudget {
		return true, nil, nil
	}

	budgetRemaining := atomic.LoadInt64(&h.budgetRemaining)
	if budgetRemaining <= 0 {
		return false, nil, nil
	}

	return true, []*sqlf.Query{sqlf.Sprintf("(upload_size IS NULL OR upload_size <= %s)", budgetRemaining)}, nil
}

func (h *handler) PreHandle(_ context.Context, _ log.Logger, upload uploadsshared.Upload) {
	uncompressedSize := pointers.DerefZero(upload.UncompressedSize)
	h.uploadSizeGauge.Add(float64(uncompressedSize))

	gzipSize := pointers.DerefZero(upload.UploadSize)
	atomic.AddInt64(&h.budgetRemaining, -gzipSize)
}

func (h *handler) PostHandle(_ context.Context, _ log.Logger, upload uploadsshared.Upload) {
	uncompressedSize := pointers.DerefZero(upload.UncompressedSize)
	h.uploadSizeGauge.Sub(float64(uncompressedSize))

	gzipSize := pointers.DerefZero(upload.UploadSize)
	atomic.AddInt64(&h.budgetRemaining, +gzipSize)
}

func createLogFields(upload uploadsshared.Upload) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Int("uploadID", upload.ID),
		attribute.Int("repositoryID", upload.RepositoryID),
		attribute.String("commit", upload.Commit),
		attribute.String("root", upload.Root),
		attribute.String("indexer", upload.Indexer),
		attribute.Stringer("queueDuration", time.Since(upload.UploadedAt)),
	}

	if upload.UploadSize != nil {
		attrs = append(attrs, attribute.Int64("uploadSize", *upload.UploadSize))
	}

	return attrs
}

// defaultBranchContains tells if the default branch contains the given commit ID.
func (c *handler) defaultBranchContains(ctx context.Context, repo api.RepoName, commit string) (bool, error) {
	// Determine default branch name.
	defaultBranchName, _, err := c.gitserverClient.GetDefaultBranch(ctx, repo, true)
	if err != nil {
		return false, err
	}

	// Determine if branch contains commit.
	branches, err := c.gitserverClient.BranchesContaining(ctx, repo, api.CommitID(commit))
	if err != nil {
		return false, err
	}
	for _, branch := range branches {
		if branch == defaultBranchName {
			return true, nil
		}
	}
	return false, nil
}

// HandleRawUpload converts a raw upload into a dump within the given transaction context. Returns true if the
// upload record was requeued and false otherwise.
func (h *handler) HandleRawUpload(ctx context.Context, logger log.Logger, upload uploadsshared.Upload, uploadStore uploadstore.Store, trace observation.TraceLogger) (requeued bool, err error) {
	repo, err := h.repoStore.Get(ctx, api.RepoID(upload.RepositoryID))
	if err != nil {
		return false, errors.Wrap(err, "Repos.Get")
	}

	if requeued, err := requeueIfCloningOrCommitUnknown(ctx, logger, h.gitserverClient, h.workerStore, upload, repo); err != nil || requeued {
		return requeued, err
	}

	// Determine if the upload is for the default Git branch.
	isDefaultBranch, err := h.defaultBranchContains(ctx, repo.Name, upload.Commit)
	if err != nil {
		return false, errors.Wrap(err, "gitserver.DefaultBranchContains")
	}

	trace.AddEvent("TODO Domain Owner", attribute.Bool("defaultBranch", isDefaultBranch))

	getChildren := func(ctx context.Context, dirnames []string) (map[string][]string, error) {
		directoryChildren, err := h.gitserverClient.ListDirectoryChildren(ctx, repo.Name, api.CommitID(upload.Commit), dirnames)
		if err != nil {
			return nil, errors.Wrap(err, "gitserverClient.DirectoryChildren")
		}
		return directoryChildren, nil
	}

	return false, withUploadData(ctx, logger, uploadStore, upload.SizeStats(), trace, func(indexReader gzipReadSeeker) (err error) {
		const (
			lsifContentType = "application/x-ndjson+lsif"
			scipContentType = "application/x-protobuf+scip"
		)
		if upload.ContentType == lsifContentType {
			return errors.New("LSIF support is deprecated")
		} else if upload.ContentType != scipContentType {
			return errors.Newf("unsupported content type %q", upload.ContentType)
		}

		// Find the commit date for the commit attached to this upload record and insert it into the
		// database (if not already present). We need to have the commit data of every processed upload
		// for a repository when calculating the commit graph (triggered at the end of this handler).

		commit, err := h.gitserverClient.GetCommit(ctx, repo.Name, api.CommitID(upload.Commit))
		if err != nil {
			if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
				return errCommitDoesNotExist
			}
			return errors.Wrap(err, "failed to determine commit date")
		}

		commitDate := commit.Committer.Date

		trace.AddEvent("TODO Domain Owner", attribute.String("commitDate", commitDate.String()))

		// We do the update here outside of the transaction started below to reduce the long blocking
		// behavior we see when multiple uploads are being processed for the same repository and commit.
		// We do choose to perform this before this the following transaction rather than after so that
		// we can guarantee the presence of the date for this commit by the time the repository is set
		// as dirty.
		if err := h.store.UpdateCommittedAt(ctx, upload.RepositoryID, upload.Commit, commitDate.Format(time.RFC3339)); err != nil {
			return errors.Wrap(err, "store.CommitDate")
		}

		scipDataStream, err := prepareSCIPDataStream(ctx, indexReader, upload.Root, getChildren)
		if err != nil {
			return errors.Wrap(err, "prepareSCIPDataStream")
		}

		// Note: this is writing to a different database than the block below, so we need to use a
		// different transaction context (managed by the writeData function).
		pkgData, err := writeSCIPDocuments(ctx, logger, h.lsifStore, upload, scipDataStream, trace)
		if err != nil {
			if isUniqueConstraintViolation(err) {
				// If this is a unique constraint violation, then we've previously processed this same
				// upload record up to this point, but failed to perform the transaction below. We can
				// safely assume that the entire index's data is in the codeintel database, as it's
				// parsed deterministically and written atomically.
				logger.Warn("SCIP data already exists for upload record")
				trace.AddEvent("TODO Domain Owner", attribute.Bool("rewriting", true))
			} else {
				return err
			}
		}

		// Start a nested transaction with Postgres savepoints. In the event that something after this
		// point fails, we want to update the upload record with an error message but do not want to
		// alter any other data in the database. Rolling back to this savepoint will allow us to discard
		// any other changes but still commit the transaction as a whole.
		return inTransaction(ctx, h.store, func(tx store.Store) error {
			// Before we mark the upload as complete, we need to delete any existing completed uploads
			// that have the same repository_id, commit, root, and indexer values. Otherwise, the transaction
			// will fail as these values form a unique constraint.
			if err := tx.DeleteOverlappingCompletedUploads(ctx, upload.RepositoryID, upload.Commit, upload.Root, upload.Indexer); err != nil {
				return errors.Wrap(err, "store.DeleteOverlappingCompletedUploads")
			}

			trace.AddEvent("TODO Domain Owner", attribute.Int("packages", len(pkgData.Packages)))
			// Update package and package reference data to support cross-repo queries.
			if err := tx.UpdatePackages(ctx, upload.ID, pkgData.Packages); err != nil {
				return errors.Wrap(err, "store.UpdatePackages")
			}
			trace.AddEvent("TODO Domain Owner", attribute.Int("packageReferences", len(pkgData.PackageReferences)))
			if err := tx.UpdatePackageReferences(ctx, upload.ID, pkgData.PackageReferences); err != nil {
				return errors.Wrap(err, "store.UpdatePackageReferences")
			}

			// Insert a companion record to this upload that will asynchronously trigger other workers to
			// sync/create referenced dependency repositories and queue auto-index records for the monikers
			// written into the lsif_references table attached by this index processing job.
			if _, err := tx.InsertDependencySyncingJob(ctx, upload.ID); err != nil {
				return errors.Wrap(err, "store.InsertDependencyIndexingJob")
			}

			// Mark this repository so that the commit updater process will pull the full commit graph from
			// gitserver and recalculate the nearest upload for each commit as well as which uploads are visible
			// from the tip of the default branch. We don't do this inside of the transaction as we re-calculate
			// the entire set of data from scratch and we want to be able to coalesce requests for the same
			// repository rather than having a set of uploads for the same repo re-calculate nearly identical
			// data multiple times.
			if err := tx.SetRepositoryAsDirty(ctx, upload.RepositoryID); err != nil {
				return errors.Wrap(err, "store.MarkRepositoryAsDirty")
			}

			return nil
		})
	})
}

func inTransaction(ctx context.Context, dbStore store.Store, fn func(tx store.Store) error) (err error) {
	return dbStore.WithTransaction(ctx, fn)
}

// requeueDelay is the delay between processing attempts to process a record when waiting on
// gitserver to refresh. We'll requeue a record with this delay while the repo is cloning or
// while we're waiting for a commit to become available to the remote code host.
const requeueDelay = time.Minute

// requeueIfCloningOrCommitUnknown ensures that the repo and revision are resolvable. If the repo is currently
// cloning or if the commit does not exist, then the upload will be requeued and this function returns a true
// valued flag. Otherwise, the repo does not exist or there is an unexpected infrastructure error, which we'll
// fail on.
func requeueIfCloningOrCommitUnknown(ctx context.Context, logger log.Logger, gitserverClient gitserver.Client, workerStore dbworkerstore.Store[uploadsshared.Upload], upload uploadsshared.Upload, repo *types.Repo) (requeued bool, _ error) {
	_, err := gitserverClient.ResolveRevision(ctx, repo.Name, upload.Commit, gitserver.ResolveRevisionOptions{})
	if err == nil {
		// commit is resolvable
		return false, nil
	}

	var reason string
	if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
		reason = "commit not found"
	} else if gitdomain.IsCloneInProgress(err) {
		reason = "repository still cloning"
	} else {
		return false, errors.Wrap(err, "repos.ResolveRev")
	}

	after := time.Now().UTC().Add(requeueDelay)

	if err := workerStore.Requeue(ctx, upload.ID, after); err != nil {
		return false, errors.Wrap(err, "store.Requeue")
	}
	logger.Warn("Requeued LSIF upload record",
		log.Int("id", upload.ID),
		log.String("reason", reason))
	return true, nil
}

// NOTE(scip-index-size-stats) In practice, the following seem to be true:
//   - The size of an uncompressed index is about 5x-10x the size of
//     the gzip-compressed index
//   - The memory usage of a full deserialized scip.Index is about
//     2.5x-4.5x times the size of the uncompressed index byte slice.
//
// The code intel worker sometimes has as little as 2GB of RAM, so 1/4-th of
// that is 512MiB. There is no simple portable API to determine the
// max available memory to the process, so use a constant for now.
//
// Marked as a 'var' only for testing.
var uncompressedSizeLimitBytes int64 = 512 * 1024 * 1024

type gzipReadSeeker struct {
	inner      io.ReadSeeker
	gzipReader *gzip.Reader
}

func newGzipReadSeeker(rs io.ReadSeeker) (gzipReadSeeker, error) {
	gzipReader, err := gzip.NewReader(rs)
	return gzipReadSeeker{rs, gzipReader}, err
}

func (grs gzipReadSeeker) Read(buf []byte) (int, error) {
	return grs.gzipReader.Read(buf)
}

func (grs *gzipReadSeeker) seekToStart() (err error) {
	if _, err := grs.inner.Seek(0, io.SeekStart); err != nil {
		return err
	}
	grs.gzipReader, err = gzip.NewReader(grs.inner)
	return err
}

// withUploadData will invoke the given function with a reader of the upload's raw data. The
// consumer should expect raw newline-delimited JSON content. If the function returns without
// an error, the upload file will be deleted.
func withUploadData(ctx context.Context, logger log.Logger, uploadStore uploadstore.Store, uploadStats uploadsshared.UploadSizeStats, trace observation.TraceLogger, fn func(r gzipReadSeeker) error) error {
	uploadFilename := fmt.Sprintf("upload-%d.lsif.gz", uploadStats.ID)

	trace.AddEvent("TODO Domain Owner", attribute.String("uploadFilename", uploadFilename))

	// Pull raw uploaded data from bucket
	rc, err := uploadStore.Get(ctx, uploadFilename)
	if err != nil {
		return errors.Wrap(err, "uploadStore.Get")
	}
	defer rc.Close()

	indexReader, cleanup, err := func() (_ gzipReadSeeker, cleanup func() error, err error) {
		// If the uncompressed upload size is available and exceeds what we want
		// to keep resident in memory during processing, write the upload to
		// a temporary file, to allow processing in multiple passes.
		shouldWriteToDisk := uploadStats.UncompressedSize != nil && *uploadStats.UncompressedSize > uncompressedSizeLimitBytes

		if !shouldWriteToDisk {
			compressedSizeHint := int64(0)
			if uploadStats.UploadSize != nil {
				compressedSizeHint = *uploadStats.UploadSize
			}
			buf, err := readAllWithSizeHint(rc, compressedSizeHint)
			if err != nil {
				return gzipReadSeeker{}, nil, errors.Wrap(err, "failed to read upload file")
			}

			if uploadStats.UncompressedSize == nil {
				// Make a best-effort estimate for the uncompressed size, as it may
				// make sense to write it the upload to disk despite having read
				// it into memory to avoid OOM during processing.
				// The factor of 5 is based on ~worst-case gzip compression ratio.
				// See NOTE(scip-index-size-stats).
				compressedSize := len(buf)
				uncompressedSizeEstimate := compressedSize * 5
				shouldWriteToDisk = int64(uncompressedSizeEstimate) > uncompressedSizeLimitBytes
			}

			if !shouldWriteToDisk {
				// No temp files created, nothing to cleanup
				cleanup := func() error { return nil }

				// Payload is small enough to process in-memory, return a reader backed
				// by the slice we've already read from the blobstore
				indexReader, err := newGzipReadSeeker(bytes.NewReader(buf))
				return indexReader, cleanup, err
			}

			// Fallthrough:
			// Replace the reader we'll write to disk with the content we've already read
			rc = io.NopCloser(bytes.NewReader(buf))
		}

		tempFile, err := os.CreateTemp("", fmt.Sprintf("upload-%d-tmp.gz", uploadStats.ID))
		if err != nil {
			return gzipReadSeeker{}, nil, errors.Wrap(err, "failed to create temporary file to save upload for streaming")
		}

		// Immediately create cleanup function after successful creation of a temporary file
		// and on any non-nil error from this function ensure we clean up any resources we've
		// created on the failure path.
		cleanup = func() error { return os.RemoveAll(tempFile.Name()) }
		defer func() {
			if err != nil {
				_ = cleanup()
			}
		}()

		if _, err = io.Copy(tempFile, rc); err != nil {
			return gzipReadSeeker{}, nil, errors.Wrap(err, "failed to copy buffer to temporary file")
		}
		if _, err = tempFile.Seek(0, io.SeekStart); err != nil {
			return gzipReadSeeker{}, nil, errors.Wrap(err, "failed to seek to start")
		}

		// Wrap the file reader
		indexReader, err := newGzipReadSeeker(tempFile)
		return indexReader, cleanup, errors.Wrapf(err, "failed to decompress file %q", tempFile.Name())
	}()
	if err != nil {
		return err
	}
	defer func() { _ = cleanup() }()

	if err := fn(indexReader); err != nil {
		return err
	}

	if err := uploadStore.Delete(ctx, uploadFilename); err != nil {
		logger.Warn("Failed to delete upload file",
			log.NamedError("err", err),
			log.String("filename", uploadFilename))
	}

	return nil
}

func isUniqueConstraintViolation(err error) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && e.Code == "23505"
}

// errCommitDoesNotExist occurs when gitserver does not recognize the commit attached to the upload.
var errCommitDoesNotExist = errors.Errorf("commit does not exist")
