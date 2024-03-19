package uploadhandler

import (
	"context"
)

type Upload[T any] struct {
	ID int
	// TODO(id: state-refactoring) Change this to shared.UploadState
	State            string
	NumParts         int
	UploadedParts    []int
	UploadSize       *int64
	UncompressedSize *int64
	Metadata         T
}

type DBStore[T any] interface {
	WithTransaction(ctx context.Context, f func(tx DBStore[T]) error) error

	GetUploadByID(ctx context.Context, uploadID int) (Upload[T], bool, error)
	InsertUpload(ctx context.Context, upload Upload[T]) (int, error)
	AddUploadPart(ctx context.Context, uploadID, partIndex int) error
	MarkQueued(ctx context.Context, id int, uploadSize *int64) error
	MarkFailed(ctx context.Context, id int, reason string) error
}
