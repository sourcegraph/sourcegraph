package batches

import (
	"context"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/background"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/syncer"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

// InitBackgroundJobs starts all jobs required to run batches. Currently, it is called from
// repo-updater and in the future will be the main entry point for the batch changes worker.
func InitBackgroundJobs(
	ctx context.Context,
	db dbutil.DB,
	cf *httpcli.Factory,
) interface {
	// EnqueueChangesetSyncs will queue the supplied changesets to sync ASAP.
	EnqueueChangesetSyncs(ctx context.Context, ids []int64) error
	// HandleExternalServiceSync should be called when an external service changes so that
	// the registry can start or stop the syncer associated with the service
	HandleExternalServiceSync(es api.ExternalService)
} {
	cstore := store.New(db)

	repoStore := cstore.Repos()
	esStore := cstore.ExternalServices()

	// We use an internal actor so that we can freely load dependencies from
	// the database without repository permissions being enforced.
	// We do check for repository permissions conciously in the Rewirer when
	// creating new changesets and in the executor, when talking to the code
	// host, we manually check for BatchChangesCredentials.
	ctx = actor.WithInternalActor(ctx)

	syncRegistry := syncer.NewSyncRegistry(ctx, cstore, repoStore, esStore, cf)

	go goroutine.MonitorBackgroundRoutines(ctx, background.Routines(ctx, cstore, cf)...)

	return syncRegistry
}
