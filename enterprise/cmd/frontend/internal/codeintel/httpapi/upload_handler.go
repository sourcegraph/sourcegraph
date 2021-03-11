package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	store "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/uploadstore"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	codeintelutils "github.com/sourcegraph/sourcegraph/lib/codeintel/utils"
)

type UploadHandler struct {
	dbStore     DBStore
	uploadStore uploadstore.Store
	internal    bool
}

func NewUploadHandler(dbStore DBStore, uploadStore uploadstore.Store, internal bool) http.Handler {
	handler := &UploadHandler{
		dbStore:     dbStore,
		uploadStore: uploadStore,
		internal:    internal,
	}

	return http.HandlerFunc(handler.handleEnqueue)
}

var revhashPattern = lazyregexp.New(`^[a-z0-9]{40}$`)

// POST /upload
func (h *UploadHandler) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var repositoryID int
	if !hasQuery(r, "uploadId") {
		repoName := getQuery(r, "repository")
		commit := getQuery(r, "commit")

		if !revhashPattern.Match([]byte(commit)) {
			http.Error(w, "Commit must be a 40-character revhash", http.StatusBadRequest)
			return
		}

		repo, ok := ensureRepoAndCommitExist(ctx, w, repoName, commit)
		if !ok {
			return
		}
		repositoryID = int(repo.ID)

		// 🚨 SECURITY: Ensure we return before proxying to the precise-code-intel-api-server upload
		// endpoint. This endpoint is unprotected, so we need to make sure the user provides a valid
		// token proving contributor access to the repository.
		if !h.internal && conf.Get().LsifEnforceAuth && !isSiteAdmin(ctx) && !enforceAuth(ctx, w, r, repoName) {
			return
		}
	}

	payload, err := h.handleEnqueueErr(w, r, repositoryID)
	if err != nil {
		if cerr, ok := err.(*ClientError); ok {
			http.Error(w, cerr.Error(), http.StatusBadRequest)
			return
		}

		if err == codeintelutils.ErrMetadataExceedsBuffer {
			http.Error(w, "Could not read indexer name from metaData vertex. Please supply it explicitly.", http.StatusBadRequest)
			return
		}

		log15.Error("Failed to enqueue payload", "error", err)
		http.Error(w, fmt.Sprintf("failed to enqueue payload: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if payload != nil {
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, payload)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

// UploadArgs are common arguments required to enqueue an upload for both
// single-payload and multipart uploads.
type UploadArgs struct {
	Commit            string
	Root              string
	RepositoryID      int
	Indexer           string
	AssociatedIndexID int
}

type enqueuePayload struct {
	ID string `json:"id"`
}

// handleEnqueueErr dispatches to the correct handler function based on query args. Running the
// `src lsif upload` command will cause one of two sequences of requests to occur. For uploads that
// are small enough repos (that can be uploaded in one-shot), only one request will be made:
//
//    - POST `/upload?repositoryId,commit,root,indexerName`
//
// For larger uploads, the requests are broken up into a setup request, a serires of upload requests,
// and a finalization request:
//
//   - POST `/upload?repositoryId,commit,root,indexerName,multiPart=true,numParts={n}`
//   - POST `/upload?uploadId={id},index={i}`
//   - POST `/upload?uploadId={id},done=true`
//
// See the functions the following functions for details on how each request is handled:
//
//   - handleEnqueueSinglePayload
//   - handleEnqueueMultipartSetup
//   - handleEnqueueMultipartUpload
//   - handleEnqueueMultipartFinalize
func (h *UploadHandler) handleEnqueueErr(w http.ResponseWriter, r *http.Request, repositoryID int) (interface{}, error) {
	ctx := r.Context()

	uploadArgs := UploadArgs{
		Commit:            getQuery(r, "commit"),
		Root:              sanitizeRoot(getQuery(r, "root")),
		RepositoryID:      repositoryID,
		Indexer:           getQuery(r, "indexerName"),
		AssociatedIndexID: getQueryInt(r, "associatedIndexId"),
	}

	if !hasQuery(r, "multiPart") && !hasQuery(r, "uploadId") {
		return h.handleEnqueueSinglePayload(r, uploadArgs)
	}

	if hasQuery(r, "multiPart") {
		if numParts := getQueryInt(r, "numParts"); numParts <= 0 {
			return nil, clientError("illegal number of parts: %d", numParts)
		} else {
			return h.handleEnqueueMultipartSetup(r, uploadArgs, numParts)
		}
	}

	if !hasQuery(r, "uploadId") {
		return nil, clientError("no uploadId supplied")
	}

	upload, exists, err := h.dbStore.GetUploadByID(ctx, getQueryInt(r, "uploadId"))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, clientError("upload not found")
	}

	if hasQuery(r, "index") {
		if partIndex := getQueryInt(r, "index"); partIndex < 0 || partIndex >= upload.NumParts {
			return nil, clientError("illegal part index: index %d is outside the range [0, %d)", partIndex, upload.NumParts)
		} else {
			return h.handleEnqueueMultipartUpload(r, upload, partIndex)
		}
	}

	if hasQuery(r, "done") {
		return h.handleEnqueueMultipartFinalize(r, upload)
	}

	return nil, clientError("no index supplied")
}

// handleEnqueueSinglePayload handles a non-multipart upload. This creates an upload record
// with state 'queued', proxies the data to the bundle manager, and returns the generated ID.
func (h *UploadHandler) handleEnqueueSinglePayload(r *http.Request, uploadArgs UploadArgs) (_ interface{}, err error) {
	ctx := r.Context()

	// Newer versions of src-cli will do this same check before uploading the file. However,
	// older versions of src-cli will not guarantee that the index name query parameter is
	// sent. Requiring it now will break valid workflows. We only need ot maintain backwards
	// compatibility on single-payload uploads, as everything else is as new as the version
	// of src-cli that always sends the indexer name.
	if uploadArgs.Indexer == "" {
		// Tee all reads from the body into a buffer so that we don't destructively consume
		// any data from the body payload.
		var buf bytes.Buffer
		teeReader := io.TeeReader(r.Body, &buf)

		gzipReader, err := gzip.NewReader(teeReader)
		if err != nil {
			return nil, err
		}

		name, err := codeintelutils.ReadIndexerName(gzipReader)
		if err != nil {
			return nil, err
		}
		uploadArgs.Indexer = name

		// Replace the body of the request with a reader that will produce all of the same
		// content: all of the data that was already read from r.Body, plus the remaining
		// content from r.Body.
		r.Body = ioutil.NopCloser(io.MultiReader(bytes.NewReader(buf.Bytes()), r.Body))
	}

	tx, err := h.dbStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	id, err := tx.InsertUpload(ctx, store.Upload{
		Commit:            uploadArgs.Commit,
		Root:              uploadArgs.Root,
		RepositoryID:      uploadArgs.RepositoryID,
		Indexer:           uploadArgs.Indexer,
		AssociatedIndexID: &uploadArgs.AssociatedIndexID,
		State:             "uploading",
		NumParts:          1,
		UploadedParts:     []int{0},
	})
	if err != nil {
		return nil, err
	}

	size, err := h.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.lsif.gz", id), r.Body)
	if err != nil {
		return nil, err
	}

	if err := tx.MarkQueued(ctx, id, &size); err != nil {
		return nil, err
	}

	log15.Info(
		"Enqueued upload",
		"id", id,
		"repository_id", uploadArgs.RepositoryID,
		"commit", uploadArgs.Commit,
	)

	// older versions of src-cli expect a string
	return enqueuePayload{strconv.Itoa(id)}, nil
}

// handleEnqueueMultipartSetup handles the first request in a multipart upload. This creates a
// new upload record with state 'uploading' and returns the generated ID to be used in subsequent
// requests for the same upload.
func (h *UploadHandler) handleEnqueueMultipartSetup(r *http.Request, uploadArgs UploadArgs, numParts int) (interface{}, error) {
	ctx := r.Context()

	id, err := h.dbStore.InsertUpload(ctx, store.Upload{
		Commit:            uploadArgs.Commit,
		Root:              uploadArgs.Root,
		RepositoryID:      uploadArgs.RepositoryID,
		Indexer:           uploadArgs.Indexer,
		AssociatedIndexID: &uploadArgs.AssociatedIndexID,
		State:             "uploading",
		NumParts:          numParts,
		UploadedParts:     nil,
	})
	if err != nil {
		return nil, err
	}

	log15.Info(
		"Enqueued upload",
		"id", id,
		"repository_id", uploadArgs.RepositoryID,
		"commit", uploadArgs.Commit,
	)

	// older versions of src-cli expect a string
	return enqueuePayload{strconv.Itoa(id)}, nil
}

// handleEnqueueMultipartUpload handles a partial upload in a multipart upload. This proxies the
// data to the bundle manager and marks the part index in the upload record.
func (h *UploadHandler) handleEnqueueMultipartUpload(r *http.Request, upload store.Upload, partIndex int) (_ interface{}, err error) {
	ctx := r.Context()

	tx, err := h.dbStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	if err := tx.AddUploadPart(ctx, upload.ID, partIndex); err != nil {
		return nil, err
	}
	if _, err := h.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.%d.lsif.gz", upload.ID, partIndex), r.Body); err != nil {
		return nil, err
	}

	return nil, nil
}

// handleEnqueueMultipartFinalize handles the final request of a multipart upload. This transitions the
// upload from 'uploading' to 'queued', then instructs the bundle manager to concatenate all of the part
// files together.
func (h *UploadHandler) handleEnqueueMultipartFinalize(r *http.Request, upload store.Upload) (_ interface{}, err error) {
	ctx := r.Context()

	if len(upload.UploadedParts) != upload.NumParts {
		return nil, clientError("upload is missing %d parts", upload.NumParts-len(upload.UploadedParts))
	}

	tx, err := h.dbStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	var sources []string
	for partNumber := 0; partNumber < upload.NumParts; partNumber++ {
		sources = append(sources, fmt.Sprintf("upload-%d.%d.lsif.gz", upload.ID, partNumber))
	}

	size, err := h.uploadStore.Compose(ctx, fmt.Sprintf("upload-%d.lsif.gz", upload.ID), sources...)
	if err != nil {
		return nil, err
	}

	if err := tx.MarkQueued(ctx, upload.ID, &size); err != nil {
		return nil, err
	}

	return nil, nil
}

func ensureRepoAndCommitExist(ctx context.Context, w http.ResponseWriter, repoName, commit string) (*types.Repo, bool) {
	repo, err := backend.Repos.GetByName(ctx, api.RepoName(repoName))
	if err != nil {
		if errcode.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("unknown repository %q", repoName), http.StatusNotFound)
			return nil, false
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}

	if _, err := backend.Repos.ResolveRev(ctx, repo, commit); err != nil {
		if gitserver.IsRevisionNotFound(err) {
			http.Error(w, fmt.Sprintf("unknown commit %q", commit), http.StatusNotFound)
			return nil, false
		}

		// If the repository is currently being cloned (which is most likely to happen on dotcom),
		// then we want to continue to queue the LSIF upload record to unblock the client, then have
		// the worker wait until the rev is resolvable before starting to process.
		if !vcs.IsCloneInProgress(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return nil, false
		}
	}

	return repo, true
}
