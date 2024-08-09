package search

import (
	"context"
	"sync"
	"time"

	workerjob "github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/object"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive/service"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive/store"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

// config stores shared config we can override in each worker. We don't expose
// it as an env.Config since we currently only use it for testing.
type config struct {
	// WorkerInterval sets WorkerOptions.Interval for every worker
	WorkerInterval time.Duration
}

type searchJob struct {
	config config

	// workerDB if non-nil is used instead of calling workerdb.InitDB. Used
	// for testing
	workerDB database.DB

	once         sync.Once
	err          error
	workerStores []interface {
		CountByState(_ context.Context, bitset dbworkerstore.RecordState) (int, error)
	}
	workers []goroutine.BackgroundRoutine
}

func NewSearchJob() workerjob.Job {
	return &searchJob{
		config: config{
			WorkerInterval: 1 * time.Second,
		},
	}
}

func (j *searchJob) Description() string {
	return ""
}

func (j *searchJob) Config() []env.Config {
	return []env.Config{search.ObjectStorageConfigInst}
}

func (j *searchJob) Routines(_ context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	if !exhaustive.IsEnabled(conf.Get()) {
		return nil, nil
	}
	workCtx := actor.WithInternalActor(context.Background())

	uploadStore, err := search.NewObjectStorage(workCtx, observationCtx, search.ObjectStorageConfigInst)
	if err != nil {
		j.err = err
		return nil, err
	}

	newSearcherFactory := func(observationCtx *observation.Context, db database.DB) service.NewSearcher {
		searchClient := client.New(observationCtx.Logger, db, gitserver.NewClient("searchjobs.search"))
		return service.FromSearchClient(searchClient)
	}

	return j.newSearchJobRoutines(workCtx, observationCtx, uploadStore, newSearcherFactory)
}

func (j *searchJob) newSearchJobRoutines(
	workCtx context.Context,
	observationCtx *observation.Context,
	uploadStore object.Storage,
	newSearcherFactory func(*observation.Context, database.DB) service.NewSearcher,
) ([]goroutine.BackgroundRoutine, error) {
	j.once.Do(func() {
		db := j.workerDB
		if db == nil {
			db, j.err = workerdb.InitDB(observationCtx)
			if j.err != nil {
				return
			}
		}

		newSearcher := newSearcherFactory(observationCtx, db)

		exhaustiveSearchStore := store.New(db, observationCtx)

		searchWorkerStore := store.NewExhaustiveSearchJobWorkerStore(observationCtx, db.Handle())
		repoWorkerStore := store.NewRepoSearchJobWorkerStore(observationCtx, db.Handle())
		revWorkerStore := store.NewRevSearchJobWorkerStore(observationCtx, db.Handle())

		svc := service.New(observationCtx, exhaustiveSearchStore, uploadStore, newSearcher)

		j.workerStores = append(j.workerStores,
			searchWorkerStore,
			repoWorkerStore,
			revWorkerStore,
		)

		observationCtx = observation.ContextWithLogger(
			observationCtx.Logger.Scoped("routines"),
			observationCtx,
		)

		j.workers = []goroutine.BackgroundRoutine{
			newExhaustiveSearchWorker(workCtx, observationCtx, searchWorkerStore, exhaustiveSearchStore, newSearcher, j.config),
			newExhaustiveSearchRepoWorker(workCtx, observationCtx, repoWorkerStore, exhaustiveSearchStore, newSearcher, j.config),
			newExhaustiveSearchRepoRevisionWorker(workCtx, observationCtx, revWorkerStore, exhaustiveSearchStore, newSearcher, uploadStore, j.config),

			// resetters
			newExhaustiveSearchWorkerResetter(workCtx, observationCtx, searchWorkerStore),
			newExhaustiveSearchRepoWorkerResetter(workCtx, observationCtx, repoWorkerStore),
			newExhaustiveSearchRepoRevisionWorkerResetter(workCtx, observationCtx, revWorkerStore),

			newJanitorJob(observationCtx, db, svc),
		}
	})

	return j.workers, j.err
}

// hasWork returns true if any of the workers have work in its queue or is
// processing something. This is only exposed for tests.
func (j *searchJob) hasWork(ctx context.Context) bool {
	statesBitset := dbworkerstore.StateQueued | dbworkerstore.StateErrored | dbworkerstore.StateProcessing
	for _, w := range j.workerStores {
		if count, _ := w.CountByState(ctx, statesBitset); count > 0 {
			return true
		}
	}
	return false
}
