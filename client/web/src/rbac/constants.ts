// Generated code - DO NOT EDIT. Regenerate by running 'bazel run //client/web/src/rbac:write_generated'

export const BatchChangesReadPermission: RbacPermission = 'BATCH_CHANGES#READ'

export const BatchChangesWritePermission: RbacPermission = 'BATCH_CHANGES#WRITE'

export const OwnershipAssignPermission: RbacPermission = 'OWNERSHIP#ASSIGN'

export const RepoMetadataWritePermission: RbacPermission = 'REPO_METADATA#WRITE'

export const CodyAccessPermission: RbacPermission = 'CODY#ACCESS'

export const ProductSubscriptionsReadPermission: RbacPermission = 'PRODUCT_SUBSCRIPTIONS#READ'

export const ProductSubscriptionsWritePermission: RbacPermission = 'PRODUCT_SUBSCRIPTIONS#WRITE'

export type RbacPermission =
    | 'BATCH_CHANGES#READ'
    | 'BATCH_CHANGES#WRITE'
    | 'OWNERSHIP#ASSIGN'
    | 'REPO_METADATA#WRITE'
    | 'CODY#ACCESS'
    | 'PRODUCT_SUBSCRIPTIONS#READ'
    | 'PRODUCT_SUBSCRIPTIONS#WRITE'
