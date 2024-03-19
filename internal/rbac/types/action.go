// Generated code - DO NOT EDIT. Regenerate by running 'bazel run //internal/rbac:write_generated'
package types

// NamespaceAction represents the action permitted in a namespace.
type NamespaceAction string

func (a NamespaceAction) String() string {
	return string(a)
}

const BatchChangesReadAction NamespaceAction = "READ"
const BatchChangesWriteAction NamespaceAction = "WRITE"
const OwnershipAssignAction NamespaceAction = "ASSIGN"
const RepoMetadataWriteAction NamespaceAction = "WRITE"
const CodyAccessAction NamespaceAction = "ACCESS"
const ProductSubscriptionsReadAction NamespaceAction = "READ"
const ProductSubscriptionsWriteAction NamespaceAction = "WRITE"
