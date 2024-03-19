import { useCallback, useEffect, useRef, useState } from 'react'

import { logger } from '@sourcegraph/common'
import { gql, useLazyQuery } from '@sourcegraph/http-client'
import type { RecentSearch } from '@sourcegraph/shared/src/settings/temporary/recentSearches'
import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'

import type { SearchHistoryEventLogsQueryResult, SearchHistoryEventLogsQueryVariables } from '../../graphql-operations'

import { RecentSearchesManager } from './recentSearches'

const MAX_RECENT_SEARCHES = 20

export const SEARCH_HISTORY_EVENT_LOGS_QUERY = gql`
    query SearchHistoryEventLogsQuery($first: Int!) {
        currentUser {
            __typename
            recentSearchLogs: eventLogs(first: $first, eventName: "SearchResultsFetched") {
                nodes {
                    argument
                    timestamp
                }
            }
        }
    }
`

// Returns all recent searches from temporary settings and a function to add a new search to the list.
// If no recent searches exist, the temporary settings is initialized with
// the user's recent searches from the event log.
export function useRecentSearches(): {
    recentSearches: RecentSearch[] | undefined
    addRecentSearch: (query: string, resultCount: number, limitHit: boolean) => void
    state: 'loading' | 'success'
} {
    const [recentSearches, setRecentSearches] = useTemporarySetting('search.input.recentSearches', [])
    const [state, setState] = useState<'loading' | 'success'>('loading')
    const recentSearchesManager = useRef(new RecentSearchesManager({ persist: setRecentSearches }))

    // If recentSearches from temporary settings is empty, fetch recent searches from the event log
    // and populate temporary settings with that instead.
    // This is a temporary solution to get users some starting data for this feature.
    // Once this feature is enabled for a while we can remove this.
    const [loadFromEventLog] = useLazyQuery<SearchHistoryEventLogsQueryResult, SearchHistoryEventLogsQueryVariables>(
        SEARCH_HISTORY_EVENT_LOGS_QUERY,
        {
            // Note: It's possible that we end up with less than MAX_RECENT_SEARCHES after removing duplicates.
            // This should be fine, since this is meant to be a starting point
            // for when the user first gets this feature.
            variables: { first: MAX_RECENT_SEARCHES },
        }
    )

    useEffect(() => {
        if (state !== 'success' && recentSearches) {
            if (recentSearches && recentSearches.length > 0) {
                recentSearchesManager.current.setRecentSearches(recentSearches)
                setState('success')
            } else {
                loadFromEventLog()
                    .then(result => {
                        if (result.data) {
                            const processedLogs = processEventLogs(result.data)
                            recentSearchesManager.current.setRecentSearches(processedLogs)
                            setRecentSearches(processedLogs)
                        }
                        setState('success')
                    })
                    .catch(() => {
                        logger.error('Error fetching recent searches from event log')
                        setState('success') // Ignore the error and use the empty list from temporary settings
                    })
            }
        }
    }, [recentSearches, loadFromEventLog, recentSearchesManager, state, setRecentSearches])

    // Adds non-empty queries. A query is considered empty if it's an empty
    // string or only contains a context: filter.
    // If the search is being added after the list is finished loading,
    // add it immediately.
    // If the search is being added before the list is finished loading,
    // queue it to be added after loading is complete.
    const addRecentSearch = useCallback(
        (query: string, resultCount: number, limitHit: boolean) => {
            recentSearchesManager.current.addRecentSearch({ query, resultCount, limitHit })
        },
        [recentSearchesManager]
    )

    return { recentSearches, addRecentSearch, state }
}

function processEventLogs(data: SearchHistoryEventLogsQueryResult): RecentSearch[] {
    if (data.currentUser?.__typename !== 'User') {
        return []
    }
    const searches = data.currentUser.recentSearchLogs.nodes
        .filter(node => node.argument && node.timestamp)
        .map(node => {
            const argument = node.argument
                ? (JSON.parse(node.argument) as {
                      code_search?: {
                          results?: { results_count?: number; limit_hit?: boolean }
                          query_data?: { combined?: string }
                      }
                  })
                : {}

            return {
                query: argument.code_search?.query_data?.combined || '',
                resultCount: argument.code_search?.results?.results_count || 0,
                limitHit: argument.code_search?.results?.limit_hit || false,
                timestamp: node.timestamp,
            }
        })
        .filter(search => search.query)
        .filter(
            // Remove duplicates
            // Items are sorted by timestamp, so the first item is the most recent.
            // If a search appears earlier in the list, it is a duplicate.
            (search, index, self) => index === self.findIndex(item => item.query === search.query)
        )
    return searches
}
