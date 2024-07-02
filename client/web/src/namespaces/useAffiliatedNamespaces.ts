import { gql, useQuery } from '@sourcegraph/http-client'

import type {
    Scalars,
    ViewerAffiliatedNamespacesResult,
    ViewerAffiliatedNamespacesVariables,
} from '../graphql-operations'

type Namespace = ViewerAffiliatedNamespacesResult['viewer']['affiliatedNamespaces'][number]

/**
 * React hook that fetches all affiliated namespaces for the viewer. A user's affiliated namespaces
 * are their own user account plus all organizations of which they are a member.
 * @param initialNamespaceID The ID of the namespace to return in {@link initialNamespace}. If not
 * provided, the user's user account is used.
 */
export const useAffiliatedNamespaces = (
    initialNamespaceID?: Scalars['ID']
): {
    namespaces?: NonEmptyArray<Namespace>
    initialNamespace?: Namespace
    loading: boolean
    error?: Error
} => {
    const { data, loading, error } = useQuery<ViewerAffiliatedNamespacesResult, ViewerAffiliatedNamespacesVariables>(
        viewerAffiliatedNamespacesQuery,
        { fetchPolicy: 'cache-and-network' }
    )

    if (error) {
        return { error, loading }
    }
    if (!data || loading) {
        return {
            error: loading ? undefined : new Error('Unable to get affiliated namespaces'),
            loading,
        }
    }

    const namespaces = data.viewer.affiliatedNamespaces
    const initialNamespace = initialNamespaceID
        ? namespaces.find(ns => ns.id === initialNamespaceID)
        : namespaces.find(ns => ns.__typename === 'User')

    if (!isNonEmptyArray(namespaces) || !initialNamespace) {
        // This should never happen, but if it does, surface an error because our callers won't be
        // expecting it.
        return {
            error: new Error('Unexpected empty list of affiliated namespaces'),
            loading,
        }
    }

    return {
        namespaces,
        loading: false,
        initialNamespace,
    }
}

type NonEmptyArray<T> = [T, ...T[]]
function isNonEmptyArray<T>(array: T[]): array is NonEmptyArray<T> {
    return array.length > 0
}

export const viewerAffiliatedNamespacesQuery = gql`
    query ViewerAffiliatedNamespaces {
        viewer {
            affiliatedNamespaces {
                __typename
                id
                namespaceName
                ... on Org {
                    displayName
                }
            }
        }
    }
`
