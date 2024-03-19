package resolvers

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	bgql "github.com/sourcegraph/sourcegraph/internal/batches/graphql"
	"github.com/sourcegraph/sourcegraph/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/batches"
)

func TestChangesetApplyPreviewResolver(t *testing.T) {
	logger := logtest.Scoped(t)
	if testing.Short() {
		t.Skip()
	}

	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))

	userID := bt.CreateTestUser(t, db, false).ID

	bstore := store.New(db, &observation.TestContext, nil)

	// Create a batch spec for the target batch change.
	oldBatchSpec := &btypes.BatchSpec{
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := bstore.CreateBatchSpec(ctx, oldBatchSpec); err != nil {
		t.Fatal(err)
	}
	// Create a batch change and create a new spec targetting the same batch change again.
	batchChangeName := "test-apply-preview-resolver"
	batchChange := bt.CreateBatchChange(t, ctx, bstore, batchChangeName, userID, oldBatchSpec.ID)
	batchSpec := bt.CreateBatchSpec(t, ctx, bstore, batchChangeName, userID, batchChange.ID)

	esStore := database.ExternalServicesWith(logger, bstore)
	repoStore := database.ReposWith(logger, bstore)

	rs := make([]*types.Repo, 0, 3)
	for i := range cap(rs) {
		name := fmt.Sprintf("github.com/sourcegraph/test-changeset-apply-preview-repo-%d", i)
		r := newGitHubTestRepo(name, newGitHubExternalService(t, esStore))
		if err := repoStore.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
		rs = append(rs, r)
	}

	changesetSpecs := make([]*btypes.ChangesetSpec, 0, 2)
	for i, r := range rs[:2] {
		s := bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      r.ID,
			HeadRef:   fmt.Sprintf("d34db33f-%d", i),
			Typ:       btypes.ChangesetSpecTypeBranch,
		})

		changesetSpecs = append(changesetSpecs, s)
	}

	// Add one changeset that doesn't match any new spec anymore but was there before (close, detach).
	closingChangesetSpec := bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
		User:      userID,
		Repo:      rs[2].ID,
		BatchSpec: oldBatchSpec.ID,
		HeadRef:   "d34db33f-2",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})
	closingChangeset := bt.CreateChangeset(t, ctx, bstore, bt.TestChangesetOpts{
		Repo:             rs[2].ID,
		BatchChange:      batchChange.ID,
		CurrentSpec:      closingChangesetSpec.ID,
		PublicationState: btypes.ChangesetPublicationStatePublished,
	})

	// Add one changeset that doesn't matches a new spec (update).
	updatedChangesetSpec := bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
		BatchSpec: oldBatchSpec.ID,
		User:      userID,
		Repo:      changesetSpecs[1].BaseRepoID,
		HeadRef:   changesetSpecs[1].HeadRef,
		Typ:       btypes.ChangesetSpecTypeBranch,
	})
	updatedChangeset := bt.CreateChangeset(t, ctx, bstore, bt.TestChangesetOpts{
		Repo:               rs[1].ID,
		BatchChange:        batchChange.ID,
		CurrentSpec:        updatedChangesetSpec.ID,
		PublicationState:   btypes.ChangesetPublicationStatePublished,
		OwnedByBatchChange: batchChange.ID,
	})

	s, err := newSchema(db, &Resolver{store: bstore})
	if err != nil {
		t.Fatal(err)
	}

	apiID := string(marshalBatchSpecRandID(batchSpec.RandID))

	input := map[string]any{"batchSpec": apiID}
	var response struct{ Node apitest.BatchSpec }
	apitest.MustExec(ctx, t, s, input, &response, queryChangesetApplyPreview)

	haveApplyPreview := response.Node.ApplyPreview.Nodes

	wantApplyPreview := []apitest.ChangesetApplyPreview{
		{
			Typename:   "VisibleChangesetApplyPreview",
			Operations: []btypes.ReconcilerOperation{btypes.ReconcilerOperationDetach},
			Targets: apitest.ChangesetApplyPreviewTargets{
				Typename:  "VisibleApplyPreviewTargetsDetach",
				Changeset: apitest.Changeset{ID: string(bgql.MarshalChangesetID(closingChangeset.ID))},
			},
		},
		{
			Typename:   "VisibleChangesetApplyPreview",
			Operations: []btypes.ReconcilerOperation{},
			Targets: apitest.ChangesetApplyPreviewTargets{
				Typename:      "VisibleApplyPreviewTargetsAttach",
				ChangesetSpec: apitest.ChangesetSpec{ID: string(marshalChangesetSpecRandID(changesetSpecs[0].RandID))},
			},
		},
		{
			Typename:   "VisibleChangesetApplyPreview",
			Operations: []btypes.ReconcilerOperation{},
			Targets: apitest.ChangesetApplyPreviewTargets{
				Typename:      "VisibleApplyPreviewTargetsUpdate",
				ChangesetSpec: apitest.ChangesetSpec{ID: string(marshalChangesetSpecRandID(changesetSpecs[1].RandID))},
				Changeset:     apitest.Changeset{ID: string(bgql.MarshalChangesetID(updatedChangeset.ID))},
			},
		},
	}

	if diff := cmp.Diff(wantApplyPreview, haveApplyPreview); diff != "" {
		t.Fatalf("unexpected response (-want +got):\n%s", diff)
	}
}

const queryChangesetApplyPreview = `
query ($batchSpec: ID!, $first: Int = 50, $after: String, $publicationStates: [ChangesetSpecPublicationStateInput!]) {
    node(id: $batchSpec) {
      __typename
      ... on BatchSpec {
        id
        applyPreview(first: $first, after: $after, publicationStates: $publicationStates) {
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

func TestChangesetApplyPreviewResolverWithPublicationStates(t *testing.T) {
	// We have multiple scenarios to test here: these essentially act as
	// integration tests for the applyPreview() resolver when publication states
	// are set.
	//
	// The first is the case where we don't have a batch change yet (we're
	// applying a new batch spec), and some changeset specs have associated
	// publication states. We should get the appropriate actions on those
	// changeset specs.
	//
	// The second is the case where we do have a batch change, and we're
	// updating some publication states. Again, we should get the appropriate
	// actions.
	//
	// Another interesting case is ensuring that we handle a scenario where a
	// previously spec-published changeset is now UI-published (because the
	// published field was removed from the spec). This should result in no
	// action, since the changeset is already published.
	//
	// Finally, we need to ensure that providing a conflicting UI publication
	// state results in an error.
	//
	// As ever, let's start with some boilerplate.
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))

	userID := bt.CreateTestUser(t, db, false).ID

	bstore := store.New(db, &observation.TestContext, nil)
	esStore := database.ExternalServicesWith(logger, bstore)
	repoStore := database.ReposWith(logger, bstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/test", newGitHubExternalService(t, esStore))
	require.Nil(t, repoStore.Create(ctx, repo))

	s, err := newSchema(db, &Resolver{store: bstore})
	require.Nil(t, err)

	// To make it easier to assert against the operations in a preview node,
	// here are some canned operations that we expect when publishing.
	var (
		publishOps = []btypes.ReconcilerOperation{
			btypes.ReconcilerOperationPush,
			btypes.ReconcilerOperationPublish,
		}
		publishDraftOps = []btypes.ReconcilerOperation{
			btypes.ReconcilerOperationPush,
			btypes.ReconcilerOperationPublishDraft,
		}
		noOps = []btypes.ReconcilerOperation{}
	)

	t.Run("new batch change", func(t *testing.T) {
		fx := newApplyPreviewTestFixture(t, ctx, bstore, userID, repo.ID, "new")

		// We'll use a page size of 1 here to ensure that the publication states
		// are correctly handled across pages.
		previews := repeatApplyPreview(
			ctx, t, s,
			fx.DecorateInput(map[string]any{}),
			queryChangesetApplyPreview,
			1,
		)

		assert.Len(t, previews, 5)
		assertOperations(t, previews, fx.specPublished, publishOps)
		assertOperations(t, previews, fx.specToBePublished, publishOps)
		assertOperations(t, previews, fx.specToBeDraft, publishDraftOps)
		assertOperations(t, previews, fx.specToBeUnpublished, noOps)
		assertOperations(t, previews, fx.specToBeOmitted, noOps)
	})

	t.Run("existing batch change", func(t *testing.T) {
		createdFx := newApplyPreviewTestFixture(t, ctx, bstore, userID, repo.ID, "existing")

		// Apply the batch spec so we have an existing batch change.
		svc := service.New(bstore)
		batchChange, err := svc.ApplyBatchChange(ctx, service.ApplyBatchChangeOpts{
			BatchSpecRandID:   createdFx.batchSpec.RandID,
			PublicationStates: createdFx.DefaultUiPublicationStates(),
		})
		require.Nil(t, err)
		require.NotNil(t, batchChange)

		// Now we need a fresh batch spec.
		newFx := newApplyPreviewTestFixture(t, ctx, bstore, userID, repo.ID, "existing")

		// Same as above, but this time we'll use a page size of 2 just to mix
		// it up.
		previews := repeatApplyPreview(
			ctx, t, s,
			newFx.DecorateInput(map[string]any{}),
			queryChangesetApplyPreview,
			2,
		)

		assert.Len(t, previews, 5)
		assertOperations(t, previews, newFx.specPublished, publishOps)
		assertOperations(t, previews, newFx.specToBePublished, publishOps)
		assertOperations(t, previews, newFx.specToBeDraft, publishDraftOps)
		assertOperations(t, previews, newFx.specToBeUnpublished, noOps)
		assertOperations(t, previews, newFx.specToBeOmitted, noOps)
	})

	t.Run("already published changeset", func(t *testing.T) {
		// The set up on this is pretty similar to the previous test case, but
		// with the extra step of then modifying the relevant changeset to make
		// it look like it's been published.
		createdFx := newApplyPreviewTestFixture(t, ctx, bstore, userID, repo.ID, "already-published")

		// Apply the batch spec so we have an existing batch change.
		svc := service.New(bstore)
		batchChange, err := svc.ApplyBatchChange(ctx, service.ApplyBatchChangeOpts{
			BatchSpecRandID:   createdFx.batchSpec.RandID,
			PublicationStates: createdFx.DefaultUiPublicationStates(),
		})
		require.Nil(t, err)
		require.NotNil(t, batchChange)

		// Find the changeset for specPublished, and mock it up to look open.
		changesets, _, err := bstore.ListChangesets(ctx, store.ListChangesetsOpts{
			BatchChangeID: batchChange.ID,
		})
		require.Nil(t, err)
		for _, changeset := range changesets {
			if changeset.CurrentSpecID == createdFx.specPublished.ID {
				changeset.PublicationState = btypes.ChangesetPublicationStatePublished
				changeset.ExternalID = "12345"
				changeset.ExternalState = btypes.ChangesetExternalStateOpen
				require.Nil(t, bstore.UpdateChangeset(ctx, changeset))
				break
			}
		}

		// Now we need a fresh batch spec.
		newFx := newApplyPreviewTestFixture(t, ctx, bstore, userID, repo.ID, "already-published")

		// We need to modify the changeset spec to not have a published field.
		newFx.specPublished.Published = batches.PublishedValue{Val: nil}
		q := sqlf.Sprintf(`UPDATE changeset_specs SET published = %s WHERE id = %s`, nil, newFx.specPublished.ID)
		if _, err := db.ExecContext(context.Background(), q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
			t.Fatal(err)
		}

		// Same as above, but this time we'll use a page size of 3 just to mix
		// it up.
		previews := repeatApplyPreview(
			ctx, t, s,
			newFx.DecorateInput(map[string]any{
				"publicationStates": []map[string]any{
					{
						"changesetSpec":    marshalChangesetSpecRandID(newFx.specPublished.RandID),
						"publicationState": true,
					},
				},
			}),
			queryChangesetApplyPreview,
			3,
		)

		// The key point here is that specPublished has no operations, since
		// it's already published.
		assert.Len(t, previews, 5)
		assertOperations(t, previews, newFx.specPublished, noOps)
		assertOperations(t, previews, newFx.specToBePublished, publishOps)
		assertOperations(t, previews, newFx.specToBeDraft, publishDraftOps)
		assertOperations(t, previews, newFx.specToBeUnpublished, noOps)
		assertOperations(t, previews, newFx.specToBeOmitted, noOps)
	})

	t.Run("conflicting publication state", func(t *testing.T) {
		fx := newApplyPreviewTestFixture(t, ctx, bstore, userID, repo.ID, "conflicting")

		var response struct{ Node apitest.BatchSpec }
		err := apitest.Exec(
			ctx, t, s,
			fx.DecorateInput(map[string]any{
				"publicationStates": []map[string]any{
					{
						"changesetSpec":    marshalChangesetSpecRandID(fx.specPublished.RandID),
						"publicationState": true,
					},
				},
			}),
			&response,
			queryChangesetApplyPreview,
		)

		assert.Greater(t, len(err), 0)
		assert.Error(t, err[0])
	})
}

// assertOperations asserts that the given operations appear for the given
// changeset spec within the array of preview nodes.
func assertOperations(
	t *testing.T,
	previews []apitest.ChangesetApplyPreview,
	spec *btypes.ChangesetSpec,
	want []btypes.ReconcilerOperation,
) {
	t.Helper()

	preview := findPreviewForChangesetSpec(previews, spec)
	if preview == nil {
		t.Fatal("could not find changeset spec")
	}

	assert.Equal(t, want, preview.Operations)
}

func findPreviewForChangesetSpec(
	previews []apitest.ChangesetApplyPreview,
	spec *btypes.ChangesetSpec,
) *apitest.ChangesetApplyPreview {
	id := string(marshalChangesetSpecRandID(spec.RandID))
	for _, preview := range previews {
		if preview.Targets.ChangesetSpec.ID == id {
			return &preview
		}
	}

	return nil
}

// repeatApplyPreview tests the applyPreview resolver's pagination behaviour by
// retrieving the entire set of previews for the given input by making repeated
// requests.
func repeatApplyPreview(
	ctx context.Context,
	t *testing.T,
	schema *graphql.Schema,
	in map[string]any,
	query string,
	pageSize int,
) []apitest.ChangesetApplyPreview {
	t.Helper()

	in["first"] = pageSize
	in["after"] = nil
	out := []apitest.ChangesetApplyPreview{}

	for {
		var response struct{ Node apitest.BatchSpec }
		apitest.MustExec(ctx, t, schema, in, &response, query)
		out = append(out, response.Node.ApplyPreview.Nodes...)

		if response.Node.ApplyPreview.PageInfo.HasNextPage {
			in["after"] = *response.Node.ApplyPreview.PageInfo.EndCursor
		} else {
			return out
		}
	}
}

type applyPreviewTestFixture struct {
	batchSpec           *btypes.BatchSpec
	specPublished       *btypes.ChangesetSpec
	specToBePublished   *btypes.ChangesetSpec
	specToBeDraft       *btypes.ChangesetSpec
	specToBeUnpublished *btypes.ChangesetSpec
	specToBeOmitted     *btypes.ChangesetSpec
}

func newApplyPreviewTestFixture(
	t *testing.T, ctx context.Context, bstore *store.Store,
	userID int32,
	repoID api.RepoID,
	name string,
) *applyPreviewTestFixture {
	// We need a batch spec and a set of changeset specs that we can use to
	// verify that the behaviour is as expected. We'll create one changeset spec
	// with an explicit published field (so we can verify that UI publication
	// states can't override that), and four changeset specs without published
	// fields (one for each possible publication state).
	batchSpec := bt.CreateBatchSpec(t, ctx, bstore, name, userID, 0)

	return &applyPreviewTestFixture{
		batchSpec: batchSpec,
		specPublished: bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      repoID,
			HeadRef:   "published " + name,
			Typ:       btypes.ChangesetSpecTypeBranch,
			Published: true,
		}),
		specToBePublished: bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      repoID,
			HeadRef:   "to be published " + name,
			Typ:       btypes.ChangesetSpecTypeBranch,
		}),
		specToBeDraft: bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      repoID,
			HeadRef:   "to be draft " + name,
			Typ:       btypes.ChangesetSpecTypeBranch,
		}),
		specToBeUnpublished: bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      repoID,
			HeadRef:   "to be unpublished " + name,
			Typ:       btypes.ChangesetSpecTypeBranch,
		}),
		specToBeOmitted: bt.CreateChangesetSpec(t, ctx, bstore, bt.TestSpecOpts{
			BatchSpec: batchSpec.ID,
			User:      userID,
			Repo:      repoID,
			HeadRef:   "to be omitted " + name,
			Typ:       btypes.ChangesetSpecTypeBranch,
		}),
	}
}

func (fx *applyPreviewTestFixture) DecorateInput(in map[string]any) map[string]any {
	commonInputs := map[string]any{
		"batchSpec":         marshalBatchSpecRandID(fx.batchSpec.RandID),
		"publicationStates": fx.DefaultPublicationStates(),
	}

	for k, v := range in {
		commonInputs[k] = v
	}

	return commonInputs
}

func (fx *applyPreviewTestFixture) DefaultPublicationStates() []map[string]any {
	return []map[string]any{
		{
			"changesetSpec":    marshalChangesetSpecRandID(fx.specToBePublished.RandID),
			"publicationState": true,
		},
		{
			"changesetSpec":    marshalChangesetSpecRandID(fx.specToBeDraft.RandID),
			"publicationState": "draft",
		},
		{
			"changesetSpec":    marshalChangesetSpecRandID(fx.specToBeUnpublished.RandID),
			"publicationState": false,
		},
		// We'll also toss in a spec that doesn't exist, since applyPreview() is
		// documented to ignore unknown changeset specs due to its pagination
		// behaviour.
		{
			"changesetSpec":    marshalChangesetSpecRandID("this is not a valid random ID"),
			"publicationState": true,
		},
	}
}

func (fx *applyPreviewTestFixture) DefaultUiPublicationStates() service.UiPublicationStates {
	ups := service.UiPublicationStates{}

	for spec, state := range map[*btypes.ChangesetSpec]any{
		fx.specToBePublished:   true,
		fx.specToBeDraft:       "draft",
		fx.specToBeUnpublished: false,
	} {
		ups.Add(spec.RandID, batches.PublishedValue{Val: state})
	}

	return ups
}
