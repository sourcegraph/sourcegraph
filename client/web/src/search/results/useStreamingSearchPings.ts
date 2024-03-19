import { useCallback, useEffect } from 'react'

import { limitHit } from '@sourcegraph/branded'
import { asError } from '@sourcegraph/common'
import { collectMetrics } from '@sourcegraph/shared/src/search/query/metrics'
import { sanitizeQueryForTelemetry } from '@sourcegraph/shared/src/search/query/transformer'
import type { AggregateStreamingSearchResults } from '@sourcegraph/shared/src/search/stream'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { useNavbarQueryState } from '../../stores'
import { smartSearchEvent } from '../suggestion/SmartSearch'

interface useStreamingSearchPingsProps extends TelemetryProps {
    isAuauthenticated: boolean
    isSourcegraphDotCom: boolean
    results: AggregateStreamingSearchResults | undefined
}

interface StreamingSearchPingsAPI {
    logSearchResultClicked: (index: number, type: string, resultsLength: number) => void
}

export function useStreamingSearchPings(props: useStreamingSearchPingsProps): StreamingSearchPingsAPI {
    const { isAuauthenticated, isSourcegraphDotCom, results, telemetryService } = props

    const submittedURLQuery = useNavbarQueryState(state => state.searchQueryFromURL)

    // Log view event on first load
    useEffect(
        () => {
            telemetryService.logViewEvent('SearchResults')
        },
        // Only log view on initial load
        // eslint-disable-next-line react-hooks/exhaustive-deps
        []
    )

    // Log search query event when URL changes
    useEffect(() => {
        const metrics = submittedURLQuery ? collectMetrics(submittedURLQuery) : undefined

        telemetryService.log(
            'SearchResultsQueried',
            {
                code_search: {
                    query_data: {
                        query: metrics,
                        combined: submittedURLQuery,
                        empty: !submittedURLQuery,
                    },
                },
            },
            {
                code_search: {
                    query_data: {
                        // 🚨 PRIVACY: never provide any private query data in the
                        // { code_search: query_data: query } property,
                        // which is also potentially exported in pings data.
                        query: metrics,

                        // 🚨 PRIVACY: Only collect the full query string for unauthenticated users
                        // on Sourcegraph.com, and only after sanitizing to remove certain filters.
                        combined:
                            !isAuauthenticated && isSourcegraphDotCom
                                ? sanitizeQueryForTelemetry(submittedURLQuery)
                                : undefined,
                        empty: !submittedURLQuery,
                    },
                },
            }
        )
        // Only log when the query changes
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [submittedURLQuery])

    // Log events when search completes or fails
    useEffect(() => {
        if (results?.state === 'complete') {
            telemetryService.log('SearchResultsFetched', {
                code_search: {
                    // 🚨 PRIVACY: never provide any private data in { code_search: { results } }.
                    query_data: {
                        combined: submittedURLQuery,
                    },
                    results: {
                        results_count: results.progress.matchCount,
                        limit_hit: limitHit(results.progress),
                        any_cloning: results.progress.skipped.some(skipped => skipped.reason === 'repository-cloning'),
                        alert: results.alert ? results.alert.title : null,
                    },
                },
            })
            if (results.results.length > 0) {
                telemetryService.log('SearchResultsNonEmpty')
            }
        } else if (results?.state === 'error') {
            telemetryService.log('SearchResultsFetchFailed', {
                code_search: { error_message: asError(results.error).message },
            })
        }
    }, [results, submittedURLQuery, telemetryService])

    useEffect(() => {
        if (
            (results?.alert?.kind === 'smart-search-additional-results' ||
                results?.alert?.kind === 'smart-search-pure-results') &&
            results?.alert?.title &&
            results.alert.proposedQueries
        ) {
            const events = smartSearchEvent(
                results.alert.kind,
                results.alert.title,
                results.alert.proposedQueries.map(entry => entry.description || '')
            )
            for (const event of events) {
                telemetryService.log(event)
            }
        }
    }, [results, telemetryService])

    const logSearchResultClicked = useCallback(
        (index: number, type: string, resultsLength: number) => {
            telemetryService.log('SearchResultClicked')
            // This data ends up in Prometheus and is not part of the ping payload.
            telemetryService.log('search.ranking.result-clicked', {
                index,
                type,
                resultsLength,
            })
        },
        [telemetryService]
    )

    return { logSearchResultClicked }
}
