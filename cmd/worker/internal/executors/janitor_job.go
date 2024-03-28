package executors

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/env"
	executortypes "github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

type janitorJob struct{}

func NewJanitorJob() job.Job {
	return &janitorJob{}
}

func (j *janitorJob) Description() string {
	return ""
}

func (j *janitorJob) Config() []env.Config {
	return []env.Config{janitorConfigInst}
}

func (j *janitorJob) Routines(_ context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return nil, err
	}

	dequeueCache := rcache.New(executortypes.DequeueCachePrefix, false)

	routines := []goroutine.BackgroundRoutine{
		goroutine.NewPeriodicGoroutine(
			context.Background(),
			goroutine.HandlerFunc(func(ctx context.Context) error {
				return db.Executors().DeleteInactiveHeartbeats(ctx, janitorConfigInst.HeartbeatRecordsMaxAge)
			}),
			goroutine.WithName("executor.heartbeat-janitor"),
			goroutine.WithDescription("clean up executor heartbeat records for presumed dead executors"),
			goroutine.WithInterval(janitorConfigInst.CleanupTaskInterval),
		),
		NewMultiqueueCacheCleaner(executortypes.ValidQueueNames, dequeueCache, janitorConfigInst.CacheDequeueTtl, janitorConfigInst.CacheCleanupInterval),
	}

	return routines, nil
}
