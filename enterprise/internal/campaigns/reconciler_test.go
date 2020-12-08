package campaigns

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/campaigns/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/campaigns"
	"github.com/sourcegraph/sourcegraph/internal/db"
	"github.com/sourcegraph/sourcegraph/internal/db/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/db/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	gitprotocol "github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

func TestReconcilerProcess(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)

	now := timeutil.Now()
	clock := func() time.Time { return now }
	store := NewStoreWithClock(dbconn.Global, clock)

	admin := createTestUser(t, true)

	rs, extSvc := ct.CreateTestRepos(t, ctx, dbconn.Global, 1)

	state := ct.MockChangesetSyncState(&protocol.RepoInfo{
		Name: api.RepoName(rs[0].Name),
		VCS:  protocol.VCSInfo{URL: rs[0].URI},
	})
	defer state.Unmock()

	internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
	defer func() { internalClient = api.InternalClient }()

	githubPR := buildGithubPR(clock(), campaigns.ChangesetExternalStateOpen)
	githubHeadRef := git.EnsureRefPrefix(githubPR.HeadRefName)
	draftGithubPR := buildGithubPR(clock(), campaigns.ChangesetExternalStateDraft)
	closedGitHubPR := buildGithubPR(clock(), campaigns.ChangesetExternalStateClosed)

	notFoundErr := repos.ChangesetNotFoundError{
		Changeset: &repos.Changeset{
			Changeset: &campaigns.Changeset{ExternalID: "100000"},
		},
	}

	campaignSpec := createCampaignSpec(t, ctx, store, "reconciler-test-campaign", admin.ID)
	campaign := createCampaign(t, ctx, store, "reconciler-test-campaign", admin.ID, campaignSpec.ID)

	type testCase struct {
		changeset    testChangesetOpts
		currentSpec  *testSpecOpts
		previousSpec *testSpecOpts

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

		wantChangeset       changesetAssertions
		wantNonRetryableErr bool
	}

	tests := map[string]testCase{
		"published unsynced changeset without changesetSpec": {
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				unsynced:         true,
			},
			sourcerMetadata: githubPR,

			wantLoadFromCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				unsynced:         false,
				title:            githubPR.Title,
				body:             githubPR.Body,
				diffStat:         state.DiffStat,
			},
		},
		"unpublished changeset stay unpublished": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/repo-1-branch-1",
				published: false,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
			},
			sourcerMetadata: githubPR,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
				externalState:    "",
				externalID:       "",
				externalBranch:   "",
			},
		},
		"publish changeset": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: githubPR,

			wantCreateOnCodeHost: true,
			wantGitserverCommit:  true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				title:            githubPR.Title,
				body:             githubPR.Body,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
			},
		},
		"retry publish changeset": {
			// This test case makes sure that everything works when the code host says
			// that the changeset already exists.
			alreadyExists: true,
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,
			},
			changeset: testChangesetOpts{
				failureMessage:   "publication failed",
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: githubPR,

			// We first do a create and since that fails with "already exists"
			// we update.
			wantCreateOnCodeHost: true,
			wantUpdateOnCodeHost: true,
			wantGitserverCommit:  true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				title:            githubPR.Title,
				body:             githubPR.Body,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
			},
		},
		"update published changeset metadata": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "new title",
				body:  "new body",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "old title",
				body:  "old body",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       "12345",
				externalBranch:   git.EnsureRefPrefix("head-ref-on-github"),
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: githubPR,

			// We don't want a new commit, only an update on the code host.
			wantUpdateOnCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
				// We update the title/body but want the title/body returned by the code host.
				title: githubPR.Title,
				body:  githubPR.Body,
			},
		},
		"retry update published changeset metadata": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "new title",
				body:  "new body",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "old title",
				body:  "old body",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				ownedByCampaign:  campaign.ID,
				// Previous update failed:
				failureMessage: "failed to update changeset metadata",
			},
			sourcerMetadata: githubPR,

			wantUpdateOnCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				title:            githubPR.Title,
				body:             githubPR.Body,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
				// failureMessage should be nil
			},
		},
		"update published changeset commit": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				// Title and body the same, but commit changed
				commitDiff:    "new diff",
				commitMessage: "new message",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				commitDiff:    "old diff",
				commitMessage: "old message",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       "12345",
				externalBranch:   git.EnsureRefPrefix("head-ref-on-github"),
				externalState:    campaigns.ChangesetExternalStateOpen,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: githubPR,

			// We don't want an update on the code host, only a new commit pushed.
			wantGitserverCommit: true,
			// And we want the changeset to be synced after pushing the commit.
			wantLoadFromCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalState:    campaigns.ChangesetExternalStateOpen,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
			},
		},
		"retry update published changeset commit": {
			currentSpec: &testSpecOpts{
				headRef:       "refs/heads/head-ref-on-github",
				published:     true,
				commitDiff:    "new diff",
				commitMessage: "new message",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				commitDiff:    "old diff",
				commitMessage: "old message",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       "12345",
				externalBranch:   git.EnsureRefPrefix("head-ref-on-github"),
				externalState:    campaigns.ChangesetExternalStateOpen,
				ownedByCampaign:  campaign.ID,

				// Previous update failed:
				failureMessage: "failed to update changeset commit",
			},
			sourcerMetadata: githubPR,

			wantGitserverCommit:  true,
			wantLoadFromCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalState:    campaigns.ChangesetExternalStateOpen,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
				// failureMessage should be nil
			},
		},
		"update published changeset commit author": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				// Everything the same, except author changed
				commitAuthorName:  "Fernando the fish",
				commitAuthorEmail: "fernando@deep.sea",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				// Old author data
				commitAuthorName:  "Larry the Llama",
				commitAuthorEmail: "larry@winamp.com",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       "12345",
				externalBranch:   git.EnsureRefPrefix("head-ref-on-github"),
				externalState:    campaigns.ChangesetExternalStateOpen,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: githubPR,

			// We don't want an update on the code host, only a new commit pushed.
			wantGitserverCommit: true,
			// And we want the changeset to be synced after pushing the commit.
			wantLoadFromCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalState:    campaigns.ChangesetExternalStateOpen,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				diffStat:         state.DiffStat,
				ownedByCampaign:  campaign.ID,
			},
		},
		"reprocess published changeset without changes": {
			// ChangesetSpec is already published and has no previous spec.
			// Simply a reprocessing of the same changeset.
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "title",
				body:  "body",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
			},
			sourcerMetadata: githubPR,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
			},
		},
		"closing published open changeset": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "title",
				body:  "body",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateOpen,
				closing:          true,
				ownedByCampaign:  campaign.ID,
			},
			// We return a closed GitHub PR here
			sourcerMetadata: closedGitHubPR,

			wantCloseOnCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				closing:          false,

				externalID:     closedGitHubPR.ID,
				externalBranch: git.EnsureRefPrefix(closedGitHubPR.HeadRefName),
				externalState:  campaigns.ChangesetExternalStateClosed,

				title:    closedGitHubPR.Title,
				body:     closedGitHubPR.Body,
				diffStat: state.DiffStat,

				ownedByCampaign: campaign.ID,
			},
		},
		"closing non-open changeset": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "title",
				body:  "body",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateClosed,
				closing:          true,
				ownedByCampaign:  campaign.ID,
			},
			// We return a closed GitHub PR here, but since it's a noop, we
			// don't sync and thus don't set its attributes on the changeset.
			sourcerMetadata: closedGitHubPR,

			// Should be a noop
			wantCloseOnCodeHost: false,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				closing:          false,

				externalID:     closedGitHubPR.ID,
				externalBranch: git.EnsureRefPrefix(closedGitHubPR.HeadRefName),
				externalState:  campaigns.ChangesetExternalStateClosed,
			},
		},
		"reopening closed changeset without updates": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "title",
				body:  "body",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "title",
				body:  "body",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateClosed,
				ownedByCampaign:  campaign.ID,
				closing:          false,
			},
			// We return the open GitHub PR here
			sourcerMetadata: githubPR,

			wantReopenOnCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,

				externalID:     githubPR.ID,
				externalBranch: githubHeadRef,
				externalState:  campaigns.ChangesetExternalStateOpen,

				title:    githubPR.Title,
				body:     githubPR.Body,
				diffStat: state.DiffStat,
			},
		},

		"reopening closed changeset with updates": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "title",
				body:  "body",
			},
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,

				title: "old title",
				body:  "old body",

				commitDiff:    "old diff",
				commitMessage: "old message",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       githubPR.ID,
				externalBranch:   githubHeadRef,
				externalState:    campaigns.ChangesetExternalStateClosed,
				ownedByCampaign:  campaign.ID,
				closing:          false,
			},
			sourcerMetadata: githubPR,

			// Reopen it
			wantReopenOnCodeHost: true,
			// Update the metadata
			wantUpdateOnCodeHost: true,
			// Update the commit
			wantGitserverCommit: true,

			wantLoadFromCodeHost: false,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,

				externalID:     githubPR.ID,
				externalBranch: githubHeadRef,
				externalState:  campaigns.ChangesetExternalStateOpen,

				title:    githubPR.Title,
				body:     githubPR.Body,
				diffStat: state.DiffStat,
			},
		},

		"publish as draft mode for supported codehost": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: "draft",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: draftGithubPR,

			// Update the commit
			wantGitserverCommit:       true,
			wantCreateDraftOnCodeHost: true,

			wantLoadFromCodeHost: false,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,

				externalID:     draftGithubPR.ID,
				externalBranch: git.EnsureRefPrefix(draftGithubPR.HeadRefName),
				externalState:  campaigns.ChangesetExternalStateDraft,

				title:    draftGithubPR.Title,
				body:     draftGithubPR.Body,
				diffStat: state.DiffStat,
			},
		},

		"published false to published draft": {
			previousSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: false,
			},
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: "draft",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: draftGithubPR,

			// Update the commit
			wantGitserverCommit:       true,
			wantCreateDraftOnCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,

				externalID:     draftGithubPR.ID,
				externalBranch: git.EnsureRefPrefix(draftGithubPR.HeadRefName),
				externalState:  campaigns.ChangesetExternalStateDraft,

				title:    draftGithubPR.Title,
				body:     draftGithubPR.Body,
				diffStat: state.DiffStat,
			},
		},

		"undraft a changeset": {
			currentSpec: &testSpecOpts{
				headRef:   "refs/heads/head-ref-on-github",
				published: true,
			},
			previousSpec: &testSpecOpts{
				published: "draft",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalState:    campaigns.ChangesetExternalStateDraft,
				ownedByCampaign:  campaign.ID,
			},
			sourcerMetadata: githubPR,

			wantUndraftOnCodeHost: true,

			wantChangeset: changesetAssertions{
				publicationState: campaigns.ChangesetPublicationStatePublished,

				externalID:     githubPR.ID,
				externalBranch: githubHeadRef,
				externalState:  campaigns.ChangesetExternalStateOpen,

				title:    githubPR.Title,
				body:     githubPR.Body,
				diffStat: state.DiffStat,
			},
		},
		"syncing not found changeset": {
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalID:       "100000",
				unsynced:         true,
			},
			sourcerErr: notFoundErr,

			wantLoadFromCodeHost: true,

			wantNonRetryableErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Clean up database.
			truncateTables(t, dbconn.Global, "changeset_events", "changesets", "campaigns", "campaign_specs", "changeset_specs")

			// Create necessary associations.
			campaignSpec := createCampaignSpec(t, ctx, store, "reconciler-test-campaign", admin.ID)
			campaign := createCampaign(t, ctx, store, "reconciler-test-campaign", admin.ID, campaignSpec.ID)

			// Create the changesetSpec with associations wired up correctly.
			var changesetSpec *campaigns.ChangesetSpec
			if tc.currentSpec != nil {
				specOpts := *tc.currentSpec
				specOpts.user = admin.ID
				specOpts.repo = rs[0].ID
				specOpts.campaignSpec = campaignSpec.ID
				changesetSpec = createChangesetSpec(t, ctx, store, specOpts)
			}

			// If we need a previous spec, we need to set that up too.
			var previousSpec *campaigns.ChangesetSpec
			if tc.previousSpec != nil {
				previousCampaignSpec := createCampaignSpec(t, ctx, store, "previous-campaign-spec", admin.ID)
				specOpts := *tc.previousSpec
				specOpts.user = admin.ID
				specOpts.repo = rs[0].ID
				specOpts.campaignSpec = previousCampaignSpec.ID
				previousSpec = createChangesetSpec(t, ctx, store, specOpts)
			}

			// Create the changeset with correct associations.
			changesetOpts := tc.changeset
			changesetOpts.repo = rs[0].ID
			changesetOpts.campaign = campaign.ID
			if changesetSpec != nil {
				changesetOpts.currentSpec = changesetSpec.ID
			}
			if previousSpec != nil {
				changesetOpts.previousSpec = previousSpec.ID
			}
			changeset := createChangeset(t, ctx, store, changesetOpts)

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
				FakeMetadata:    tc.sourcerMetadata,
			}
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
				if tc.wantNonRetryableErr && errcode.IsNonRetryable(err) {
					// all good
				} else {
					t.Fatalf("reconciler process failed: %s", err)
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
			assertions.repo = rs[0].ID
			assertions.ownedByCampaign = changesetOpts.ownedByCampaign
			if changesetSpec != nil {
				assertions.currentSpec = changesetSpec.ID
			}
			if previousSpec != nil {
				assertions.previousSpec = previousSpec.ID
			}
			reloadAndAssertChangeset(t, ctx, store, changeset, assertions)

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
				if !strings.Contains(rcs.Body, "Created by Sourcegraph campaign") {
					t.Errorf("did not find backlink in body: %q", rcs.Body)
				}
			}
		})
	}
}

func TestDetermineReconcilerPlan(t *testing.T) {
	tcs := []struct {
		name           string
		previousSpec   testSpecOpts
		currentSpec    testSpecOpts
		changeset      testChangesetOpts
		wantOperations ReconcilerOperations
	}{
		{
			name: "GitHub publish",
			currentSpec: testSpecOpts{
				published: true,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationPush, campaigns.ReconcilerOperationPublish},
		},
		{
			name: "GitHub publish as draft",
			currentSpec: testSpecOpts{
				published: "draft",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationPush, campaigns.ReconcilerOperationPublishDraft},
		},
		{
			name: "GitHub publish false",
			currentSpec: testSpecOpts{
				published: false,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
			},
			wantOperations: ReconcilerOperations{},
		},
		{
			name: "set to draft but unsupported",
			currentSpec: testSpecOpts{
				published: "draft",
			},
			changeset: testChangesetOpts{
				externalServiceType: extsvc.TypeBitbucketServer,
				publicationState:    campaigns.ChangesetPublicationStateUnpublished,
			},
			wantOperations: ReconcilerOperations{},
		},
		{
			name: "set from draft to publish true",
			previousSpec: testSpecOpts{
				published: "draft",
			},
			currentSpec: testSpecOpts{
				published: true,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationUndraft},
		},
		{
			name: "set from draft to publish true on unpublished",
			previousSpec: testSpecOpts{
				published: "draft",
			},
			currentSpec: testSpecOpts{
				published: true,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStateUnpublished,
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationPush, campaigns.ReconcilerOperationPublish},
		},
		{
			name: "changeset spec changed attribute, needs update",
			previousSpec: testSpecOpts{
				published: true,
				title:     "Before",
			},
			currentSpec: testSpecOpts{
				published: true,
				title:     "After",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationUpdate},
		},
		{
			name: "changeset spec changed, needs new commit but no update",
			previousSpec: testSpecOpts{
				published:  true,
				commitDiff: "testDiff",
			},
			currentSpec: testSpecOpts{
				published:  true,
				commitDiff: "newTestDiff",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationPush, campaigns.ReconcilerOperationSleep, campaigns.ReconcilerOperationSync},
		},
		{
			name: "changeset merged and spec changed is noop",
			previousSpec: testSpecOpts{
				published:  true,
				commitDiff: "testDiff",
			},
			currentSpec: testSpecOpts{
				published:  true,
				commitDiff: "newTestDiff",
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalState:    campaigns.ChangesetExternalStateMerged,
			},
			wantOperations: ReconcilerOperations{},
		},
		{
			name: "changeset closed-and-detached will reopen",
			previousSpec: testSpecOpts{
				published: true,
			},
			currentSpec: testSpecOpts{
				published: true,
			},
			changeset: testChangesetOpts{
				publicationState: campaigns.ChangesetPublicationStatePublished,
				externalState:    campaigns.ChangesetExternalStateClosed,
				ownedByCampaign:  1234,
				campaignIDs:      []int64{1234},
			},
			wantOperations: ReconcilerOperations{campaigns.ReconcilerOperationReopen},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var previousSpec *campaigns.ChangesetSpec
			if tc.previousSpec != (testSpecOpts{}) {
				previousSpec = buildChangesetSpec(t, tc.previousSpec)
			}

			currentSpec := buildChangesetSpec(t, tc.currentSpec)
			cs := buildChangeset(tc.changeset)

			plan, err := DetermineReconcilerPlan(previousSpec, currentSpec, cs)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := plan.Ops, tc.wantOperations; !have.Equal(want) {
				t.Fatalf("incorrect plan determined, want=%v have=%v", want, have)
			}
		})
	}
}

func TestReconcilerProcess_PublishedChangesetDuplicateBranch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)

	store := NewStore(dbconn.Global)

	admin := createTestUser(t, true)
	rs, _ := ct.CreateTestRepos(t, ctx, dbconn.Global, 1)

	state := ct.MockChangesetSyncState(&protocol.RepoInfo{
		Name: api.RepoName(rs[0].Name),
		VCS:  protocol.VCSInfo{URL: rs[0].URI},
	})
	defer state.Unmock()

	commonHeadRef := "refs/heads/collision"

	// Create a published changeset.
	campaignSpec := createCampaignSpec(t, ctx, store, "reconciler-test-campaign", admin.ID)
	campaign := createCampaign(t, ctx, store, "reconciler-test-campaign", admin.ID, campaignSpec.ID)
	changesetSpec := createChangesetSpec(t, ctx, store, testSpecOpts{
		user:         admin.ID,
		repo:         rs[0].ID,
		campaignSpec: campaignSpec.ID,
		headRef:      commonHeadRef,
	})
	createChangeset(t, ctx, store, testChangesetOpts{
		repo:             rs[0].ID,
		publicationState: campaigns.ChangesetPublicationStatePublished,
		campaign:         campaign.ID,
		ownedByCampaign:  campaign.ID,
		currentSpec:      changesetSpec.ID,
		externalBranch:   commonHeadRef,
		externalID:       "123",
	})

	// Try to publish a changeset on the same HeadRef/ExternalBranch.
	otherCampaignSpec := createCampaignSpec(t, ctx, store, "other-test-campaign", admin.ID)
	otherCampaign := createCampaign(t, ctx, store, "other-test-campaign", admin.ID, otherCampaignSpec.ID)
	otherChangesetSpec := createChangesetSpec(t, ctx, store, testSpecOpts{
		user:         admin.ID,
		repo:         rs[0].ID,
		campaignSpec: otherCampaignSpec.ID,
		headRef:      commonHeadRef,
		published:    true,
	})
	otherChangeset := createChangeset(t, ctx, store, testChangesetOpts{
		repo:             rs[0].ID,
		publicationState: campaigns.ChangesetPublicationStateUnpublished,
		campaign:         otherCampaign.ID,
		ownedByCampaign:  otherCampaign.ID,
		currentSpec:      otherChangesetSpec.ID,
	})

	// Run the reconciler
	rec := Reconciler{
		noSleepBeforeSync: true,
		Sourcer:           repos.NewFakeSourcer(nil, &ct.FakeChangesetSource{}),
		Store:             store,
	}

	err := rec.process(ctx, store, otherChangeset)
	if err == nil {
		t.Fatal("reconciler did not return error")
	}

	// We expect a non-retryable error to be returned.
	if !errcode.IsNonRetryable(err) {
		t.Fatalf("error is not non-retryabe. have=%s", err)
	}
}

func buildGithubPR(now time.Time, externalState campaigns.ChangesetExternalState) *github.PullRequest {
	state := string(externalState)

	pr := &github.PullRequest{
		ID:          "12345",
		Number:      12345,
		Title:       state + " GitHub PR",
		Body:        state + " GitHub PR",
		State:       state,
		HeadRefName: git.AbbreviateRef("head-ref-on-github"),
		TimelineItems: []github.TimelineItem{
			{Type: "PullRequestCommit", Item: &github.PullRequestCommit{
				Commit: github.Commit{
					OID:           "new-f00bar",
					PushedDate:    now,
					CommittedDate: now,
				},
			}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if externalState == campaigns.ChangesetExternalStateDraft {
		pr.State = "OPEN"
		pr.IsDraft = true
	}

	if externalState == campaigns.ChangesetExternalStateClosed {
		// We add a "ClosedEvent" so that the SyncChangesets call that happens after closing
		// the PR has the "correct" state to set the ExternalState
		pr.TimelineItems = append(pr.TimelineItems, github.TimelineItem{
			Type: "ClosedEvent",
			Item: &github.ClosedEvent{CreatedAt: now.Add(1 * time.Hour)},
		})
		pr.UpdatedAt = now.Add(1 * time.Hour)
	}

	return pr
}

func TestDecorateChangesetBody(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	db.Mocks.Namespaces.GetByID = func(ctx context.Context, org, user int32) (*db.Namespace, error) {
		return &db.Namespace{Name: "my-user", User: user}, nil
	}
	defer func() { db.Mocks.Namespaces.GetByID = nil }()

	internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
	defer func() { internalClient = api.InternalClient }()

	fs := &FakeStore{
		GetCampaignMock: func(ctx context.Context, opts GetCampaignOpts) (*campaigns.Campaign, error) {
			return &campaigns.Campaign{ID: 1234, Name: "reconciler-test-campaign"}, nil
		},
	}

	cs := buildChangeset(testChangesetOpts{ownedByCampaign: 1234})

	body := "body"
	rcs := &repos.Changeset{Body: body, Changeset: cs}
	if err := decorateChangesetBody(context.Background(), fs, rcs); err != nil {
		t.Errorf("unexpected non-nil error: %v", err)
	}
	if want := body + "\n\n[_Created by Sourcegraph campaign `my-user/reconciler-test-campaign`._](https://sourcegraph.test/users/my-user/campaigns/reconciler-test-campaign)"; rcs.Body != want {
		t.Errorf("repos.Changeset body unexpectedly changed:\nhave=%q\nwant=%q", rcs.Body, want)
	}
}

func TestCampaignURL(t *testing.T) {
	ctx := context.Background()

	t.Run("errors", func(t *testing.T) {
		for name, tc := range map[string]*mockInternalClient{
			"ExternalURL error": {err: errors.New("foo")},
			"invalid URL":       {externalURL: "foo://:bar"},
		} {
			t.Run(name, func(t *testing.T) {
				internalClient = tc
				defer func() { internalClient = api.InternalClient }()

				if _, err := campaignURL(ctx, nil, nil); err == nil {
					t.Error("unexpected nil error")
				}
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		internalClient = &mockInternalClient{externalURL: "https://sourcegraph.test"}
		defer func() { internalClient = api.InternalClient }()

		url, err := campaignURL(
			ctx,
			&db.Namespace{Name: "foo", Organization: 123},
			&campaigns.Campaign{Name: "bar"},
		)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if want := "https://sourcegraph.test/organizations/foo/campaigns/bar"; url != want {
			t.Errorf("unexpected URL: have=%q want=%q", url, want)
		}
	})
}

func TestNamespaceURL(t *testing.T) {
	for name, tc := range map[string]struct {
		ns   *db.Namespace
		want string
	}{
		"user": {
			ns:   &db.Namespace{User: 123, Name: "user"},
			want: "/users/user",
		},
		"org": {
			ns:   &db.Namespace{Organization: 123, Name: "org"},
			want: "/organizations/org",
		},
		"neither": {
			ns:   &db.Namespace{Name: "user"},
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

func TestExecutor_LoadAuthenticator(t *testing.T) {
	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)

	store := NewStore(dbconn.Global)

	admin := createTestUser(t, true)
	user := createTestUser(t, false)

	rs, _ := ct.CreateTestRepos(t, ctx, dbconn.Global, 1)
	repo := rs[0]

	campaignSpec := createCampaignSpec(t, ctx, store, "reconciler-test-campaign", admin.ID)
	adminCampaign := createCampaign(t, ctx, store, "reconciler-test-campaign", admin.ID, campaignSpec.ID)
	userCampaign := createCampaign(t, ctx, store, "reconciler-test-campaign", user.ID, campaignSpec.ID)

	t.Run("imported changeset uses global token", func(t *testing.T) {
		a, err := (&executor{
			ch: &campaigns.Changeset{
				OwnedByCampaignID: 0,
			},
		}).loadAuthenticator(ctx)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if a != nil {
			t.Errorf("unexpected non-nil authenticator: %v", a)
		}
	})

	t.Run("owned by missing campaign", func(t *testing.T) {
		_, err := (&executor{
			ch: &campaigns.Changeset{
				OwnedByCampaignID: 1234,
			},
			tx: store,
		}).loadAuthenticator(ctx)
		if err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("owned by admin user without credential", func(t *testing.T) {
		a, err := (&executor{
			ch: &campaigns.Changeset{
				OwnedByCampaignID: adminCampaign.ID,
			},
			repo: repo,
			tx:   store,
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
			ch: &campaigns.Changeset{
				OwnedByCampaignID: userCampaign.ID,
			},
			repo: repo,
			tx:   store,
		}).loadAuthenticator(ctx)
		if err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("owned by admin user with credential", func(t *testing.T) {
		token := &auth.OAuthBearerToken{Token: "abcdef"}
		if _, err := db.UserCredentials.Create(ctx, db.UserCredentialScope{
			Domain:              db.UserCredentialDomainCampaigns,
			UserID:              admin.ID,
			ExternalServiceType: repo.ExternalRepo.ServiceType,
			ExternalServiceID:   repo.ExternalRepo.ServiceID,
		}, token); err != nil {
			t.Fatal(err)
		}

		a, err := (&executor{
			ch: &campaigns.Changeset{
				OwnedByCampaignID: adminCampaign.ID,
			},
			repo: repo,
			tx:   store,
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
		if _, err := db.UserCredentials.Create(ctx, db.UserCredentialScope{
			Domain:              db.UserCredentialDomainCampaigns,
			UserID:              user.ID,
			ExternalServiceType: repo.ExternalRepo.ServiceType,
			ExternalServiceID:   repo.ExternalRepo.ServiceID,
		}, token); err != nil {
			t.Fatal(err)
		}

		a, err := (&executor{
			ch: &campaigns.Changeset{
				OwnedByCampaignID: userCampaign.ID,
			},
			repo: repo,
			tx:   store,
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
	dbtesting.SetupGlobalTestDB(t)

	store := NewStore(dbconn.Global)

	admin := createTestUser(t, true)
	user := createTestUser(t, false)

	rs, extSvc := ct.CreateTestRepos(t, ctx, dbconn.Global, 1)
	gitHubRepo := rs[0]
	gitHubRepoCloneURL := gitHubRepo.Sources[extSvc.URN()].CloneURL

	gitLabRepos, gitLabExtSvc := ct.CreateGitlabTestRepos(t, ctx, dbconn.Global, 1)
	gitLabRepo := gitLabRepos[0]
	gitLabRepoCloneURL := gitLabRepo.Sources[gitLabExtSvc.URN()].CloneURL

	bbsRepos, bbsExtSvc := ct.CreateBbsTestRepos(t, ctx, dbconn.Global, 1)
	bbsRepo := bbsRepos[0]
	bbsRepoCloneURL := bbsRepo.Sources[bbsExtSvc.URN()].CloneURL

	gitClient := &ct.FakeGitserverClient{ResponseErr: nil}
	fakeSource := &ct.FakeChangesetSource{Svc: extSvc}
	sourcer := repos.NewFakeSourcer(nil, fakeSource)

	plan := &ReconcilerPlan{}
	plan.AddOp(campaigns.ReconcilerOperationPush)

	tests := []struct {
		name           string
		user           *types.User
		repo           *repos.Repo
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
				RemoteURL: "https://my-secret-github-token@github.com/" + gitHubRepo.Name,
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
				RemoteURL: "https://git:my-secret-gitlab-token@gitlab.com/" + gitLabRepo.Name,
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
				RemoteURL: "https://fredwoard%20johnssen:my-secret-bbs-token@bitbucket.sourcegraph.com/scm/" + bbsRepo.Name,
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
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.credentials != nil {
				cred, err := db.UserCredentials.Create(ctx, db.UserCredentialScope{
					Domain:              db.UserCredentialDomainCampaigns,
					UserID:              tt.user.ID,
					ExternalServiceType: tt.repo.ExternalRepo.ServiceType,
					ExternalServiceID:   tt.repo.ExternalRepo.ServiceID,
				}, tt.credentials)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { db.UserCredentials.Delete(ctx, cred.ID) }()
			}

			campaignSpec := createCampaignSpec(t, ctx, store, fmt.Sprintf("reconciler-credentials-%d", i), tt.user.ID)
			campaign := createCampaign(t, ctx, store, fmt.Sprintf("reconciler-credentials-%d", i), tt.user.ID, campaignSpec.ID)

			ex := &executor{
				ch: &campaigns.Changeset{
					OwnedByCampaignID: campaign.ID,
					RepoID:            tt.repo.ID,
				},
				spec: buildChangesetSpec(t, testSpecOpts{
					headRef:    "refs/heads/my-branch",
					published:  true,
					commitDiff: "testdiff",
				}),
				sourcer:         sourcer,
				gitserverClient: gitClient,
				tx:              store,
			}

			err := ex.ExecutePlan(context.Background(), plan)
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

type mockInternalClient struct {
	externalURL string
	err         error
}

func (c *mockInternalClient) ExternalURL(ctx context.Context) (string, error) {
	return c.externalURL, c.err
}
