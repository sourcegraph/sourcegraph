package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

func TestReconcilerProcess_IntegrationTest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	store := store.New(db)

	admin := ct.CreateTestUser(t, db, true)

	rs, extSvc := ct.CreateTestRepos(t, ctx, db, 1)

	state := ct.MockChangesetSyncState(&protocol.RepoInfo{
		Name: rs[0].Name,
		VCS:  protocol.VCSInfo{URL: rs[0].URI},
	})
	defer state.Unmock()

	internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
	defer func() { internalClient = api.InternalClient }()

	githubPR := buildGithubPR(time.Now(), batches.ChangesetExternalStateOpen)
	githubHeadRef := git.EnsureRefPrefix(githubPR.HeadRefName)

	type testCase struct {
		changeset    ct.TestChangesetOpts
		currentSpec  *ct.TestSpecOpts
		previousSpec *ct.TestSpecOpts

		wantChangeset ct.ChangesetAssertions
	}

	tests := map[string]testCase{
		"update a published changeset": {
			currentSpec: &ct.TestSpecOpts{
				HeadRef:   "refs/heads/head-ref-on-github",
				Published: true,
			},

			previousSpec: &ct.TestSpecOpts{
				HeadRef:   "refs/heads/head-ref-on-github",
				Published: true,

				Title:         "old title",
				Body:          "old body",
				CommitDiff:    "old diff",
				CommitMessage: "old message",
			},

			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       "12345",
				ExternalBranch:   "head-ref-on-github",
			},

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateOpen,
				DiffStat:         state.DiffStat,
				// We update the title/body but want the title/body returned by the code host.
				Title: githubPR.Title,
				Body:  githubPR.Body,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create necessary associations.
			previousBatchSpec := ct.CreateBatchSpec(t, ctx, store, "reconciler-test-batch-change", admin.ID)
			batchSpec := ct.CreateBatchSpec(t, ctx, store, "reconciler-test-batch-change", admin.ID)
			batchChange := ct.CreateBatchChange(t, ctx, store, "reconciler-test-batch-change", admin.ID, batchSpec.ID)

			// Create the specs.
			specOpts := *tc.currentSpec
			specOpts.User = admin.ID
			specOpts.Repo = rs[0].ID
			specOpts.BatchSpec = batchSpec.ID
			changesetSpec := ct.CreateChangesetSpec(t, ctx, store, specOpts)

			previousSpecOpts := *tc.previousSpec
			previousSpecOpts.User = admin.ID
			previousSpecOpts.Repo = rs[0].ID
			previousSpecOpts.BatchSpec = previousBatchSpec.ID
			previousSpec := ct.CreateChangesetSpec(t, ctx, store, previousSpecOpts)

			// Create the changeset with correct associations.
			changesetOpts := tc.changeset
			changesetOpts.Repo = rs[0].ID
			changesetOpts.BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: batchChange.ID}}
			changesetOpts.OwnedByBatchChange = batchChange.ID
			if changesetSpec != nil {
				changesetOpts.CurrentSpec = changesetSpec.ID
			}
			if previousSpec != nil {
				changesetOpts.PreviousSpec = previousSpec.ID
			}
			changeset := ct.CreateChangeset(t, ctx, store, changesetOpts)

			// Setup gitserver dependency.
			gitClient := &ct.FakeGitserverClient{ResponseErr: nil}
			if changesetSpec != nil {
				gitClient.Response = changesetSpec.Spec.HeadRef
			}

			// Setup the sourcer that's used to create a Source with which
			// to create/update a changeset.
			fakeSource := &ct.FakeChangesetSource{Svc: extSvc, FakeMetadata: githubPR}
			if changesetSpec != nil {
				fakeSource.WantHeadRef = changesetSpec.Spec.HeadRef
				fakeSource.WantBaseRef = changesetSpec.Spec.BaseRef
			}

			sourcer := repos.NewFakeSourcer(nil, fakeSource)

			// Run the reconciler
			rec := Reconciler{
				noSleepBeforeSync: true,
				GitserverClient:   gitClient,
				Sourcer:           sourcer,
				Store:             store,
			}
			err := rec.process(ctx, store, changeset)
			if err != nil {
				t.Fatalf("reconciler process failed: %s", err)
			}

			// Assert that the changeset in the database looks like we want
			assertions := tc.wantChangeset
			assertions.Repo = rs[0].ID
			assertions.OwnedByBatchChange = changesetOpts.OwnedByBatchChange
			assertions.AttachedTo = []int64{batchChange.ID}
			assertions.CurrentSpec = changesetSpec.ID
			assertions.PreviousSpec = previousSpec.ID
			ct.ReloadAndAssertChangeset(t, ctx, store, changeset, assertions)
		})

		// Clean up database.
		ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
	}
}
