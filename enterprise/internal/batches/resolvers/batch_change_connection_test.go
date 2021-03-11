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
)

func TestBatchChangeConnectionResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID

	cstore := store.New(db)
	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/batch-change-connection-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	spec1 := &batches.BatchSpec{
		NamespaceUserID: userID,
		UserID:          userID,
	}
	if err := cstore.CreateBatchSpec(ctx, spec1); err != nil {
		t.Fatal(err)
	}
	spec2 := &batches.BatchSpec{
		NamespaceUserID: userID,
		UserID:          userID,
	}
	if err := cstore.CreateBatchSpec(ctx, spec2); err != nil {
		t.Fatal(err)
	}

	batchChange1 := &batches.BatchChange{
		Name:             "my-unique-name",
		NamespaceUserID:  userID,
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
		BatchSpecID:      spec1.ID,
	}
	if err := cstore.CreateBatchChange(ctx, batchChange1); err != nil {
		t.Fatal(err)
	}
	batchChange2 := &batches.BatchChange{
		Name:             "my-other-unique-name",
		NamespaceUserID:  userID,
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
		BatchSpecID:      spec2.ID,
	}
	if err := cstore.CreateBatchChange(ctx, batchChange2); err != nil {
		t.Fatal(err)
	}

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Batch changes are returned in reverse order.
	nodes := []apitest.BatchChange{
		{
			ID: string(marshalBatchChangeID(batchChange2.ID)),
		},
		{
			ID: string(marshalBatchChangeID(batchChange1.ID)),
		},
	}

	tests := []struct {
		firstParam      int
		wantHasNextPage bool
		wantTotalCount  int
		wantNodes       []apitest.BatchChange
	}{
		{firstParam: 1, wantHasNextPage: true, wantTotalCount: 2, wantNodes: nodes[:1]},
		{firstParam: 2, wantHasNextPage: false, wantTotalCount: 2, wantNodes: nodes},
		{firstParam: 3, wantHasNextPage: false, wantTotalCount: 2, wantNodes: nodes},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("first=%d", tc.firstParam), func(t *testing.T) {
			input := map[string]interface{}{"first": int64(tc.firstParam)}
			var response struct{ BatchChanges apitest.BatchChangeConnection }
			apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryBatchChangesConnection)

			wantConnection := apitest.BatchChangeConnection{
				TotalCount: tc.wantTotalCount,
				PageInfo: apitest.PageInfo{
					HasNextPage: tc.wantHasNextPage,
					// We don't test on the cursor here.
					EndCursor: response.BatchChanges.PageInfo.EndCursor,
				},
				Nodes: tc.wantNodes,
			}

			if diff := cmp.Diff(wantConnection, response.BatchChanges); diff != "" {
				t.Fatalf("wrong batchChanges response (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("Cursor based pagination", func(t *testing.T) {
		var endCursor *string
		for i := range nodes {
			input := map[string]interface{}{"first": 1}
			if endCursor != nil {
				input["after"] = *endCursor
			}
			wantHasNextPage := i != len(nodes)-1

			var response struct{ BatchChanges apitest.BatchChangeConnection }
			apitest.MustExec(ctx, t, s, input, &response, queryBatchChangesConnection)

			if diff := cmp.Diff(1, len(response.BatchChanges.Nodes)); diff != "" {
				t.Fatalf("unexpected number of nodes (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(len(nodes), response.BatchChanges.TotalCount); diff != "" {
				t.Fatalf("unexpected total count (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(wantHasNextPage, response.BatchChanges.PageInfo.HasNextPage); diff != "" {
				t.Fatalf("unexpected hasNextPage (-want +got):\n%s", diff)
			}

			endCursor = response.BatchChanges.PageInfo.EndCursor
			if want, have := wantHasNextPage, endCursor != nil; have != want {
				t.Fatalf("unexpected endCursor existence. want=%t, have=%t", want, have)
			}
		}
	})
}

const queryBatchChangesConnection = `
query($first: Int, $after: String) {
  batchChanges(first: $first, after: $after) {
    totalCount
    pageInfo {
	  hasNextPage
	  endCursor
    }
    nodes {
      id
    }
  }
}
`

func TestBatchChangesListing(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))

	orgID := ct.InsertTestOrg(t, db, "org")

	store := store.New(db)

	r := &Resolver{store: store}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	createBatchSpec := func(t *testing.T, spec *batches.BatchSpec) {
		t.Helper()

		spec.UserID = userID
		spec.NamespaceUserID = userID
		if err := store.CreateBatchSpec(ctx, spec); err != nil {
			t.Fatal(err)
		}
	}

	createBatchChange := func(t *testing.T, c *batches.BatchChange) {
		t.Helper()

		c.Name = "n"
		c.InitialApplierID = userID
		if err := store.CreateBatchChange(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("listing a users batch changes", func(t *testing.T) {
		spec := &batches.BatchSpec{}
		createBatchSpec(t, spec)

		batchChange := &batches.BatchChange{
			NamespaceUserID: userID,
			BatchSpecID:     spec.ID,
			LastApplierID:   userID,
			LastAppliedAt:   time.Now(),
		}
		createBatchChange(t, batchChange)

		userAPIID := string(graphqlbackend.MarshalUserID(userID))
		input := map[string]interface{}{"node": userAPIID}

		var response struct{ Node apitest.User }
		apitest.MustExec(actorCtx, t, s, input, &response, listNamespacesBatchChanges)

		want := apitest.User{
			ID: userAPIID,
			BatchChanges: apitest.BatchChangeConnection{
				TotalCount: 1,
				Nodes: []apitest.BatchChange{
					{ID: string(marshalBatchChangeID(batchChange.ID))},
				},
			},
		}

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
		}
	})

	t.Run("listing an orgs batch changes", func(t *testing.T) {
		spec := &batches.BatchSpec{}
		createBatchSpec(t, spec)

		batchChange := &batches.BatchChange{
			NamespaceOrgID: orgID,
			BatchSpecID:    spec.ID,
			LastApplierID:  userID,
			LastAppliedAt:  time.Now(),
		}
		createBatchChange(t, batchChange)

		orgAPIID := string(graphqlbackend.MarshalOrgID(orgID))
		input := map[string]interface{}{"node": orgAPIID}

		var response struct{ Node apitest.Org }
		apitest.MustExec(actorCtx, t, s, input, &response, listNamespacesBatchChanges)

		want := apitest.Org{
			ID: orgAPIID,
			BatchChanges: apitest.BatchChangeConnection{
				TotalCount: 1,
				Nodes: []apitest.BatchChange{
					{ID: string(marshalBatchChangeID(batchChange.ID))},
				},
			},
		}

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
		}
	})
}

const listNamespacesBatchChanges = `
query($node: ID!) {
  node(id: $node) {
    ... on User {
      id
      batchChanges {
        totalCount
        nodes {
          id
        }
      }
    }

    ... on Org {
      id
      batchChanges {
        totalCount
        nodes {
          id
        }
      }
    }
  }
}
`
