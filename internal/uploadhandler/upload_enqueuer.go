package uploadhandler

import (
	"context"
	"fmt"
	"io"

	sglog "github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
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
	UploadID int
}

func (u *UploadEnqueuer[T]) EnqueueSinglePayload(ctx context.Context, metadata T, uncompressedSize *int64, body io.Reader) (_ *UploadResult, err error) {

	ctx, trace, endObservation := u.operations.enqueueSinglePayload.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{}})
	}()

	var uploadID int
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
		trace.AddEvent("TODO Domain Owner", attribute.Int("uploadID", id))

		size, err := u.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.lsif.gz", id), body)
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner", attribute.Int("gzippedUploadSize", int(size)))

		if err := tx.MarkQueued(ctx, id, &size); err != nil {
			return err
		}

		uploadID = id
		return nil
	}); err != nil {
		return nil, err
	}

	u.logger.Info(
		"enqueued upload",
		sglog.Int("id", uploadID),
	)

	return &UploadResult{uploadID}, nil
}
