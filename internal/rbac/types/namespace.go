// Generated code - DO NOT EDIT. Regenerate by running 'bazel run //internal/rbac:write_generated'
package types

// A PermissionNamespace represents a distinct context within which permission policies
// are defined and enforced.
type PermissionNamespace string

func (n PermissionNamespace) String() string {
	return string(n)
}

const BatchChangesNamespace PermissionNamespace = "BATCH_CHANGES"
const OwnershipNamespace PermissionNamespace = "OWNERSHIP"
const RepoMetadataNamespace PermissionNamespace = "REPO_METADATA"
const LicenseManagerNamespace PermissionNamespace = "LICENSE_MANAGER"
const CodyNamespace PermissionNamespace = "CODY"

// Valid checks if a namespace is valid and supported by Sourcegraph's RBAC system.
func (n PermissionNamespace) Valid() bool {
	switch n {
	case BatchChangesNamespace, OwnershipNamespace, RepoMetadataNamespace, LicenseManagerNamespace, CodyNamespace:
		return true
	default:
		return false
	}
}
