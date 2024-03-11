package commitgraph

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/database/locker"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewCommitGraphUpdater(
	store store.Store,
	gitserverClient gitserver.Client,
	config *Config,
) goroutine.BackgroundRoutine {
	updater := &commitGraphUpdater{
		store:           store,
		locker:          locker.NewWith(store.Handle(), "codeintel"),
		gitserverClient: gitserverClient,
	}

	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(context.Background()),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			return updater.UpdateAllDirtyCommitGraphs(ctx, config.MaxAgeForNonStaleBranches, config.MaxAgeForNonStaleTags)
		}),
		goroutine.WithName("codeintel.commitgraph-updater"),
		goroutine.WithDescription("updates the visibility commit graph for dirty repos"),
		goroutine.WithInterval(config.Interval),
	)
}

type commitGraphUpdater struct {
	store           store.Store
	locker          Locker
	gitserverClient gitserver.Client
}

// Handle periodically re-calculates the commit and upload visibility graph for repositories
// that are marked as dirty by the worker process. This is done out-of-band from the rest of
// the upload processing as it is likely that we are processing multiple uploads concurrently
// for the same repository and should not repeat the work since the last calculation performed
// will always be the one we want.
func (s *commitGraphUpdater) UpdateAllDirtyCommitGraphs(ctx context.Context, maxAgeForNonStaleBranches time.Duration, maxAgeForNonStaleTags time.Duration) (err error) {
	dirtyRepositories, err := s.store.GetDirtyRepositories(ctx)
	if err != nil {
		return errors.Wrap(err, "uploadSvc.DirtyRepositories")
	}

	var updateErr error
	for _, dirtyRepository := range dirtyRepositories {
		if err := s.lockAndUpdateUploadsVisibleToCommits(
			ctx,
			dirtyRepository.RepositoryID,
			dirtyRepository.RepositoryName,
			dirtyRepository.DirtyToken,
			maxAgeForNonStaleBranches,
			maxAgeForNonStaleTags,
		); err != nil {
			if updateErr == nil {
				updateErr = err
			} else {
				updateErr = errors.Append(updateErr, err)
			}
		}
	}

	return updateErr
}

// lockAndUpdateUploadsVisibleToCommits will call UpdateUploadsVisibleToCommits while holding an advisory lock to give exclusive access to the
// update procedure for this repository. If the lock is already held, this method will simply do nothing.
func (s *commitGraphUpdater) lockAndUpdateUploadsVisibleToCommits(ctx context.Context, repositoryID int, repositoryName string, dirtyToken int, maxAgeForNonStaleBranches time.Duration, maxAgeForNonStaleTags time.Duration) (err error) {
	ok, unlock, err := s.locker.Lock(ctx, int32(repositoryID), false)
	if err != nil || !ok {
		return errors.Wrap(err, "locker.Lock")
	}
	defer func() {
		err = unlock(err)
	}()

	repo := api.RepoName(repositoryName)

	// The following process pulls the commit graph for the given repository from gitserver, pulls the set of LSIF
	// upload objects for the given repository from Postgres, and correlates them into a visibility
	// graph. This graph is then upserted back into Postgres for use by find closest dumps queries.
	//
	// The user should supply a dirty token that is associated with the given repository so that
	// the repository can be unmarked as long as the repository is not marked as dirty again before
	// the update completes.

	// Construct a view of the git graph that we will later decorate with upload information.
	commitGraph, err := s.getCommitGraph(ctx, repositoryID, repo)
	if err != nil {
		return err
	}

	refs, err := s.gitserverClient.ListRefs(ctx, repo, gitserver.ListRefsOpts{HeadsOnly: true, TagsOnly: true})
	if err != nil {
		return errors.Wrap(err, "gitserver.ListRefs")
	}

	// Decorate the commit graph with the set of processed uploads are visible from each commit,
	// then bulk update the denormalized view in Postgres. We call this with an empty graph as well
	// so that we end up clearing the stale data and bulk inserting nothing.
	if err := s.store.UpdateUploadsVisibleToCommits(ctx, repositoryID, commitGraph, mapRefsToCommits(refs), maxAgeForNonStaleBranches, maxAgeForNonStaleTags, dirtyToken, time.Time{}); err != nil {
		return errors.Wrap(err, "uploadSvc.UpdateUploadsVisibleToCommits")
	}

	return nil
}

// mapRefsToCommits indexes a set of refs by commit ID.
func mapRefsToCommits(refs []gitdomain.Ref) map[string][]gitdomain.Ref {
	commitsByRef := make(map[string][]gitdomain.Ref, len(refs))
	for _, ref := range refs {
		commitsByRef[string(ref.CommitID)] = append(commitsByRef[string(ref.CommitID)], ref)
	}
	return commitsByRef
}

// getCommitGraph builds a partial commit graph that includes the most recent commits on each branch
// extending back as as the date of the oldest commit for which we have a processed upload for this
// repository.
//
// This optimization is necessary as decorating the commit graph is an operation that scales with
// the size of both the git graph and the number of uploads (multiplicatively). For repositories with
// a very large number of commits or distinct roots (most monorepos) this is a necessary optimization.
//
// The number of commits pulled back here should not grow over time unless the repo is growing at an
// accelerating rate, as we routinely expire old information for active repositories in a janitor
// process.
func (s *commitGraphUpdater) getCommitGraph(ctx context.Context, repositoryID int, repo api.RepoName) (*gitdomain.CommitGraph, error) {
	commitDate, ok, err := s.store.GetOldestCommitDate(ctx, repositoryID)
	if err != nil {
		return nil, err
	}
	if !ok {
		// No uploads exist for this repository
		return gitdomain.ParseCommitGraph(nil), nil
	}

	// The --since flag for git log is exclusive, but we want to include the commit where the
	// oldest dump is defined. This flag only has second resolution, so we shouldn't be pulling
	// back any more data than we wanted.
	commitDate = commitDate.Add(-time.Second)

	commitGraph, err := s.gitserverClient.CommitGraph(ctx, repo, gitserver.CommitGraphOptions{
		AllRefs: true,
		Since:   &commitDate,
	})
	if err != nil {
		return nil, errors.Wrap(err, "gitserver.CommitGraph")
	}

	return commitGraph, nil
}
