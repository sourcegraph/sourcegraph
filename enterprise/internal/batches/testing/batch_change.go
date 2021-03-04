package testing

import (
	"context"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/batches"
)

type CreateCampaigner interface {
	CreateCampaign(ctx context.Context, campaign *batches.BatchChange) error
	Clock() func() time.Time
}

func BuildBatchChange(store CreateCampaigner, name string, userID int32, spec int64) *batches.BatchChange {
	b := &batches.BatchChange{
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    store.Clock()(),
		NamespaceUserID:  userID,
		CampaignSpecID:   spec,
		Name:             name,
		Description:      "campaign description",
	}
	return b
}

func CreateBatchChange(t *testing.T, ctx context.Context, store CreateCampaigner, name string, userID int32, spec int64) *batches.BatchChange {
	t.Helper()

	b := BuildBatchChange(store, name, userID, spec)

	if err := store.CreateCampaign(ctx, b); err != nil {
		t.Fatal(err)
	}

	return b
}
