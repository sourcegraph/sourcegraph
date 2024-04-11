package embeddings

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

func ScheduleRepositories(
	ctx context.Context,
	repoNames []api.RepoName,
	forceReschedule bool,
	db database.DB,
	repoEmbeddingJobsStore repo.RepoEmbeddingJobsStore,
	gitserverClient gitserver.Client,
) (err error) {
	tx, err := repoEmbeddingJobsStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	repoStore := db.Repos()
	for _, repoName := range repoNames {
		r, err := repoStore.GetByName(ctx, repoName)
		if err != nil {
			return err
		}

		refName, latestRevision, err := gitserverClient.GetDefaultBranch(ctx, r.ID, false)
		if err != nil || refName == "" {
			return err
		}

		if !forceReschedule {
			job, _ := tx.GetLastRepoEmbeddingJobForRevision(ctx, r.ID, latestRevision)
			// We skip creating a repo embedding job for a repo at revision if there already exists
			// an identical job that has been completed, or is scheduled to run (processing or queued).
			if job != nil && job.IsRepoEmbeddingJobScheduledOrCompleted() {
				continue
			}
		}

		_, err = tx.CreateRepoEmbeddingJob(ctx, r.ID, latestRevision)
		if err != nil {
			return err
		}
	}
	return nil
}

func ScheduleRepositoriesForPolicy(
	ctx context.Context,
	repoIDs []api.RepoID,
	db database.DB,
	repoEmbeddingJobsStore repo.RepoEmbeddingJobsStore,
	gitserverClient gitserver.Client,
) (err error) {
	if len(repoIDs) == 0 {
		return nil
	}
	tx, err := repoEmbeddingJobsStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	repoStore := db.Repos()
	repos, err := repoStore.ListMinimalRepos(ctx, database.ReposListOptions{IDs: repoIDs})
	if err != nil {
		return err
	}

	for _, r := range repos {
		refName, latestRevision, err := gitserverClient.GetDefaultBranch(ctx, r.ID, false)
		// enqueue with an empty revision and let handler determine whether job can execute
		if err != nil || refName == "" {
			latestRevision = ""
		}

		// We skip creating a repo embedding job for a repo at revision if we already attempted a job at the same
		// revision. In the case of failed jobs, this helps prevent us from constantly  retrying the job and
		// failing again, which can be very expensive.
		job, _ := tx.GetLastRepoEmbeddingJobForRevision(ctx, r.ID, latestRevision)
		if job != nil {
			continue
		}

		_, err = tx.CreateRepoEmbeddingJob(ctx, r.ID, latestRevision)
		if err != nil {
			return err
		}
	}
	return nil
}
