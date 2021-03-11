package background

import (
	"context"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/repos"
)

func Routines(ctx context.Context, batchesStore *store.Store, cf *httpcli.Factory) []goroutine.BackgroundRoutine {
	sourcer := repos.NewSourcer(cf)

	metrics := newMetrics()

	routines := []goroutine.BackgroundRoutine{
		newWorker(ctx, batchesStore, gitserver.DefaultClient, sourcer, metrics),
		newWorkerResetter(batchesStore, metrics),
		newSpecExpireWorker(ctx, batchesStore),
	}
	return routines
}
