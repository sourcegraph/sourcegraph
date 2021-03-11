package reconciler

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	gitprotocol "github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

func TestExecutor_ExecutePlan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	now := timeutil.Now()
	clock := func() time.Time { return now }
	cstore := store.NewWithClock(db, clock)

	admin := ct.CreateTestUser(t, db, true)

	rs, extSvc := ct.CreateTestRepos(t, ctx, db, 1)

	state := ct.MockChangesetSyncState(&protocol.RepoInfo{
		Name: rs[0].Name,
		VCS:  protocol.VCSInfo{URL: rs[0].URI},
	})
	defer state.Unmock()

	internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
	defer func() { internalClient = api.InternalClient }()

	githubPR := buildGithubPR(clock(), batches.ChangesetExternalStateOpen)
	githubHeadRef := git.EnsureRefPrefix(githubPR.HeadRefName)
	draftGithubPR := buildGithubPR(clock(), batches.ChangesetExternalStateDraft)
	closedGitHubPR := buildGithubPR(clock(), batches.ChangesetExternalStateClosed)

	notFoundErr := repos.ChangesetNotFoundError{
		Changeset: &repos.Changeset{
			Changeset: &batches.Changeset{ExternalID: "100000"},
		},
	}

	type testCase struct {
		changeset      ct.TestChangesetOpts
		hasCurrentSpec bool
		plan           *Plan

		sourcerMetadata interface{}
		sourcerErr      error
		// Whether or not the source responds to CreateChangeset with "already exists"
		alreadyExists bool

		wantCreateOnCodeHost      bool
		wantCreateDraftOnCodeHost bool
		wantUndraftOnCodeHost     bool
		wantUpdateOnCodeHost      bool
		wantCloseOnCodeHost       bool
		wantLoadFromCodeHost      bool
		wantReopenOnCodeHost      bool

		wantGitserverCommit bool

		wantChangeset       ct.ChangesetAssertions
		wantNonRetryableErr bool
	}

	tests := map[string]testCase{
		"noop": {
			hasCurrentSpec: true,
			changeset:      ct.TestChangesetOpts{},
			plan:           &Plan{Ops: Operations{}},

			wantChangeset: ct.ChangesetAssertions{},
		},
		"import": {
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				ExternalID:       githubPR.ID,
			},
			plan: &Plan{
				Ops: Operations{batches.ReconcilerOperationImport},
			},

			wantLoadFromCodeHost: true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateOpen,
				Title:            githubPR.Title,
				Body:             githubPR.Body,
				DiffStat:         state.DiffStat,
			},
		},
		"import and not-found error": {
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				ExternalID:       githubPR.ID,
			},
			plan: &Plan{
				Ops: Operations{batches.ReconcilerOperationImport},
			},
			sourcerErr: notFoundErr,

			wantLoadFromCodeHost: true,

			wantNonRetryableErr: true,
		},
		"push and publish": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},
			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationPush,
					batches.ReconcilerOperationPublish,
				},
			},

			wantCreateOnCodeHost: true,
			wantGitserverCommit:  true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateOpen,
				Title:            githubPR.Title,
				Body:             githubPR.Body,
				DiffStat:         state.DiffStat,
			},
		},
		"retry push and publish": {
			// This test case makes sure that everything works when the code host says
			// that the changeset already exists.
			alreadyExists:  true,
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				// The reconciler resets the failure message before passing the
				// changeset to the executor.
				// We simulate that here by not setting FailureMessage.
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},
			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationPush,
					batches.ReconcilerOperationPublish,
				},
			},

			// We first do a create and since that fails with "already exists"
			// we update.
			wantCreateOnCodeHost: true,
			wantUpdateOnCodeHost: true,
			wantGitserverCommit:  true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateOpen,
				Title:            githubPR.Title,
				Body:             githubPR.Body,
				DiffStat:         state.DiffStat,
			},
		},
		"update": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       "12345",
				ExternalBranch:   "head-ref-on-github",
			},

			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationUpdate,
				},
			},

			// We don't want a new commit, only an update on the code host.
			wantUpdateOnCodeHost: true,

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
		"push sleep sync": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       "12345",
				ExternalBranch:   git.EnsureRefPrefix("head-ref-on-github"),
				ExternalState:    batches.ChangesetExternalStateOpen,
			},

			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationPush,
					batches.ReconcilerOperationSleep,
					batches.ReconcilerOperationSync,
				},
			},

			wantGitserverCommit:  true,
			wantLoadFromCodeHost: true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalState:    batches.ChangesetExternalStateOpen,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				DiffStat:         state.DiffStat,
			},
		},
		"close open changeset": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateOpen,
				Closing:          true,
			},
			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationClose,
				},
			},
			// We return a closed GitHub PR here
			sourcerMetadata: closedGitHubPR,

			wantCloseOnCodeHost: true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				Closing:          false,

				ExternalID:     closedGitHubPR.ID,
				ExternalBranch: git.EnsureRefPrefix(closedGitHubPR.HeadRefName),
				ExternalState:  batches.ChangesetExternalStateClosed,

				Title:    closedGitHubPR.Title,
				Body:     closedGitHubPR.Body,
				DiffStat: state.DiffStat,
			},
		},
		"close closed changeset": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateClosed,
				Closing:          true,
			},
			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationClose,
				},
			},

			// We return a closed GitHub PR here, but since it's a noop, we
			// don't sync and thus don't set its attributes on the changeset.
			sourcerMetadata: closedGitHubPR,

			// Should be a noop
			wantCloseOnCodeHost: false,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,
				Closing:          false,

				ExternalID:     closedGitHubPR.ID,
				ExternalBranch: git.EnsureRefPrefix(closedGitHubPR.HeadRefName),
				ExternalState:  batches.ChangesetExternalStateClosed,
			},
		},
		"reopening closed changeset without updates": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalID:       githubPR.ID,
				ExternalBranch:   githubHeadRef,
				ExternalState:    batches.ChangesetExternalStateClosed,
			},
			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationReopen,
				},
			},

			wantReopenOnCodeHost: true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,

				ExternalID:     githubPR.ID,
				ExternalBranch: githubHeadRef,
				ExternalState:  batches.ChangesetExternalStateOpen,

				Title:    githubPR.Title,
				Body:     githubPR.Body,
				DiffStat: state.DiffStat,
			},
		},
		"push and publishdraft": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},

			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationPush,
					batches.ReconcilerOperationPublishDraft,
				},
			},

			sourcerMetadata: draftGithubPR,

			wantCreateDraftOnCodeHost: true,
			wantGitserverCommit:       true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,

				ExternalID:     draftGithubPR.ID,
				ExternalBranch: git.EnsureRefPrefix(draftGithubPR.HeadRefName),
				ExternalState:  batches.ChangesetExternalStateDraft,

				Title:    draftGithubPR.Title,
				Body:     draftGithubPR.Body,
				DiffStat: state.DiffStat,
			},
		},
		"undraft": {
			hasCurrentSpec: true,
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalState:    batches.ChangesetExternalStateDraft,
			},

			plan: &Plan{
				Ops: Operations{
					batches.ReconcilerOperationUndraft,
				},
			},

			wantUndraftOnCodeHost: true,

			wantChangeset: ct.ChangesetAssertions{
				PublicationState: batches.ChangesetPublicationStatePublished,

				ExternalID:     githubPR.ID,
				ExternalBranch: githubHeadRef,
				ExternalState:  batches.ChangesetExternalStateOpen,

				Title:    githubPR.Title,
				Body:     githubPR.Body,
				DiffStat: state.DiffStat,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create necessary associations.
			batchSpec := ct.CreateBatchSpec(t, ctx, cstore, "executor-test-batch-change", admin.ID)
			batchChange := ct.CreateBatchChange(t, ctx, cstore, "executor-test-batch-change", admin.ID, batchSpec.ID)

			// Create the changesetSpec with associations wired up correctly.
			var changesetSpec *batches.ChangesetSpec
			if tc.hasCurrentSpec {
				// The attributes of the spec don't really matter, but the
				// associations do.
				specOpts := ct.TestSpecOpts{}
				specOpts.User = admin.ID
				specOpts.Repo = rs[0].ID
				specOpts.BatchSpec = batchSpec.ID
				changesetSpec = ct.CreateChangesetSpec(t, ctx, cstore, specOpts)
			}

			// Create the changeset with correct associations.
			changesetOpts := tc.changeset
			changesetOpts.Repo = rs[0].ID
			changesetOpts.BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: batchChange.ID}}
			changesetOpts.OwnedByBatchChange = batchChange.ID
			if changesetSpec != nil {
				changesetOpts.CurrentSpec = changesetSpec.ID
			}
			changeset := ct.CreateChangeset(t, ctx, cstore, changesetOpts)

			// Setup gitserver dependency.
			gitClient := &ct.FakeGitserverClient{ResponseErr: nil}
			if changesetSpec != nil {
				gitClient.Response = changesetSpec.Spec.HeadRef
			}

			// Setup the sourcer that's used to create a Source with which
			// to create/update a changeset.
			fakeSource := &ct.FakeChangesetSource{
				Svc:             extSvc,
				Err:             tc.sourcerErr,
				ChangesetExists: tc.alreadyExists,
			}

			if tc.sourcerMetadata != nil {
				fakeSource.FakeMetadata = tc.sourcerMetadata
			} else {
				fakeSource.FakeMetadata = githubPR
			}
			if changesetSpec != nil {
				fakeSource.WantHeadRef = changesetSpec.Spec.HeadRef
				fakeSource.WantBaseRef = changesetSpec.Spec.BaseRef
			}

			sourcer := repos.NewFakeSourcer(nil, fakeSource)

			tc.plan.Changeset = changeset
			tc.plan.ChangesetSpec = changesetSpec

			// Execute the plan
			err := ExecutePlan(
				ctx,
				gitClient,
				sourcer,
				// Don't actually sleep for the sake of testing.
				true,
				cstore,
				tc.plan,
			)
			if err != nil {
				if tc.wantNonRetryableErr && errcode.IsNonRetryable(err) {
					// all good
				} else {
					t.Fatalf("ExecutePlan failed: %s", err)
				}
			}

			// Assert that all the calls happened
			if have, want := gitClient.CreateCommitFromPatchCalled, tc.wantGitserverCommit; have != want {
				t.Fatalf("wrong CreateCommitFromPatch call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.CreateDraftChangesetCalled, tc.wantCreateDraftOnCodeHost; have != want {
				t.Fatalf("wrong CreateDraftChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.UndraftedChangesetsCalled, tc.wantUndraftOnCodeHost; have != want {
				t.Fatalf("wrong UndraftChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.CreateChangesetCalled, tc.wantCreateOnCodeHost; have != want {
				t.Fatalf("wrong CreateChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.UpdateChangesetCalled, tc.wantUpdateOnCodeHost; have != want {
				t.Fatalf("wrong UpdateChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.ReopenChangesetCalled, tc.wantReopenOnCodeHost; have != want {
				t.Fatalf("wrong ReopenChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.LoadChangesetCalled, tc.wantLoadFromCodeHost; have != want {
				t.Fatalf("wrong LoadChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if have, want := fakeSource.CloseChangesetCalled, tc.wantCloseOnCodeHost; have != want {
				t.Fatalf("wrong CloseChangeset call. wantCalled=%t, wasCalled=%t", want, have)
			}

			if tc.wantNonRetryableErr {
				return
			}

			// Assert that the changeset in the database looks like we want
			assertions := tc.wantChangeset
			assertions.Repo = rs[0].ID
			assertions.OwnedByBatchChange = changesetOpts.OwnedByBatchChange
			assertions.AttachedTo = []int64{batchChange.ID}
			if changesetSpec != nil {
				assertions.CurrentSpec = changesetSpec.ID
			}
			ct.ReloadAndAssertChangeset(t, ctx, cstore, changeset, assertions)

			// Assert that the body included a backlink if needed. We'll do
			// more detailed unit tests of decorateChangesetBody elsewhere;
			// we're just looking for a basic marker here that _something_
			// happened.
			var rcs *repos.Changeset
			if tc.wantCreateOnCodeHost && fakeSource.CreateChangesetCalled {
				rcs = fakeSource.CreatedChangesets[0]
			} else if tc.wantUpdateOnCodeHost && fakeSource.UpdateChangesetCalled {
				rcs = fakeSource.UpdatedChangesets[0]
			}

			if rcs != nil {
				if !strings.Contains(rcs.Body, "Created by Sourcegraph batch change") {
					t.Errorf("did not find backlink in body: %q", rcs.Body)
				}
			}
		})

		// After each test: clean up database.
		ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
	}
}

func TestExecutor_ExecutePlan_PublishedChangesetDuplicateBranch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	cstore := store.New(db)

	rs, _ := ct.CreateTestRepos(t, ctx, db, 1)

	commonHeadRef := "refs/heads/collision"

	// Create a published changeset.
	ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:             rs[0].ID,
		PublicationState: batches.ChangesetPublicationStatePublished,
		ExternalBranch:   commonHeadRef,
		ExternalID:       "123",
	})

	// Plan only needs a push operation, since that's where we check
	plan := &Plan{}
	plan.AddOp(batches.ReconcilerOperationPush)

	// Build a changeset that would be pushed on the same HeadRef/ExternalBranch.
	plan.ChangesetSpec = ct.BuildChangesetSpec(t, ct.TestSpecOpts{
		Repo:      rs[0].ID,
		HeadRef:   commonHeadRef,
		Published: true,
	})
	plan.Changeset = ct.BuildChangeset(ct.TestChangesetOpts{Repo: rs[0].ID})

	err := ExecutePlan(ctx, nil, repos.NewFakeSourcer(nil, &ct.FakeChangesetSource{}), true, cstore, plan)
	if err == nil {
		t.Fatal("reconciler did not return error")
	}

	// We expect a non-retryable error to be returned.
	if !errcode.IsNonRetryable(err) {
		t.Fatalf("error is not non-retryabe. have=%s", err)
	}
}

func TestExecutor_LoadAuthenticator(t *testing.T) {
	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	cstore := store.New(db)

	admin := ct.CreateTestUser(t, db, true)
	user := ct.CreateTestUser(t, db, false)

	rs, _ := ct.CreateTestRepos(t, ctx, db, 1)
	repo := rs[0]

	batchSpec := ct.CreateBatchSpec(t, ctx, cstore, "reconciler-test-batch-change", admin.ID)
	adminBatchChange := ct.CreateBatchChange(t, ctx, cstore, "reconciler-test-batch-change", admin.ID, batchSpec.ID)
	userBatchChange := ct.CreateBatchChange(t, ctx, cstore, "reconciler-test-batch-change", user.ID, batchSpec.ID)

	t.Run("imported changeset uses global token", func(t *testing.T) {
		a, err := (&executor{
			ch: &batches.Changeset{
				OwnedByBatchChangeID: 0,
			},
		}).loadAuthenticator(ctx)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if a != nil {
			t.Errorf("unexpected non-nil authenticator: %v", a)
		}
	})

	t.Run("owned by missing batch change", func(t *testing.T) {
		_, err := (&executor{
			ch: &batches.Changeset{
				OwnedByBatchChangeID: 1234,
			},
			tx: cstore,
		}).loadAuthenticator(ctx)
		if err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("owned by admin user without credential", func(t *testing.T) {
		a, err := (&executor{
			ch: &batches.Changeset{
				OwnedByBatchChangeID: adminBatchChange.ID,
			},
			repo: repo,
			tx:   cstore,
		}).loadAuthenticator(ctx)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if a != nil {
			t.Errorf("unexpected non-nil authenticator: %v", a)
		}
	})

	t.Run("owned by normal user without credential", func(t *testing.T) {
		_, err := (&executor{
			ch: &batches.Changeset{
				OwnedByBatchChangeID: userBatchChange.ID,
			},
			repo: repo,
			tx:   cstore,
		}).loadAuthenticator(ctx)
		if err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("owned by admin user with credential", func(t *testing.T) {
		token := &auth.OAuthBearerToken{Token: "abcdef"}
		if _, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
			Domain:              database.UserCredentialDomainBatches,
			UserID:              admin.ID,
			ExternalServiceType: repo.ExternalRepo.ServiceType,
			ExternalServiceID:   repo.ExternalRepo.ServiceID,
		}, token); err != nil {
			t.Fatal(err)
		}

		a, err := (&executor{
			ch: &batches.Changeset{
				OwnedByBatchChangeID: adminBatchChange.ID,
			},
			repo: repo,
			tx:   cstore,
		}).loadAuthenticator(ctx)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if diff := cmp.Diff(token, a); diff != "" {
			t.Errorf("unexpected authenticator:\n%s", diff)
		}
	})

	t.Run("owned by normal user with credential", func(t *testing.T) {
		token := &auth.OAuthBearerToken{Token: "abcdef"}
		if _, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
			Domain:              database.UserCredentialDomainBatches,
			UserID:              user.ID,
			ExternalServiceType: repo.ExternalRepo.ServiceType,
			ExternalServiceID:   repo.ExternalRepo.ServiceID,
		}, token); err != nil {
			t.Fatal(err)
		}

		a, err := (&executor{
			ch: &batches.Changeset{
				OwnedByBatchChangeID: userBatchChange.ID,
			},
			repo: repo,
			tx:   cstore,
		}).loadAuthenticator(ctx)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if diff := cmp.Diff(token, a); diff != "" {
			t.Errorf("unexpected authenticator:\n%s", diff)
		}
	})
}

func TestExecutor_UserCredentialsForGitserver(t *testing.T) {
	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	cstore := store.New(db)

	admin := ct.CreateTestUser(t, db, true)
	user := ct.CreateTestUser(t, db, false)

	rs, extSvc := ct.CreateTestRepos(t, ctx, db, 1)
	gitHubRepo := rs[0]
	gitHubRepoCloneURL := gitHubRepo.Sources[extSvc.URN()].CloneURL

	gitLabRepos, gitLabExtSvc := ct.CreateGitlabTestRepos(t, ctx, db, 1)
	gitLabRepo := gitLabRepos[0]
	gitLabRepoCloneURL := gitLabRepo.Sources[gitLabExtSvc.URN()].CloneURL

	bbsRepos, bbsExtSvc := ct.CreateBbsTestRepos(t, ctx, db, 1)
	bbsRepo := bbsRepos[0]
	bbsRepoCloneURL := bbsRepo.Sources[bbsExtSvc.URN()].CloneURL

	bbsSSHRepos, bbsSSHExtsvc := ct.CreateBbsSSHTestRepos(t, ctx, db, 1)
	bbsSSHRepo := bbsSSHRepos[0]
	bbsSSHRepoCloneURL := bbsSSHRepo.Sources[bbsSSHExtsvc.URN()].CloneURL

	gitClient := &ct.FakeGitserverClient{ResponseErr: nil}
	fakeSource := &ct.FakeChangesetSource{Svc: extSvc}
	sourcer := repos.NewFakeSourcer(nil, fakeSource)

	plan := &Plan{}
	plan.AddOp(batches.ReconcilerOperationPush)

	tests := []struct {
		name           string
		user           *types.User
		repo           *types.Repo
		credentials    auth.Authenticator
		wantErr        bool
		wantPushConfig *gitprotocol.PushConfig
	}{
		{
			name:        "github OAuthBearerToken",
			user:        user,
			repo:        gitHubRepo,
			credentials: &auth.OAuthBearerToken{Token: "my-secret-github-token"},
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://my-secret-github-token@github.com/" + string(gitHubRepo.Name),
			},
		},
		{
			name:    "github no credentials",
			user:    user,
			repo:    gitHubRepo,
			wantErr: true,
		},
		{
			name: "github site-admin and no credentials",
			repo: gitHubRepo,
			user: admin,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: gitHubRepoCloneURL,
			},
		},
		{
			name:        "gitlab OAuthBearerToken",
			user:        user,
			repo:        gitLabRepo,
			credentials: &auth.OAuthBearerToken{Token: "my-secret-gitlab-token"},
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://git:my-secret-gitlab-token@gitlab.com/" + string(gitLabRepo.Name),
			},
		},
		{
			name:    "gitlab no credentials",
			user:    user,
			repo:    gitLabRepo,
			wantErr: true,
		},
		{
			name: "gitlab site-admin and no credentials",
			user: admin,
			repo: gitLabRepo,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: gitLabRepoCloneURL,
			},
		},
		{
			name:        "bitbucketServer BasicAuth",
			user:        user,
			repo:        bbsRepo,
			credentials: &auth.BasicAuth{Username: "fredwoard johnssen", Password: "my-secret-bbs-token"},
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://fredwoard%20johnssen:my-secret-bbs-token@bitbucket.sourcegraph.com/scm/" + string(bbsRepo.Name),
			},
		},
		{
			name:    "bitbucketServer no credentials",
			user:    user,
			repo:    bbsRepo,
			wantErr: true,
		},
		{
			name: "bitbucketServer site-admin and no credentials",
			user: admin,
			repo: bbsRepo,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: bbsRepoCloneURL,
			},
		},
		{
			name:    "ssh clone URL no credentials",
			user:    user,
			repo:    bbsSSHRepo,
			wantErr: true,
		},
		{
			name: "ssh clone URL no credentials admin",
			user: admin,
			repo: bbsSSHRepo,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: bbsSSHRepoCloneURL,
			},
		},
		{
			name: "ssh clone URL SSH credential",
			user: admin,
			repo: bbsSSHRepo,
			credentials: &auth.OAuthBearerTokenWithSSH{
				OAuthBearerToken: auth.OAuthBearerToken{Token: "test"},
				PrivateKey:       "private key",
				PublicKey:        "public key",
				Passphrase:       "passphrase",
			},
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL:  bbsSSHRepoCloneURL,
				PrivateKey: "private key",
				Passphrase: "passphrase",
			},
		},
		{
			name:        "ssh clone URL non-SSH credential",
			user:        admin,
			repo:        bbsSSHRepo,
			credentials: &auth.OAuthBearerToken{Token: "test"},
			wantErr:     true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.credentials != nil {
				cred, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
					Domain:              database.UserCredentialDomainBatches,
					UserID:              tt.user.ID,
					ExternalServiceType: tt.repo.ExternalRepo.ServiceType,
					ExternalServiceID:   tt.repo.ExternalRepo.ServiceID,
				}, tt.credentials)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { cstore.UserCredentials().Delete(ctx, cred.ID) }()
			}

			batchSpec := ct.CreateBatchSpec(t, ctx, cstore, fmt.Sprintf("reconciler-credentials-%d", i), tt.user.ID)
			batchChange := ct.CreateBatchChange(t, ctx, cstore, fmt.Sprintf("reconciler-credentials-%d", i), tt.user.ID, batchSpec.ID)

			plan.Changeset = &batches.Changeset{
				OwnedByBatchChangeID: batchChange.ID,
				RepoID:               tt.repo.ID,
			}
			plan.ChangesetSpec = ct.BuildChangesetSpec(t, ct.TestSpecOpts{
				HeadRef:    "refs/heads/my-branch",
				Published:  true,
				CommitDiff: "testdiff",
			})

			err := ExecutePlan(
				context.Background(),
				gitClient,
				sourcer,
				true,
				cstore,
				plan,
			)

			if !tt.wantErr && err != nil {
				t.Fatalf("executing plan failed: %s", err)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				} else {
					return
				}
			}

			if diff := cmp.Diff(tt.wantPushConfig, gitClient.CreateCommitFromPatchReq.Push); diff != "" {
				t.Errorf("unexpected push options:\n%s", diff)
			}
		})
	}
}

func TestDecorateChangesetBody(t *testing.T) {
	database.Mocks.Namespaces.GetByID = func(ctx context.Context, org, user int32) (*database.Namespace, error) {
		return &database.Namespace{Name: "my-user", User: user}, nil
	}
	defer func() { database.Mocks.Namespaces.GetByID = nil }()

	internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
	defer func() { internalClient = api.InternalClient }()

	fs := &FakeStore{
		GetBatchChangeMock: func(ctx context.Context, opts store.CountBatchChangeOpts) (*batches.BatchChange, error) {
			return &batches.BatchChange{ID: 1234, Name: "reconciler-test-batch-change"}, nil
		},
	}

	cs := ct.BuildChangeset(ct.TestChangesetOpts{OwnedByBatchChange: 1234})

	body := "body"
	rcs := &repos.Changeset{Body: body, Changeset: cs}
	if err := decorateChangesetBody(context.Background(), fs, database.Namespaces(new(dbtesting.MockDB)), rcs); err != nil {
		t.Errorf("unexpected non-nil error: %v", err)
	}
	if want := body + "\n\n[_Created by Sourcegraph batch change `my-user/reconciler-test-batch-change`._](https://sourcegraph.test/users/my-user/batch-changes/reconciler-test-batch-change)"; rcs.Body != want {
		t.Errorf("repos.Changeset body unexpectedly changed:\nhave=%q\nwant=%q", rcs.Body, want)
	}
}

func TestBatchChangeURL(t *testing.T) {
	ctx := context.Background()

	t.Run("errors", func(t *testing.T) {
		for name, tc := range map[string]*mockInternalClient{
			"ExternalURL error": {err: errors.New("foo")},
			"invalid URL":       {externalURL: "foo://:bar"},
		} {
			t.Run(name, func(t *testing.T) {
				internalClient = tc
				defer func() { internalClient = api.InternalClient }()

				if _, err := batchChangeURL(ctx, nil, nil); err == nil {
					t.Error("unexpected nil error")
				}
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
		defer func() { internalClient = api.InternalClient }()

		url, err := batchChangeURL(
			ctx,
			&database.Namespace{Name: "foo", Organization: 123},
			&batches.BatchChange{Name: "bar"},
		)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if want := "https://sourcegraph.test/organizations/foo/batch-changes/bar"; url != want {
			t.Errorf("unexpected URL: have=%q want=%q", url, want)
		}
	})
}

func TestNamespaceURL(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ns   *database.Namespace
		want string
	}{
		"user": {
			ns:   &database.Namespace{User: 123, Name: "user"},
			want: "/users/user",
		},
		"org": {
			ns:   &database.Namespace{Organization: 123, Name: "org"},
			want: "/organizations/org",
		},
		"neither": {
			ns:   &database.Namespace{Name: "user"},
			want: "/users/user",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if have := namespaceURL(tc.ns); have != tc.want {
				t.Errorf("unexpected URL: have=%q want=%q", have, tc.want)
			}
		})
	}
}

type mockInternalClient struct {
	externalURL string
	err         error
}

func (c *mockInternalClient) ExternalURL(ctx context.Context) (string, error) {
	return c.externalURL, c.err
}

func TestBuildPushConfig(t *testing.T) {
	oauthHTTPSAuthenticator := auth.OAuthBearerToken{Token: "bearer-test"}
	oauthSSHAuthenticator := auth.OAuthBearerTokenWithSSH{
		OAuthBearerToken: oauthHTTPSAuthenticator,
		PrivateKey:       "private-key",
		Passphrase:       "passphrase",
		PublicKey:        "public-key",
	}
	basicHTTPSAuthenticator := auth.BasicAuth{Username: "basic", Password: "pw"}
	basicSSHAuthenticator := auth.BasicAuthWithSSH{
		BasicAuth:  basicHTTPSAuthenticator,
		PrivateKey: "private-key",
		Passphrase: "passphrase",
		PublicKey:  "public-key",
	}
	tcs := []struct {
		name                string
		externalServiceType string
		cloneURL            string
		authenticator       auth.Authenticator
		wantPushConfig      *gitprotocol.PushConfig
		wantErr             error
	}{
		// Without authenticator:
		{
			name:                "GitHub HTTPS no token",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "https://github.com/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://github.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitHub HTTPS token",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "https://token@github.com/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://token@github.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitHub SSH",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "git@github.com:sourcegraph/sourcegraph.git",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "git@github.com:sourcegraph/sourcegraph.git",
			},
		},
		{
			name:                "GitLab HTTPS no token",
			externalServiceType: extsvc.TypeGitLab,
			cloneURL:            "https://gitlab.com/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://gitlab.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitLab HTTPS token",
			externalServiceType: extsvc.TypeGitLab,
			cloneURL:            "https://git:token@gitlab.com/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://git:token@gitlab.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitLab SSH",
			externalServiceType: extsvc.TypeGitLab,
			cloneURL:            "git@gitlab.com:sourcegraph/sourcegraph.git",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "git@gitlab.com:sourcegraph/sourcegraph.git",
			},
		},
		{
			name:                "Bitbucket server HTTPS no token",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "https://bitbucket.sgdev.org/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://bitbucket.sgdev.org/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "Bitbucket server HTTPS token",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "https://token@bitbucket.sgdev.org/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://token@bitbucket.sgdev.org/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "Bitbucket server SSH",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "ssh://git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph",
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "ssh://git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph",
			},
		},
		// With authenticator:
		{
			name:                "GitHub HTTPS no token with authenticator",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "https://github.com/sourcegraph/sourcegraph",
			authenticator:       &oauthHTTPSAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://bearer-test@github.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitHub HTTPS token with authenticator",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "https://token@github.com/sourcegraph/sourcegraph",
			authenticator:       &oauthHTTPSAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://bearer-test@github.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitHub SSH with authenticator",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "git@github.com:sourcegraph/sourcegraph.git",
			authenticator:       &oauthSSHAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL:  "git@github.com:sourcegraph/sourcegraph.git",
				PrivateKey: "private-key",
				Passphrase: "passphrase",
			},
		},
		{
			name:                "GitLab HTTPS no token with authenticator",
			externalServiceType: extsvc.TypeGitLab,
			cloneURL:            "https://gitlab.com/sourcegraph/sourcegraph",
			authenticator:       &oauthHTTPSAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://git:bearer-test@gitlab.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitLab HTTPS token with authenticator",
			externalServiceType: extsvc.TypeGitLab,
			cloneURL:            "https://git:token@gitlab.com/sourcegraph/sourcegraph",
			authenticator:       &oauthHTTPSAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://git:bearer-test@gitlab.com/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "GitLab SSH with authenticator",
			externalServiceType: extsvc.TypeGitLab,
			cloneURL:            "git@gitlab.com:sourcegraph/sourcegraph.git",
			authenticator:       &oauthSSHAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL:  "git@gitlab.com:sourcegraph/sourcegraph.git",
				PrivateKey: "private-key",
				Passphrase: "passphrase",
			},
		},
		{
			name:                "Bitbucket server HTTPS no token with authenticator",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "https://bitbucket.sgdev.org/sourcegraph/sourcegraph",
			authenticator:       &basicHTTPSAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://basic:pw@bitbucket.sgdev.org/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "Bitbucket server HTTPS token with authenticator",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "https://token@bitbucket.sgdev.org/sourcegraph/sourcegraph",
			authenticator:       &basicHTTPSAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL: "https://basic:pw@bitbucket.sgdev.org/sourcegraph/sourcegraph",
			},
		},
		{
			name:                "Bitbucket server SSH with authenticator",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "ssh://git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph",
			authenticator:       &basicSSHAuthenticator,
			wantPushConfig: &gitprotocol.PushConfig{
				RemoteURL:  "ssh://git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph",
				PrivateKey: "private-key",
				Passphrase: "passphrase",
			},
		},
		// Errors
		{
			name:                "Bitbucket server SSH no keypair",
			externalServiceType: extsvc.TypeBitbucketServer,
			cloneURL:            "ssh://git@bitbucket.sgdev.org:7999/sourcegraph/sourcegraph",
			authenticator:       &basicHTTPSAuthenticator,
			wantErr:             ErrNoSSHCredential{},
		},
		{
			name:                "Invalid credential type",
			externalServiceType: extsvc.TypeGitHub,
			cloneURL:            "https://github.com/sourcegraph/sourcegraph",
			authenticator:       &auth.OAuthClient{},
			wantErr:             ErrNoPushCredentials{credentialsType: "*auth.OAuthClient"},
		},
	}
	for _, tt := range tcs {
		t.Run(tt.name, func(t *testing.T) {
			havePushConfig, haveErr := buildPushConfig(tt.externalServiceType, tt.cloneURL, tt.authenticator)
			if haveErr != tt.wantErr {
				t.Fatalf("invalid error returned, want=%v have=%v", tt.wantErr, haveErr)
			}
			if diff := cmp.Diff(havePushConfig, tt.wantPushConfig); diff != "" {
				t.Fatalf("invalid push config returned: %s", diff)
			}
		})
	}
}
