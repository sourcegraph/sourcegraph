package resolvers

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestChangesetResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID

	now := timeutil.Now()
	clock := func() time.Time { return now }
	cstore := store.NewWithClock(db, clock)
	esStore := database.ExternalServicesWith(cstore)
	repoStore := database.ReposWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/changeset-resolver-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	// We use the same mocks for both changesets, even though the unpublished
	// changesets doesn't have a HeadRev (since no commit has been made). The
	// PreviewRepositoryComparison uses a subset of the mocks, though.
	baseRev := "53339e93a17b7934abf3bc4aae3565c15a0631a9"
	headRev := "fa9e174e4847e5f551b31629542377395d6fc95a"
	// These are needed for preview repository comparisons.
	mockBackendCommits(t, api.CommitID(baseRev))
	mockRepoComparison(t, baseRev, headRev, testDiff)

	unpublishedSpec := ct.CreateChangesetSpec(t, ctx, cstore, ct.TestSpecOpts{
		User:          userID,
		Repo:          repo.ID,
		HeadRef:       "refs/heads/my-new-branch",
		Published:     false,
		Title:         "ChangesetSpec Title",
		Body:          "ChangesetSpec Body",
		CommitMessage: "The commit message",
		CommitDiff:    testDiff,
		BaseRev:       baseRev,
		BaseRef:       "refs/heads/master",
	})
	unpublishedChangeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		CurrentSpec:         unpublishedSpec.ID,
		ExternalServiceType: "github",
		PublicationState:    batches.ChangesetPublicationStateUnpublished,
		ReconcilerState:     batches.ReconcilerStateCompleted,
	})
	erroredSpec := ct.CreateChangesetSpec(t, ctx, cstore, ct.TestSpecOpts{
		User:          userID,
		Repo:          repo.ID,
		HeadRef:       "refs/heads/my-failing-branch",
		Published:     true,
		Title:         "ChangesetSpec Title",
		Body:          "ChangesetSpec Body",
		CommitMessage: "The commit message",
		CommitDiff:    testDiff,
		BaseRev:       baseRev,
		BaseRef:       "refs/heads/master",
	})
	erroredChangeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		CurrentSpec:         erroredSpec.ID,
		ExternalServiceType: "github",
		PublicationState:    batches.ChangesetPublicationStateUnpublished,
		ReconcilerState:     batches.ReconcilerStateErrored,
		FailureMessage:      "very bad error",
	})

	labelEventDescriptionText := "the best label in town"

	syncedGitHubChangeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		ExternalServiceType: "github",
		ExternalID:          "12345",
		ExternalBranch:      "open-pr",
		ExternalState:       batches.ChangesetExternalStateOpen,
		ExternalCheckState:  batches.ChangesetCheckStatePending,
		ExternalReviewState: batches.ChangesetReviewStateChangesRequested,
		PublicationState:    batches.ChangesetPublicationStatePublished,
		ReconcilerState:     batches.ReconcilerStateCompleted,
		Metadata: &github.PullRequest{
			ID:          "12345",
			Title:       "GitHub PR Title",
			Body:        "GitHub PR Body",
			Number:      12345,
			State:       "OPEN",
			URL:         "https://github.com/sourcegraph/sourcegraph/pull/12345",
			HeadRefName: "open-pr",
			HeadRefOid:  headRev,
			BaseRefOid:  baseRev,
			BaseRefName: "master",
			TimelineItems: []github.TimelineItem{
				{Type: "PullRequestCommit", Item: &github.PullRequestCommit{
					Commit: github.Commit{
						OID:           "d34db33f",
						PushedDate:    now,
						CommittedDate: now,
					},
				}},
				{Type: "LabeledEvent", Item: &github.LabelEvent{
					CreatedAt: now.Add(5 * time.Second),
					Label: github.Label{
						ID:          "label-event",
						Name:        "cool-label",
						Color:       "blue",
						Description: labelEventDescriptionText,
					},
				}},
			},
			Labels: struct{ Nodes []github.Label }{
				Nodes: []github.Label{
					{ID: "label-no-description", Name: "no-description", Color: "121212"},
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	})
	events := syncedGitHubChangeset.Events()
	if err := cstore.UpsertChangesetEvents(ctx, events...); err != nil {
		t.Fatal(err)
	}

	unsyncedChangeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		ExternalServiceType: "github",
		ExternalID:          "9876",
		PublicationState:    batches.ChangesetPublicationStateUnpublished,
		ReconcilerState:     batches.ReconcilerStateQueued,
	})

	spec := &batches.BatchSpec{
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := cstore.CreateBatchSpec(ctx, spec); err != nil {
		t.Fatal(err)
	}

	batchChange := &batches.BatchChange{
		Name:             "my-unique-name",
		NamespaceUserID:  userID,
		InitialApplierID: userID,
		BatchSpecID:      spec.ID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
	}
	if err := cstore.CreateBatchChange(ctx, batchChange); err != nil {
		t.Fatal(err)
	}
	// Associate the changeset with a batch change, so it's considered in syncer logic.
	addChangeset(t, ctx, cstore, syncedGitHubChangeset, batchChange.ID)

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		changeset *batches.Changeset
		want      apitest.Changeset
	}{
		{
			name:      "unpublished changeset",
			changeset: unpublishedChangeset,
			want: apitest.Changeset{
				Typename:   "ExternalChangeset",
				Title:      unpublishedSpec.Spec.Title,
				Body:       unpublishedSpec.Spec.Body,
				Repository: apitest.Repository{Name: string(repo.Name)},
				// Not scheduled for sync, because it's not published.
				NextSyncAt: "",
				Labels:     []apitest.Label{},
				Diff: apitest.Comparison{
					Typename:  "PreviewRepositoryComparison",
					FileDiffs: testDiffGraphQL,
				},
				State:       string(batches.ChangesetStateUnpublished),
				CurrentSpec: apitest.ChangesetSpec{ID: string(marshalChangesetSpecRandID(unpublishedSpec.RandID))},
			},
		},
		{
			name:      "errored changeset",
			changeset: erroredChangeset,
			want: apitest.Changeset{
				Typename:   "ExternalChangeset",
				Title:      erroredSpec.Spec.Title,
				Body:       erroredSpec.Spec.Body,
				Repository: apitest.Repository{Name: string(repo.Name)},
				// Not scheduled for sync, because it's not published.
				NextSyncAt: "",
				Labels:     []apitest.Label{},
				Diff: apitest.Comparison{
					Typename:  "PreviewRepositoryComparison",
					FileDiffs: testDiffGraphQL,
				},
				State:       string(batches.ChangesetStateRetrying),
				Error:       "very bad error",
				CurrentSpec: apitest.ChangesetSpec{ID: string(marshalChangesetSpecRandID(erroredSpec.RandID))},
			},
		},
		{
			name:      "synced github changeset",
			changeset: syncedGitHubChangeset,
			want: apitest.Changeset{
				Typename:    "ExternalChangeset",
				Title:       "GitHub PR Title",
				Body:        "GitHub PR Body",
				ExternalID:  "12345",
				CheckState:  "PENDING",
				ReviewState: "CHANGES_REQUESTED",
				NextSyncAt:  marshalDateTime(t, now.Add(8*time.Hour)),
				Repository:  apitest.Repository{Name: string(repo.Name)},
				ExternalURL: apitest.ExternalURL{
					URL:         "https://github.com/sourcegraph/sourcegraph/pull/12345",
					ServiceKind: "GITHUB",
					ServiceType: "github",
				},
				State: string(batches.ChangesetStateOpen),
				Events: apitest.ChangesetEventConnection{
					TotalCount: 2,
				},
				Labels: []apitest.Label{
					{Text: "cool-label", Color: "blue", Description: &labelEventDescriptionText},
					{Text: "no-description", Color: "121212", Description: nil},
				},
				Diff: apitest.Comparison{
					Typename:  "RepositoryComparison",
					FileDiffs: testDiffGraphQL,
				},
			},
		},
		{
			name:      "unsynced changeset",
			changeset: unsyncedChangeset,
			want: apitest.Changeset{
				Typename:   "ExternalChangeset",
				ExternalID: "9876",
				Repository: apitest.Repository{Name: string(repo.Name)},
				Labels:     []apitest.Label{},
				State:      string(batches.ChangesetStateProcessing),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			apiID := marshalChangesetID(tc.changeset.ID)
			input := map[string]interface{}{"changeset": apiID}

			var response struct{ Node apitest.Changeset }
			apitest.MustExec(ctx, t, s, input, &response, queryChangeset)

			tc.want.ID = string(apiID)
			if diff := cmp.Diff(tc.want, response.Node); diff != "" {
				t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
			}
		})
	}
}

const queryChangeset = `
fragment fileDiffNode on FileDiff {
    oldPath
    newPath
    oldFile { name }
    hunks {
      body
      oldRange { startLine, lines }
      newRange { startLine, lines }
    }
    stat { added, changed, deleted }
}

query($changeset: ID!) {
  node(id: $changeset) {
    __typename

    ... on ExternalChangeset {
      id

      title
      body

      externalID
      state
      reviewState
      checkState
      externalURL { url, serviceKind, serviceType }
      nextSyncAt
      error

      repository { name }

      events(first: 100) { totalCount }
      labels { text, color, description }

      currentSpec { id }

      diff {
        __typename

        ... on RepositoryComparison {
          fileDiffs {
             totalCount
             rawDiff
             diffStat { added, changed, deleted }
             nodes {
               ... fileDiffNode
             }
          }
        }

        ... on PreviewRepositoryComparison {
          fileDiffs {
             totalCount
             rawDiff
             diffStat { added, changed, deleted }
             nodes {
               ... fileDiffNode
             }
          }
        }
      }
    }
  }
}
`
