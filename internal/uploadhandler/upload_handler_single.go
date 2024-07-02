package uploadhandler

import (
	"context"
	"io"
	"net/http"
	"strconv"

	sglog "github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// handleEnqueueSinglePayload handles a non-multipart upload. This creates an upload record
// with state 'queued', proxies the data to the bundle manager, and returns the generated ID.
func (h *UploadHandler[T]) handleEnqueueSinglePayload(ctx context.Context, uploadState uploadState[T], body io.Reader) (_ any, statusCode int, err error) {
	ctx, _, endObservation := h.operations.handleEnqueueSinglePayload.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("statusCode", statusCode),
		}})
	}()

	uploadResult, err := h.enqueuer.EnqueueSinglePayload(ctx, uploadState.metadata, uploadState.uncompressedSize, body)

	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	h.logger.Info(
		"uploadhandler: enqueued upload",
		sglog.Int("id", uploadResult.UploadID),
	)

	// older versions of src-cli expect a string
	return struct {
		ID string `json:"id"`
	}{ID: strconv.Itoa(uploadResult.UploadID)}, 0, nil
}
