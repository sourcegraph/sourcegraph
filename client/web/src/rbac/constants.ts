// Generated code - DO NOT EDIT. Regenerate by running 'bazel run //client/web/src/rbac:write_generated'

export const BatchChangesReadPermission: RbacPermission = 'BATCH_CHANGES#READ'

export const BatchChangesWritePermission: RbacPermission = 'BATCH_CHANGES#WRITE'

export const OwnershipAssignPermission: RbacPermission = 'OWNERSHIP#ASSIGN'

export const RepoMetadataWritePermission = 'REPO_METADATA#WRITE'

export const LicenseManagerReadPermission = 'LICENSE_MANAGER#READ'

export const LicenseManagerWritePermission = 'LICENSE_MANAGER#WRITE'

export const CodyAccessPermission: RbacPermission = 'CODY#ACCESS'

export type RbacPermission =
    | 'BATCH_CHANGES#READ'
    | 'BATCH_CHANGES#WRITE'
    | 'OWNERSHIP#ASSIGN'
    | 'REPO_METADATA#WRITE'
    | 'LICENSE_MANAGER#READ'
    | 'LICENSE_MANAGER#WRITE'
    | 'CODY#ACCESS'
