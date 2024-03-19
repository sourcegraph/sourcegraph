package backfiller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	shared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
)

func TestBackfillCommittedAtBatch(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()
	gitserverClient := gitserver.NewMockClient()
	svc := &backfiller{
		store:           store,
		gitserverClient: gitserverClient,
	}

	// Return self for txn
	store.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(s shared.Store) error) error { return f(store) })

	n := 50
	t0 := time.Unix(1587396557, 0).UTC()
	expectedCommitDates := make(map[string]time.Time, n)
	for i := range n {
		expectedCommitDates[fmt.Sprintf("%040d", i)] = t0.Add(time.Second * time.Duration(i))
	}

	gitserverClient.GetCommitFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, commitID api.CommitID) (*gitdomain.Commit, error) {
		commitDate, ok := expectedCommitDates[string(commitID)]
		if !ok {
			return nil, &gitdomain.RevisionNotFoundError{Repo: repo, Spec: string(commitID)}
		}
		return &gitdomain.Commit{
			ID: commitID,
			Committer: &gitdomain.Signature{
				Date: commitDate,
			},
		}, nil
	})

	pageSize := 50
	for i := 0; i < n; i += pageSize {
		commitsByRepo := map[int][]string{}
		for j := range pageSize {
			repositoryID := 42 + (i+j)/(n/2) // 50% id=42, 50% id=43
			commitsByRepo[repositoryID] = append(commitsByRepo[repositoryID], fmt.Sprintf("%040d", i+j))
		}

		sourcedCommits := []shared.SourcedCommits{}
		for repositoryID, commits := range commitsByRepo {
			sourcedCommits = append(sourcedCommits, shared.SourcedCommits{
				RepositoryID: repositoryID,
				Commits:      commits,
			})
		}

		store.SourcedCommitsWithoutCommittedAtFunc.PushReturn(sourcedCommits, nil)
	}

	for range n / pageSize {
		if err := svc.BackfillCommittedAtBatch(ctx, pageSize); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	committedAtByCommit := map[string]time.Time{}
	history := store.UpdateCommittedAtFunc.history

	for i := range n {
		if len(history) <= i {
			t.Fatalf("not enough calls to UpdateCommittedAtFunc")
		}

		call := history[i]
		commit := call.Arg2
		rawCommittedAt := call.Arg3

		committedAt, err := time.Parse(time.RFC3339, rawCommittedAt)
		if err != nil {
			t.Fatalf("unexpected non-time %q: %s", rawCommittedAt, err)
		}

		committedAtByCommit[commit] = committedAt
	}

	if diff := cmp.Diff(committedAtByCommit, expectedCommitDates); diff != "" {
		t.Errorf("unexpected commit dates (-want +got):\n%s", diff)
	}
}

func TestBackfillCommittedAtBatchUnknownCommits(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()
	gitserverClient := gitserver.NewMockClient()
	svc := &backfiller{
		store:           store,
		gitserverClient: gitserverClient,
	}

	// Return self for txn
	store.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(s shared.Store) error) error { return f(store) })

	n := 50
	t0 := time.Unix(1587396557, 0).UTC()
	expectedCommitDates := make(map[string]time.Time, n)
	for i := range n {
		if i%3 == 0 {
			// Unknown commits
			continue
		}

		expectedCommitDates[fmt.Sprintf("%040d", i)] = t0.Add(time.Second * time.Duration(i))
	}

	gitserverClient.GetCommitFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, commitID api.CommitID) (*gitdomain.Commit, error) {
		commitDate, ok := expectedCommitDates[string(commitID)]
		if !ok {
			return nil, &gitdomain.RevisionNotFoundError{Repo: repo, Spec: string(commitID)}
		}
		return &gitdomain.Commit{
			ID: commitID,
			Committer: &gitdomain.Signature{
				Date: commitDate,
			},
		}, nil
	})

	pageSize := 50
	for i := 0; i < n; i += pageSize {
		commitsByRepo := map[int][]string{}
		for j := range pageSize {
			repositoryID := 42 + (i+j)/(n/2) // 50% id=42, 50% id=43
			commitsByRepo[repositoryID] = append(commitsByRepo[repositoryID], fmt.Sprintf("%040d", i+j))
		}

		sourcedCommits := []shared.SourcedCommits{}
		for repositoryID, commits := range commitsByRepo {
			sourcedCommits = append(sourcedCommits, shared.SourcedCommits{
				RepositoryID: repositoryID,
				Commits:      commits,
			})
		}

		store.SourcedCommitsWithoutCommittedAtFunc.PushReturn(sourcedCommits, nil)
	}

	for range n / pageSize {
		if err := svc.BackfillCommittedAtBatch(ctx, pageSize); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	committedAtByCommit := map[string]time.Time{}
	history := store.UpdateCommittedAtFunc.history

	for i := range n {
		if len(history) <= i {
			t.Fatalf("not enough calls to UpdateCommittedAtFunc")
		}

		call := history[i]
		commit := call.Arg2
		rawCommittedAt := call.Arg3

		if rawCommittedAt == "-infinity" {
			// Unknown commits
			continue
		}

		committedAt, err := time.Parse(time.RFC3339, rawCommittedAt)
		if err != nil {
			t.Fatalf("unexpected non-time %q: %s", rawCommittedAt, err)
		}

		committedAtByCommit[commit] = committedAt
	}

	if diff := cmp.Diff(committedAtByCommit, expectedCommitDates); diff != "" {
		t.Errorf("unexpected commit dates (-want +got):\n%s", diff)
	}
}
