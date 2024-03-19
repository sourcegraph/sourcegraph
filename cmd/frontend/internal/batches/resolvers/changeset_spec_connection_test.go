package resolvers

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestChangesetSpecConnectionResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))

	userID := bt.CreateTestUser(t, db, false).ID

	bstore := store.New(db, &observation.TestContext, nil)

	batchSpec := &btypes.BatchSpec{
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := bstore.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	repoStore := database.ReposWith(logger, bstore)
	esStore := database.ExternalServicesWith(logger, bstore)

	rs := make([]*types.Repo, 0, 3)
	for i := range cap(rs) {
		name := fmt.Sprintf("github.com/sourcegraph/test-changeset-spec-connection-repo-%d", i)
		r := newGitHubTestRepo(name, newGitHubExternalService(t, esStore))
		if err := repoStore.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
		rs = append(rs, r)
	}

	changesetSpecs := make([]*btypes.ChangesetSpec, 0, len(rs))
	for _, r := range rs {
		repoID := graphqlbackend.MarshalRepositoryID(r.ID)
		s, err := btypes.NewChangesetSpecFromRaw(bt.NewRawChangesetSpecGitBranch(repoID, "d34db33f"))
		if err != nil {
			t.Fatal(err)
		}
		s.BatchSpecID = batchSpec.ID
		s.UserID = userID
		s.BaseRepoID = r.ID

		if err := bstore.CreateChangesetSpec(ctx, s); err != nil {
			t.Fatal(err)
		}

		changesetSpecs = append(changesetSpecs, s)
	}

	s, err := newSchema(db, &Resolver{store: bstore})
	if err != nil {
		t.Fatal(err)
	}

	apiID := string(marshalBatchSpecRandID(batchSpec.RandID))

	tests := []struct {
		first int

		wantTotalCount  int
		wantHasNextPage bool
	}{
		{first: 1, wantTotalCount: 3, wantHasNextPage: true},
		{first: 2, wantTotalCount: 3, wantHasNextPage: true},
		{first: 3, wantTotalCount: 3, wantHasNextPage: false},
	}

	for _, tc := range tests {
		input := map[string]any{"batchSpec": apiID, "first": tc.first}
		var response struct{ Node apitest.BatchSpec }
		apitest.MustExec(ctx, t, s, input, &response, queryChangesetSpecConnection)

		specs := response.Node.ChangesetSpecs
		if diff := cmp.Diff(tc.wantTotalCount, specs.TotalCount); diff != "" {
			t.Fatalf("first=%d, unexpected total count (-want +got):\n%s", tc.first, diff)
		}

		if diff := cmp.Diff(tc.wantHasNextPage, specs.PageInfo.HasNextPage); diff != "" {
			t.Fatalf("first=%d, unexpected hasNextPage (-want +got):\n%s", tc.first, diff)
		}
	}

	var endCursor *string
	for i := range changesetSpecs {
		input := map[string]any{"batchSpec": apiID, "first": 1}
		if endCursor != nil {
			input["after"] = *endCursor
		}
		wantHasNextPage := i != len(changesetSpecs)-1

		var response struct{ Node apitest.BatchSpec }
		apitest.MustExec(ctx, t, s, input, &response, queryChangesetSpecConnection)

		specs := response.Node.ChangesetSpecs
		if diff := cmp.Diff(1, len(specs.Nodes)); diff != "" {
			t.Fatalf("unexpected number of nodes (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(len(changesetSpecs), specs.TotalCount); diff != "" {
			t.Fatalf("unexpected total count (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(wantHasNextPage, specs.PageInfo.HasNextPage); diff != "" {
			t.Fatalf("unexpected hasNextPage (-want +got):\n%s", diff)
		}

		endCursor = specs.PageInfo.EndCursor
		if want, have := wantHasNextPage, endCursor != nil; have != want {
			t.Fatalf("unexpected endCursor existence. want=%t, have=%t", want, have)
		}
	}
}

const queryChangesetSpecConnection = `
query($batchSpec: ID!, $first: Int!, $after: String) {
  node(id: $batchSpec) {
    __typename

    ... on BatchSpec {
      id

      changesetSpecs(first: $first, after: $after) {
        totalCount
        pageInfo { hasNextPage, endCursor }

        nodes {
          __typename
          ... on HiddenChangesetSpec { id }
          ... on VisibleChangesetSpec { id }
        }
      }
    }
  }
}
`
