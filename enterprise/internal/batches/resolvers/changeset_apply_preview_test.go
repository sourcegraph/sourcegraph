package resolvers

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestChangesetApplyPreviewResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, false).ID

	cstore := store.New(db)

	// Create a batch spec for the target batch change.
	oldBatchSpec := &batches.BatchSpec{
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := cstore.CreateBatchSpec(ctx, oldBatchSpec); err != nil {
		t.Fatal(err)
	}
	// Create a batch change and create a new spec targetting the same batch change again.
	batchChangeName := "test-apply-preview-resolver"
	batchChange := ct.CreateBatchChange(t, ctx, cstore, batchChangeName, userID, oldBatchSpec.ID)
	batchSpec := ct.CreateBatchSpec(t, ctx, cstore, batchChangeName, userID)

	esStore := database.ExternalServicesWith(cstore)
	repoStore := database.ReposWith(cstore)

	rs := make([]*types.Repo, 0, 3)
	for i := 0; i < cap(rs); i++ {
		name := fmt.Sprintf("github.com/sourcegraph/test-changeset-apply-preview-repo-%d", i)
		r := newGitHubTestRepo(name, newGitHubExternalService(t, esStore))
		if err := repoStore.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
		rs = append(rs, r)
	}

	changesetSpecs := make([]*batches.ChangesetSpec, 0, 2)
	for i, r := range rs[:2] {
		s := ct.CreateChangesetSpec(t, ctx, cstore, ct.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      r.ID,
			HeadRef:   fmt.Sprintf("d34db33f-%d", i),
		})

		changesetSpecs = append(changesetSpecs, s)
	}

	// Add one changeset that doesn't match any new spec anymore but was there before (close, detach).
	closingChangesetSpec := ct.CreateChangesetSpec(t, ctx, cstore, ct.TestSpecOpts{
		User:      userID,
		Repo:      rs[2].ID,
		BatchSpec: oldBatchSpec.ID,
		HeadRef:   "d34db33f-2",
	})
	closingChangeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:             rs[2].ID,
		BatchChange:      batchChange.ID,
		CurrentSpec:      closingChangesetSpec.ID,
		PublicationState: batches.ChangesetPublicationStatePublished,
	})

	// Add one changeset that doesn't matches a new spec (update).
	updatedChangesetSpec := ct.CreateChangesetSpec(t, ctx, cstore, ct.TestSpecOpts{
		BatchSpec: oldBatchSpec.ID,
		User:      userID,
		Repo:      changesetSpecs[1].RepoID,
		HeadRef:   changesetSpecs[1].Spec.HeadRef,
	})
	updatedChangeset := ct.CreateChangeset(t, ctx, cstore, ct.TestChangesetOpts{
		Repo:               rs[1].ID,
		BatchChange:        batchChange.ID,
		CurrentSpec:        updatedChangesetSpec.ID,
		PublicationState:   batches.ChangesetPublicationStatePublished,
		OwnedByBatchChange: batchChange.ID,
	})

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	apiID := string(marshalBatchSpecRandID(batchSpec.RandID))

	input := map[string]interface{}{"batchSpec": apiID}
	var response struct{ Node apitest.BatchSpec }
	apitest.MustExec(ctx, t, s, input, &response, queryChangesetApplyPreview)

	haveApplyPreview := response.Node.ApplyPreview.Nodes

	wantApplyPreview := []apitest.ChangesetApplyPreview{
		{
			Typename:   "VisibleChangesetApplyPreview",
			Operations: []batches.ReconcilerOperation{batches.ReconcilerOperationDetach},
			Targets: apitest.ChangesetApplyPreviewTargets{
				Typename:  "VisibleApplyPreviewTargetsDetach",
				Changeset: apitest.Changeset{ID: string(marshalChangesetID(closingChangeset.ID))},
			},
		},
		{
			Typename:   "VisibleChangesetApplyPreview",
			Operations: []batches.ReconcilerOperation{},
			Targets: apitest.ChangesetApplyPreviewTargets{
				Typename:      "VisibleApplyPreviewTargetsAttach",
				ChangesetSpec: apitest.ChangesetSpec{ID: string(marshalChangesetSpecRandID(changesetSpecs[0].RandID))},
			},
		},
		{
			Typename:   "VisibleChangesetApplyPreview",
			Operations: []batches.ReconcilerOperation{},
			Targets: apitest.ChangesetApplyPreviewTargets{
				Typename:      "VisibleApplyPreviewTargetsUpdate",
				ChangesetSpec: apitest.ChangesetSpec{ID: string(marshalChangesetSpecRandID(changesetSpecs[1].RandID))},
				Changeset:     apitest.Changeset{ID: string(marshalChangesetID(updatedChangeset.ID))},
			},
		},
	}

	if diff := cmp.Diff(wantApplyPreview, haveApplyPreview); diff != "" {
		t.Fatalf("unexpected response (-want +got):\n%s", diff)
	}
}

const queryChangesetApplyPreview = `
query ($batchSpec: ID!) {
    node(id: $batchSpec) {
      __typename
      ... on BatchSpec {
        id
        applyPreview {
          totalCount
          pageInfo {
            hasNextPage
            endCursor
          }
          nodes {
            __typename
            ... on VisibleChangesetApplyPreview {
			  operations
              delta {
                titleChanged
                bodyChanged
                undraft
                baseRefChanged
                diffChanged
                commitMessageChanged
                authorNameChanged
                authorEmailChanged
              }
              targets {
                __typename
                ... on VisibleApplyPreviewTargetsAttach {
                  changesetSpec {
                    id
                  }
                }
                ... on VisibleApplyPreviewTargetsUpdate {
                  changesetSpec {
                    id
                  }
                  changeset {
                    id
                  }
                }
                ... on VisibleApplyPreviewTargetsDetach {
                  changeset {
                    id
                  }
                }
              }
            }
            ... on HiddenChangesetApplyPreview {
              operations
              targets {
                __typename
                ... on HiddenApplyPreviewTargetsAttach {
                  changesetSpec {
                    id
                  }
                }
                ... on HiddenApplyPreviewTargetsUpdate {
                  changesetSpec {
                    id
                  }
                  changeset {
                    id
                  }
                }
                ... on HiddenApplyPreviewTargetsDetach {
                  changeset {
                    id
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
