import { useMemo } from 'react'

import { gql } from '@sourcegraph/http-client'

import type { Scalars } from '../graphql-operations'

import type { PartialNamespace } from '.'

export interface UseNamespacesResult {
    namespaces: PartialNamespace[]
    defaultSelectedNamespace: PartialNamespace
}

/**
 * React hook that fetches all affiliated namespaces for the viewer. A user's affiliated
 * namespaces are their own user account plus all organizations of which they are a member.
 * @param initialNamespaceID The ID of the namespace to return in
 * {@link UseNamespacesResult.defaultSelectedNamespace}. If not provided, the user's user account is
 * used.
 */
export const useAffiliatedNamespaces = (initialNamespaceID?: Scalars['ID']): UseNamespacesResult => {
    const { organizations, ...userDetails } = authenticatedUser

    const organizationNamespaces = organizations.nodes
    const userNamespace = userDetails

    const namespaces = useMemo<UseNamespacesResult['namespaces']>(
        () => [userNamespace, ...organizationNamespaces],
        [userNamespace, organizationNamespaces]
    )

    // The default namespace selected from the dropdown should match whatever the initial
    // namespace was, or else default to the user's namespace.
    const defaultSelectedNamespace = useMemo(() => {
        if (initialNamespaceID) {
            return namespaces.find(namespace => namespace.id === initialNamespaceID) || userNamespace
        }
        return userNamespace
    }, [namespaces, initialNamespaceID, userNamespace])

    return {
        userNamespace,
        namespaces,
        defaultSelectedNamespace,
    }
}

const viewerAffiliatedNamespacesQuery = gql`
    query ViewerAffiliatedNamespaces {
        viewer {
            organizations {
                nodes {
                    id
                    displayName
                    name
                }
            }
        }
    }
`
