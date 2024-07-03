package uploadhandler

import (
	"context"
	"fmt"
	"io"

	sglog "github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"go.opentelemetry.io/otel/attribute"
)

type UploadEnqueuer[T any] struct {
	logger      sglog.Logger
	dbStore     DBStore[T]
	uploadStore uploadstore.Store
	operations  EnqueuerOperations
}

func NewUploadEnqueuer[T any](observationCtx *observation.Context, dbStore DBStore[T], uploadStore uploadstore.Store) UploadEnqueuer[T] {
	return UploadEnqueuer[T]{
		logger:      observationCtx.Logger.Scoped("upload_enqueuer"),
		dbStore:     dbStore,
		uploadStore: uploadStore,
		operations:  *NewEnqueuerOperations(observationCtx),
	}
}

type UploadResult struct {
	UploadID       int
	CompressedSize int64
}

func (u *UploadEnqueuer[T]) EnqueueSinglePayload(ctx context.Context, metadata T, uncompressedSize *int64, body io.Reader) (_ *UploadResult, err error) {

	ctx, trace, endObservation := u.operations.enqueueSinglePayload.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{}})
	}()

	var uploadID int
	var compressedSize int64
	if err := u.dbStore.WithTransaction(ctx, func(tx DBStore[T]) error {
		id, err := tx.InsertUpload(ctx, Upload[T]{
			State:            "uploading",
			NumParts:         1,
			UploadedParts:    []int{0},
			UncompressedSize: uncompressedSize,
			Metadata:         metadata,
		})
		if err != nil {
			return err
		}
		trace.AddEvent("InsertUpload", attribute.Int("uploadID", id))

		compressedSize, err = u.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.lsif.gz", id), body)
		if err != nil {
			return errors.Newf("Failed to upload data to upload store (id=%d): %s", id, err)
		}
		trace.AddEvent("uploadStore.Upload", attribute.Int64("gzippedUploadSize", compressedSize))

		if err := tx.MarkQueued(ctx, id, &compressedSize); err != nil {
			return errors.Newf("Failed to mark upload (id=%d) as queued: %s", id, err)
		}

		uploadID = id
		return nil
	}); err != nil {
		return nil, err
	}

	trace.Info(
		"enqueued upload",
		sglog.Int("id", uploadID),
	)

	return &UploadResult{uploadID, compressedSize}, nil
}
