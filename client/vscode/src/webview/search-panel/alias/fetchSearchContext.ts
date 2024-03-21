import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'

export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] }
export type Maybe<T> = T | null

/** All built-in and custom scalars, mapped to their actual values */
export interface Scalars {
    ID: string
    String: string
    Boolean: boolean
    Int: number
    Float: number
    /** A quadruple that represents all possible states of the published value: true, false, 'draft', or null. */
    PublishedValue: boolean | 'draft'
    /** A valid JSON value. */
    JSONValue: unknown
    /** A string that contains valid JSON, with additional support for //-style comments and trailing commas. */
    JSONCString: string
    /** A Git object ID (SHA-1 hash, 40 hexadecimal characters). */
    GitObjectID: string
    /** An arbitrarily large integer encoded as a decimal string. */
    BigInt: string
    /**
     * An RFC 3339-encoded UTC date string, such as 1973-11-29T21:33:09Z. This value can be parsed into a
     * JavaScript Date using Date.parse. To produce this value from a JavaScript Date instance, use
     * Date#toISOString.
     */
    DateTime: string
}

/**
 * This is a copy of the fetchSearchContexts function from @sourcegraph/search created for VSCE use
 * We have removed `query` from SearchContext to support instances below v3.36.0
 * as query does not exist in Search Context type in older instances
 * More context in https://github.com/sourcegraph/sourcegraph/issues/31022
 **/

const searchContextFragment = gql`
    fragment SearchContextFields on SearchContext {
        __typename
        id
        name
        namespace {
            __typename
            id
            namespaceName
        }
        spec
        description
        public
        autoDefined
        updatedAt
        viewerCanManage
        viewerHasStarred
        viewerHasAsDefault
        repositories {
            __typename
            repository {
                name
            }
            revisions
        }
    }
`

export function fetchSearchContexts({
    first,
    namespaces,
    query,
    after,
    orderBy,
    descending,
    platformContext,
}: {
    first: number
    query?: string
    namespaces?: Maybe<Scalars['ID']>[]
    after?: string
    orderBy?: SearchContextsOrderBy
    descending?: boolean
    platformContext: Pick<PlatformContext, 'requestGraphQL'>
}): Observable<ListSearchContextsResult['searchContexts']> {
    return platformContext
        .requestGraphQL<ListSearchContextsResult, ListSearchContextsVariables>({
            request: gql`
                query ListSearchContexts(
                    $first: Int!
                    $after: String
                    $query: String
                    $namespaces: [ID]
                    $orderBy: SearchContextsOrderBy
                    $descending: Boolean
                ) {
                    searchContexts(
                        first: $first
                        after: $after
                        query: $query
                        namespaces: $namespaces
                        orderBy: $orderBy
                        descending: $descending
                    ) {
                        nodes {
                            ...SearchContextFields
                        }
                        pageInfo {
                            hasNextPage
                            endCursor
                        }
                        totalCount
                    }
                }
                ${searchContextFragment}
            `,
            variables: {
                first,
                after: after ?? null,
                query: query ?? null,
                namespaces: namespaces ?? [],
                orderBy: orderBy ?? SearchContextsOrderBy.SEARCH_CONTEXT_SPEC,
                descending: descending ?? false,
            },
            mightContainPrivateInfo: true,
        })
        .pipe(
            map(dataOrThrowErrors),
            map(data => data.searchContexts)
        )
}

export interface SearchContextFields {
    __typename: 'SearchContext'
    id: string
    name: string
    spec: string
    description: string
    public: boolean
    autoDefined: boolean
    updatedAt: string
    viewerCanManage: boolean
    viewerHasAsDefault: boolean
    viewerHasStarred: boolean
    query: string
    namespace: Maybe<
        | { __typename: 'User'; id: string; namespaceName: string }
        | { __typename: 'Org'; id: string; namespaceName: string }
    >
    repositories: {
        __typename: 'SearchContextRepositoryRevisions'
        revisions: string[]
        repository: { __typename?: 'Repository'; name: string }
    }[]
}

export type ListSearchContextsVariables = Exact<{
    first: Scalars['Int']
    after: Maybe<Scalars['String']>
    query: Maybe<Scalars['String']>
    namespaces: Maybe<Maybe<Scalars['ID']>[]>
    orderBy: Maybe<SearchContextsOrderBy>
    descending: Maybe<Scalars['Boolean']>
}>

export interface ListSearchContextsResult {
    __typename?: 'Query'
    searchContexts: {
        __typename?: 'SearchContextConnection'
        totalCount: number
        nodes: ({ __typename?: 'SearchContext' } & SearchContextFields)[]
        pageInfo: { __typename?: 'PageInfo'; hasNextPage: boolean; endCursor: Maybe<string> }
    }
}

/** SearchContextsOrderBy enumerates the ways a search contexts list can be ordered. */
export enum SearchContextsOrderBy {
    SEARCH_CONTEXT_SPEC = 'SEARCH_CONTEXT_SPEC',
    SEARCH_CONTEXT_UPDATED_AT = 'SEARCH_CONTEXT_UPDATED_AT',
}
