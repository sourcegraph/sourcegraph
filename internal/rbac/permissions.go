package rbac

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

//go:embed schema.yaml
var schema embed.FS

var RBACSchema = func() Schema {
	contents, err := schema.ReadFile("schema.yaml")
	if err != nil {
		panic(fmt.Sprintf("malformed rbac schema definition: %s", err.Error()))
	}

	var parsedSchema Schema
	if err := yaml.Unmarshal(contents, &parsedSchema); err != nil {
		panic(fmt.Sprintf("malformed rbac schema definition: %s", err.Error()))
	}

	return parsedSchema
}()

// ComparePermissions takes two slices of permissions (one from the database and another from the schema file)
// and extracts permissions that need to be added / deleted in the database based on those contained in the schema file.
func ComparePermissions(dbPerms []*types.Permission, schemaPerms Schema) (added []database.CreatePermissionOpts, deleted []database.DeletePermissionOpts) {
	// Create map to hold the union of both permissions in the database and those in the schema file. `internal/rbac/schema.yaml`
	ps := make(map[string]struct {
		count int
		id    int32
	})

	// save all database permissions to the map
	for _, p := range dbPerms {
		currentPerm := p.DisplayName()
		// Since dbPerms contain an ID we save the ID which will be used to delete redundant permissions.
		// This also ensures all permissions are unique and we never have duplicate permissions.
		ps[currentPerm] = struct {
			count int
			id    int32
		}{
			id:    p.ID,
			count: 1,
		}
	}

	var parsedSchemaPerms []*types.Permission

	for _, n := range schemaPerms.Namespaces {
		// If this is a dotcom-specific namespace, and we are NOT in dotcom mode,
		// do NOT track anything in this namespace.
		if n.DotCom && !dotcom.SourcegraphDotComMode() {
			continue
		}

		for _, a := range n.Actions {
			parsedSchemaPerms = append(parsedSchemaPerms, &types.Permission{
				Namespace: n.Name,
				Action:    a,
			})
		}
	}

	// Check items in schema file to see which exists in the database
	for _, p := range parsedSchemaPerms {
		currentPerm := p.DisplayName()

		if perm, ok := ps[currentPerm]; !ok {
			// If item is not in map, it means it doesn't exist in the database so we
			// add it to the `added` slice.
			added = append(added, database.CreatePermissionOpts{
				Namespace: p.Namespace,
				Action:    p.Action,
			})
		} else {
			// If item is in map, it means it already exist in the database
			ps[currentPerm] = struct {
				count int
				id    int32
			}{
				count: perm.count + 1,
				id:    perm.id,
			}
		}
	}

	// Iterate over map and append permissions with value == 1 to the deleted slice since
	// they only exist in the database and have been removed from the schema file.
	for _, val := range ps {
		if val.count == 1 {
			deleted = append(deleted, database.DeletePermissionOpts{
				ID: val.id,
			})
		}
	}

	return
}
