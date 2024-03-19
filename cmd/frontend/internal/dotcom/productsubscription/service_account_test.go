package productsubscription

import (
	"context"
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestHasRBACPermsOrOwnerOrSiteAdmin(t *testing.T) {
	var actorID, anotherID int32 = 1, 2
	for _, tc := range []struct {
		name           string
		rbacRoles      map[string]bool
		actorSiteAdmin bool

		ownerUserID            *int32
		serviceAccountCanWrite bool

		wantGrantReason string
		wantErr         autogold.Value
	}{
		{
			name: "subscriptions reader account",
			rbacRoles: map[string]bool{
				rbac.ProductSubscriptionsReadPermission: true,
			},
			wantErr:         nil,
			wantGrantReason: rbac.ProductSubscriptionsReadPermission,
		},
		{
			name: "subscriptions writer account",
			rbacRoles: map[string]bool{
				rbac.ProductSubscriptionsWritePermission: true,
			},
			wantErr:         nil,
			wantGrantReason: rbac.ProductSubscriptionsWritePermission,
		},
		{
			name:            "same user",
			ownerUserID:     &actorID,
			wantErr:         nil,
			wantGrantReason: "is_owner",
		},
		{
			name:        "different user",
			ownerUserID: &anotherID,
			wantErr:     autogold.Expect("unauthorized"),
		},
		{
			name:            "site admin",
			actorSiteAdmin:  true,
			wantErr:         nil,
			wantGrantReason: "site_admin",
		},
		{
			name:            "site admin can access another user",
			actorSiteAdmin:  true,
			ownerUserID:     &anotherID,
			wantErr:         nil,
			wantGrantReason: "site_admin",
		},
		{
			name:    "not a site admin, not accessing a user-specific resource",
			wantErr: autogold.Expect("unauthorized"),
		},
		{
			name: "account needs writer",
			rbacRoles: map[string]bool{
				rbac.ProductSubscriptionsReadPermission: true,
			},
			serviceAccountCanWrite: true,
			wantErr:                autogold.Expect("unauthorized"),
		},
		{
			name: "account fulfills writer",
			rbacRoles: map[string]bool{
				rbac.ProductSubscriptionsWritePermission: true,
			},
			serviceAccountCanWrite: true,
			wantErr:                nil,
			wantGrantReason:        rbac.ProductSubscriptionsWritePermission,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			db := dbmocks.NewMockDB()
			mockUsers := dbmocks.NewMockUserStore()

			user := &types.User{ID: actorID, SiteAdmin: tc.actorSiteAdmin}
			mockUsers.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
			mockUsers.GetByIDFunc.SetDefaultReturn(user, nil)
			db.UsersFunc.SetDefaultReturn(mockUsers)

			permsStore := dbmocks.NewMockPermissionStore()
			permsStore.GetPermissionForUserFunc.SetDefaultHook(func(_ context.Context, opts database.GetPermissionForUserOpts) (*types.Permission, error) {
				if opts.UserID != actorID {
					return nil, errors.Newf("unexpected user ID %d", opts.UserID)
				}
				if len(tc.rbacRoles) == 0 {
					return nil, errors.New("user has no roles")
				}
				roleName := fmt.Sprintf("%s#%s", opts.Namespace, opts.Action)
				ok := tc.rbacRoles[roleName]
				if !ok {
					return nil, errors.Newf("%s not allowed", roleName)
				}
				// Value of types.Permission doesn't really matter, just needs
				// to be non-nil
				return &types.Permission{Namespace: opts.Namespace, Action: opts.Action}, nil
			})
			db.PermissionsFunc.SetDefaultReturn(permsStore)

			grantReason, err := hasRBACPermsOrOwnerOrSiteAdmin(
				actor.WithActor(context.Background(), &actor.Actor{UID: actorID}),
				db,
				tc.ownerUserID,
				tc.serviceAccountCanWrite,
			)
			if tc.wantErr != nil {
				require.Error(t, err)
				tc.wantErr.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantGrantReason, grantReason)
			}
		})
	}
}
