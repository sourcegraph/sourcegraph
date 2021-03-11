package resolvers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestPermissionLevels(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	db := dbtesting.GetDB(t)

	cstore := store.New(db)
	sr := New(cstore)
	s, err := graphqlbackend.NewSchema(db, sr, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// SyncChangeset uses EnqueueChangesetSync and tries to talk to repo-updater, hence we need to mock it.
	repoupdater.MockEnqueueChangesetSync = func(ctx context.Context, ids []int64) error {
		return nil
	}
	t.Cleanup(func() { repoupdater.MockEnqueueChangesetSync = nil })

	ctx := context.Background()

	// Global test data that we reuse in every test
	adminID := ct.CreateTestUser(t, db, true).ID
	userID := ct.CreateTestUser(t, db, false).ID

	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	repo := newGitHubTestRepo("github.com/sourcegraph/permission-levels-test", newGitHubExternalService(t, esStore))
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	changeset := &batches.Changeset{
		RepoID:              repo.ID,
		ExternalServiceType: "github",
		ExternalID:          "1234",
	}
	if err := cstore.CreateChangeset(ctx, changeset); err != nil {
		t.Fatal(err)
	}

	createBatchChange := func(t *testing.T, s *store.Store, name string, userID int32, batchSpecID int64) (batchChangeID int64) {
		t.Helper()

		c := &batches.BatchChange{
			Name:             name,
			InitialApplierID: userID,
			NamespaceUserID:  userID,
			LastApplierID:    userID,
			LastAppliedAt:    time.Now(),
			BatchSpecID:      batchSpecID,
		}
		if err := s.CreateBatchChange(ctx, c); err != nil {
			t.Fatal(err)
		}

		// We attach the changeset to the batch change so we can test syncChangeset
		changeset.BatchChanges = append(changeset.BatchChanges, batches.BatchChangeAssoc{BatchChangeID: c.ID})
		if err := s.UpdateChangeset(ctx, changeset); err != nil {
			t.Fatal(err)
		}

		cs := &batches.BatchSpec{UserID: userID, NamespaceUserID: userID}
		if err := s.CreateBatchSpec(ctx, cs); err != nil {
			t.Fatal(err)
		}

		return c.ID
	}

	createBatchSpec := func(t *testing.T, s *store.Store, userID int32) (randID string, id int64) {
		t.Helper()

		cs := &batches.BatchSpec{UserID: userID, NamespaceUserID: userID}
		if err := s.CreateBatchSpec(ctx, cs); err != nil {
			t.Fatal(err)
		}

		return cs.RandID, cs.ID
	}

	cleanUpBatchChanges := func(t *testing.T, s *store.Store) {
		t.Helper()

		batchChanges, next, err := s.ListBatchChanges(ctx, store.ListBatchChangesOpts{LimitOpts: store.LimitOpts{Limit: 1000}})
		if err != nil {
			t.Fatal(err)
		}
		if next != 0 {
			t.Fatalf("more batch changes in store")
		}

		for _, c := range batchChanges {
			if err := s.DeleteBatchChange(ctx, c.ID); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("queries", func(t *testing.T) {
		cleanUpBatchChanges(t, cstore)

		adminBatchSpec, adminBatchSpecID := createBatchSpec(t, cstore, adminID)
		adminBatchChange := createBatchChange(t, cstore, "admin", adminID, adminBatchSpecID)
		userBatchSpec, userBatchSpecID := createBatchSpec(t, cstore, userID)
		userBatchChange := createBatchChange(t, cstore, "user", userID, userBatchSpecID)

		t.Run("BatchChangeByID", func(t *testing.T) {
			tests := []struct {
				name                    string
				currentUser             int32
				batchChange             int64
				wantViewerCanAdminister bool
			}{
				{
					name:                    "site-admin viewing own batch change",
					currentUser:             adminID,
					batchChange:             adminBatchChange,
					wantViewerCanAdminister: true,
				},
				{
					name:                    "non-site-admin viewing other's batch change",
					currentUser:             userID,
					batchChange:             adminBatchChange,
					wantViewerCanAdminister: false,
				},
				{
					name:                    "site-admin viewing other's batch change",
					currentUser:             adminID,
					batchChange:             userBatchChange,
					wantViewerCanAdminister: true,
				},
				{
					name:                    "non-site-admin viewing own batch change",
					currentUser:             userID,
					batchChange:             userBatchChange,
					wantViewerCanAdminister: true,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					graphqlID := string(marshalBatchChangeID(tc.batchChange))

					var res struct{ Node apitest.BatchChange }

					input := map[string]interface{}{"batchChange": graphqlID}
					queryBatchChange := `
				  query($batchChange: ID!) {
				    node(id: $batchChange) { ... on BatchChange { id, viewerCanAdminister } }
				  }
                `

					actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))
					apitest.MustExec(actorCtx, t, s, input, &res, queryBatchChange)

					if have, want := res.Node.ID, graphqlID; have != want {
						t.Fatalf("queried batch change has wrong id %q, want %q", have, want)
					}
					if have, want := res.Node.ViewerCanAdminister, tc.wantViewerCanAdminister; have != want {
						t.Fatalf("queried batch change's ViewerCanAdminister is wrong %t, want %t", have, want)
					}
				})
			}
		})

		t.Run("BatchSpecByID", func(t *testing.T) {
			tests := []struct {
				name                    string
				currentUser             int32
				batchSpec               string
				wantViewerCanAdminister bool
			}{
				{
					name:                    "site-admin viewing own batch spec",
					currentUser:             adminID,
					batchSpec:               adminBatchSpec,
					wantViewerCanAdminister: true,
				},
				{
					name:                    "non-site-admin viewing other's batch spec",
					currentUser:             userID,
					batchSpec:               adminBatchSpec,
					wantViewerCanAdminister: false,
				},
				{
					name:                    "site-admin viewing other's batch spec",
					currentUser:             adminID,
					batchSpec:               userBatchSpec,
					wantViewerCanAdminister: true,
				},
				{
					name:                    "non-site-admin viewing own batch spec",
					currentUser:             userID,
					batchSpec:               userBatchSpec,
					wantViewerCanAdminister: true,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					graphqlID := string(marshalBatchSpecRandID(tc.batchSpec))

					var res struct{ Node apitest.BatchSpec }

					input := map[string]interface{}{"batchSpec": graphqlID}
					queryBatchSpec := `
				  query($batchSpec: ID!) {
				    node(id: $batchSpec) { ... on BatchSpec { id, viewerCanAdminister } }
				  }
                `

					actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))
					apitest.MustExec(actorCtx, t, s, input, &res, queryBatchSpec)

					if have, want := res.Node.ID, graphqlID; have != want {
						t.Fatalf("queried batch spec has wrong id %q, want %q", have, want)
					}
					if have, want := res.Node.ViewerCanAdminister, tc.wantViewerCanAdminister; have != want {
						t.Fatalf("queried batch spec's ViewerCanAdminister is wrong %t, want %t", have, want)
					}
				})
			}
		})

		t.Run("BatchChangesCodeHosts", func(t *testing.T) {
			tests := []struct {
				name        string
				currentUser int32
				user        int32
				wantErr     bool
			}{
				{
					name:        "site-admin viewing other user",
					currentUser: adminID,
					user:        userID,
					wantErr:     false,
				},
				{
					name:        "non-site-admin viewing other's hosts",
					currentUser: userID,
					user:        adminID,
					wantErr:     true,
				},
				{
					name:        "non-site-admin viewing own hosts",
					currentUser: userID,
					user:        userID,
					wantErr:     false,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					pruneUserCredentials(t, db)

					graphqlID := string(graphqlbackend.MarshalUserID(tc.user))

					var res struct{ Node apitest.User }

					input := map[string]interface{}{"user": graphqlID}
					queryCodeHosts := `
				  query($user: ID!) {
				    node(id: $user) { ... on User { batchChangesCodeHosts { totalCount } } }
				  }
                `

					actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))
					errors := apitest.Exec(actorCtx, t, s, input, &res, queryCodeHosts)
					if !tc.wantErr && len(errors) != 0 {
						t.Fatalf("got error but didn't expect one: %+v", errors)
					} else if tc.wantErr && len(errors) == 0 {
						t.Fatal("expected error but got none")
					}
				})
			}
		})

		t.Run("BatchChangesCredentialByID", func(t *testing.T) {
			tests := []struct {
				name        string
				currentUser int32
				user        int32
				wantErr     bool
			}{
				{
					name:        "site-admin viewing other user",
					currentUser: adminID,
					user:        userID,
					wantErr:     false,
				},
				{
					name:        "non-site-admin viewing other's credential",
					currentUser: userID,
					user:        adminID,
					wantErr:     true,
				},
				{
					name:        "non-site-admin viewing own credential",
					currentUser: userID,
					user:        userID,
					wantErr:     false,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					pruneUserCredentials(t, db)

					cred, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
						Domain:              database.UserCredentialDomainBatches,
						ExternalServiceID:   "https://github.com/",
						ExternalServiceType: extsvc.TypeGitHub,
						UserID:              tc.user,
					}, &auth.OAuthBearerToken{Token: "SOSECRET"})
					if err != nil {
						t.Fatal(err)
					}
					graphqlID := string(marshalBatchChangesCredentialID(cred.ID))

					var res struct {
						Node apitest.BatchChangesCredential
					}

					input := map[string]interface{}{"id": graphqlID}
					queryCodeHosts := `
				  query($id: ID!) {
				    node(id: $id) { ... on BatchChangesCredential { id } }
				  }
                `

					actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))
					errors := apitest.Exec(actorCtx, t, s, input, &res, queryCodeHosts)
					if !tc.wantErr && len(errors) != 0 {
						t.Fatalf("got error but didn't expect one: %v", errors)
					} else if tc.wantErr && len(errors) == 0 {
						t.Fatal("expected error but got none")
					}
					if !tc.wantErr {
						if have, want := res.Node.ID, graphqlID; have != want {
							t.Fatalf("invalid node returned, wanted ID=%q, have=%q", want, have)
						}
					}
				})
			}
		})

		t.Run("DeleteBatchChangesCredential", func(t *testing.T) {
			tests := []struct {
				name        string
				currentUser int32
				user        int32
				wantAuthErr bool
			}{
				{
					name:        "site-admin for other user",
					currentUser: adminID,
					user:        userID,
					wantAuthErr: false,
				},
				{
					name:        "non-site-admin for other user",
					currentUser: userID,
					user:        adminID,
					wantAuthErr: true,
				},
				{
					name:        "non-site-admin for self",
					currentUser: userID,
					user:        userID,
					wantAuthErr: false,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					pruneUserCredentials(t, db)

					cred, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
						Domain:              database.UserCredentialDomainBatches,
						ExternalServiceID:   "https://github.com/",
						ExternalServiceType: extsvc.TypeGitHub,
						UserID:              tc.user,
					}, &auth.OAuthBearerToken{Token: "SOSECRET"})
					if err != nil {
						t.Fatal(err)
					}

					var res struct {
						Node apitest.BatchChangesCredential
					}

					input := map[string]interface{}{
						"batchChangesCredential": marshalBatchChangesCredentialID(cred.ID),
					}
					mutationDeleteBatchChangesCredential := `
					mutation($batchChangesCredential: ID!) {
						deleteBatchChangesCredential(batchChangesCredential: $batchChangesCredential) { alwaysNil }
					}
                `

					actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))
					errors := apitest.Exec(actorCtx, t, s, input, &res, mutationDeleteBatchChangesCredential)
					if tc.wantAuthErr {
						if len(errors) != 1 {
							t.Fatalf("expected 1 error, but got %d: %s", len(errors), errors)
						}
						if !strings.Contains(errors[0].Error(), "must be authenticated") {
							t.Fatalf("wrong error: %s %T", errors[0], errors[0])
						}
					} else {
						// We don't care about other errors, we only want to
						// check that we didn't get an auth error.
						for _, e := range errors {
							if strings.Contains(e.Error(), "must be authenticated") {
								t.Fatalf("auth error wrongly returned: %s %T", errors[0], errors[0])
							}
						}
					}
				})
			}
		})

		t.Run("BatchChanges", func(t *testing.T) {
			tests := []struct {
				name                string
				currentUser         int32
				viewerCanAdminister bool
				wantBatchChanges    []int64
			}{
				{
					name:                "admin listing viewerCanAdminister: true",
					currentUser:         adminID,
					viewerCanAdminister: true,
					wantBatchChanges:    []int64{adminBatchChange, userBatchChange},
				},
				{
					name:                "user listing viewerCanAdminister: true",
					currentUser:         userID,
					viewerCanAdminister: true,
					wantBatchChanges:    []int64{userBatchChange},
				},
				{
					name:                "admin listing viewerCanAdminister: false",
					currentUser:         adminID,
					viewerCanAdminister: false,
					wantBatchChanges:    []int64{adminBatchChange, userBatchChange},
				},
				{
					name:                "user listing viewerCanAdminister: false",
					currentUser:         userID,
					viewerCanAdminister: false,
					wantBatchChanges:    []int64{adminBatchChange, userBatchChange},
				},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					actorCtx := actor.WithActor(context.Background(), actor.FromUser(tc.currentUser))
					expectedIDs := make(map[string]bool, len(tc.wantBatchChanges))
					for _, c := range tc.wantBatchChanges {
						graphqlID := string(marshalBatchChangeID(c))
						expectedIDs[graphqlID] = true
					}

					query := fmt.Sprintf(`
				query {
					batchChanges(viewerCanAdminister: %t) { totalCount, nodes { id } }
					node(id: %q) {
						id
						... on ExternalChangeset {
							batchChanges(viewerCanAdminister: %t) { totalCount, nodes { id } }
						}
					}
					}`, tc.viewerCanAdminister, marshalChangesetID(changeset.ID), tc.viewerCanAdminister)
					var res struct {
						BatchChanges apitest.BatchChangeConnection
						Node         apitest.Changeset
					}
					apitest.MustExec(actorCtx, t, s, nil, &res, query)
					for _, conn := range []apitest.BatchChangeConnection{res.BatchChanges, res.Node.BatchChanges} {
						if have, want := conn.TotalCount, len(tc.wantBatchChanges); have != want {
							t.Fatalf("wrong count of batch changes returned, want=%d have=%d", want, have)
						}
						if have, want := conn.TotalCount, len(conn.Nodes); have != want {
							t.Fatalf("totalCount and nodes length don't match, want=%d have=%d", want, have)
						}
						for _, node := range conn.Nodes {
							if _, ok := expectedIDs[node.ID]; !ok {
								t.Fatalf("received wrong batch change with id %q", node.ID)
							}
						}
					}
				})
			}
		})
	})

	t.Run("batch change mutations", func(t *testing.T) {
		mutations := []struct {
			name         string
			mutationFunc func(batchChangeID, changesetID, batchSpecID string) string
		}{
			{
				name: "createBatchChange",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { createBatchChange(batchSpec: %q) { id } }`, batchSpecID)
				},
			},
			{
				name: "closeBatchChange",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { closeBatchChange(batchChange: %q, closeChangesets: false) { id } }`, batchChangeID)
				},
			},
			{
				name: "deleteBatchChange",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { deleteBatchChange(batchChange: %q) { alwaysNil } } `, batchChangeID)
				},
			},
			{
				name: "syncChangeset",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { syncChangeset(changeset: %q) { alwaysNil } }`, changesetID)
				},
			},
			{
				name: "reenqueueChangeset",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { reenqueueChangeset(changeset: %q) { id } }`, changesetID)
				},
			},
			{
				name: "applyBatchChange",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { applyBatchChange(batchSpec: %q) { id } }`, batchSpecID)
				},
			},
			{
				name: "moveBatchChange",
				mutationFunc: func(batchChangeID, changesetID, batchSpecID string) string {
					return fmt.Sprintf(`mutation { moveBatchChange(batchChange: %q, newName: "foobar") { id } }`, batchChangeID)
				},
			},
		}

		for _, m := range mutations {
			t.Run(m.name, func(t *testing.T) {
				tests := []struct {
					name              string
					currentUser       int32
					batchChangeAuthor int32
					wantAuthErr       bool

					// If batches.restrictToAdmins is enabled, should an error
					// be generated?
					wantDisabledErr bool
				}{
					{
						name:              "unauthorized",
						currentUser:       userID,
						batchChangeAuthor: adminID,
						wantAuthErr:       true,
						wantDisabledErr:   true,
					},
					{
						name:              "authorized batch change owner",
						currentUser:       userID,
						batchChangeAuthor: userID,
						wantAuthErr:       false,
						wantDisabledErr:   true,
					},
					{
						name:              "authorized site-admin",
						currentUser:       adminID,
						batchChangeAuthor: userID,
						wantAuthErr:       false,
						wantDisabledErr:   false,
					},
				}

				for _, tc := range tests {
					for _, restrict := range []bool{true, false} {
						t.Run(fmt.Sprintf("%s restrict: %v", tc.name, restrict), func(t *testing.T) {
							cleanUpBatchChanges(t, cstore)

							batchSpecRandID, batchSpecID := createBatchSpec(t, cstore, tc.batchChangeAuthor)
							batchChagneID := createBatchChange(t, cstore, "test-batch-change", tc.batchChangeAuthor, batchSpecID)

							// We add the changeset to the batch change. It doesn't
							// matter for the addChangesetsToBatchChange mutation,
							// since that is idempotent and we want to solely
							// check for auth errors.
							changeset.BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: batchChagneID}}
							if err := cstore.UpdateChangeset(ctx, changeset); err != nil {
								t.Fatal(err)
							}

							mutation := m.mutationFunc(
								string(marshalBatchChangeID(batchChagneID)),
								string(marshalChangesetID(changeset.ID)),
								string(marshalBatchSpecRandID(batchSpecRandID)),
							)

							actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))

							conf.Mock(&conf.Unified{
								SiteConfiguration: schema.SiteConfiguration{
									BatchChangesRestrictToAdmins: &restrict,
								},
							})
							defer conf.Mock(nil)

							var response struct{}
							errs := apitest.Exec(actorCtx, t, s, nil, &response, mutation)

							// We don't care about other errors, we only want to
							// check that we didn't get an auth error.
							if restrict && tc.wantDisabledErr {
								if len(errs) != 1 {
									t.Fatalf("expected 1 error, but got %d: %s", len(errs), errs)
								}
								if !strings.Contains(errs[0].Error(), "batch changes are disabled for non-site-admin users") {
									t.Fatalf("wrong error: %s %T", errs[0], errs[0])
								}
							} else if tc.wantAuthErr {
								if len(errs) != 1 {
									t.Fatalf("expected 1 error, but got %d: %s", len(errs), errs)
								}
								if !strings.Contains(errs[0].Error(), "must be authenticated") {
									t.Fatalf("wrong error: %s %T", errs[0], errs[0])
								}
							} else {
								// We don't care about other errors, we only
								// want to check that we didn't get an auth
								// or site admin error.
								for _, e := range errs {
									if strings.Contains(e.Error(), "must be authenticated") {
										t.Fatalf("auth error wrongly returned: %s %T", errs[0], errs[0])
									} else if strings.Contains(e.Error(), "batch changes are disabled for non-site-admin users") {
										t.Fatalf("site admin error wrongly returned: %s %T", errs[0], errs[0])
									}
								}
							}
						})
					}
				}
			})
		}
	})

	t.Run("spec mutations", func(t *testing.T) {
		mutations := []struct {
			name         string
			mutationFunc func(userID string) string
		}{
			{
				name: "createChangesetSpec",
				mutationFunc: func(_ string) string {
					return `mutation { createChangesetSpec(changesetSpec: "{}") { type } }`
				},
			},
			{
				name: "createBatchSpec",
				mutationFunc: func(userID string) string {
					return fmt.Sprintf(`
					mutation {
						createBatchSpec(namespace: %q, batchSpec: "{}", changesetSpecs: []) {
							id
						}
					}`, userID)
				},
			},
		}

		for _, m := range mutations {
			t.Run(m.name, func(t *testing.T) {
				tests := []struct {
					name        string
					currentUser int32
					wantAuthErr bool
				}{
					{name: "no user", currentUser: 0, wantAuthErr: true},
					{name: "user", currentUser: userID, wantAuthErr: false},
					{name: "site-admin", currentUser: adminID, wantAuthErr: false},
				}

				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						cleanUpBatchChanges(t, cstore)

						namespaceID := string(graphqlbackend.MarshalUserID(tc.currentUser))
						if tc.currentUser == 0 {
							// If we don't have a currentUser we try to create
							// a batch change in another namespace, solely for the
							// purposes of this test.
							namespaceID = string(graphqlbackend.MarshalUserID(userID))
						}
						mutation := m.mutationFunc(namespaceID)

						actorCtx := actor.WithActor(ctx, actor.FromUser(tc.currentUser))

						var response struct{}
						errs := apitest.Exec(actorCtx, t, s, nil, &response, mutation)

						if tc.wantAuthErr {
							if len(errs) != 1 {
								t.Fatalf("expected 1 error, but got %d: %s", len(errs), errs)
							}
							if !strings.Contains(errs[0].Error(), "not authenticated") {
								t.Fatalf("wrong error: %s %T", errs[0], errs[0])
							}
						} else {
							// We don't care about other errors, we only want to
							// check that we didn't get an auth error.
							for _, e := range errs {
								if strings.Contains(e.Error(), "must be site admin") {
									t.Fatalf("auth error wrongly returned: %s %T", errs[0], errs[0])
								}
							}
						}
					})
				}
			})
		}
	})
}

func TestRepositoryPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	db := dbtesting.GetDB(t)

	cstore := store.New(db)
	sr := &Resolver{store: cstore}
	s, err := graphqlbackend.NewSchema(db, sr, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	testRev := api.CommitID("b69072d5f687b31b9f6ae3ceafdc24c259c4b9ec")
	mockBackendCommits(t, testRev)

	// Global test data that we reuse in every test
	userID := ct.CreateTestUser(t, db, false).ID

	repoStore := database.ReposWith(cstore)
	esStore := database.ExternalServicesWith(cstore)

	// Create 2 repositories
	repos := make([]*types.Repo, 0, 2)
	for i := 0; i < cap(repos); i++ {
		name := fmt.Sprintf("github.com/sourcegraph/test-repository-permissions-repo-%d", i)
		r := newGitHubTestRepo(name, newGitHubExternalService(t, esStore))
		if err := repoStore.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
		repos = append(repos, r)
	}

	t.Run("BatchChange and changesets", func(t *testing.T) {
		// Create 2 changesets for 2 repositories
		changesetBaseRefOid := "f00b4r"
		changesetHeadRefOid := "b4rf00"
		mockRepoComparison(t, changesetBaseRefOid, changesetHeadRefOid, testDiff)
		changesetDiffStat := apitest.DiffStat{Added: 0, Changed: 2, Deleted: 0}

		changesets := make([]*batches.Changeset, 0, len(repos))
		for _, r := range repos {
			c := &batches.Changeset{
				RepoID:              r.ID,
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalID:          fmt.Sprintf("external-%d", r.ID),
				ExternalState:       batches.ChangesetExternalStateOpen,
				ExternalCheckState:  batches.ChangesetCheckStatePassed,
				ExternalReviewState: batches.ChangesetReviewStateChangesRequested,
				PublicationState:    batches.ChangesetPublicationStatePublished,
				ReconcilerState:     batches.ReconcilerStateCompleted,
				Metadata: &github.PullRequest{
					BaseRefOid: changesetBaseRefOid,
					HeadRefOid: changesetHeadRefOid,
				},
			}
			c.SetDiffStat(changesetDiffStat.ToDiffStat())
			if err := cstore.CreateChangeset(ctx, c); err != nil {
				t.Fatal(err)
			}
			changesets = append(changesets, c)
		}

		spec := &batches.BatchSpec{
			NamespaceUserID: userID,
			UserID:          userID,
		}
		if err := cstore.CreateBatchSpec(ctx, spec); err != nil {
			t.Fatal(err)
		}

		batchChange := &batches.BatchChange{
			Name:             "my batch change",
			InitialApplierID: userID,
			NamespaceUserID:  userID,
			LastApplierID:    userID,
			LastAppliedAt:    time.Now(),
			BatchSpecID:      spec.ID,
		}
		if err := cstore.CreateBatchChange(ctx, batchChange); err != nil {
			t.Fatal(err)
		}
		// We attach the two changesets to the batch change
		for _, c := range changesets {
			c.BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: batchChange.ID}}
			if err := cstore.UpdateChangeset(ctx, c); err != nil {
				t.Fatal(err)
			}
		}

		// Query batch change and check that we get all changesets
		userCtx := actor.WithActor(ctx, actor.FromUser(userID))

		input := map[string]interface{}{
			"batchChange": string(marshalBatchChangeID(batchChange.ID)),
		}
		testBatchChangeResponse(t, s, userCtx, input, wantBatchChangeResponse{
			changesetTypes:  map[string]int{"ExternalChangeset": 2},
			changesetsCount: 2,
			changesetStats:  apitest.ChangesetsStats{Open: 2, Total: 2},
			batchChangeDiffStat: apitest.DiffStat{
				Added:   2 * changesetDiffStat.Added,
				Changed: 2 * changesetDiffStat.Changed,
				Deleted: 2 * changesetDiffStat.Deleted,
			},
		})

		for _, c := range changesets {
			// Both changesets are visible still, so both should be ExternalChangesets
			testChangesetResponse(t, s, userCtx, c.ID, "ExternalChangeset")
		}

		// Now we set permissions and filter out the repository of one changeset
		filteredRepo := changesets[0].RepoID
		accessibleRepo := changesets[1].RepoID
		ct.MockRepoPermissions(t, db, userID, accessibleRepo)

		// Send query again and check that for each filtered repository we get a
		// HiddenChangeset
		want := wantBatchChangeResponse{
			changesetTypes: map[string]int{
				"ExternalChangeset":       1,
				"HiddenExternalChangeset": 1,
			},
			changesetsCount: 2,
			changesetStats:  apitest.ChangesetsStats{Open: 2, Total: 2},
			batchChangeDiffStat: apitest.DiffStat{
				Added:   1 * changesetDiffStat.Added,
				Changed: 1 * changesetDiffStat.Changed,
				Deleted: 1 * changesetDiffStat.Deleted,
			},
		}
		testBatchChangeResponse(t, s, userCtx, input, want)

		for _, c := range changesets {
			// The changeset whose repository has been filtered should be hidden
			if c.RepoID == filteredRepo {
				testChangesetResponse(t, s, userCtx, c.ID, "HiddenExternalChangeset")
			} else {
				testChangesetResponse(t, s, userCtx, c.ID, "ExternalChangeset")
			}
		}

		// Now we query with more filters for the changesets. The hidden changesets
		// should not be returned, since that would leak information about the
		// hidden changesets.
		input = map[string]interface{}{
			"batchChange": string(marshalBatchChangeID(batchChange.ID)),
			"checkState":  string(batches.ChangesetCheckStatePassed),
		}
		wantCheckStateResponse := want
		wantCheckStateResponse.changesetsCount = 1
		wantCheckStateResponse.changesetTypes = map[string]int{
			"ExternalChangeset": 1,
			// No HiddenExternalChangeset
		}
		testBatchChangeResponse(t, s, userCtx, input, wantCheckStateResponse)

		input = map[string]interface{}{
			"batchChange": string(marshalBatchChangeID(batchChange.ID)),
			"reviewState": string(batches.ChangesetReviewStateChangesRequested),
		}
		wantReviewStateResponse := wantCheckStateResponse
		testBatchChangeResponse(t, s, userCtx, input, wantReviewStateResponse)
	})

	t.Run("BatchSpec and changesetSpecs", func(t *testing.T) {
		batchSpec := &batches.BatchSpec{
			UserID:          userID,
			NamespaceUserID: userID,
			Spec:            batches.BatchSpecFields{Name: "batch-spec-and-changeset-specs"},
		}
		if err := cstore.CreateBatchSpec(ctx, batchSpec); err != nil {
			t.Fatal(err)
		}

		changesetSpecs := make([]*batches.ChangesetSpec, 0, len(repos))
		for _, r := range repos {
			c := &batches.ChangesetSpec{
				RepoID:          r.ID,
				UserID:          userID,
				BatchSpecID:     batchSpec.ID,
				DiffStatAdded:   4,
				DiffStatChanged: 4,
				DiffStatDeleted: 4,
			}
			if err := cstore.CreateChangesetSpec(ctx, c); err != nil {
				t.Fatal(err)
			}
			changesetSpecs = append(changesetSpecs, c)
		}

		// Query BatchSpec and check that we get all changesetSpecs
		userCtx := actor.WithActor(ctx, actor.FromUser(userID))
		testBatchSpecResponse(t, s, userCtx, batchSpec.RandID, wantBatchSpecResponse{
			changesetSpecTypes:    map[string]int{"VisibleChangesetSpec": 2},
			changesetSpecsCount:   2,
			changesetPreviewTypes: map[string]int{"VisibleChangesetApplyPreview": 2},
			changesetPreviewCount: 2,
			batchSpecDiffStat: apitest.DiffStat{
				Added: 8, Changed: 8, Deleted: 8,
			},
		})

		// Now query the changesetSpecs as single nodes, to make sure that fetching/preloading
		// of repositories works
		for _, c := range changesetSpecs {
			// Both changesetSpecs are visible still, so both should be VisibleChangesetSpec
			testChangesetSpecResponse(t, s, userCtx, c.RandID, "VisibleChangesetSpec")
		}

		// Now we set permissions and filter out the repository of one changeset
		filteredRepo := changesetSpecs[0].RepoID
		accessibleRepo := changesetSpecs[1].RepoID
		ct.MockRepoPermissions(t, db, userID, accessibleRepo)

		// Send query again and check that for each filtered repository we get a
		// HiddenChangesetSpec.
		testBatchSpecResponse(t, s, userCtx, batchSpec.RandID, wantBatchSpecResponse{
			changesetSpecTypes: map[string]int{
				"VisibleChangesetSpec": 1,
				"HiddenChangesetSpec":  1,
			},
			changesetSpecsCount:   2,
			changesetPreviewTypes: map[string]int{"VisibleChangesetApplyPreview": 1, "HiddenChangesetApplyPreview": 1},
			changesetPreviewCount: 2,
			batchSpecDiffStat: apitest.DiffStat{
				Added: 4, Changed: 4, Deleted: 4,
			},
		})

		// Query the single changesetSpec nodes again
		for _, c := range changesetSpecs {
			// The changesetSpec whose repository has been filtered should be hidden
			if c.RepoID == filteredRepo {
				testChangesetSpecResponse(t, s, userCtx, c.RandID, "HiddenChangesetSpec")
			} else {
				testChangesetSpecResponse(t, s, userCtx, c.RandID, "VisibleChangesetSpec")
			}
		}
	})
}

type wantBatchChangeResponse struct {
	changesetTypes      map[string]int
	changesetsCount     int
	changesetStats      apitest.ChangesetsStats
	batchChangeDiffStat apitest.DiffStat
}

func testBatchChangeResponse(t *testing.T, s *graphql.Schema, ctx context.Context, in map[string]interface{}, w wantBatchChangeResponse) {
	t.Helper()

	var response struct{ Node apitest.BatchChange }
	apitest.MustExec(ctx, t, s, in, &response, queryBatchChangePermLevels)

	if have, want := response.Node.ID, in["batchChange"]; have != want {
		t.Fatalf("batch change id is wrong. have %q, want %q", have, want)
	}

	if diff := cmp.Diff(w.changesetsCount, response.Node.Changesets.TotalCount); diff != "" {
		t.Fatalf("unexpected changesets total count (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(w.changesetStats, response.Node.ChangesetsStats); diff != "" {
		t.Fatalf("unexpected changesets stats (-want +got):\n%s", diff)
	}

	changesetTypes := map[string]int{}
	for _, c := range response.Node.Changesets.Nodes {
		changesetTypes[c.Typename]++
	}
	if diff := cmp.Diff(w.changesetTypes, changesetTypes); diff != "" {
		t.Fatalf("unexpected changesettypes (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(w.batchChangeDiffStat, response.Node.DiffStat); diff != "" {
		t.Fatalf("unexpected batch change diff stat (-want +got):\n%s", diff)
	}
}

const queryBatchChangePermLevels = `
query($batchChange: ID!, $reviewState: ChangesetReviewState, $checkState: ChangesetCheckState) {
  node(id: $batchChange) {
    ... on BatchChange {
	  id

	  changesetsStats { unpublished, open, merged, closed, total }

      changesets(first: 100, reviewState: $reviewState, checkState: $checkState) {
        totalCount
        nodes {
          __typename
          ... on HiddenExternalChangeset {
            id
          }
          ... on ExternalChangeset {
            id
            repository {
              id
              name
            }
          }
        }
      }

      diffStat {
        added
        changed
        deleted
      }
    }
  }
}
`

func testChangesetResponse(t *testing.T, s *graphql.Schema, ctx context.Context, id int64, wantType string) {
	t.Helper()

	var res struct{ Node apitest.Changeset }
	query := fmt.Sprintf(queryChangesetPermLevels, marshalChangesetID(id))
	apitest.MustExec(ctx, t, s, nil, &res, query)

	if have, want := res.Node.Typename, wantType; have != want {
		t.Fatalf("changeset has wrong typename. want=%q, have=%q", want, have)
	}

	if have, want := res.Node.State, string(batches.ChangesetStateOpen); have != want {
		t.Fatalf("changeset has wrong state. want=%q, have=%q", want, have)
	}

	if have, want := res.Node.BatchChanges.TotalCount, 1; have != want {
		t.Fatalf("changeset has wrong batch changes totalcount. want=%d, have=%d", want, have)
	}

	if parseJSONTime(t, res.Node.CreatedAt).IsZero() {
		t.Fatalf("changeset createdAt is zero")
	}

	if parseJSONTime(t, res.Node.UpdatedAt).IsZero() {
		t.Fatalf("changeset updatedAt is zero")
	}

	if parseJSONTime(t, res.Node.NextSyncAt).IsZero() {
		t.Fatalf("changeset next sync at is zero")
	}
}

const queryChangesetPermLevels = `
query {
  node(id: %q) {
    __typename

    ... on HiddenExternalChangeset {
      id

	  state
	  createdAt
	  updatedAt
	  nextSyncAt
	  batchChanges {
	    totalCount
	  }
    }
    ... on ExternalChangeset {
      id

	  state
	  createdAt
	  updatedAt
	  nextSyncAt
	  batchChanges {
	    totalCount
	  }

      repository {
        id
        name
      }
    }
  }
}
`

type wantBatchSpecResponse struct {
	changesetPreviewTypes map[string]int
	changesetPreviewCount int
	changesetSpecTypes    map[string]int
	changesetSpecsCount   int
	batchSpecDiffStat     apitest.DiffStat
}

func testBatchSpecResponse(t *testing.T, s *graphql.Schema, ctx context.Context, batchSpecRandID string, w wantBatchSpecResponse) {
	t.Helper()

	in := map[string]interface{}{
		"batchSpec": string(marshalBatchSpecRandID(batchSpecRandID)),
	}

	var response struct{ Node apitest.BatchSpec }
	apitest.MustExec(ctx, t, s, in, &response, queryBatchSpecPermLevels)

	if have, want := response.Node.ID, in["batchSpec"]; have != want {
		t.Fatalf("batch spec id is wrong. have %q, want %q", have, want)
	}

	if diff := cmp.Diff(w.changesetSpecsCount, response.Node.ChangesetSpecs.TotalCount); diff != "" {
		t.Fatalf("unexpected changesetSpecs total count (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(w.changesetPreviewCount, response.Node.ApplyPreview.TotalCount); diff != "" {
		t.Fatalf("unexpected applyPreview total count (-want +got):\n%s", diff)
	}

	changesetSpecTypes := map[string]int{}
	for _, c := range response.Node.ChangesetSpecs.Nodes {
		changesetSpecTypes[c.Typename]++
	}
	if diff := cmp.Diff(w.changesetSpecTypes, changesetSpecTypes); diff != "" {
		t.Fatalf("unexpected changesetSpec types (-want +got):\n%s", diff)
	}

	changesetPreviewTypes := map[string]int{}
	for _, c := range response.Node.ApplyPreview.Nodes {
		changesetPreviewTypes[c.Typename]++
	}
	if diff := cmp.Diff(w.changesetPreviewTypes, changesetPreviewTypes); diff != "" {
		t.Fatalf("unexpected applyPreview types (-want +got):\n%s", diff)
	}
}

const queryBatchSpecPermLevels = `
query($batchSpec: ID!) {
  node(id: $batchSpec) {
    ... on BatchSpec {
      id

      applyPreview(first: 100) {
        totalCount
        nodes {
          __typename
          ... on HiddenChangesetApplyPreview {
              targets {
                  __typename
              }
          }
          ... on VisibleChangesetApplyPreview {
              targets {
                  __typename
              }
          }
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

func testChangesetSpecResponse(t *testing.T, s *graphql.Schema, ctx context.Context, randID, wantType string) {
	t.Helper()

	var res struct{ Node apitest.ChangesetSpec }
	query := fmt.Sprintf(queryChangesetSpecPermLevels, marshalChangesetSpecRandID(randID))
	apitest.MustExec(ctx, t, s, nil, &res, query)

	if have, want := res.Node.Typename, wantType; have != want {
		t.Fatalf("changesetspec has wrong typename. want=%q, have=%q", want, have)
	}
}

const queryChangesetSpecPermLevels = `
query {
  node(id: %q) {
    __typename

    ... on HiddenChangesetSpec {
      id
      type
    }

    ... on VisibleChangesetSpec {
      id
      type

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
`
