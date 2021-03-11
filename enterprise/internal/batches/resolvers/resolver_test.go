package resolvers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/batch-change-utils/overridable"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestNullIDResilience(t *testing.T) {
	db := dbtesting.GetDB(t)
	sr := New(store.New(db))

	s, err := graphqlbackend.NewSchema(db, sr, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := backend.WithAuthzBypass(context.Background())

	ids := []graphql.ID{
		marshalBatchChangeID(0),
		marshalChangesetID(0),
		marshalBatchSpecRandID(""),
		marshalChangesetSpecRandID(""),
		marshalBatchChangesCredentialID(0),
	}

	for _, id := range ids {
		var response struct{ Node struct{ ID string } }

		query := fmt.Sprintf(`query { node(id: %q) { id } }`, id)
		apitest.MustExec(ctx, t, s, nil, &response, query)

		if have, want := response.Node.ID, ""; have != want {
			t.Fatalf("node has wrong ID. have=%q, want=%q", have, want)
		}
	}

	mutations := []string{
		fmt.Sprintf(`mutation { closeBatchChange(batchChange: %q) { id } }`, marshalBatchChangeID(0)),
		fmt.Sprintf(`mutation { deleteBatchChange(batchChange: %q) { alwaysNil } }`, marshalBatchChangeID(0)),
		fmt.Sprintf(`mutation { syncChangeset(changeset: %q) { alwaysNil } }`, marshalChangesetID(0)),
		fmt.Sprintf(`mutation { reenqueueChangeset(changeset: %q) { id } }`, marshalChangesetID(0)),
		fmt.Sprintf(`mutation { applyBatchChange(batchSpec: %q) { id } }`, marshalBatchSpecRandID("")),
		fmt.Sprintf(`mutation { createBatchChange(batchSpec: %q) { id } }`, marshalBatchSpecRandID("")),
		fmt.Sprintf(`mutation { moveBatchChange(batchChange: %q, newName: "foobar") { id } }`, marshalBatchChangeID(0)),
		fmt.Sprintf(`mutation { createBatchChangesCredential(externalServiceKind: GITHUB, externalServiceURL: "http://test", credential: "123123", user: %q) { id } }`, graphqlbackend.MarshalUserID(0)),
		fmt.Sprintf(`mutation { deleteBatchChangesCredential(batchChangesCredential: %q) { alwaysNil } }`, marshalBatchChangesCredentialID(0)),
	}

	for _, m := range mutations {
		var response struct{}
		errs := apitest.Exec(ctx, t, s, nil, &response, m)
		if len(errs) == 0 {
			t.Fatalf("expected errors but none returned (mutation: %q)", m)
		}
		if have, want := errs[0].Error(), fmt.Sprintf("graphql: %s", ErrIDIsZero{}); have != want {
			t.Fatalf("wrong errors. have=%s, want=%s (mutation: %q)", have, want, m)
		}
	}
}

func TestCreateBatchSpec(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	user := ct.CreateTestUser(t, db, true)
	userID := user.ID

	cstore := store.New(db)
	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/create-batch-spec-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	// Create enough changeset specs to hit the licence check.
	changesetSpecs := make([]*batches.ChangesetSpec, maxUnlicensedChangesets+1)
	for i := range changesetSpecs {
		changesetSpecs[i] = &batches.ChangesetSpec{
			Spec: &batches.ChangesetSpecDescription{
				BaseRepository: graphqlbackend.MarshalRepositoryID(repo.ID),
			},
			RepoID: repo.ID,
			UserID: userID,
		}
		if err := cstore.CreateChangesetSpec(ctx, changesetSpecs[i]); err != nil {
			t.Fatal(err)
		}
	}

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	userAPIID := string(graphqlbackend.MarshalUserID(userID))
	rawSpec := ct.TestRawBatchSpec

	for name, tc := range map[string]struct {
		changesetSpecs []*batches.ChangesetSpec
		hasLicenseFor  map[licensing.Feature]struct{}
		wantErr        bool
	}{
		"batch changes license, over the limit": {
			changesetSpecs: changesetSpecs,
			hasLicenseFor: map[licensing.Feature]struct{}{
				licensing.FeatureBatchChanges: {},
			},
			wantErr: false,
		},
		"campaigns license, over the limit": {
			changesetSpecs: changesetSpecs,
			hasLicenseFor: map[licensing.Feature]struct{}{
				licensing.FeatureCampaigns: {},
			},
			wantErr: false,
		},
		"no licence, but under the limit": {
			changesetSpecs: changesetSpecs[0:maxUnlicensedChangesets],
			hasLicenseFor:  map[licensing.Feature]struct{}{},
			wantErr:        false,
		},
		"no licence, over the limit": {
			changesetSpecs: changesetSpecs,
			hasLicenseFor:  map[licensing.Feature]struct{}{},
			wantErr:        true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			oldMock := licensing.MockCheckFeature
			licensing.MockCheckFeature = func(feature licensing.Feature) error {
				if _, ok := tc.hasLicenseFor[feature]; !ok {
					return licensing.NewFeatureNotActivatedError("no batch changes for you!")
				}
				return nil
			}

			defer func() {
				licensing.MockCheckFeature = oldMock
			}()

			changesetSpecIDs := make([]graphql.ID, len(tc.changesetSpecs))
			for i, spec := range tc.changesetSpecs {
				changesetSpecIDs[i] = marshalChangesetSpecRandID(spec.RandID)
			}

			input := map[string]interface{}{
				"namespace":      userAPIID,
				"batchSpec":      rawSpec,
				"changesetSpecs": changesetSpecIDs,
			}

			var response struct{ CreateBatchSpec apitest.BatchSpec }

			actorCtx := actor.WithActor(ctx, actor.FromUser(userID))
			errs := apitest.Exec(actorCtx, t, s, input, &response, mutationCreateBatchSpec)
			if tc.wantErr {
				if errs == nil {
					t.Error("unexpected lack of errors")
				}
			} else {
				if errs != nil {
					t.Errorf("unexpected error(s): %+v", errs)
				}

				var unmarshaled interface{}
				err = json.Unmarshal([]byte(rawSpec), &unmarshaled)
				if err != nil {
					t.Fatal(err)
				}
				have := response.CreateBatchSpec

				wantNodes := make([]apitest.ChangesetSpec, len(changesetSpecIDs))
				for i, id := range changesetSpecIDs {
					wantNodes[i] = apitest.ChangesetSpec{
						Typename: "VisibleChangesetSpec",
						ID:       string(id),
					}
				}

				want := apitest.BatchSpec{
					ID:            have.ID,
					CreatedAt:     have.CreatedAt,
					ExpiresAt:     have.ExpiresAt,
					OriginalInput: rawSpec,
					ParsedInput:   graphqlbackend.JSONValue{Value: unmarshaled},
					ApplyURL:      fmt.Sprintf("/users/%s/batch-changes/apply/%s", user.Username, have.ID),
					Namespace:     apitest.UserOrg{ID: userAPIID, DatabaseID: userID, SiteAdmin: true},
					Creator:       &apitest.User{ID: userAPIID, DatabaseID: userID, SiteAdmin: true},
					ChangesetSpecs: apitest.ChangesetSpecConnection{
						Nodes: wantNodes,
					},
				}

				if diff := cmp.Diff(want, have); diff != "" {
					t.Fatalf("unexpected response (-want +got):\n%s", diff)
				}
			}
		})
	}
}

const mutationCreateBatchSpec = `
fragment u on User { id, databaseID, siteAdmin }
fragment o on Org  { id, name }

mutation($namespace: ID!, $batchSpec: String!, $changesetSpecs: [ID!]!){
  createBatchSpec(namespace: $namespace, batchSpec: $batchSpec, changesetSpecs: $changesetSpecs) {
    id
    originalInput
    parsedInput

    creator  { ...u }
    namespace {
      ... on User { ...u }
      ... on Org  { ...o }
    }

    applyURL

	changesetSpecs {
	  nodes {
		  __typename
		  ... on VisibleChangesetSpec {
			  id
		  }
	  }
	}

    createdAt
    expiresAt
  }
}
`

func TestCreateChangesetSpec(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID

	cstore := store.New(db)
	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/create-changeset-spec-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]interface{}{
		"changesetSpec": ct.NewRawChangesetSpecGitBranch(graphqlbackend.MarshalRepositoryID(repo.ID), "d34db33f"),
	}

	var response struct{ CreateChangesetSpec apitest.ChangesetSpec }

	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))
	apitest.MustExec(actorCtx, t, s, input, &response, mutationCreateChangesetSpec)

	have := response.CreateChangesetSpec

	want := apitest.ChangesetSpec{
		Typename:  "VisibleChangesetSpec",
		ID:        have.ID,
		ExpiresAt: have.ExpiresAt,
	}

	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatalf("unexpected response (-want +got):\n%s", diff)
	}

	randID, err := unmarshalChangesetSpecID(graphql.ID(want.ID))
	if err != nil {
		t.Fatal(err)
	}

	cs, err := cstore.GetChangesetSpec(ctx, store.GetChangesetSpecOpts{RandID: randID})
	if err != nil {
		t.Fatal(err)
	}

	if have, want := cs.RepoID, repo.ID; have != want {
		t.Fatalf("wrong RepoID. want=%d, have=%d", want, have)
	}
}

const mutationCreateChangesetSpec = `
mutation($changesetSpec: String!){
  createChangesetSpec(changesetSpec: $changesetSpec) {
	__typename
	... on VisibleChangesetSpec {
		id
		expiresAt
	}
  }
}
`

func TestApplyBatchChange(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID

	now := timeutil.Now()
	clock := func() time.Time { return now }
	cstore := store.NewWithClock(db, clock)
	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/apply-batch-change-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	repoAPIID := graphqlbackend.MarshalRepositoryID(repo.ID)

	batchSpec := &batches.BatchSpec{
		RawSpec: ct.TestRawBatchSpec,
		Spec: batches.BatchSpecFields{
			Name:        "my-batch-change",
			Description: "My description",
			ChangesetTemplate: batches.ChangesetTemplate{
				Title:  "Hello there",
				Body:   "This is the body",
				Branch: "my-branch",
				Commit: batches.CommitTemplate{
					Message: "Add hello world",
				},
				Published: overridable.FromBoolOrString(false),
			},
		},
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := cstore.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	changesetSpec := &batches.ChangesetSpec{
		BatchSpecID: batchSpec.ID,
		Spec: &batches.ChangesetSpecDescription{
			BaseRepository: repoAPIID,
		},
		RepoID: repo.ID,
		UserID: userID,
	}
	if err := cstore.CreateChangesetSpec(ctx, changesetSpec); err != nil {
		t.Fatal(err)
	}

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	userAPIID := string(graphqlbackend.MarshalUserID(userID))
	input := map[string]interface{}{
		"batchSpec": string(marshalBatchSpecRandID(batchSpec.RandID)),
	}

	var response struct{ ApplyBatchChange apitest.BatchChange }
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))
	apitest.MustExec(actorCtx, t, s, input, &response, mutationApplyBatchChange)

	apiUser := &apitest.User{
		ID:         userAPIID,
		DatabaseID: userID,
		SiteAdmin:  true,
	}

	have := response.ApplyBatchChange
	want := apitest.BatchChange{
		ID:          have.ID,
		Name:        batchSpec.Spec.Name,
		Description: batchSpec.Spec.Description,
		Namespace: apitest.UserOrg{
			ID:         userAPIID,
			DatabaseID: userID,
			SiteAdmin:  true,
		},
		InitialApplier: apiUser,
		LastApplier:    apiUser,
		LastAppliedAt:  marshalDateTime(t, now),
		Changesets: apitest.ChangesetConnection{
			Nodes: []apitest.Changeset{
				{Typename: "ExternalChangeset", State: string(batches.ChangesetStateProcessing)},
			},
			TotalCount: 1,
		},
	}

	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatalf("unexpected response (-want +got):\n%s", diff)
	}

	// Now we execute it again and make sure we get the same batch change back
	apitest.MustExec(actorCtx, t, s, input, &response, mutationApplyBatchChange)
	have2 := response.ApplyBatchChange
	if diff := cmp.Diff(want, have2); diff != "" {
		t.Fatalf("unexpected response (-want +got):\n%s", diff)
	}

	// Execute it again with ensureBatchChange set to correct batch change's ID
	input["ensureBatchChange"] = have2.ID
	apitest.MustExec(actorCtx, t, s, input, &response, mutationApplyBatchChange)
	have3 := response.ApplyBatchChange
	if diff := cmp.Diff(want, have3); diff != "" {
		t.Fatalf("unexpected response (-want +got):\n%s", diff)
	}

	// Execute it again but ensureBatchChange set to wrong batch change ID
	batchChangeID, err := unmarshalBatchChangeID(graphql.ID(have3.ID))
	if err != nil {
		t.Fatal(err)
	}
	input["ensureBatchChange"] = marshalBatchChangeID(batchChangeID + 999)
	errs := apitest.Exec(actorCtx, t, s, input, &response, mutationApplyBatchChange)
	if len(errs) == 0 {
		t.Fatalf("expected errors, got none")
	}
}

const mutationApplyBatchChange = `
fragment u on User { id, databaseID, siteAdmin }
fragment o on Org  { id, name }

mutation($batchSpec: ID!, $ensureBatchChange: ID){
  applyBatchChange(batchSpec: $batchSpec, ensureBatchChange: $ensureBatchChange) {
    id, name, description
    initialApplier    { ...u }
    lastApplier       { ...u }
    lastAppliedAt
    namespace {
        ... on User { ...u }
        ... on Org  { ...o }
    }

    changesets {
      nodes {
        __typename
        state
      }

      totalCount
    }
  }
}
`

func TestCreateBatchChange(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	userID := ct.CreateTestUser(t, db, true).ID

	cstore := store.New(db)

	batchSpec := &batches.BatchSpec{
		RawSpec: ct.TestRawBatchSpec,
		Spec: batches.BatchSpecFields{
			Name:        "my-batch-change",
			Description: "My description",
		},
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := cstore.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]interface{}{
		"batchSpec": string(marshalBatchSpecRandID(batchSpec.RandID)),
	}

	var response struct{ CreateBatchChange apitest.BatchChange }
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))

	// First time it should work, because no batch change exists
	apitest.MustExec(actorCtx, t, s, input, &response, mutationCreateBatchChange)

	if response.CreateBatchChange.ID == "" {
		t.Fatalf("expected batch change to be created, but was not")
	}

	// Second time it should fail
	errors := apitest.Exec(actorCtx, t, s, input, &response, mutationCreateBatchChange)

	if len(errors) != 1 {
		t.Fatalf("expected single errors, but got none")
	}
	if have, want := errors[0].Message, service.ErrMatchingBatchChangeExists.Error(); have != want {
		t.Fatalf("wrong error. want=%q, have=%q", want, have)
	}
}

const mutationCreateBatchChange = `
mutation($batchSpec: ID!){
  createBatchChange(batchSpec: $batchSpec) { id }
}
`

func TestMoveBatchChange(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	user := ct.CreateTestUser(t, db, true)
	userID := user.ID

	orgName := "move-batch-change-test"
	orgID := ct.InsertTestOrg(t, db, orgName)

	cstore := store.New(db)

	batchSpec := &batches.BatchSpec{
		RawSpec:         ct.TestRawBatchSpec,
		UserID:          userID,
		NamespaceUserID: userID,
	}
	if err := cstore.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	batchChange := &batches.BatchChange{
		BatchSpecID:      batchSpec.ID,
		Name:             "old-name",
		InitialApplierID: userID,
		LastApplierID:    userID,
		LastAppliedAt:    time.Now(),
		NamespaceUserID:  batchSpec.UserID,
	}
	if err := cstore.CreateBatchChange(ctx, batchChange); err != nil {
		t.Fatal(err)
	}

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Move to a new name
	batchChangeAPIID := string(marshalBatchChangeID(batchChange.ID))
	newBatchChagneName := "new-name"
	input := map[string]interface{}{
		"batchChange": batchChangeAPIID,
		"newName":     newBatchChagneName,
	}

	var response struct{ MoveBatchChange apitest.BatchChange }
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))
	apitest.MustExec(actorCtx, t, s, input, &response, mutationMoveBatchChange)

	haveBatchChange := response.MoveBatchChange
	if diff := cmp.Diff(input["newName"], haveBatchChange.Name); diff != "" {
		t.Fatalf("unexpected name (-want +got):\n%s", diff)
	}

	wantURL := fmt.Sprintf("/users/%s/batch-changes/%s", user.Username, newBatchChagneName)
	if diff := cmp.Diff(wantURL, haveBatchChange.URL); diff != "" {
		t.Fatalf("unexpected URL (-want +got):\n%s", diff)
	}

	// Move to a new namespace
	orgAPIID := graphqlbackend.MarshalOrgID(orgID)
	input = map[string]interface{}{
		"batchChange":  string(marshalBatchChangeID(batchChange.ID)),
		"newNamespace": orgAPIID,
	}

	apitest.MustExec(actorCtx, t, s, input, &response, mutationMoveBatchChange)

	haveBatchChange = response.MoveBatchChange
	if diff := cmp.Diff(string(orgAPIID), haveBatchChange.Namespace.ID); diff != "" {
		t.Fatalf("unexpected namespace (-want +got):\n%s", diff)
	}
	wantURL = fmt.Sprintf("/organizations/%s/batch-changes/%s", orgName, newBatchChagneName)
	if diff := cmp.Diff(wantURL, haveBatchChange.URL); diff != "" {
		t.Fatalf("unexpected URL (-want +got):\n%s", diff)
	}
}

const mutationMoveBatchChange = `
fragment u on User { id, databaseID, siteAdmin }
fragment o on Org  { id, name }

mutation($batchChange: ID!, $newName: String, $newNamespace: ID){
  moveBatchChange(batchChange: $batchChange, newName: $newName, newNamespace: $newNamespace) {
	id, name, description
	initialApplier  { ...u }
	namespace {
		... on User { ...u }
		... on Org  { ...o }
	}
	url
  }
}
`

func TestListChangesetOptsFromArgs(t *testing.T) {
	var wantFirst int32 = 10
	wantPublicationStates := []batches.ChangesetPublicationState{
		"PUBLISHED",
		"INVALID",
	}
	wantStates := []batches.ChangesetState{"OPEN", "INVALID"}
	wantExternalStates := []batches.ChangesetExternalState{"OPEN"}
	wantReviewStates := []batches.ChangesetReviewState{"APPROVED", "INVALID"}
	wantCheckStates := []batches.ChangesetCheckState{"PENDING", "INVALID"}
	wantOnlyPublishedByThisBatchChange := []bool{true}
	wantSearches := []search.TextSearchTerm{{Term: "foo"}, {Term: "bar", Not: true}}
	var batchChangeID int64 = 1

	tcs := []struct {
		args       *graphqlbackend.ListChangesetsArgs
		wantSafe   bool
		wantErr    string
		wantParsed store.ListChangesetsOpts
	}{
		// No args given.
		{
			args:       nil,
			wantSafe:   true,
			wantParsed: store.ListChangesetsOpts{},
		},
		// First argument is set in opts, and considered safe.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				First: wantFirst,
			},
			wantSafe:   true,
			wantParsed: store.ListChangesetsOpts{LimitOpts: store.LimitOpts{Limit: 10}},
		},
		// Setting state is safe and transferred to opts.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				State: &wantStates[0],
			},
			wantSafe: true,
			wantParsed: store.ListChangesetsOpts{
				ExternalState:    &wantExternalStates[0],
				PublicationState: &wantPublicationStates[0],
				ReconcilerStates: []batches.ReconcilerState{batches.ReconcilerStateCompleted},
			},
		},
		// Setting invalid state fails.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				State: &wantStates[1],
			},
			wantErr: "changeset state not valid",
		},
		// Setting review state is not safe and transferred to opts.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				ReviewState: &wantReviewStates[0],
			},
			wantSafe:   false,
			wantParsed: store.ListChangesetsOpts{ExternalReviewState: &wantReviewStates[0]},
		},
		// Setting invalid review state fails.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				ReviewState: &wantReviewStates[1],
			},
			wantErr: "changeset review state not valid",
		},
		// Setting check state is not safe and transferred to opts.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				CheckState: &wantCheckStates[0],
			},
			wantSafe:   false,
			wantParsed: store.ListChangesetsOpts{ExternalCheckState: &wantCheckStates[0]},
		},
		// Setting invalid check state fails.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				CheckState: &wantCheckStates[1],
			},
			wantErr: "changeset check state not valid",
		},
		// Setting OnlyPublishedByThisCampaign true.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				OnlyPublishedByThisCampaign: &wantOnlyPublishedByThisBatchChange[0],
			},
			wantSafe: true,
			wantParsed: store.ListChangesetsOpts{
				PublicationState:     &wantPublicationStates[0],
				OwnedByBatchChangeID: batchChangeID,
			},
		},
		// Setting OnlyPublishedByThisBatchChange true.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				OnlyPublishedByThisBatchChange: &wantOnlyPublishedByThisBatchChange[0],
			},
			wantSafe: true,
			wantParsed: store.ListChangesetsOpts{
				PublicationState:     &wantPublicationStates[0],
				OwnedByBatchChangeID: batchChangeID,
			},
		},
		// Setting a positive search.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				Search: stringPtr("foo"),
			},
			wantSafe: false,
			wantParsed: store.ListChangesetsOpts{
				TextSearch: wantSearches[0:1],
			},
		},
		// Setting a negative search.
		{
			args: &graphqlbackend.ListChangesetsArgs{
				Search: stringPtr("-bar"),
			},
			wantSafe: false,
			wantParsed: store.ListChangesetsOpts{
				TextSearch: wantSearches[1:],
			},
		},
	}
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			haveParsed, haveSafe, err := listChangesetOptsFromArgs(tc.args, batchChangeID)
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			haveErr := fmt.Sprintf("%v", err)
			wantErr := tc.wantErr
			if wantErr == "" {
				wantErr = "<nil>"
			}
			if have, want := haveErr, wantErr; have != want {
				t.Errorf("wrong error returned. have=%q want=%q", have, want)
			}
			if diff := cmp.Diff(haveParsed, tc.wantParsed); diff != "" {
				t.Errorf("wrong args returned. diff=%s", diff)
			}
			if have, want := haveSafe, tc.wantSafe; have != want {
				t.Errorf("wrong safe value returned. have=%t want=%t", have, want)
			}
		})
	}
}

func TestCreateBatchChangesCredential(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	pruneUserCredentials(t, db)

	userID := ct.CreateTestUser(t, db, false).ID

	cstore := store.New(db)

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]interface{}{
		"user":                graphqlbackend.MarshalUserID(userID),
		"externalServiceKind": string(extsvc.KindGitHub),
		"externalServiceURL":  "https://github.com/",
		"credential":          "SOSECRET",
	}

	var response struct {
		CreateBatchChangesCredential apitest.BatchChangesCredential
	}
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))

	// First time it should work, because no credential exists
	apitest.MustExec(actorCtx, t, s, input, &response, mutationCreateCredential)

	if response.CreateBatchChangesCredential.ID == "" {
		t.Fatalf("expected credential to be created, but was not")
	}

	// Second time it should fail
	errors := apitest.Exec(actorCtx, t, s, input, &response, mutationCreateCredential)

	if len(errors) != 1 {
		t.Fatalf("expected single errors, but got none")
	}
	if have, want := errors[0].Extensions["code"], "ErrDuplicateCredential"; have != want {
		t.Fatalf("wrong error code. want=%q, have=%q", want, have)
	}
}

const mutationCreateCredential = `
mutation($user: ID!, $externalServiceKind: ExternalServiceKind!, $externalServiceURL: String!, $credential: String!) {
  createBatchChangesCredential(user: $user, externalServiceKind: $externalServiceKind, externalServiceURL: $externalServiceURL, credential: $credential) { id }
}
`

func TestDeleteBatchChangesCredential(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := dbtesting.GetDB(t)

	pruneUserCredentials(t, db)

	userID := ct.CreateTestUser(t, db, true).ID

	cstore := store.New(db)

	cred, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
		Domain:              database.UserCredentialDomainBatches,
		ExternalServiceType: extsvc.TypeGitHub,
		ExternalServiceID:   "https://github.com/",
		UserID:              userID,
	}, &auth.OAuthBearerToken{Token: "SOSECRET"})
	if err != nil {
		t.Fatal(err)
	}

	r := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, r, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	input := map[string]interface{}{
		"batchChangesCredential": marshalBatchChangesCredentialID(cred.ID),
	}

	var response struct{ DeleteBatchChangesCredential apitest.EmptyResponse }
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))

	// First time it should work, because a credential exists
	apitest.MustExec(actorCtx, t, s, input, &response, mutationDeleteCredential)

	// Second time it should fail
	errors := apitest.Exec(actorCtx, t, s, input, &response, mutationDeleteCredential)

	if len(errors) != 1 {
		t.Fatalf("expected single errors, but got none")
	}
	if have, want := errors[0].Message, fmt.Sprintf("user credential not found: [%d]", cred.ID); have != want {
		t.Fatalf("wrong error code. want=%q, have=%q", want, have)
	}
}

const mutationDeleteCredential = `
mutation($batchChangesCredential: ID!) {
  deleteBatchChangesCredential(batchChangesCredential: $batchChangesCredential) { alwaysNil }
}
`

func stringPtr(s string) *string { return &s }
