package rbac

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	rtypes "github.com/sourcegraph/sourcegraph/internal/rbac/types"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestComparePermissions(t *testing.T) {
	dbPerms := []*types.Permission{
		{ID: 1, Namespace: "TEST-NAMESPACE", Action: "READ"},
		{ID: 2, Namespace: "TEST-NAMESPACE", Action: "WRITE"},
		{ID: 3, Namespace: "TEST-NAMESPACE-2", Action: "READ"},
		{ID: 4, Namespace: "TEST-NAMESPACE-2", Action: "WRITE"},
		{ID: 5, Namespace: "TEST-NAMESPACE-3", Action: "READ"},
	}

	t.Run("no changes to permissions", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, false)

		schemaPerms := Schema{
			Namespaces: []Namespace{
				{Name: "TEST-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-2", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-3", Actions: []rtypes.NamespaceAction{"READ"}},
				{Name: "DOTCOM-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ"},
					DotCom: true}, // not included by default
			},
		}

		added, deleted := ComparePermissions(dbPerms, schemaPerms)

		require.Len(t, added, 0)
		require.Len(t, deleted, 0)
	})

	t.Run("permissions deleted", func(t *testing.T) {
		schemaPerms := Schema{
			Namespaces: []Namespace{
				{Name: "TEST-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-2", Actions: []rtypes.NamespaceAction{"READ"}},
			},
		}

		want := []database.DeletePermissionOpts{
			{ID: int32(4)},
			{ID: int32(5)},
		}

		added, deleted := ComparePermissions(dbPerms, schemaPerms)

		require.Len(t, added, 0)
		require.Len(t, deleted, 2)
		if diff := cmp.Diff(want, deleted, cmpopts.SortSlices(sortDeletePermissionOptSlice)); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("permissions added", func(t *testing.T) {
		schemaPerms := Schema{
			Namespaces: []Namespace{
				{Name: "TEST-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-2", Actions: []rtypes.NamespaceAction{"READ", "WRITE", "EXECUTE"}},
				{Name: "TEST-NAMESPACE-3", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-4", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
			},
		}

		want := []database.CreatePermissionOpts{
			{Namespace: "TEST-NAMESPACE-2", Action: "EXECUTE"},
			{Namespace: "TEST-NAMESPACE-3", Action: "WRITE"},
			{Namespace: "TEST-NAMESPACE-4", Action: "READ"},
			{Namespace: "TEST-NAMESPACE-4", Action: "WRITE"},
		}

		added, deleted := ComparePermissions(dbPerms, schemaPerms)

		require.Len(t, added, 4)
		require.Len(t, deleted, 0)
		if diff := cmp.Diff(want, added); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("permissions deleted and added", func(t *testing.T) {
		schemaPerms := Schema{
			Namespaces: []Namespace{
				{Name: "TEST-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ"}},
				{Name: "TEST-NAMESPACE-2", Actions: []rtypes.NamespaceAction{"READ", "WRITE", "EXECUTE"}},
				{Name: "TEST-NAMESPACE-3", Actions: []rtypes.NamespaceAction{"WRITE"}},
				{Name: "TEST-NAMESPACE-4", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
			},
		}

		wantAdded := []database.CreatePermissionOpts{
			{Namespace: "TEST-NAMESPACE-2", Action: "EXECUTE"},
			{Namespace: "TEST-NAMESPACE-3", Action: "WRITE"},
			{Namespace: "TEST-NAMESPACE-4", Action: "READ"},
			{Namespace: "TEST-NAMESPACE-4", Action: "WRITE"},
		}

		wantDeleted := []database.DeletePermissionOpts{
			// Represents TEST-NAMESPACE-3#READ
			{ID: 5},
			// Represents TEST-NAMESPACE#WRITE
			{ID: 2},
		}

		// do stuff
		added, deleted := ComparePermissions(dbPerms, schemaPerms)

		require.Len(t, added, 4)
		if diff := cmp.Diff(wantAdded, added); diff != "" {
			t.Error(diff)
		}

		require.Len(t, deleted, 2)
		less := func(a, b database.DeletePermissionOpts) bool { return a.ID < b.ID }
		if diff := cmp.Diff(wantDeleted, deleted, cmpopts.SortSlices(less)); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("dotcom permission added", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		schemaPerms := Schema{
			Namespaces: []Namespace{
				{Name: "TEST-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-2", Actions: []rtypes.NamespaceAction{"READ", "WRITE"}},
				{Name: "TEST-NAMESPACE-3", Actions: []rtypes.NamespaceAction{"READ"}},
				{Name: "DOTCOM-NAMESPACE", Actions: []rtypes.NamespaceAction{"READ"},
					DotCom: true}, // now should be included
			},
		}

		added, deleted := ComparePermissions(dbPerms, schemaPerms)

		assert.Len(t, added, 1)
		assert.Len(t, deleted, 0)

		if diff := cmp.Diff([]database.CreatePermissionOpts{
			{Namespace: "DOTCOM-NAMESPACE", Action: "READ"},
		}, added); diff != "" {
			t.Error(diff)
		}
	})
}

func sortDeletePermissionOptSlice(a, b database.DeletePermissionOpts) bool { return a.ID < b.ID }
