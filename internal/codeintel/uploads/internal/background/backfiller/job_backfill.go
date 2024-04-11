package backfiller

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewCommittedAtBackfiller(store store.Store, gitserverClient gitserver.Client, config *Config) goroutine.BackgroundRoutine {
	backfiller := &backfiller{
		store:           store,
		gitserverClient: gitserverClient,
		batchSize:       config.BatchSize,
	}
	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(context.Background()),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			return backfiller.BackfillCommittedAtBatch(ctx, config.BatchSize)
		}),
		goroutine.WithName("codeintel.committed-at-backfiller"),
		goroutine.WithDescription("backfills the committed_at column for code-intel uploads"),
		goroutine.WithInterval(config.Interval),
	)
}

type backfiller struct {
	batchSize       int
	store           store.Store
	gitserverClient gitserver.Client
}

// BackfillCommittedAtBatch calculates the committed_at value for a batch of upload records that do not have
// this value set. This method is used to backfill old upload records prior to this value being reliably set
// during processing.
func (s *backfiller) BackfillCommittedAtBatch(ctx context.Context, batchSize int) (err error) {
	return s.store.WithTransaction(ctx, func(tx store.Store) error {
		batch, err := tx.SourcedCommitsWithoutCommittedAt(ctx, batchSize)
		if err != nil {
			return errors.Wrap(err, "store.SourcedCommitsWithoutCommittedAt")
		}

		for _, sourcedCommits := range batch {
			for _, commit := range sourcedCommits.Commits {
				commitDateString, err := s.getCommitDate(ctx, sourcedCommits.RepositoryID, commit)
				if err != nil {
					return err
				}

				// Update commit date of all uploads attached to this this repository and commit
				if err := tx.UpdateCommittedAt(ctx, sourcedCommits.RepositoryID, commit, commitDateString); err != nil {
					return errors.Wrap(err, "store.UpdateCommittedAt")
				}
			}

			// Mark repository as dirty so the commit graph is recalculated with fresh data
			if err := tx.SetRepositoryAsDirty(ctx, sourcedCommits.RepositoryID); err != nil {
				return errors.Wrap(err, "store.SetRepositoryAsDirty")
			}
		}

		return nil
	})
}

func (s *backfiller) getCommitDate(ctx context.Context, repositoryID int, commitID string) (string, error) {
	commit, err := s.gitserverClient.GetCommit(ctx, api.RepoID(repositoryID), api.CommitID(commitID))
	if err != nil {
		if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			// Set a value here that we'll filter out on the query side so that we don't
			// reprocess the same failing batch infinitely. We could alternatively soft
			// delete the record, but it would be better to keep record deletion behavior
			// together in the same place (so we have unified metrics on that event).
			return "-infinity", nil
		}
		return "", err
	}

	return commit.Committer.Date.Format(time.RFC3339), nil
}
