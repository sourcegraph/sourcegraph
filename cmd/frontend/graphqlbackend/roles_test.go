package graphqlbackend

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/apitest"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestRoleConnectionResolver(t *testing.T) {
	logger := logtest.Scoped(t)
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	userID := createTestUser(t, db, false).ID
	userCtx := actor.WithActor(ctx, actor.FromUser(userID))

	adminID := createTestUser(t, db, true).ID
	adminCtx := actor.WithActor(ctx, actor.FromUser(adminID))

	s, err := NewSchemaWithoutResolvers(db)
	if err != nil {
		t.Fatal(err)
	}

	// All sourcegraph instances are seeded with two system roles at migration,
	// so we take those into account when querying roles.
	siteAdminRole, err := db.Roles().Get(ctx, database.GetRoleOpts{
		Name: string(types.SiteAdministratorSystemRole),
	})
	require.NoError(t, err)

	userRole, err := db.Roles().Get(ctx, database.GetRoleOpts{
		Name: string(types.UserSystemRole),
	})
	require.NoError(t, err)

	r, err := db.Roles().Create(ctx, "TEST-ROLE", false)
	require.NoError(t, err)

	t.Run("as non site-administrator", func(t *testing.T) {
		input := map[string]any{"first": 1}
		var response struct{ Permissions apitest.PermissionConnection }
		errs := apitest.Exec(userCtx, t, s, input, &response, queryRoleConnection)

		require.Len(t, errs, 1)
		require.Equal(t, errs[0].Message, "must be site admin")
	})

	t.Run("as site-administrator", func(t *testing.T) {
		want := []apitest.Role{
			{
				ID: string(MarshalRoleID(userRole.ID)),
			},
			{
				ID: string(MarshalRoleID(siteAdminRole.ID)),
			},
			{
				ID: string(MarshalRoleID(r.ID)),
			},
		}

		tests := []struct {
			firstParam          int
			wantHasNextPage     bool
			wantHasPreviousPage bool
			wantTotalCount      int
			wantNodes           []apitest.Role
		}{
			{firstParam: 1, wantHasNextPage: true, wantHasPreviousPage: false, wantTotalCount: 3, wantNodes: want[:1]},
			{firstParam: 2, wantHasNextPage: true, wantHasPreviousPage: false, wantTotalCount: 3, wantNodes: want[:2]},
			{firstParam: 3, wantHasNextPage: false, wantHasPreviousPage: false, wantTotalCount: 3, wantNodes: want},
			{firstParam: 4, wantHasNextPage: false, wantHasPreviousPage: false, wantTotalCount: 3, wantNodes: want},
		}

		for _, tc := range tests {
			t.Run(fmt.Sprintf("first=%d", tc.firstParam), func(t *testing.T) {
				input := map[string]any{"first": int64(tc.firstParam)}
				var response struct{ Roles apitest.RoleConnection }
				apitest.MustExec(adminCtx, t, s, input, &response, queryRoleConnection)

				wantConnection := apitest.RoleConnection{
					TotalCount: tc.wantTotalCount,
					PageInfo: apitest.PageInfo{
						HasNextPage:     tc.wantHasNextPage,
						HasPreviousPage: tc.wantHasPreviousPage,
					},
					Nodes: tc.wantNodes,
				}

				if diff := cmp.Diff(wantConnection, response.Roles); diff != "" {
					t.Fatalf("wrong roles response (-want +got):\n%s", diff)
				}
			})
		}
	})
}

const queryRoleConnection = `
query($first: Int!) {
	roles(first: $first) {
		totalCount
		pageInfo {
			hasNextPage
			hasPreviousPage
		}
		nodes {
			id
		}
	}
}
`

func TestUserRoleListing(t *testing.T) {
	logger := logtest.Scoped(t)
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	userID := createTestUser(t, db, false).ID
	actorCtx := actor.WithActor(ctx, actor.FromUser(userID))

	adminUserID := createTestUser(t, db, true).ID
	adminActorCtx := actor.WithActor(ctx, actor.FromUser(adminUserID))

	s, err := NewSchemaWithoutResolvers(db)
	if err != nil {
		t.Fatal(err)
	}
	require.NoError(t, err)

	// create a new role
	role, err := db.Roles().Create(ctx, "TEST-ROLE", false)
	require.NoError(t, err)

	err = db.UserRoles().Assign(ctx, database.AssignUserRoleOpts{
		RoleID: role.ID,
		UserID: userID,
	})
	require.NoError(t, err)

	err = db.UserRoles().Assign(ctx, database.AssignUserRoleOpts{
		RoleID: role.ID,
		UserID: adminUserID,
	})
	require.NoError(t, err)

	t.Run("on sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		t.Run("non-admin", func(t *testing.T) {
			userAPIID := string(MarshalUserID(userID))
			input := map[string]any{"node": userAPIID}

			var response struct{ Node apitest.User }
			errs := apitest.Exec(actorCtx, t, s, input, &response, listUserRoles)
			require.ErrorContains(t, errs[0], "unauthorized")
		})

		t.Run("admin", func(t *testing.T) {
			userAPIID := string(MarshalUserID(adminUserID))
			input := map[string]any{"node": userAPIID}

			var response struct{ Node apitest.User }
			errs := apitest.Exec(adminActorCtx, t, s, input, &response, listUserRoles)
			assert.Len(t, errs, 0)
		})
	})

	t.Run("listing a user's roles (same user)", func(t *testing.T) {
		userAPIID := string(MarshalUserID(userID))
		input := map[string]any{"node": userAPIID}

		want := apitest.User{
			ID: userAPIID,
			Roles: apitest.RoleConnection{
				TotalCount: 1,
				Nodes: []apitest.Role{
					{
						ID: string(MarshalRoleID(role.ID)),
					},
				},
			},
		}

		var response struct{ Node apitest.User }
		apitest.MustExec(actorCtx, t, s, input, &response, listUserRoles)

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("wrong role response (-want +got):\n%s", diff)
		}
	})

	t.Run("listing a user's roles (site admin)", func(t *testing.T) {
		userAPIID := string(MarshalUserID(userID))
		input := map[string]any{"node": userAPIID}

		want := apitest.User{
			ID: userAPIID,
			Roles: apitest.RoleConnection{
				TotalCount: 1,
				Nodes: []apitest.Role{
					{
						ID: string(MarshalRoleID(role.ID)),
					},
				},
			},
		}

		var response struct{ Node apitest.User }
		apitest.MustExec(adminActorCtx, t, s, input, &response, listUserRoles)

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("wrong roles response (-want +got):\n%s", diff)
		}
	})

	t.Run("non site-admin listing another user's roles", func(t *testing.T) {
		userAPIID := string(MarshalUserID(adminUserID))
		input := map[string]any{"node": userAPIID}

		var response struct{ Node apitest.User }
		apitest.MustExec(actorCtx, t, s, input, &response, listUserRoles)

		want := apitest.User{
			ID: userAPIID,
			Roles: apitest.RoleConnection{
				TotalCount: 1,
				Nodes: []apitest.Role{
					{
						ID: string(MarshalRoleID(role.ID)),
					},
				},
			},
		}

		if diff := cmp.Diff(want, response.Node); diff != "" {
			t.Fatalf("wrong roles response (-want +got):\n%s", diff)
		}
	})
}

const listUserRoles = `
query ($node: ID!) {
	node(id: $node) {
		... on User {
			id
			roles(first: 50) {
				totalCount
				nodes {
					id
				}
			}
		}
	}
}
`
