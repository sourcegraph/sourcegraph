package graphqlbackend

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/gofrs/uuid"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz/permssync"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestOrganization(t *testing.T) {
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

	orgMembers := dbmocks.NewMockOrgMemberStore()
	orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, nil)

	orgs := dbmocks.NewMockOrgStore()
	mockedOrg := types.Org{ID: 1, Name: "acme"}
	orgs.GetByNameFunc.SetDefaultReturn(&mockedOrg, nil)
	orgs.GetByIDFunc.SetDefaultReturn(&mockedOrg, nil)

	db := dbmocks.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)

	t.Run("anyone can access by default", func(t *testing.T) {
		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": {
						"name": "acme"
					}
				}
			`,
			},
		})
	})

	t.Run("users not invited or not a member cannot access on Sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": null
				}
				`,
				ExpectedErrors: []*gqlerrors.QueryError{
					{
						Message: "org not found: name acme",
						Path:    []any{"organization"},
					},
				},
			},
		})
	})

	t.Run("org members can access on Sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		orgMembers := dbmocks.NewMockOrgMemberStore()
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(&types.OrgMembership{OrgID: 1, UserID: 1}, nil)

		db := dbmocks.NewMockDBFrom(db)
		db.UsersFunc.SetDefaultReturn(users)
		db.OrgMembersFunc.SetDefaultReturn(orgMembers)

		RunTests(t, []*Test{
			{
				Schema:  mustParseGraphQLSchema(t, db),
				Context: ctx,
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": {
						"name": "acme"
					}
				}
				`,
			},
		})
	})

	t.Run("invited users can access on Sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		orgMembers := dbmocks.NewMockOrgMemberStore()
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})

		orgInvites := dbmocks.NewMockOrgInvitationStore()
		orgInvites.GetPendingFunc.SetDefaultReturn(nil, nil)

		db := dbmocks.NewMockDBFrom(db)
		db.OrgsFunc.SetDefaultReturn(orgs)
		db.UsersFunc.SetDefaultReturn(users)
		db.OrgMembersFunc.SetDefaultReturn(orgMembers)
		db.OrgInvitationsFunc.SetDefaultReturn(orgInvites)

		RunTests(t, []*Test{
			{
				Schema:  mustParseGraphQLSchema(t, db),
				Context: ctx,
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": {
						"name": "acme"
					}
				}
				`,
			},
		})
	})

	t.Run("invited users can access org by ID on Sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		orgMembers := dbmocks.NewMockOrgMemberStore()
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})

		orgInvites := dbmocks.NewMockOrgInvitationStore()
		orgInvites.GetPendingFunc.SetDefaultReturn(nil, nil)

		db := dbmocks.NewMockDBFrom(db)
		db.OrgsFunc.SetDefaultReturn(orgs)
		db.UsersFunc.SetDefaultReturn(users)
		db.OrgMembersFunc.SetDefaultReturn(orgMembers)
		db.OrgInvitationsFunc.SetDefaultReturn(orgInvites)

		RunTests(t, []*Test{
			{
				Schema:  mustParseGraphQLSchema(t, db),
				Context: ctx,
				Query: `
				{
					node(id: "T3JnOjE=") {
						__typename
						id
						... on Org {
						  name
						}
					}
				}
				`,
				ExpectedResult: `
				{
					"node": {
						"__typename":"Org",
						"id":"T3JnOjE=", "name":"acme"
					}
				}
				`,
			},
		})
	})
}

func TestCreateOrganization(t *testing.T) {
	userID := int32(1)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID, SiteAdmin: false}, nil)

	mockedOrg := types.Org{ID: 42, Name: "acme"}
	orgs := dbmocks.NewMockOrgStore()
	orgs.CreateFunc.SetDefaultReturn(&mockedOrg, nil)

	orgMembers := dbmocks.NewMockOrgMemberStore()
	orgMembers.CreateFunc.SetDefaultReturn(&types.OrgMembership{OrgID: mockedOrg.ID, UserID: userID}, nil)

	db := dbmocks.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: userID})

	t.Run("Creates organization", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation CreateOrganization($name: String!, $displayName: String) {
				createOrganization(name: $name, displayName: $displayName) {
					id
                    name
				}
			}`,
			ExpectedResult: fmt.Sprintf(`
			{
				"createOrganization": {
					"id": "%s",
					"name": "%s"
				}
			}
			`, MarshalOrgID(mockedOrg.ID), mockedOrg.Name),
			Variables: map[string]any{
				"name": "acme",
			},
		})
	})

	t.Run("Creates organization and sets statistics", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		id, err := uuid.NewV4()
		if err != nil {
			t.Fatal(err)
		}

		orgs.UpdateOrgsOpenBetaStatsFunc.SetDefaultReturn(nil)
		defer func() {
			orgs.UpdateOrgsOpenBetaStatsFunc = nil
		}()

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation CreateOrganization($name: String!, $displayName: String, $statsID: ID) {
				createOrganization(name: $name, displayName: $displayName, statsID: $statsID) {
					id
                    name
				}
			}`,
			ExpectedResult: fmt.Sprintf(`
			{
				"createOrganization": {
					"id": "%s",
					"name": "%s"
				}
			}
			`, MarshalOrgID(mockedOrg.ID), mockedOrg.Name),
			Variables: map[string]any{
				"name":    "acme",
				"statsID": id.String(),
			},
		})
	})

	t.Run("Fails for unauthenticated user", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: context.Background(),
			Query: `mutation CreateOrganization($name: String!, $displayName: String) {
				createOrganization(name: $name, displayName: $displayName) {
					id
                    name
				}
			}`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "no current user",
					Path:    []any{"createOrganization"},
				},
			},
			Variables: map[string]any{
				"name": "test",
			},
		})
	})

	t.Run("Fails for suspicious organization name", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation CreateOrganization($name: String!, $displayName: String) {
				createOrganization(name: $name, displayName: $displayName) {
					id
                    name
				}
			}`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: `rejected suspicious name "test"`,
					Path:    []any{"createOrganization"},
				},
			},
			Variables: map[string]any{
				"name": "test",
			},
		})
	})
}

func TestAddOrganizationMember(t *testing.T) {
	userID := int32(2)
	userName := "add-org-member"
	orgID := int32(1)
	orgIDString := string(MarshalOrgID(orgID))

	orgs := dbmocks.NewMockOrgStore()
	orgs.GetByNameFunc.SetDefaultReturn(&types.Org{ID: orgID, Name: "acme"}, nil)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
	users.GetByUsernameFunc.SetDefaultReturn(&types.User{ID: 2, Username: userName}, nil)

	orgMembers := dbmocks.NewMockOrgMemberStore()
	orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})
	orgMembers.CreateFunc.SetDefaultReturn(&types.OrgMembership{OrgID: orgID, UserID: userID}, nil)

	featureFlags := dbmocks.NewMockFeatureFlagStore()
	featureFlags.GetOrgFeatureFlagFunc.SetDefaultReturn(true, nil)

	// tests below depend on config being there
	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{AuthProviders: []schema.AuthProviders{{Builtin: &schema.BuiltinAuthProvider{}}}, EmailSmtp: nil}})

	// mock permission sync scheduling
	permssync.MockSchedulePermsSync = func(_ context.Context, logger log.Logger, _ database.DB, _ permssync.ScheduleSyncOpts) {}
	defer func() { permssync.MockSchedulePermsSync = nil }()

	db := dbmocks.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)
	db.FeatureFlagsFunc.SetDefaultReturn(featureFlags)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("Works for site admin if not on Cloud", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation AddUserToOrganization($organization: ID!, $username: String!) {
				addUserToOrganization(organization: $organization, username: $username) {
					alwaysNil
				}
			}`,
			ExpectedResult: `{
				"addUserToOrganization": {
					"alwaysNil": null
				}
			}`,
			Variables: map[string]any{
				"organization": orgIDString,
				"username":     userName,
			},
		})
	})

	t.Run("Does not work for site admin on Cloud", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation AddUserToOrganization($organization: ID!, $username: String!) {
				addUserToOrganization(organization: $organization, username: $username) {
					alwaysNil
				}
			}`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "Must be a member of the organization to add members%!(EXTRA *withstack.withStack=current user is not an org member)",
					Path:    []any{"addUserToOrganization"},
				},
			},
			Variables: map[string]any{
				"organization": orgIDString,
				"username":     userName,
			},
		})
	})

	t.Run("Works on Cloud if site admin is org member", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(ctx context.Context, orgID int32, userID int32) (*types.OrgMembership, error) {
			if userID == 1 {
				return &types.OrgMembership{OrgID: orgID, UserID: 1}, nil
			} else if userID == 2 {
				return nil, &database.ErrOrgMemberNotFound{}
			}
			t.Fatalf("Unexpected user ID received for OrgMembers.GetByOrgIDAndUserID: %d", userID)
			return nil, nil
		})

		defer func() {
			orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})
		}()

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation AddUserToOrganization($organization: ID!, $username: String!) {
				addUserToOrganization(organization: $organization, username: $username) {
					alwaysNil
				}
			}`,
			ExpectedResult: `{
				"addUserToOrganization": {
					"alwaysNil": null
				}
			}`,
			Variables: map[string]any{
				"organization": orgIDString,
				"username":     userName,
			},
		})
	})
}

func TestOrganizationRepositories_OSS(t *testing.T) {
	db := dbmocks.NewMockDB()
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					organization(name: "acme") {
						name,
						repositories {
							nodes {
								name
							}
						}
					}
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{{
				Message:   `Cannot query field "repositories" on type "Org".`,
				Locations: []gqlerrors.Location{{Line: 5, Column: 7}},
				Rule:      "FieldsOnCorrectType",
			}},
			Context: ctx,
		},
	})
}

func TestNode_Org(t *testing.T) {
	orgs := dbmocks.NewMockOrgStore()
	orgs.GetByIDFunc.SetDefaultReturn(&types.Org{ID: 1, Name: "acme"}, nil)

	db := dbmocks.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					node(id: "T3JnOjE=") {
						id
						... on Org {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"node": {
						"id": "T3JnOjE=",
						"name": "acme"
					}
				}
			`,
		},
	})
}

func TestUnmarshalOrgID(t *testing.T) {
	t.Run("Valid org ID is parsed correctly", func(t *testing.T) {
		const id = int32(1)
		namespaceOrgID := relay.MarshalID("Org", id)
		orgID, err := UnmarshalOrgID(namespaceOrgID)
		assert.NoError(t, err)
		assert.Equal(t, id, orgID)
	})

	t.Run("Returns error for invalid org ID", func(t *testing.T) {
		const id = 1
		namespaceOrgID := relay.MarshalID("User", id)
		_, err := UnmarshalOrgID(namespaceOrgID)
		assert.Error(t, err)
	})
}

func TestMembersConnectionStore(t *testing.T) {
	ctx := context.Background()

	db := database.NewDB(logtest.Scoped(t), dbtest.NewDB(t))

	org, err := db.Orgs().Create(ctx, "test-org", nil)
	require.NoError(t, err)

	for i := range 10 {
		user, err := db.Users().Create(ctx, database.NewUser{
			Username:        "test" + strconv.Itoa(i),
			Email:           fmt.Sprintf("test%d@sourcegraph.com", i),
			EmailIsVerified: true,
		})
		require.NoError(t, err)
		_, err = db.OrgMembers().Create(ctx, org.ID, user.ID)
		require.NoError(t, err)
	}

	connectionStore := &membersConnectionStore{
		db:    db,
		orgID: org.ID,
	}

	graphqlutil.TestConnectionResolverStoreSuite(t, connectionStore)
}
