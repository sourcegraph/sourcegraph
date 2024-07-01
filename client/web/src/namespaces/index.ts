import type { OrgSettingFields, UserSettingFields } from '@sourcegraph/shared/src/graphql-operations'

/**
 * Common props for components underneath a namespace (e.g., a user or organization).
 */
export interface NamespaceProps {
    /**
     * The namespace.
     */
    namespace: PartialNamespace
}

export type PartialNamespace =
    | Pick<UserSettingFields, '__typename' | 'id' | 'username' | 'displayName' | 'namespaceName' | 'url'>
    | Pick<OrgSettingFields, '__typename' | 'id' | 'name' | 'displayName' | 'namespaceName' | 'url'>
