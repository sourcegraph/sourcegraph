package testing

import (
	"context"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/batches"
)

type CreateBatchChanger interface {
	CreateBatchChange(ctx context.Context, batchChange *batches.BatchChange) error
	Clock() func() time.Time
}

func BuildBatchChange(store CreateBatchChanger, name string, userID int32, spec int64) *batches.BatchChange {
	b := &batches.BatchChange{
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    store.Clock()(),
		NamespaceUserID:  userID,
		BatchSpecID:      spec,
		Name:             name,
		Description:      "batch change description",
	}
	return b
}

func CreateBatchChange(t *testing.T, ctx context.Context, store CreateBatchChanger, name string, userID int32, spec int64) *batches.BatchChange {
	t.Helper()

	b := BuildBatchChange(store, name, userID, spec)

	if err := store.CreateBatchChange(ctx, b); err != nil {
		t.Fatal(err)
	}

	return b
}
