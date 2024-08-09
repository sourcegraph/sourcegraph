import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import classNames from 'classnames'
import type { Observable } from 'rxjs'
import { useDeepCompareEffectNoCheck } from 'use-deep-compare-effect'

import { type IEditor, SearchBox, StreamingProgress, StreamingSearchResultsList } from '@sourcegraph/branded'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { type FetchFileParameters, fetchHighlightedFileLineRanges } from '@sourcegraph/shared/src/backend/file'
import { getUserSearchContextNamespaces, type QueryState, type SearchMode } from '@sourcegraph/shared/src/search'
import { collectMetrics } from '@sourcegraph/shared/src/search/query/metrics'
import {
    appendContextFilter,
    sanitizeQueryForTelemetry,
    updateFilters,
} from '@sourcegraph/shared/src/search/query/transformer'
import { LATEST_VERSION, type RepositoryMatch, type SearchMatch } from '@sourcegraph/shared/src/search/stream'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'

import { SearchPatternType } from '../../graphql-operations'
import type { SearchResultsState } from '../../state'
import type { WebviewPageProps } from '../platform/context'

import { fetchSearchContexts } from './alias/fetchSearchContext'
import { setFocusSearchBox } from './api'
import { SearchResultsInfoBar } from './components/SearchResultsInfoBar'
import { MatchHandlersContext, useMatchHandlers } from './MatchHandlersContext'
import { RepoView } from './RepoView'

import styles from './index.module.scss'

export interface SearchResultsViewProps extends WebviewPageProps {
    context: SearchResultsState['context']
}

export const SearchResultsView: React.FunctionComponent<React.PropsWithChildren<SearchResultsViewProps>> = ({
    extensionCoreAPI,
    authenticatedUser,
    platformContext,
    settingsCascade,
    context,
    instanceURL,
}) => {
    const [userQueryState, setUserQueryState] = useState<QueryState>(context.submittedSearchQueryState.queryState)
    const [repoToShow, setRepoToShow] = useState<Pick<
        RepositoryMatch,
        'repository' | 'branches' | 'description'
    > | null>(null)

    const isSourcegraphDotCom = useMemo(() => {
        const hostname = new URL(instanceURL).hostname
        return hostname === 'sourcegraph.com' || hostname === 'www.sourcegraph.com'
    }, [instanceURL])

    // Editor focus.
    const editorReference = useRef<IEditor>()
    const setEditor = useCallback((editor: IEditor) => {
        editorReference.current = editor
        setTimeout(() => editor.focus(), 0)
    }, [])

    // TODO explain
    useEffect(() => {
        setFocusSearchBox(() => editorReference.current?.focus())

        return () => {
            setFocusSearchBox(null)
        }
    }, [])

    const onChange = useCallback(
        (newState: QueryState) => {
            setUserQueryState(newState)

            extensionCoreAPI
                .setSidebarQueryState({
                    queryState: newState,
                    searchCaseSensitivity: context.submittedSearchQueryState?.searchCaseSensitivity,
                    searchPatternType: context.submittedSearchQueryState?.searchPatternType,
                    searchMode: context.submittedSearchQueryState?.searchMode,
                })
                .catch(error => {
                    // TODO surface error to users
                    console.error('Error updating sidebar query state from panel', error)
                })
        },
        [
            extensionCoreAPI,
            context.submittedSearchQueryState.searchCaseSensitivity,
            context.submittedSearchQueryState.searchPatternType,
            context.submittedSearchQueryState.searchMode,
        ]
    )

    const [allExpanded, setAllExpanded] = useState(false)
    const onExpandAllResultsToggle = useCallback(() => {
        setAllExpanded(oldValue => !oldValue)
        platformContext.telemetryService.log(allExpanded ? 'allResultsExpanded' : 'allResultsCollapsed')
    }, [allExpanded, platformContext])

    // Update local query state on sidebar query state updates.
    useDeepCompareEffectNoCheck(() => {
        if (context.searchSidebarQueryState.proposedQueryState?.queryState) {
            setUserQueryState(context.searchSidebarQueryState.proposedQueryState?.queryState)
        }
    }, [context.searchSidebarQueryState.proposedQueryState?.queryState])

    // Update local search query state on sidebar search submission.
    useDeepCompareEffectNoCheck(() => {
        setUserQueryState(context.submittedSearchQueryState.queryState)
        // It's a whole new object on each state update, so we need
        // to compare (alternatively, construct full query TODO)

        // Clear repo view
        setRepoToShow(null)
    }, [context.submittedSearchQueryState.queryState])

    // Track sidebar + keyboard shortcut search submissions
    useEffect(() => {
        platformContext.telemetryService.log('IDESearchSubmitted')
    }, [platformContext, context.submittedSearchQueryState.queryState.query])

    const onSubmit = useCallback(
        (options?: {
            caseSensitive?: boolean
            patternType?: SearchPatternType
            newQuery?: string
            searchMode?: SearchMode
        }) => {
            const previousSearchQueryState = context.submittedSearchQueryState

            const query = options?.newQuery ?? userQueryState.query
            const caseSensitive = options?.caseSensitive ?? previousSearchQueryState.searchCaseSensitivity
            const patternType = options?.patternType ?? previousSearchQueryState.searchPatternType
            const searchMode = options?.searchMode ?? previousSearchQueryState.searchMode

            extensionCoreAPI
                .streamSearch(query, {
                    caseSensitive,
                    patternType,
                    searchMode,
                    version: LATEST_VERSION,
                    trace: undefined,
                    chunkMatches: true,
                })
                .then(() => {
                    editorReference.current?.focus()
                })
                .catch(error => {
                    // TODO surface error to users? Errors will typically be caught and
                    // surfaced throught streaming search reuls.
                    console.error(error)
                })

            extensionCoreAPI
                .setSidebarQueryState({
                    queryState: { query },
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
            // Clear repo view
            setRepoToShow(null)
        },
        [
            context.submittedSearchQueryState,
            context.selectedSearchContextSpec,
            userQueryState.query,
            extensionCoreAPI,
            instanceURL,
            platformContext.telemetryService,
            authenticatedUser,
            isSourcegraphDotCom,
        ]
    )

    // Submit new search on change
    const setCaseSensitivity = useCallback(
        (caseSensitive: boolean) => {
            onSubmit({ caseSensitive })
        },
        [onSubmit]
    )

    // Submit new search on change
    const setPatternType = useCallback(
        (patternType: SearchPatternType) => {
            onSubmit({ patternType })
        },
        [onSubmit]
    )

    const setSearchMode = useCallback(
        (searchMode: SearchMode) => {
            onSubmit({ searchMode })
        },
        [onSubmit]
    )

    const fetchHighlightedFileLineRangesWithContext = useCallback(
        (parameters: FetchFileParameters) => fetchHighlightedFileLineRanges({ ...parameters, platformContext }),
        [platformContext]
    )

    const fetchStreamSuggestions = useCallback(
        (query: string): Observable<SearchMatch[]> =>
            wrapRemoteObservable(extensionCoreAPI.fetchStreamSuggestions(query, instanceURL)),
        [extensionCoreAPI, instanceURL]
    )

    const setSelectedSearchContextSpec = useCallback(
        (spec: string) => {
            extensionCoreAPI
                .setSelectedSearchContextSpec(spec)
                .catch(error => {
                    console.error('Error persisting search context spec.', error)
                })
                .finally(() => {
                    // Execute search with new context state
                    onSubmit()
                })
        },
        [extensionCoreAPI, onSubmit]
    )

    const onSearchAgain = useCallback(
        (additionalFilters: string[]) => {
            platformContext.telemetryService.log('SearchSkippedResultsAgainClicked')
            onSubmit({
                newQuery: applyAdditionalFilters(context.submittedSearchQueryState.queryState.query, additionalFilters),
            })
        },
        [context.submittedSearchQueryState.queryState, platformContext, onSubmit]
    )

    const onShareResultsClick = useCallback(async (): Promise<void> => {
        const queryState = context.submittedSearchQueryState

        const path = `/search?${buildSearchURLQuery(
            queryState.queryState.query,
            queryState.searchPatternType,
            queryState.searchCaseSensitivity,
            context.selectedSearchContextSpec
        )}&utm_campaign=vscode-extension&utm_medium=direct_traffic&utm_source=vscode-extension&utm_content=save-search`
        await extensionCoreAPI.copyLink(new URL(path, instanceURL).href)
        platformContext.telemetryService.log('VSCEShareLinkClick')
    }, [context, instanceURL, extensionCoreAPI, platformContext])

    const fullQuery = useMemo(
        () =>
            appendContextFilter(
                context.submittedSearchQueryState.queryState.query ?? '',
                context.selectedSearchContextSpec
            ),
        [context]
    )

    const matchHandlers = useMatchHandlers({
        platformContext,
        extensionCoreAPI,
        authenticatedUser,
        onRepoSelected: setRepoToShow,
        instanceURL,
    })

    const clearRepositoryToShow = (): void => setRepoToShow(null)

    return (
        <div className={styles.resultsViewLayout}>
            {/* eslint-disable-next-line react/forbid-elements */}
            <form
                className="d-flex w-100 my-2 pb-2 border-bottom"
                onSubmit={event => {
                    event.preventDefault()
                    onSubmit()
                }}
            >
                <SearchBox
                    caseSensitive={context.submittedSearchQueryState?.searchCaseSensitivity}
                    setCaseSensitivity={setCaseSensitivity}
                    patternType={context.submittedSearchQueryState?.searchPatternType}
                    defaultPatternType={SearchPatternType.standard}
                    setPatternType={setPatternType}
                    searchMode={context.submittedSearchQueryState?.searchMode}
                    setSearchMode={setSearchMode}
                    isSourcegraphDotCom={isSourcegraphDotCom}
                    structuralSearchDisabled={false}
                    queryState={userQueryState}
                    onChange={onChange}
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
                    telemetryRecorder={platformContext.telemetryRecorder}
                    platformContext={platformContext}
                    className={classNames('flex-grow-1 flex-shrink-past-contents', styles.searchBox)}
                    containerClassName={styles.searchBoxContainer}
                    autoFocus={true}
                    onEditorCreated={setEditor}
                />
            </form>

            {!repoToShow ? (
                <div className={styles.resultsViewScrollContainer}>
                    <SearchResultsInfoBar
                        onShareResultsClick={onShareResultsClick}
                        extensionCoreAPI={extensionCoreAPI}
                        patternType={context.submittedSearchQueryState.searchPatternType}
                        authenticatedUser={authenticatedUser}
                        platformContext={platformContext}
                        stats={
                            <StreamingProgress
                                query={context.submittedSearchQueryState.queryState.query}
                                progress={
                                    context.searchResults?.progress || { durationMs: 0, matchCount: 0, skipped: [] }
                                }
                                state={context.searchResults?.state || 'loading'}
                                onSearchAgain={onSearchAgain}
                                showTrace={false}
                                telemetryService={platformContext.telemetryService}
                                telemetryRecorder={platformContext.telemetryRecorder}
                            />
                        }
                        allExpanded={allExpanded}
                        onExpandAllResultsToggle={onExpandAllResultsToggle}
                        instanceURL={instanceURL}
                        fullQuery={fullQuery}
                    />
                    <MatchHandlersContext.Provider value={{ ...matchHandlers, instanceURL }}>
                        <StreamingSearchResultsList
                            settingsCascade={settingsCascade}
                            telemetryService={platformContext.telemetryService}
                            telemetryRecorder={platformContext.telemetryRecorder}
                            allExpanded={allExpanded}
                            // Debt: dotcom prop used only for "run search" link
                            // for search examples. Fix on VSCE.
                            isSourcegraphDotCom={false}
                            searchContextsEnabled={true}
                            platformContext={platformContext}
                            results={context.searchResults ?? undefined}
                            fetchHighlightedFileLineRanges={fetchHighlightedFileLineRangesWithContext}
                            executedQuery={context.submittedSearchQueryState.queryState.query}
                            resultClassName="mr-0"
                            selectedSearchContextSpec={context.selectedSearchContextSpec}
                            // The VSCode extension does not support file preview, so turn that off in the search results.
                            hideFilePreviewButton={true}
                            showQueryExamplesOnNoResultsPage={true}
                            // For VSCode, the default pattern type is hardcoded to "standard".
                            showQueryExamplesForKeywordSearch={false}
                        />
                    </MatchHandlersContext.Provider>
                </div>
            ) : (
                <div className={styles.resultsViewScrollContainer}>
                    <RepoView
                        extensionCoreAPI={extensionCoreAPI}
                        platformContext={platformContext}
                        onBackToSearchResults={clearRepositoryToShow}
                        repositoryMatch={repoToShow}
                        setQueryState={setUserQueryState}
                        instanceURL={instanceURL}
                    />
                </div>
            )}
        </div>
    )
}

const applyAdditionalFilters = (query: string, additionalFilters: string[]): string => {
    let newQuery = query
    for (const filter of additionalFilters) {
        const fieldValue = filter.split(':', 2)
        newQuery = updateFilters(newQuery, fieldValue[0], fieldValue[1])
    }
    return newQuery
}
