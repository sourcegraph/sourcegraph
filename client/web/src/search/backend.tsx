import { of, type Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'

import { queryGraphQL, requestGraphQL } from '../backend/graphql'
import type { EventLogsDataResult, EventLogsDataVariables, ReposByQueryResult, Scalars } from '../graphql-operations'

export function fetchReposByQuery(query: string): Observable<{ name: string; url: string }[]> {
    return queryGraphQL<ReposByQueryResult>(
        gql`
            query ReposByQuery($query: String!) {
                search(query: $query) {
                    results {
                        repositories {
                            name
                            url
                        }
                    }
                }
            }
        `,
        { query }
    ).pipe(
        map(({ data, errors }) => {
            if (!data?.search?.results?.repositories) {
                throw createAggregateError(errors)
            }
            return data.search.results.repositories
        })
    )
}

export interface EventLogResult {
    totalCount: number
    nodes: { argument: string | null; timestamp: string; url: string }[]
    pageInfo: { hasNextPage: boolean }
}

function fetchEvents(userId: Scalars['ID'], first: number, eventName: string): Observable<EventLogResult | null> {
    if (!userId) {
        return of(null)
    }

    const result = requestGraphQL<EventLogsDataResult, EventLogsDataVariables>(
        gql`
            query EventLogsData($userId: ID!, $first: Int, $eventName: String!) {
                node(id: $userId) {
                    ... on User {
                        __typename
                        eventLogs(first: $first, eventName: $eventName) {
                            nodes {
                                argument
                                timestamp
                                url
                            }
                            pageInfo {
                                hasNextPage
                            }
                            totalCount
                        }
                    }
                }
            }
        `,
        { userId, first: first ?? null, eventName }
    )

    return result.pipe(
        map(dataOrThrowErrors),
        map((data: EventLogsDataResult): EventLogResult => {
            if (!data.node || data.node.__typename !== 'User') {
                throw new Error('User not found')
            }
            return data.node.eventLogs
        })
    )
}

export function fetchRecentSearches(userId: Scalars['ID'], first: number): Observable<EventLogResult | null> {
    return fetchEvents(userId, first, 'SearchResultsQueried')
}

export function fetchRecentFileViews(userId: Scalars['ID'], first: number): Observable<EventLogResult | null> {
    return fetchEvents(userId, first, 'ViewBlob')
}
