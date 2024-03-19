package rbac

import rtypes "github.com/sourcegraph/sourcegraph/internal/rbac/types"

// Schema refers to the RBAC structure which acts as a source of truth for permissions within
// the RBAC system.
type Schema struct {
	Namespaces          []Namespace                  `yaml:"namespaces"`
	ExcludeFromUserRole []rtypes.PermissionNamespace `yaml:"excludeFromUserRole"`
}

// Namespace represents a feature to be guarded by RBAC. (example: Batch Changes, Code Insights e.t.c)
type Namespace struct {
	Name    rtypes.PermissionNamespace `yaml:"name"`
	Actions []rtypes.NamespaceAction   `yaml:"actions"`
	// DotCom delimits namespaces that should ONLY exist on Sourcegraph.com,
	// i.e. in "dotcom mode". If true, this namespace will not be distributed
	// on standard single-tenant instances.
	//
	// This is only an escape hatch - ultimately we want things that require
	// special access in "dotcom mode" to exist as standalone services:
	// https://handbook.sourcegraph.com/departments/engineering/teams/core-services/managed-services/platform
	// In the interim, this is a relatively better way to manage certain
	// features.
	DotCom bool `yaml:"dotcom"`
}
