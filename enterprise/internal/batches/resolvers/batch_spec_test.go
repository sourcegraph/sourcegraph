package resolvers

import (
	"context"
	"encoding/json"
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
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
)

func TestBatchSpecResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	cstore := store.New(db)
	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/batch-spec-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}
	repoID := graphqlbackend.MarshalRepositoryID(repo.ID)

	orgname := "test-org"
	userID := ct.CreateTestUser(t, db, false).ID
	adminID := ct.CreateTestUser(t, db, true).ID
	orgID := ct.InsertTestOrg(t, db, orgname)

	spec, err := batches.NewBatchSpecFromRaw(ct.TestRawBatchSpec)
	if err != nil {
		t.Fatal(err)
	}
	spec.UserID = userID
	spec.NamespaceOrgID = orgID
	if err := cstore.CreateBatchSpec(ctx, spec); err != nil {
		t.Fatal(err)
	}

	changesetSpec, err := batches.NewChangesetSpecFromRaw(ct.NewRawChangesetSpecGitBranch(repoID, "deadb33f"))
	if err != nil {
		t.Fatal(err)
	}
	changesetSpec.BatchSpecID = spec.ID
	changesetSpec.UserID = userID
	changesetSpec.RepoID = repo.ID

	if err := cstore.CreateChangesetSpec(ctx, changesetSpec); err != nil {
		t.Fatal(err)
	}

	matchingBatchChange := &batches.BatchChange{
		Name:             spec.Spec.Name,
		NamespaceOrgID:   orgID,
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
		BatchSpecID:      spec.ID,
	}
	if err := cstore.CreateBatchChange(ctx, matchingBatchChange); err != nil {
		t.Fatal(err)
	}

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	apiID := string(marshalBatchSpecRandID(spec.RandID))
	userAPIID := string(graphqlbackend.MarshalUserID(userID))
	orgAPIID := string(graphqlbackend.MarshalOrgID(orgID))

	var unmarshaled interface{}
	err = json.Unmarshal([]byte(spec.RawSpec), &unmarshaled)
	if err != nil {
		t.Fatal(err)
	}

	want := apitest.BatchSpec{
		Typename: "BatchSpec",
		ID:       apiID,

		OriginalInput: spec.RawSpec,
		ParsedInput:   graphqlbackend.JSONValue{Value: unmarshaled},

		ApplyURL:            fmt.Sprintf("/organizations/%s/batch-changes/apply/%s", orgname, apiID),
		Namespace:           apitest.UserOrg{ID: orgAPIID, Name: orgname},
		Creator:             &apitest.User{ID: userAPIID, DatabaseID: userID},
		ViewerCanAdminister: true,

		CreatedAt: graphqlbackend.DateTime{Time: spec.CreatedAt.Truncate(time.Second)},
		ExpiresAt: &graphqlbackend.DateTime{Time: spec.ExpiresAt().Truncate(time.Second)},

		ChangesetSpecs: apitest.ChangesetSpecConnection{
			TotalCount: 1,
			Nodes: []apitest.ChangesetSpec{
				{
					ID:       string(marshalChangesetSpecRandID(changesetSpec.RandID)),
					Typename: "VisibleChangesetSpec",
					Description: apitest.ChangesetSpecDescription{
						BaseRepository: apitest.Repository{
							ID:   string(repoID),
							Name: string(repo.Name),
						},
					},
				},
			},
		},

		DiffStat: apitest.DiffStat{
			Added:   changesetSpec.DiffStatAdded,
			Changed: changesetSpec.DiffStatChanged,
			Deleted: changesetSpec.DiffStatDeleted,
		},

		AppliesToBatchChange: apitest.BatchChange{
			ID: string(marshalBatchChangeID(matchingBatchChange.ID)),
		},

		AllCodeHosts: apitest.BatchChangesCodeHostsConnection{
			TotalCount: 1,
			Nodes:      []apitest.BatchChangesCodeHost{{ExternalServiceKind: extsvc.KindGitHub, ExternalServiceURL: "https://github.com/"}},
		},
		OnlyWithoutCredential: apitest.BatchChangesCodeHostsConnection{
			TotalCount: 1,
			Nodes:      []apitest.BatchChangesCodeHost{{ExternalServiceKind: extsvc.KindGitHub, ExternalServiceURL: "https://github.com/"}},
		},
	}

	input := map[string]interface{}{"batchSpec": apiID}
	{
		var response struct{ Node apitest.BatchSpec }
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryBatchSpecNode)

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
	}

	// Now create an updated changeset spec and check that we get a superseding
	// batch spec.
	sup, err := batches.NewBatchSpecFromRaw(ct.TestRawBatchSpec)
	if err != nil {
		t.Fatal(err)
	}
	sup.UserID = userID
	sup.NamespaceOrgID = orgID
	if err := cstore.CreateBatchSpec(ctx, sup); err != nil {
		t.Fatal(err)
	}

	{
		var response struct{ Node apitest.BatchSpec }

		// Note that we have to execute as the actual user, since a superseding
		// spec isn't returned for an admin.
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryBatchSpecNode)

		// Expect an ID on the superseding batch spec.
		want.SupersedingBatchSpec = &apitest.BatchSpec{
			ID: string(marshalBatchSpecRandID(sup.RandID)),
		}

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
	}

	// If the superseding batch spec was created by a different user, then we
	// shouldn't return it.
	sup.UserID = adminID
	if err := cstore.UpdateBatchSpec(ctx, sup); err != nil {
		t.Fatal(err)
	}

	{
		var response struct{ Node apitest.BatchSpec }

		// Note that we have to execute as the actual user, since a superseding
		// spec isn't returned for an admin.
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryBatchSpecNode)

		// Expect no superseding batch spec, since this request is run as a
		// different user.
		want.SupersedingBatchSpec = nil

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
	}

	// Now soft-delete the creator and check that the batch spec is still retrievable.
	err = database.UsersWith(cstore).Delete(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	{
		var response struct{ Node apitest.BatchSpec }
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(adminID)), t, s, input, &response, queryBatchSpecNode)

		// Expect creator to not be returned anymore.
		want.Creator = nil
		// Expect all set for admin user.
		want.OnlyWithoutCredential = apitest.BatchChangesCodeHostsConnection{
			Nodes: []apitest.BatchChangesCodeHost{},
		}
		// Expect no superseding batch spec, since this request is run as a
		// different user.
		want.SupersedingBatchSpec = nil

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
	}

	// Now hard-delete the creator and check that the batch spec is still retrievable.
	err = database.UsersWith(cstore).HardDelete(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	{
		var response struct{ Node apitest.BatchSpec }
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(adminID)), t, s, input, &response, queryBatchSpecNode)

		// Expect creator to not be returned anymore.
		want.Creator = nil
		// Expect all set for admin user.
		want.OnlyWithoutCredential = apitest.BatchChangesCodeHostsConnection{
			Nodes: []apitest.BatchChangesCodeHost{},
		}

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("unexpected response (-want +got):\n%s", diff)
		}
	}
}

const queryBatchSpecNode = `
fragment u on User { id, databaseID, siteAdmin }
fragment o on Org  { id, name }

query($batchSpec: ID!) {
  node(id: $batchSpec) {
    __typename

    ... on BatchSpec {
      id
      originalInput
      parsedInput

      creator  { ...u }
      namespace {
        ... on User { ...u }
        ... on Org  { ...o }
      }

      applyURL
      viewerCanAdminister

      createdAt
      expiresAt

      diffStat { added, deleted, changed }

	  appliesToBatchChange { id }
	  supersedingBatchSpec { id }

	  allCodeHosts: viewerBatchChangesCodeHosts {
		totalCount
		  nodes {
			  externalServiceKind
			  externalServiceURL
		  }
	  }

	  onlyWithoutCredential: viewerBatchChangesCodeHosts(onlyWithoutCredential: true) {
		  totalCount
		  nodes {
			  externalServiceKind
			  externalServiceURL
		  }
	  }

      changesetSpecs(first: 100) {
        totalCount

        nodes {
          __typename
          type

          ... on HiddenChangesetSpec {
            id
          }

          ... on VisibleChangesetSpec {
            id

            description {
              ... on ExistingChangesetReference {
                baseRepository {
                  id
                  name
                }
              }

              ... on GitBranchChangesetDescription {
                baseRepository {
                  id
                  name
                }
              }
            }
          }
        }
	  }
    }
  }
}
`
