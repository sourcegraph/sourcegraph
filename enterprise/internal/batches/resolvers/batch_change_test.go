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
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestBatchChangeResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID
	orgName := "test-batch-change-resolver-org"
	orgID := ct.InsertTestOrg(t, db, orgName)

	now := timeutil.Now()
	clock := func() time.Time { return now }
	cstore := store.NewWithClock(db, clock)

	batchSpec := &batches.BatchSpec{
		RawSpec:        ct.TestRawBatchSpec,
		UserID:         userID,
		NamespaceOrgID: orgID,
	}
	if err := cstore.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	batchChange := &batches.BatchChange{
		Name:             "my-unique-name",
		Description:      "The batch change description",
		NamespaceOrgID:   orgID,
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    now,
		BatchSpecID:      batchSpec.ID,
	}
	if err := cstore.CreateBatchChange(ctx, batchChange); err != nil {
		t.Fatal(err)
	}

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	batchChangeAPIID := string(marshalBatchChangeID(batchChange.ID))
	namespaceAPIID := string(graphqlbackend.MarshalOrgID(orgID))
	apiUser := &apitest.User{DatabaseID: userID, SiteAdmin: true}
	wantBatchChange := apitest.BatchChange{
		ID:             batchChangeAPIID,
		Name:           batchChange.Name,
		Description:    batchChange.Description,
		Namespace:      apitest.UserOrg{ID: namespaceAPIID, Name: orgName},
		InitialApplier: apiUser,
		LastApplier:    apiUser,
		SpecCreator:    apiUser,
		LastAppliedAt:  marshalDateTime(t, now),
		URL:            fmt.Sprintf("/organizations/%s/batch-changes/%s", orgName, batchChange.Name),
		CreatedAt:      marshalDateTime(t, now),
		UpdatedAt:      marshalDateTime(t, now),
		// Not closed.
		ClosedAt: "",
	}

	input := map[string]interface{}{"batchChange": batchChangeAPIID}
	{
		var response struct{ Node apitest.BatchChange }
		apitest.MustExec(ctx, t, s, input, &response, queryBatchChange)

		if diff := cmp.Diff(wantBatchChange, response.Node); diff != "" {
			t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
		}
	}
	// Test resolver by namespace and name
	byNameInput := map[string]interface{}{"name": batchChange.Name, "namespace": namespaceAPIID}
	{
		var response struct{ BatchChange apitest.BatchChange }
		apitest.MustExec(ctx, t, s, byNameInput, &response, queryBatchChangeByName)

		if diff := cmp.Diff(wantBatchChange, response.BatchChange); diff != "" {
			t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
		}
	}

	// Now soft-delete the user and check we can still access the batch change in the org namespace.
	err = database.UsersWith(cstore).Delete(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	wantBatchChange.InitialApplier = nil
	wantBatchChange.LastApplier = nil
	wantBatchChange.SpecCreator = nil

	{
		var response struct{ Node apitest.BatchChange }
		apitest.MustExec(ctx, t, s, input, &response, queryBatchChange)

		if diff := cmp.Diff(wantBatchChange, response.Node); diff != "" {
			t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
		}
	}

	// Now hard-delete the user and check we can still access the batch change in the org namespace.
	err = database.UsersWith(cstore).HardDelete(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	{
		var response struct{ Node apitest.BatchChange }
		apitest.MustExec(ctx, t, s, input, &response, queryBatchChange)

		if diff := cmp.Diff(wantBatchChange, response.Node); diff != "" {
			t.Fatalf("wrong batch change response (-want +got):\n%s", diff)
		}
	}
}

const queryBatchChange = `
fragment u on User { databaseID, siteAdmin }
fragment o on Org  { id, name }

query($batchChange: ID!){
  node(id: $batchChange) {
    ... on BatchChange {
      id, name, description
      initialApplier { ...u }
      lastApplier    { ...u }
      specCreator    { ...u }
      lastAppliedAt
      createdAt
      updatedAt
      closedAt
      namespace {
        ... on User { ...u }
        ... on Org  { ...o }
      }
      url
    }
  }
}
`
const queryBatchChangeByName = `
fragment u on User { databaseID, siteAdmin }
fragment o on Org  { id, name }

query($namespace: ID!, $name: String!){
  batchChange(namespace: $namespace, name: $name) {
    id, name, description
    initialApplier { ...u }
    lastApplier    { ...u }
    specCreator    { ...u }
    lastAppliedAt
    createdAt
    updatedAt
    closedAt
    namespace {
      ... on User { ...u }
      ... on Org  { ...o }
    }
    url
  }
}
`
