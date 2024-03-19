package uploadhandler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	sglog "github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// handleEnqueueMultipartSetup handles the first request in a multipart upload. This creates a
// new upload record with state 'uploading' and returns the generated ID to be used in subsequent
// requests for the same upload.
func (h *UploadHandler[T]) handleEnqueueMultipartSetup(ctx context.Context, uploadState uploadState[T], _ io.Reader) (_ any, statusCode int, err error) {
	ctx, trace, endObservation := h.operations.handleEnqueueMultipartSetup.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("statusCode", statusCode),
		}})
	}()

	if uploadState.numParts <= 0 {
		return nil, http.StatusBadRequest, errors.Errorf("illegal number of parts: %d", uploadState.numParts)
	}

	id, err := h.dbStore.InsertUpload(ctx, Upload[T]{
		State:            "uploading",
		NumParts:         uploadState.numParts,
		UploadedParts:    nil,
		UncompressedSize: uploadState.uncompressedSize,
		Metadata:         uploadState.metadata,
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("uploadID", id))

	h.logger.Info(
		"uploadhandler: enqueued upload",
		sglog.Int("id", id),
	)

	// older versions of src-cli expect a string
	return struct {
		ID string `json:"id"`
	}{ID: strconv.Itoa(id)}, 0, nil
}

// handleEnqueueMultipartUpload handles a partial upload in a multipart upload. This proxies the
// data to the bundle manager and marks the part index in the upload record.
func (h *UploadHandler[T]) handleEnqueueMultipartUpload(ctx context.Context, uploadState uploadState[T], body io.Reader) (_ any, statusCode int, err error) {
	ctx, trace, endObservation := h.operations.handleEnqueueMultipartUpload.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("statusCode", statusCode),
		}})
	}()

	if uploadState.index < 0 || uploadState.index >= uploadState.numParts {
		return nil, http.StatusBadRequest, errors.Errorf("illegal part index: index %d is outside the range [0, %d)", uploadState.index, uploadState.numParts)
	}

	size, err := h.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.%d.lsif.gz", uploadState.uploadID, uploadState.index), body)
	if err != nil {
		h.markUploadAsFailed(context.Background(), h.dbStore, uploadState.uploadID, err)
		return nil, http.StatusInternalServerError, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("gzippedUploadPartSize", int(size)))

	if err := h.dbStore.AddUploadPart(ctx, uploadState.uploadID, uploadState.index); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return nil, 0, nil
}

// handleEnqueueMultipartFinalize handles the final request of a multipart upload. This transitions the
// upload from 'uploading' to 'queued', then instructs the bundle manager to concatenate all of the part
// files together.
func (h *UploadHandler[T]) handleEnqueueMultipartFinalize(ctx context.Context, uploadState uploadState[T], _ io.Reader) (_ any, statusCode int, err error) {
	ctx, trace, endObservation := h.operations.handleEnqueueMultipartFinalize.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("statusCode", statusCode),
		}})
	}()

	if len(uploadState.uploadedParts) != uploadState.numParts {
		return nil, http.StatusBadRequest, errors.Errorf("upload is missing %d parts", uploadState.numParts-len(uploadState.uploadedParts))
	}

	sources := make([]string, 0, uploadState.numParts)
	for partNumber := range uploadState.numParts {
		sources = append(sources, fmt.Sprintf("upload-%d.%d.lsif.gz", uploadState.uploadID, partNumber))
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("numSources", len(sources)),
		attribute.String("sources", strings.Join(sources, ",")))

	size, err := h.uploadStore.Compose(ctx, fmt.Sprintf("upload-%d.lsif.gz", uploadState.uploadID), sources...)
	if err != nil {
		h.markUploadAsFailed(context.Background(), h.dbStore, uploadState.uploadID, err)
		return nil, http.StatusInternalServerError, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("composedObjectSize", int(size)))

	if err := h.dbStore.MarkQueued(ctx, uploadState.uploadID, &size); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return nil, 0, nil
}

// markUploadAsFailed attempts to mark the given upload as failed, extracting a human-meaningful
// error message from the given error. We assume this method to whenever an error occurs when
// interacting with the upload store so that the status of the upload is accurately reflected in
// the UI.
//
// This method does not return an error as it's best-effort cleanup. If an error occurs when
// trying to modify the record, it will be logged but will not be directly visible to the user.
func (h *UploadHandler[T]) markUploadAsFailed(ctx context.Context, tx DBStore[T], uploadID int, err error) {
	var reason string
	var e manager.MultiUploadFailure

	if errors.As(err, &e) {
		// Unwrap the root AWS/S3 error
		reason = fmt.Sprintf("object store error:\n* %s", e.Error())
	} else {
		reason = fmt.Sprintf("unknown error:\n* %s", err)
	}

	if markErr := tx.MarkFailed(ctx, uploadID, reason); markErr != nil {
		h.logger.Error("uploadhandler: failed to mark upload as failed", sglog.Error(markErr))
	}
}
