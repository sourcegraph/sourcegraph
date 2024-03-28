package rbactest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func CreateRole(ctx context.Context, t *testing.T, db database.DB, name string) *types.Role {
	t.Helper()

	role, err := db.Roles().Create(ctx, name, false)
	require.NoError(t, err)

	return role
}

func AssignPermissionToRole(ctx context.Context, t *testing.T, db database.DB, roleID int32, permissions ...int32) {
	t.Helper()

	err := db.RolePermissions().BulkAssignPermissionsToRole(ctx, database.BulkAssignPermissionsToRoleOpts{
		RoleID:      roleID,
		Permissions: permissions,
	})
	require.NoError(t, err)
}

func CreatePermission(ctx context.Context, t *testing.T, db database.DB, permission string) *types.Permission {
	t.Helper()

	ns, action, err := rbac.ParsePermissionDisplayName(permission)
	require.NoError(t, err)

	p, err := db.Permissions().Create(ctx, database.CreatePermissionOpts{
		Namespace: ns,
		Action:    action,
	})
	require.NoError(t, err)

	return p
}
