import { gql } from '@sourcegraph/http-client'

const savedSearchFragment = gql`
    fragment SavedSearchFields on SavedSearch {
        id
        description
        query
        owner {
            __typename
            id
            namespaceName
        }
    }
`

export const savedSearchesQuery = gql`
    query SavedSearches($owner: ID!, $first: Int, $last: Int, $after: String, $before: String) {
        savedSearches(owner: $owner, first: $first, last: $last, after: $after, before: $before) {
            nodes {
                ...SavedSearchFields
            }
            totalCount
            pageInfo {
                hasNextPage
                hasPreviousPage
                endCursor
                startCursor
            }
        }
    }
    ${savedSearchFragment}
`

export const savedSearchQuery = gql`
    query SavedSearch($id: ID!) {
        node(id: $id) {
            ... on SavedSearch {
                id
                description
                query
                owner {
                    id
                }
            }
        }
    }
`

export const createSavedSearchMutation = gql`
    mutation CreateSavedSearch($input: SavedSearchInput!) {
        createSavedSearch(input: $input) {
            ...SavedSearchFields
        }
    }
    ${savedSearchFragment}
`

export const updateSavedSearchMutation = gql`
    mutation UpdateSavedSearch($id: ID!, $input: SavedSearchUpdateInput!) {
        updateSavedSearch(id: $id, input: $input) {
            ...SavedSearchFields
        }
    }
    ${savedSearchFragment}
`

export const deleteSavedSearchMutation = gql`
    mutation DeleteSavedSearch($id: ID!) {
        deleteSavedSearch(id: $id) {
            alwaysNil
        }
    }
`
