package resolvers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestChangesetEventConnectionResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID

	now := timeutil.Now()
	clock := func() time.Time { return now }
	cstore := store.NewWithClock(db, clock)
	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/changeset-event-connection-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	spec := &batches.BatchSpec{
		NamespaceUserID: userID,
		UserID:          userID,
	}
	if err := cstore.CreateBatchSpec(ctx, spec); err != nil {
		t.Fatal(err)
	}

	batchChange := &batches.BatchChange{
		Name:             "my-unique-name",
		NamespaceUserID:  userID,
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
		BatchSpecID:      spec.ID,
	}
	if err := cstore.CreateBatchChange(ctx, batchChange); err != nil {
		t.Fatal(err)
	}

	changeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:                repo.ID,
		ExternalServiceType: "github",
		PublicationState:    batches.ChangesetPublicationStateUnpublished,
		ExternalReviewState: batches.ChangesetReviewStatePending,
		OwnedByBatchChange:  batchChange.ID,
		BatchChange:         batchChange.ID,
		Metadata: &github.PullRequest{
			TimelineItems: []github.TimelineItem{
				{Type: "PullRequestCommit", Item: &github.PullRequestCommit{
					Commit: github.Commit{
						OID: "d34db33f",
					},
				}},
				{Type: "LabeledEvent", Item: &github.LabelEvent{
					Label: github.Label{
						ID:    "label-event",
						Name:  "cool-label",
						Color: "blue",
					},
				}},
			},
		},
	})

	// Create ChangesetEvents from the timeline items in the metadata.
	events := changeset.Events()
	if err := cstore.UpsertChangesetEvents(ctx, events...); err != nil {
		t.Fatal(err)
	}

	addChangeset(t, ctx, cstore, changeset, batchChange.ID)

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	changesetAPIID := string(marshalChangesetID(changeset.ID))
	nodes := []apitest.ChangesetEvent{
		{
			ID:        string(marshalChangesetEventID(events[0].ID)),
			Changeset: struct{ ID string }{ID: changesetAPIID},
			CreatedAt: marshalDateTime(t, now),
		},
		{
			ID:        string(marshalChangesetEventID(events[1].ID)),
			Changeset: struct{ ID string }{ID: changesetAPIID},
			CreatedAt: marshalDateTime(t, now),
		},
	}

	tests := []struct {
		firstParam      int
		wantHasNextPage bool
		wantTotalCount  int
		wantNodes       []apitest.ChangesetEvent
	}{
		{firstParam: 1, wantHasNextPage: true, wantTotalCount: 2, wantNodes: nodes[:1]},
		{firstParam: 2, wantHasNextPage: false, wantTotalCount: 2, wantNodes: nodes},
		{firstParam: 3, wantHasNextPage: false, wantTotalCount: 2, wantNodes: nodes},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("first=%d", tc.firstParam), func(t *testing.T) {
			input := map[string]interface{}{"changeset": changesetAPIID, "first": int64(tc.firstParam)}
			var response struct{ Node apitest.Changeset }
			apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryChangesetEventConnection)

			wantEvents := apitest.ChangesetEventConnection{
				TotalCount: tc.wantTotalCount,
				PageInfo: apitest.PageInfo{
					HasNextPage: tc.wantHasNextPage,
					// This test doesn't check on the cursors, the below test does that.
					EndCursor: response.Node.Events.PageInfo.EndCursor,
				},
				Nodes: tc.wantNodes,
			}

			if diff := cmp.Diff(wantEvents, response.Node.Events); diff != "" {
				t.Fatalf("wrong changesets response (-want +got):\n%s", diff)
			}
		})
	}

	var endCursor *string
	for i := range nodes {
		input := map[string]interface{}{"changeset": changesetAPIID, "first": 1}
		if endCursor != nil {
			input["after"] = *endCursor
		}
		wantHasNextPage := i != len(nodes)-1

		var response struct{ Node apitest.Changeset }
		apitest.MustExec(ctx, t, s, input, &response, queryChangesetEventConnection)

		events := response.Node.Events
		if diff := cmp.Diff(1, len(events.Nodes)); diff != "" {
			t.Fatalf("unexpected number of nodes (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(len(nodes), events.TotalCount); diff != "" {
			t.Fatalf("unexpected total count (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(wantHasNextPage, events.PageInfo.HasNextPage); diff != "" {
			t.Fatalf("unexpected hasNextPage (-want +got):\n%s", diff)
		}

		endCursor = events.PageInfo.EndCursor
		if want, have := wantHasNextPage, endCursor != nil; have != want {
			t.Fatalf("unexpected endCursor existence. want=%t, have=%t", want, have)
		}
	}
}

const queryChangesetEventConnection = `
query($changeset: ID!, $first: Int, $after: String){
  node(id: $changeset) {
    ... on ExternalChangeset {
      events(first: $first, after: $after) {
        totalCount
        pageInfo {
          hasNextPage
          endCursor
        }
        nodes {
         id
         createdAt
         changeset {
           id
         }
        }
      }
    }
  }
}
`
