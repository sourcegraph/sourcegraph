import type { MockedResponse } from '@apollo/client/testing'

import { getDocumentNode } from '@sourcegraph/http-client'

import type {
    CreateSavedSearchResult,
    CreateSavedSearchVariables,
    DeleteSavedSearchResult,
    DeleteSavedSearchVariables,
    SavedSearchFields,
    SavedSearchResult,
    SavedSearchVariables,
    SavedSearchesResult,
    SavedSearchesVariables,
    TransferSavedSearchOwnershipResult,
    TransferSavedSearchOwnershipVariables,
    UpdateSavedSearchResult,
    UpdateSavedSearchVariables,
} from '../graphql-operations'

import {
    createSavedSearchMutation,
    deleteSavedSearchMutation,
    savedSearchQuery,
    savedSearchesQuery,
    transferSavedSearchOwnershipMutation,
    updateSavedSearchMutation,
} from './graphql'

const SAVED_SEARCH_FIELDS: Pick<
    SavedSearchFields,
    '__typename' | 'description' | 'query' | 'owner' | 'url' | 'viewerCanAdminister'
> = {
    __typename: 'SavedSearch',
    description: 'My description',
    query: 'my repo:query',
    owner: {
        __typename: 'User',
        id: 'a',
        namespaceName: 'alice',
    },
    url: '',
    viewerCanAdminister: true,
}

const savedSearchesMock: MockedResponse<SavedSearchesResult, SavedSearchesVariables> = {
    request: {
        query: getDocumentNode(savedSearchesQuery),
        variables: { query: '', owner: '1', after: null, before: null, first: 100, last: null, orderBy: null },
    },
    result: {
        data: {
            savedSearches: {
                nodes: [
                    { ...SAVED_SEARCH_FIELDS, id: '1' },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '2',
                        description: 'Another',
                        query: 'foo type:diff repo:bar',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '3',
                        description: 'Yet another with a longer description that is very long',
                        query: 'foo type:diff repo:bar and a long:query repo:bar and a long:query',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '4',
                        description: '444',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '5',
                        description: '555',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '6',
                        description: '666',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '7',
                        description: '777',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '8',
                        description: '888',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '9',
                        description: '999',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '10',
                        description: '101010',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '11',
                        description: '111111',
                    },
                    {
                        ...SAVED_SEARCH_FIELDS,
                        id: '12',
                        description: '121212',
                    },
                ],
                totalCount: 2,
                pageInfo: {
                    hasNextPage: true,
                    hasPreviousPage: false,
                    endCursor: '',
                    startCursor: '',
                },
            },
        },
    },
}

const savedSearchMock: MockedResponse<SavedSearchResult, SavedSearchVariables> = {
    request: {
        query: getDocumentNode(savedSearchQuery),
        variables: { id: '1' },
    },
    result: {
        data: {
            node: {
                ...SAVED_SEARCH_FIELDS,
                __typename: 'SavedSearch',
                id: '1',
            },
        },
    },
}

const createSavedSearchMock: MockedResponse<CreateSavedSearchResult, CreateSavedSearchVariables> = {
    request: {
        query: getDocumentNode(createSavedSearchMutation),
        variables: {
            input: {
                owner: 'a',
                description: 'My description',
                query: 'my repo:query',
            },
        },
    },
    delay: 500,
    result: {
        data: {
            createSavedSearch: {
                ...SAVED_SEARCH_FIELDS,
                id: '1',
            },
        },
    },
}

const updateSavedSearchMock: MockedResponse<UpdateSavedSearchResult, UpdateSavedSearchVariables> = {
    request: {
        query: getDocumentNode(updateSavedSearchMutation),
        variables: {
            id: '1',
            input: {
                description: 'My description',
                query: 'my repo:query',
            },
        },
    },
    delay: 500,
    result: {
        data: {
            updateSavedSearch: {
                ...SAVED_SEARCH_FIELDS,
                id: '1',
            },
        },
    },
}

const transferSavedSearchOwnershipMock: MockedResponse<
    TransferSavedSearchOwnershipResult,
    TransferSavedSearchOwnershipVariables
> = {
    request: {
        query: getDocumentNode(transferSavedSearchOwnershipMutation),
        variables: {
            id: '1',
            newOwner: 'b',
        },
    },
    delay: 500,
    result: {
        data: {
            transferSavedSearchOwnership: {
                ...SAVED_SEARCH_FIELDS,
                id: '1',
            },
        },
    },
}

const deleteSavedSearchMock: MockedResponse<DeleteSavedSearchResult, DeleteSavedSearchVariables> = {
    request: {
        query: getDocumentNode(deleteSavedSearchMutation),
        variables: { id: '1' },
    },
    delay: 500,
    result: {
        data: {
            deleteSavedSearch: {
                alwaysNil: null,
            },
        },
    },
}

export const MOCK_REQUESTS = [
    savedSearchesMock,
    savedSearchMock,
    createSavedSearchMock,
    updateSavedSearchMock,
    transferSavedSearchOwnershipMock,
    deleteSavedSearchMock,
]
