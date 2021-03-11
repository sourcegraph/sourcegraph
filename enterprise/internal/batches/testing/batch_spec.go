package testing

import (
	"context"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/batches"
)

type CreateBatchSpecer interface {
	CreateBatchSpec(ctx context.Context, batchSpec *batches.BatchSpec) error
}

func CreateBatchSpec(t *testing.T, ctx context.Context, store CreateBatchSpecer, name string, userID int32) *batches.BatchSpec {
	t.Helper()

	s := &batches.BatchSpec{
		UserID:          userID,
		NamespaceUserID: userID,
		Spec: batches.BatchSpecFields{
			Name:        name,
			Description: "the description",
			ChangesetTemplate: batches.ChangesetTemplate{
				Branch: "branch-name",
			},
		},
	}

	if err := store.CreateBatchSpec(ctx, s); err != nil {
		t.Fatal(err)
	}

	return s
}
