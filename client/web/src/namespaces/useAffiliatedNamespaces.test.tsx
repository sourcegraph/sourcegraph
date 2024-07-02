import { MockedProvider, MockedResponse } from '@apollo/client/testing'
import { renderHook } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { getDocumentNode } from '@sourcegraph/http-client'
import { waitForNextApolloResponse } from '@sourcegraph/shared/src/testing/apollo'

import { ViewerAffiliatedNamespacesResult, ViewerAffiliatedNamespacesVariables } from '../graphql-operations'

import { useAffiliatedNamespaces, viewerAffiliatedNamespacesQuery } from './useAffiliatedNamespaces'

const viewerAffiliatedNamespacesMock: MockedResponse<
    ViewerAffiliatedNamespacesResult,
    ViewerAffiliatedNamespacesVariables
> = {
    request: { query: getDocumentNode(viewerAffiliatedNamespacesQuery) },
    result: {
        data: {
            viewer: {
                affiliatedNamespaces: [
                    { __typename: 'User', id: 'user1', namespaceName: 'alice' },
                    {
                        __typename: 'Org',
                        id: 'org1',
                        namespaceName: 'abc',
                        displayName: 'ABC',
                    },
                    {
                        __typename: 'Org',
                        id: 'org2',
                        namespaceName: 'xyz',
                        displayName: 'XYZ',
                    },
                ],
            },
        },
    },
}

describe('useAffiliatedNamespaces', () => {
    test('fetches namespaces', async () => {
        const { result } = renderHook(() => useAffiliatedNamespaces(), {
            wrapper: ({ children }) => (
                <MockedProvider mocks={[viewerAffiliatedNamespacesMock]}>{children}</MockedProvider>
            ),
        })
        await waitForNextApolloResponse()

        expect(result.current.namespaces?.map(ns => ns.id)).toEqual(['user1', 'org1', 'org2'])
        expect(result.current.initialNamespace?.id).toEqual('user1')
        expect(result.current.error).toBeUndefined()
    })

    test('initialNamespaceID', async () => {
        const { result } = renderHook(() => useAffiliatedNamespaces('org2'), {
            wrapper: ({ children }) => (
                <MockedProvider mocks={[viewerAffiliatedNamespacesMock]}>{children}</MockedProvider>
            ),
        })
        await waitForNextApolloResponse()

        expect(result.current.namespaces?.map(ns => ns.id)).toEqual(['user1', 'org1', 'org2'])
        expect(result.current.initialNamespace?.id).toEqual('org2')
        expect(result.current.error).toBeUndefined()
    })
})
