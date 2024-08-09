package search

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive/service"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive/store"

	"github.com/sourcegraph/sourcegraph/internal/object"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// newExhaustiveSearchRepoRevisionWorker creates a background routine that periodically runs the exhaustive search of a revision on a repo.
func newExhaustiveSearchRepoRevisionWorker(
	ctx context.Context,
	observationCtx *observation.Context,
	workerStore dbworkerstore.Store[*types.ExhaustiveSearchRepoRevisionJob],
	exhaustiveSearchStore *store.Store,
	newSearcher service.NewSearcher,
	uploadStore object.Storage,
	config config,
) goroutine.BackgroundRoutine {
	handler := &exhaustiveSearchRepoRevHandler{
		logger:      log.Scoped("exhaustive-search-repo-revision"),
		store:       exhaustiveSearchStore,
		newSearcher: newSearcher,
		uploadStore: uploadStore,
	}

	opts := workerutil.WorkerOptions{
		Name:              "exhaustive_search_repo_revision_worker",
		Description:       "runs the exhaustive search on a revision of a repository",
		NumHandlers:       5,
		Interval:          config.WorkerInterval,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           workerutil.NewMetrics(observationCtx, "exhaustive_search_repo_revision_worker"),
	}

	return dbworker.NewWorker[*types.ExhaustiveSearchRepoRevisionJob](ctx, workerStore, handler, opts)
}

type exhaustiveSearchRepoRevHandler struct {
	logger      log.Logger
	store       *store.Store
	newSearcher service.NewSearcher
	uploadStore object.Storage
}

var _ workerutil.Handler[*types.ExhaustiveSearchRepoRevisionJob] = &exhaustiveSearchRepoRevHandler{}

func (h *exhaustiveSearchRepoRevHandler) Handle(ctx context.Context, logger log.Logger, record *types.ExhaustiveSearchRepoRevisionJob) error {
	jobID, query, repoRev, initiatorID, err := h.store.GetQueryRepoRev(ctx, record)
	if err != nil {
		return err
	}

	ctx = actor.WithActor(ctx, actor.FromUser(initiatorID))

	q, err := h.newSearcher.NewSearch(ctx, initiatorID, query)
	if err != nil {
		return err
	}

	w, err := service.NewJSONWriter(ctx, h.uploadStore, fmt.Sprintf("%d-%d", jobID, record.ID))
	if err != nil {
		return err
	}

	err = q.Search(ctx, repoRev, w)
	if closeErr := w.Flush(); closeErr != nil {
		err = errors.Append(err, closeErr)
	}

	return err
}

func newExhaustiveSearchRepoRevisionWorkerResetter(
	ctx context.Context,
	observationCtx *observation.Context,
	workerStore dbworkerstore.Store[*types.ExhaustiveSearchRepoRevisionJob],
) *dbworker.Resetter[*types.ExhaustiveSearchRepoRevisionJob] {
	options := dbworker.ResetterOptions{
		Name:     "exhaustive_search_repo_revision_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  dbworker.NewResetterMetrics(observationCtx, "exhaustive_search_repo_revision_worker"),
	}

	resetter := dbworker.NewResetter(ctx, observationCtx.Logger, workerStore, options)
	return resetter
}
