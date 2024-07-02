package uploadhandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	sglog "github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type UploadHandler[T any] struct {
	logger              sglog.Logger
	dbStore             DBStore[T]
	uploadStore         uploadstore.Store
	enqueuer            UploadEnqueuer[T]
	operations          *Operations
	metadataFromRequest func(ctx context.Context, r *http.Request) (T, int, error)
}

func NewUploadHandler[T any](
	observationCtx *observation.Context,
	dbStore DBStore[T],
	uploadStore uploadstore.Store,
	operations *Operations,
	metadataFromRequest func(ctx context.Context, r *http.Request) (T, int, error),
) http.Handler {
	handler := &UploadHandler[T]{
		logger:              observationCtx.Logger,
		dbStore:             dbStore,
		uploadStore:         uploadStore,
		operations:          operations,
		enqueuer:            NewUploadEnqueuer[T](observationCtx, dbStore, uploadStore),
		metadataFromRequest: metadataFromRequest,
	}

	return http.HandlerFunc(handler.handleEnqueue)
}

var errUnprocessableRequest = errors.New("unprocessable request: missing expected query arguments (uploadId, index, or done)")

// POST /upload
//
// handleEnqueue dispatches to the correct handler function based on the request's query args. Running
// commands such as `src code-intel upload` will cause one of two sequences of requests to occur. For
// uploads that are small enough repos (that can be uploaded in one-shot), only one request will be made:
//
//   - POST `/upload?{metadata}`
//
// where `{metadata}` contains the keys `repositoryId`, `commit`, `root`, `indexerName`, `indexerVersion`,
// and `associatedIndexId`.
//
// For larger uploads, the requests are broken up into a setup request, a serires of upload requests,
// and a finalization request:
//
//   - POST `/upload?multiPart=true,numParts={n},{metadata}`
//   - POST `/upload?uploadId={id},index={i}`
//   - POST `/upload?uploadId={id},done=true`
//
// See the functions the following functions for details on how each request is handled:
//
//   - handleEnqueueSinglePayload
//   - handleEnqueueMultipartSetup
//   - handleEnqueueMultipartUpload
//   - handleEnqueueMultipartFinalize
func (h *UploadHandler[T]) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	// Wrap the interesting bits of this in a function literal that's immediately
	// executed so that we can instrument the duration and the resulting error more
	// easily. The remainder of the function simply serializes the result to the
	// HTTP response writer.
	payload, statusCode, err := func() (_ any, statusCode int, err error) {
		// Always enable tracing for uploads, since a typical Sourcegraph instance doesn't
		// have that many uploads, and lack of traces by default makes debugging harder.
		ctx := policy.WithShouldTrace(r.Context(), true)
		ctx, trace, endObservation := h.operations.handleEnqueue.With(ctx, &err, observation.Args{})
		defer func() {
			endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
				attribute.Int("statusCode", statusCode),
			}})
		}()

		uploadState, statusCode, err := h.constructUploadState(ctx, r)
		if err != nil {
			return nil, statusCode, err
		}
		trace.AddEvent("finished constructUploadState", uploadState.Attrs()...)

		if uploadHandlerFunc := h.selectUploadHandlerFunc(uploadState); uploadHandlerFunc != nil {
			return uploadHandlerFunc(ctx, uploadState, r.Body)
		}

		return nil, http.StatusBadRequest, errUnprocessableRequest
	}()
	if err != nil {
		if statusCode >= 500 {
			h.logger.Error("uploadhandler: failed to enqueue payload", sglog.Error(err))
		}

		http.Error(w, fmt.Sprintf("failed to enqueue payload: %s", err.Error()), statusCode)
		return
	}

	if payload == nil {
		// 204 with no body
		w.WriteHeader(http.StatusNoContent)
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("uploadhandler: failed to serialize result", sglog.Error(err))
		http.Error(w, fmt.Sprintf("failed to serialize result: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// 202 with identifier payload
	w.WriteHeader(http.StatusAccepted)

	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		h.logger.Error("uploadhandler: failed to write payload to client", sglog.Error(err))
	}
}

type uploadHandlerFunc[T any] func(context.Context, uploadState[T], io.Reader) (any, int, error)

func (h *UploadHandler[T]) selectUploadHandlerFunc(uploadState uploadState[T]) uploadHandlerFunc[T] {
	if !uploadState.suppliedUploadID {
		if uploadState.multipart {
			return h.handleEnqueueMultipartSetup
		}

		return h.handleEnqueueSinglePayload
	}

	if uploadState.suppliedIndex {
		return h.handleEnqueueMultipartUpload
	}

	if uploadState.done {
		return h.handleEnqueueMultipartFinalize
	}

	return nil
}
