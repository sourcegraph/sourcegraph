import type { SharedGraphQlOperations } from '@sourcegraph/shared/src/graphql-operations'
import { sharedGraphQlResults } from '@sourcegraph/shared/src/testing/integration/graphQlResults'

import type { VSCodeGraphQlOperations } from '../src/graphql-operations'

/**
 * Predefined results for GraphQL requests that are made on almost every user flow.
 */
export const commonVSCodeGraphQlResults: Partial<VSCodeGraphQlOperations & SharedGraphQlOperations> = {
    ...sharedGraphQlResults,
    LogEvents: () => ({
        __typename: 'Mutation',
        logEvents: null,
    }),
    CurrentAuthState: () => ({
        __typename: 'Query',
        currentUser: null,
    }),
    ListSearchContexts: () => ({
        searchContexts: {
            nodes: [],
            totalCount: 0,
            pageInfo: { hasNextPage: false, endCursor: null },
        },
    }),
    IsSearchContextAvailable: () => ({
        isSearchContextAvailable: true,
    }),
    SiteProductVersion: () => ({
        __typename: 'Query',
        site: {
            __typename: 'Site',
            productVersion: '3.38.2',
        },
    }),
    RepositoryMetadata: () => ({
        __typename: 'Query',
        repositoryRedirect: null,
    }),
    TreeEntries: () => ({
        __typename: 'Query',
        repository: null,
    }),
    FileNames: () => ({
        __typename: 'Query',
        repository: null,
    }),
    BlobContent: () => ({
        __typename: 'Query',
        repository: null,
    }),
}
