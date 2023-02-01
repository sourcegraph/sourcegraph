package graphqlbackend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestDeleteUser(t *testing.T) {
	t.Run("authenticated as non-admin", func(t *testing.T) {
		users := database.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := database.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		result, err := newSchemaResolver(db, gitserver.NewClient()).DeleteUser(ctx, &struct {
			User graphql.ID
			Hard *bool
		}{
			User: MarshalUserID(1),
		})
		if want := auth.ErrMustBeSiteAdmin; err != want {
			t.Errorf("err: want %q but got %v", want, err)
		}
		if result != nil {
			t.Errorf("result: want nil but got %v", result)
		}
	})

	t.Run("delete current user", func(t *testing.T) {
		users := database.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

		db := database.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		_, err := newSchemaResolver(db, gitserver.NewClient()).DeleteUser(ctx, &struct {
			User graphql.ID
			Hard *bool
		}{
			User: MarshalUserID(1),
		})
		want := "unable to delete current user"
		if err == nil || err.Error() != want {
			t.Fatalf("err: want %q but got %v", want, err)
		}
	})

	// Mocking all database interactions here, but they are all thoroughly tested in the lower layer in "database" package.
	users := database.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	users.DeleteFunc.SetDefaultReturn(nil)
	users.HardDeleteFunc.SetDefaultReturn(nil)
	users.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id, Username: "alice"}, nil
	})
	const notFoundUID = 8
	users.ListFunc.SetDefaultHook(func(ctx context.Context, opts *database.UsersListOptions) ([]*types.User, error) {
		var users []*types.User
		for _, id := range opts.UserIDs {
			if id != notFoundUID { // test not-found user
				users = append(users, &types.User{ID: id, Username: "alice"})
			}
		}
		return users, nil
	})

	userEmails := database.NewMockUserEmailsStore()
	userEmails.ListByUserFunc.SetDefaultReturn([]*database.UserEmail{{Email: "alice@example.com"}}, nil)

	externalAccounts := database.NewMockUserExternalAccountsStore()
	externalAccountsListDefaultReturn := []*extsvc.Account{{
		AccountSpec: extsvc.AccountSpec{
			ServiceType: extsvc.TypeGitLab,
			ServiceID:   "https://gitlab.com/",
			AccountID:   "alice_gitlab",
		},
	}}
	externalAccounts.ListFunc.SetDefaultReturn(externalAccountsListDefaultReturn, nil)

	const aliceUID = 6
	authzStore := database.NewMockAuthzStore()
	authzStore.RevokeUserPermissionsFunc.SetDefaultHook(func(_ context.Context, args *database.RevokeUserPermissionsArgs) error {
		if args.UserID != aliceUID {
			return errors.Errorf("args.UserID: want 6 but got %v", args.UserID)
		}

		expAccounts := []*extsvc.Accounts{
			{
				ServiceType: extsvc.TypeGitLab,
				ServiceID:   "https://gitlab.com/",
				AccountIDs:  []string{"alice_gitlab"},
			},
			{
				ServiceType: authz.SourcegraphServiceType,
				ServiceID:   authz.SourcegraphServiceID,
				AccountIDs:  []string{"alice@example.com", "alice"},
			},
		}
		if diff := cmp.Diff(expAccounts, args.Accounts); diff != "" {
			t.Fatalf("args.Accounts: %v", diff)
		}
		return nil
	})

	db := database.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.UserEmailsFunc.SetDefaultReturn(userEmails)
	db.UserExternalAccountsFunc.SetDefaultReturn(externalAccounts)
	db.AuthzFunc.SetDefaultReturn(authzStore)

	// Disable event logging, which is triggered for SOAP users
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				EventLogging: "disabled",
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	tests := []struct {
		name     string
		setup    func(t *testing.T)
		gqlTests []*Test
	}{
		{
			name: "target is not a user",
			gqlTests: []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				mutation {
					deleteUser(user: "VXNlcjo4") {
						alwaysNil
					}
				}
			`,
					ExpectedResult: `{ "deleteUser": null }`,
					ExpectedErrors: []*gqlerrors.QueryError{{
						Path:    []any{"deleteUser"},
						Message: fmt.Sprintf("no users found with IDs: [%d]", notFoundUID),
					}},
				},
			},
		},
		{
			name: "soft delete a user",
			gqlTests: []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				mutation {
					deleteUser(user: "VXNlcjo2") {
						alwaysNil
					}
				}
			`,
					ExpectedResult: `
				{
					"deleteUser": {
						"alwaysNil": null
					}
				}
			`,
				},
			},
		},
		{
			name: "hard delete a user",
			gqlTests: []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				mutation {
					deleteUser(user: "VXNlcjo2", hard: true) {
						alwaysNil
					}
				}
			`,
					ExpectedResult: `
				{
					"deleteUser": {
						"alwaysNil": null
					}
				}
			`,
				},
			},
		},
		{
			name: "non-SOAP user cannot delete SOAP user",
			setup: func(t *testing.T) {
				t.Cleanup(func() { externalAccounts.ListFunc.SetDefaultReturn(externalAccountsListDefaultReturn, nil) })

				externalAccounts.ListFunc.SetDefaultHook(func(ctx context.Context, opts database.ExternalAccountsListOptions) ([]*extsvc.Account, error) {
					if opts.UserID == aliceUID {
						// delete target is a SOAP user
						return []*extsvc.Account{{
							AccountSpec: extsvc.AccountSpec{
								ServiceType: auth.SourcegraphOperatorProviderType,
								ServiceID:   "soap",
								AccountID:   "alice_soap",
							},
						}}, nil
					}
					return nil, errors.Newf("unexpected user %d", opts.UserID)
				})
			},
			gqlTests: []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Query: `
				mutation {
					deleteUser(user: "VXNlcjo2") {
						alwaysNil
					}
				}
			`,
					ExpectedResult: `{ "deleteUser": null }`,
					ExpectedErrors: []*gqlerrors.QueryError{
						{
							Path: []any{"deleteUser"},
							Message: fmt.Sprintf("%[1]q users cannot be deleted by non-%[1]q users",
								auth.SourcegraphOperatorProviderType),
						},
					},
				},
			},
		},
		{
			name: "SOAP user deletes SOAP user",
			setup: func(t *testing.T) {
				t.Cleanup(func() { externalAccounts.ListFunc.SetDefaultReturn(externalAccountsListDefaultReturn, nil) })

				externalAccounts.ListFunc.SetDefaultHook(func(ctx context.Context, opts database.ExternalAccountsListOptions) ([]*extsvc.Account, error) {
					if opts.UserID == aliceUID {
						// delete target is a SOAP user
						return []*extsvc.Account{{
							AccountSpec: extsvc.AccountSpec{
								ServiceType: auth.SourcegraphOperatorProviderType,
								ServiceID:   "soap",
								AccountID:   "alice_soap",
							},
						}}, nil
					}
					return nil, errors.Newf("unexpected user %d", opts.UserID)
				})
			},
			gqlTests: []*Test{
				{
					Schema: mustParseGraphQLSchema(t, db),
					Context: actor.WithActor(context.Background(),
						&actor.Actor{UID: 1, SourcegraphOperator: true}),
					Query: `
				mutation {
					deleteUser(user: "VXNlcjo2") {
						alwaysNil
					}
				}
			`,
					ExpectedResult: `
				{
					"deleteUser": {
						"alwaysNil": null
					}
				}
			`,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setup != nil {
				test.setup(t)
			}
			RunTests(t, test.gqlTests)
		})
	}
}

func TestDeleteOrganization_OnPremise(t *testing.T) {
	users := database.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

	orgMembers := database.NewMockOrgMemberStore()
	orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, nil)

	orgs := database.NewMockOrgStore()

	mockedOrg := types.Org{ID: 1, Name: "acme"}
	orgIDString := string(MarshalOrgID(mockedOrg.ID))

	db := database.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("Non admins cannot soft delete orgs", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!) {
					deleteOrganization(organization: $organization) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
			},
			ExpectedResult: `
				{
					"deleteOrganization": null
				}
				`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "must be site admin",
					Path:    []any{"deleteOrganization"},
				},
			},
		})
	})

	t.Run("Admins can soft delete orgs", func(t *testing.T) {
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
		db.UsersFunc.SetDefaultReturn(users)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!) {
					deleteOrganization(organization: $organization) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
			},
			ExpectedResult: `
				{
					"deleteOrganization": {
						"alwaysNil": null
					}
				}
				`,
		})
	})

	t.Run("Hard delete is not supported on-premise", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!, $hard: Boolean) {
					deleteOrganization(organization: $organization, hard: $hard) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
				"hard":         true,
			},
			ExpectedResult: `
			{
				"deleteOrganization": null
			}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "hard deleting organization is only supported on Sourcegraph.com",
					Path:    []any{"deleteOrganization"},
				},
			},
		})
	})
}

func TestDeleteOrganization_OnCloud(t *testing.T) {
	users := database.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

	orgMembers := database.NewMockOrgMemberStore()
	orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, nil)

	orgs := database.NewMockOrgStore()

	mockedOrg := types.Org{ID: 1, Name: "acme"}
	orgIDString := string(MarshalOrgID(mockedOrg.ID))

	db := database.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	orig := envvar.SourcegraphDotComMode()
	envvar.MockSourcegraphDotComMode(true)
	defer envvar.MockSourcegraphDotComMode(orig)

	t.Run("Returns an error when user is not an org member", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!, $hard: Boolean) {
					deleteOrganization(organization: $organization, hard: $hard) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
				"hard":         true,
			},
			ExpectedResult: `
				{
					"deleteOrganization": null
				}
				`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "current user is not an org member",
					Path:    []any{"deleteOrganization"},
				},
			},
		})
	})

	t.Run("Returns an error when feature flag is not enabled", func(t *testing.T) {
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(&types.OrgMembership{ID: 1, OrgID: 1, UserID: 1},
			nil)

		mockedFeatureFlag := featureflag.FeatureFlag{
			Name:      "org-deletion",
			Bool:      &featureflag.FeatureFlagBool{Value: false},
			Rollout:   nil,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: nil,
		}
		featureFlags := database.NewMockFeatureFlagStore()
		featureFlags.GetFeatureFlagFunc.SetDefaultReturn(&mockedFeatureFlag, nil)

		db.OrgMembersFunc.SetDefaultReturn(orgMembers)
		db.FeatureFlagsFunc.SetDefaultReturn(featureFlags)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!, $hard: Boolean) {
					deleteOrganization(organization: $organization, hard: $hard) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
				"hard":         true,
			},
			ExpectedResult: `
				{
					"deleteOrganization": null
				}
				`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "hard deleting organization is not supported",
					Path:    []any{"deleteOrganization"},
				},
			},
		})
	})

	t.Run("Returns an error when user tries to soft delete an org in Cloud mode", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!, $hard: Boolean) {
					deleteOrganization(organization: $organization, hard: $hard) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
				"hard":         false,
			},
			ExpectedResult: `
				{
					"deleteOrganization": null
				}
				`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "soft deleting organization is not supported on Sourcegraph.com",
					Path:    []any{"deleteOrganization"},
				},
			},
		})
	})

	t.Run("Org member can hard delete their org", func(t *testing.T) {
		mockedFeatureFlag := featureflag.FeatureFlag{
			Name:      "org-deletion",
			Bool:      &featureflag.FeatureFlagBool{Value: true},
			Rollout:   nil,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: nil,
		}
		featureFlags := database.NewMockFeatureFlagStore()
		featureFlags.GetFeatureFlagFunc.SetDefaultReturn(&mockedFeatureFlag, nil)

		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(&types.OrgMembership{ID: 1, OrgID: 1, UserID: 1},
			nil)
		db.OrgMembersFunc.SetDefaultReturn(orgMembers)
		db.FeatureFlagsFunc.SetDefaultReturn(featureFlags)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `
				mutation DeleteOrganization($organization: ID!, $hard: Boolean) {
					deleteOrganization(organization: $organization, hard: $hard) {
						alwaysNil
					}
				}
				`,
			Variables: map[string]any{
				"organization": orgIDString,
				"hard":         true,
			},
			ExpectedResult: `
				{
					"deleteOrganization": {
						"alwaysNil": null
					}
				}
				`,
		})
	})
}
