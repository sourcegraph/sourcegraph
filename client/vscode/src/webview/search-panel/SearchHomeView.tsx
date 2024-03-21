import React, { useCallback, useMemo, useState } from 'react'

import classNames from 'classnames'
import type { Observable } from 'rxjs'
import { useDeepCompareEffectNoCheck } from 'use-deep-compare-effect'

import { SearchBox } from '@sourcegraph/branded'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { getUserSearchContextNamespaces, type QueryState, SearchMode } from '@sourcegraph/shared/src/search'
import { collectMetrics } from '@sourcegraph/shared/src/search/query/metrics'
import { appendContextFilter, sanitizeQueryForTelemetry } from '@sourcegraph/shared/src/search/query/transformer'
import { LATEST_VERSION, type SearchMatch } from '@sourcegraph/shared/src/search/stream'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'

import { SearchPatternType } from '../../graphql-operations'
import type { SearchHomeState } from '../../state'
import type { WebviewPageProps } from '../platform/context'

import { fetchSearchContexts } from './alias/fetchSearchContext'
import { BrandHeader } from './components/BrandHeader'
import { HomeFooter } from './components/HomeFooter'

import styles from './index.module.scss'

export interface SearchHomeViewProps extends WebviewPageProps {
    context: SearchHomeState['context']
}

export const SearchHomeView: React.FunctionComponent<React.PropsWithChildren<SearchHomeViewProps>> = ({
    extensionCoreAPI,
    authenticatedUser,
    platformContext,
    settingsCascade,
    context,
    instanceURL,
}) => {
    const isLightTheme = useIsLightTheme()

    // Toggling case sensitivity or pattern type does NOT trigger a new search on home view.
    const [caseSensitive, setCaseSensitivity] = useState(false)
    const [patternType, setPatternType] = useState(SearchPatternType.standard)
    const [searchMode, setSearchMode] = useState(SearchMode.SmartSearch)

    const [userQueryState, setUserQueryState] = useState<QueryState>({
        query: '',
    })

    const isSourcegraphDotCom = useMemo(() => {
        const hostname = new URL(instanceURL).hostname
        return hostname === 'sourcegraph.com' || hostname === 'www.sourcegraph.com'
    }, [instanceURL])

    const onSubmit = useCallback(() => {
        extensionCoreAPI
            .streamSearch(userQueryState.query, {
                caseSensitive,
                patternType,
                searchMode,
                version: LATEST_VERSION,
                trace: undefined,
            })
            .catch(error => {
                // TODO surface error to users? Errors will typically be caught and
                // surfaced throught streaming search reuls.
                console.error(error)
            })

        extensionCoreAPI
            .setSidebarQueryState({
                queryState: { query: userQueryState.query },
                searchCaseSensitivity: caseSensitive,
                searchPatternType: patternType,
                searchMode,
            })
            .catch(error => {
                // TODO surface error to users
                console.error('Error updating sidebar query state from panel', error)
            })

        // Log Search History
        const hostname = new URL(instanceURL).hostname
        let queryString = `${userQueryState.query}${caseSensitive ? ' case:yes' : ''}`
        if (context.selectedSearchContextSpec) {
            queryString = appendContextFilter(queryString, context.selectedSearchContextSpec)
        }
        const metrics = queryString ? collectMetrics(queryString) : undefined
        platformContext.telemetryService.log(
            'SearchResultsQueried',
            {
                code_search: {
                    query_data: {
                        query: metrics,
                        combined: queryString,
                        empty: !queryString,
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
                            !authenticatedUser && isSourcegraphDotCom
                                ? sanitizeQueryForTelemetry(queryString)
                                : undefined,
                        empty: !queryString,
                    },
                },
            },
            `https://${hostname}/search?q=${encodeURIComponent(queryString)}&patternType=${patternType}`
        )
    }, [
        extensionCoreAPI,
        userQueryState.query,
        caseSensitive,
        patternType,
        searchMode,
        instanceURL,
        context.selectedSearchContextSpec,
        platformContext.telemetryService,
        authenticatedUser,
        isSourcegraphDotCom,
    ])

    // Update local query state on sidebar query state updates.
    useDeepCompareEffectNoCheck(() => {
        if (context.searchSidebarQueryState.proposedQueryState?.queryState) {
            setUserQueryState(context.searchSidebarQueryState.proposedQueryState?.queryState)
        }
    }, [context.searchSidebarQueryState.proposedQueryState?.queryState])

    const setSelectedSearchContextSpec = useCallback(
        (spec: string) => {
            extensionCoreAPI.setSelectedSearchContextSpec(spec).catch(error => {
                console.error('Error persisting search context spec.', error)
            })
        },
        [extensionCoreAPI]
    )

    const fetchStreamSuggestions = useCallback(
        (query: string): Observable<SearchMatch[]> =>
            wrapRemoteObservable(extensionCoreAPI.fetchStreamSuggestions(query, instanceURL)),
        [extensionCoreAPI, instanceURL]
    )

    return (
        <div className="d-flex flex-column align-items-center">
            <BrandHeader isLightTheme={isLightTheme} />

            <div className={styles.homeSearchBoxContainer}>
                {/* eslint-disable-next-line react/forbid-elements */}
                <form
                    className="d-flex my-2"
                    onSubmit={event => {
                        event.preventDefault()
                        onSubmit()
                    }}
                >
                    <SearchBox
                        caseSensitive={caseSensitive}
                        setCaseSensitivity={setCaseSensitivity}
                        patternType={patternType}
                        setPatternType={setPatternType}
                        searchMode={searchMode}
                        setSearchMode={setSearchMode}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        structuralSearchDisabled={false}
                        queryState={userQueryState}
                        onChange={setUserQueryState}
                        onSubmit={onSubmit}
                        authenticatedUser={authenticatedUser}
                        searchContextsEnabled={true}
                        showSearchContext={true}
                        showSearchContextManagement={false}
                        setSelectedSearchContextSpec={setSelectedSearchContextSpec}
                        selectedSearchContextSpec={context.selectedSearchContextSpec}
                        fetchSearchContexts={fetchSearchContexts}
                        getUserSearchContextNamespaces={getUserSearchContextNamespaces}
                        fetchStreamSuggestions={fetchStreamSuggestions}
                        telemetryService={platformContext.telemetryService}
                        platformContext={platformContext}
                        className={classNames('flex-grow-1 flex-shrink-past-contents', styles.searchBox)}
                        containerClassName={styles.searchBoxContainer}
                        autoFocus={true}
                    />
                </form>

                <HomeFooter setQuery={setUserQueryState} telemetryService={platformContext.telemetryService} />
            </div>
        </div>
    )
}
