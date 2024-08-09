package authz

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

type permsSyncerJob struct{}

func NewPermsSyncerJob() job.Job {
	return &permsSyncerJob{}
}

func (j *permsSyncerJob) Description() string {
	return "Background job that syncs repository permissions from code hosts to the database."
}

func (j *permsSyncerJob) Config() []env.Config {
	return []env.Config{}
}

func (j *permsSyncerJob) Routines(_ context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	workCtx := actor.WithInternalActor(context.Background())

	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return nil, err
	}

	store := repos.NewStore(observationCtx.Logger.Scoped("store"), db)
	{
		m := repos.NewStoreMetrics()
		m.MustRegister(prometheus.DefaultRegisterer)
		store.SetMetrics(m)
	}

	permsSyncer := newPermsSyncer(
		observationCtx.Logger.Scoped("PermsSyncer"),
		db,
		store,
		database.Perms(observationCtx.Logger, db, timeutil.Now),
		timeutil.Now,
	)
	repoWorkerStore := makeStore(observationCtx, db.Handle(), syncTypeRepo)
	userWorkerStore := makeStore(observationCtx, db.Handle(), syncTypeUser)
	permissionSyncJobStore := database.PermissionSyncJobsWith(observationCtx.Logger, db)
	routines := []goroutine.BackgroundRoutine{
		// repoSyncWorker
		makeWorker(workCtx, observationCtx, repoWorkerStore, permsSyncer, syncTypeRepo, permissionSyncJobStore),
		// userSyncWorker
		makeWorker(workCtx, observationCtx, userWorkerStore, permsSyncer, syncTypeUser, permissionSyncJobStore),
		// Type of store (repo/user) for resetter doesn't matter, because it has its
		// separate name for logging and metrics.
		makeResetter(workCtx, observationCtx, repoWorkerStore),
	}

	return routines, nil
}
