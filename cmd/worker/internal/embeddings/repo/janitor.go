package repo

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	repoembeddingsbg "github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

type repoEmbeddingJanitorJob struct{}

func NewRepoEmbeddingJanitorJob() job.Job {
	return &repoEmbeddingJanitorJob{}
}

func (j *repoEmbeddingJanitorJob) Description() string {
	return ""
}

func (j *repoEmbeddingJanitorJob) Config() []env.Config {
	return []env.Config{}
}

func (j *repoEmbeddingJanitorJob) Routines(ctx context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return nil, err
	}
	store := repoembeddingsbg.NewRepoEmbeddingJobWorkerStore(observationCtx, db.Handle())
	return []goroutine.BackgroundRoutine{newRepoEmbeddingJobResetter(ctx, observationCtx, store)}, nil
}

func newRepoEmbeddingJobResetter(ctx context.Context, observationCtx *observation.Context, workerStore dbworkerstore.Store[*repoembeddingsbg.RepoEmbeddingJob]) *dbworker.Resetter[*repoembeddingsbg.RepoEmbeddingJob] {
	return dbworker.NewResetter(ctx, observationCtx.Logger, workerStore, dbworker.ResetterOptions{
		Name:     "repo_embedding_job_worker_resetter",
		Interval: time.Minute, // Check for orphaned jobs every minute
		Metrics:  dbworker.NewResetterMetrics(observationCtx, "repo_embedding_job_worker"),
	})
}
