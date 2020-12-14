package resolvers

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/campaigns/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/campaigns/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/campaigns/testing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/campaigns"
	"github.com/sourcegraph/sourcegraph/internal/db"
	"github.com/sourcegraph/sourcegraph/internal/db/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/db/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/repos"
)

func TestChangesetConnectionResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)

	userID := ct.CreateTestUser(t, false).ID

	cstore := store.New(dbconn.Global)
	rstore := repos.NewDBStore(dbconn.Global, sql.TxOptions{})
	repoStore := db.NewRepoStoreWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/changeset-connection-test", newGitHubExternalService(t, rstore))
	inaccessibleRepo := newGitHubTestRepo("github.com/sourcegraph/private", newGitHubExternalService(t, rstore))
	if err := repoStore.Create(ctx, repo, inaccessibleRepo); err != nil {
		t.Fatal(err)
	}
	ct.MockRepoPermissions(t, userID, repo.ID)

	spec := &campaigns.CampaignSpec{
		NamespaceUserID: userID,
		UserID:          userID,
	}
	if err := cstore.CreateCampaignSpec(ctx, spec); err != nil {
		t.Fatal(err)
	}

	campaign := &campaigns.Campaign{
		Name:             "my-unique-name",
		NamespaceUserID:  userID,
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
		CampaignSpecID:   spec.ID,
	}
	if err := cstore.CreateCampaign(ctx, campaign); err != nil {
		t.Fatal(err)
	}

	changeset1 := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		ExternalServiceType: "github",
		PublicationState:    campaigns.ChangesetPublicationStateUnpublished,
		ExternalReviewState: campaigns.ChangesetReviewStatePending,
		OwnedByCampaign:     campaign.ID,
		Campaign:            campaign.ID,
	})

	changeset2 := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		ExternalServiceType: "github",
		ExternalID:          "12345",
		ExternalBranch:      "open-pr",
		ExternalState:       campaigns.ChangesetExternalStateOpen,
		PublicationState:    campaigns.ChangesetPublicationStatePublished,
		ExternalReviewState: campaigns.ChangesetReviewStatePending,
		OwnedByCampaign:     campaign.ID,
		Campaign:            campaign.ID,
	})

	changeset3 := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		ExternalServiceType: "github",
		ExternalID:          "56789",
		ExternalBranch:      "merged-pr",
		ExternalState:       campaigns.ChangesetExternalStateMerged,
		PublicationState:    campaigns.ChangesetPublicationStatePublished,
		ExternalReviewState: campaigns.ChangesetReviewStatePending,
		OwnedByCampaign:     campaign.ID,
		Campaign:            campaign.ID,
	})
	changeset4 := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                inaccessibleRepo.ID,
		ExternalServiceType: "github",
		ExternalID:          "987651",
		ExternalBranch:      "open-hidden-pr",
		ExternalState:       campaigns.ChangesetExternalStateOpen,
		PublicationState:    campaigns.ChangesetPublicationStatePublished,
		ExternalReviewState: campaigns.ChangesetReviewStatePending,
		OwnedByCampaign:     campaign.ID,
		Campaign:            campaign.ID,
	})

	addChangeset(t, ctx, cstore, changeset1, campaign.ID)
	addChangeset(t, ctx, cstore, changeset2, campaign.ID)
	addChangeset(t, ctx, cstore, changeset3, campaign.ID)
	addChangeset(t, ctx, cstore, changeset4, campaign.ID)

	s, err := graphqlbackend.NewSchema(&Resolver{store: cstore}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	campaignAPIID := string(marshalCampaignID(campaign.ID))
	nodes := []apitest.Changeset{
		{
			Typename:   "ExternalChangeset",
			ID:         string(marshalChangesetID(changeset1.ID)),
			Repository: apitest.Repository{Name: string(repo.Name)},
		},
		{
			Typename:   "ExternalChangeset",
			ID:         string(marshalChangesetID(changeset2.ID)),
			Repository: apitest.Repository{Name: string(repo.Name)},
		},
		{
			Typename:   "ExternalChangeset",
			ID:         string(marshalChangesetID(changeset3.ID)),
			Repository: apitest.Repository{Name: string(repo.Name)},
		},
		{
			Typename: "HiddenExternalChangeset",
			ID:       string(marshalChangesetID(changeset4.ID)),
		},
	}

	tests := []struct {
		firstParam      int
		useUnsafeOpts   bool
		wantHasNextPage bool
		wantEndCursor   string
		wantTotalCount  int
		wantOpen        int
		wantNodes       []apitest.Changeset
	}{
		{firstParam: 1, wantHasNextPage: true, wantEndCursor: "1", wantTotalCount: 4, wantOpen: 2, wantNodes: nodes[:1]},
		{firstParam: 2, wantHasNextPage: true, wantEndCursor: "2", wantTotalCount: 4, wantOpen: 2, wantNodes: nodes[:2]},
		{firstParam: 3, wantHasNextPage: true, wantEndCursor: "3", wantTotalCount: 4, wantOpen: 2, wantNodes: nodes[:3]},
		{firstParam: 4, wantHasNextPage: false, wantTotalCount: 4, wantOpen: 2, wantNodes: nodes[:4]},
		// Expect only 3 changesets to be returned when an unsafe filter is applied.
		{firstParam: 1, useUnsafeOpts: true, wantEndCursor: "1", wantHasNextPage: true, wantTotalCount: 3, wantOpen: 1, wantNodes: nodes[:1]},
		{firstParam: 2, useUnsafeOpts: true, wantEndCursor: "2", wantHasNextPage: true, wantTotalCount: 3, wantOpen: 1, wantNodes: nodes[:2]},
		{firstParam: 3, useUnsafeOpts: true, wantHasNextPage: false, wantTotalCount: 3, wantOpen: 1, wantNodes: nodes[:3]},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("Unsafe opts %t, first %d", tc.useUnsafeOpts, tc.firstParam), func(t *testing.T) {
			input := map[string]interface{}{"campaign": campaignAPIID, "first": int64(tc.firstParam)}
			if tc.useUnsafeOpts {
				input["reviewState"] = campaigns.ChangesetReviewStatePending
			}
			var response struct{ Node apitest.Campaign }
			apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryChangesetConnection)

			var wantEndCursor *string
			if tc.wantEndCursor != "" {
				wantEndCursor = &tc.wantEndCursor
			}

			wantChangesets := apitest.ChangesetConnection{
				TotalCount: tc.wantTotalCount,
				PageInfo: apitest.PageInfo{
					EndCursor:   wantEndCursor,
					HasNextPage: tc.wantHasNextPage,
				},
				Nodes: tc.wantNodes,
			}

			if diff := cmp.Diff(wantChangesets, response.Node.Changesets); diff != "" {
				t.Fatalf("wrong changesets response (-want +got):\n%s", diff)
			}
		})
	}

	var endCursor *string
	for i := range nodes {
		input := map[string]interface{}{"campaign": campaignAPIID, "first": 1}
		if endCursor != nil {
			input["after"] = *endCursor
		}
		wantHasNextPage := i != len(nodes)-1

		var response struct{ Node apitest.Campaign }
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryChangesetConnection)

		changesets := response.Node.Changesets
		if diff := cmp.Diff(1, len(changesets.Nodes)); diff != "" {
			t.Fatalf("unexpected number of nodes (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(len(nodes), changesets.TotalCount); diff != "" {
			t.Fatalf("unexpected total count (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(wantHasNextPage, changesets.PageInfo.HasNextPage); diff != "" {
			t.Fatalf("unexpected hasNextPage (-want +got):\n%s", diff)
		}

		endCursor = changesets.PageInfo.EndCursor
		if want, have := wantHasNextPage, endCursor != nil; have != want {
			t.Fatalf("unexpected endCursor existence. want=%t, have=%t", want, have)
		}
	}
}

const queryChangesetConnection = `
query($campaign: ID!, $first: Int, $after: String, $reviewState: ChangesetReviewState){
  node(id: $campaign) {
    ... on Campaign {
      changesets(first: $first, after: $after, reviewState: $reviewState) {
        totalCount
        nodes {
          __typename

          ... on ExternalChangeset {
            id
            repository { name }
            nextSyncAt
          }
          ... on HiddenExternalChangeset {
            id
            nextSyncAt
          }
        }
        pageInfo {
          endCursor
          hasNextPage
        }
      }
    }
  }
}
`
