package background

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
)

func newSpecExpireWorker(ctx context.Context, cstore *store.Store) goroutine.BackgroundRoutine {
	expireSpecs := goroutine.NewHandlerWithErrorMessage("expire batch changes specs", func(ctx context.Context) error {
		// We first need to delete expired ChangesetSpecs...
		if err := cstore.DeleteExpiredChangesetSpecs(ctx); err != nil {
			return errors.Wrap(err, "DeleteExpiredChangesetSpecs")
		}
		// ... and then the BatchSpecs, due to the batch_spec_id
		// foreign key on changeset_specs.
		if err := cstore.DeleteExpiredBatchSpecs(ctx); err != nil {
			return errors.Wrap(err, "DeleteExpiredBatchSpecs")
		}
		return nil
	})
	return goroutine.NewPeriodicGoroutine(ctx, 2*time.Minute, expireSpecs)
}
