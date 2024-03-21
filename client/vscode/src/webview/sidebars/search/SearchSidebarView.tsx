import React, { type FC, type ReactElement, type ReactNode, useCallback, useMemo } from 'react'

import { useLocation, useNavigate } from 'react-router-dom'
import { useDeepCompareEffectNoCheck } from 'use-deep-compare-effect'
import create from 'zustand'

import {
    getDynamicFilterLinks,
    getFiltersOfKind,
    getQuickLinks,
    getRepoFilterLinks,
    getSearchReferenceFactory,
    getSearchSnippetLinks,
    StickySearchSidebar,
    SearchSidebarSection,
    PersistSidebarStoreProvider,
} from '@sourcegraph/branded'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import {
    InitialParametersSource,
    type QueryUpdate,
    SearchMode,
    type SearchQueryState,
    type SearchQueryStateStore,
    updateQuery,
} from '@sourcegraph/shared/src/search'
import { FilterType } from '@sourcegraph/shared/src/search/query/filters'
import { type Filter, LATEST_VERSION } from '@sourcegraph/shared/src/search/stream'
import { SectionID } from '@sourcegraph/shared/src/settings/temporary/searchSidebar'
import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'
import { Code, useObservable } from '@sourcegraph/wildcard'

import { SearchPatternType } from '../../../graphql-operations'
import type { WebviewPageProps } from '../../platform/context'

import styles from './SearchSidebarView.module.scss'

interface SearchSidebarViewProps
    extends Pick<WebviewPageProps, 'settingsCascade' | 'platformContext' | 'extensionCoreAPI'> {
    filters?: Filter[] | undefined
}

export const SearchSidebarView: FC<SearchSidebarViewProps> = React.memo(function SearchSidebarView({
    settingsCascade,
    platformContext,
    extensionCoreAPI,
    filters,
}) {
    const navigate = useNavigate()
    const location = useLocation()
    const [, setCollapsed] = useTemporarySetting('search.sidebar.collapsed', false)

    // TODO: Get rid of Zustand create store since this is no longer needed after sidebar decoupling
    const useSearchQueryState: SearchQueryStateStore = useMemo(
        () =>
            create<SearchQueryState>((set, get) => ({
                parametersSource: InitialParametersSource.DEFAULT,
                queryState: { query: '' },
                searchCaseSensitivity: false,
                searchPatternType: SearchPatternType.standard,
                searchQueryFromURL: '',
                searchMode: SearchMode.Precise,

                setQueryState: queryStateUpdate => {
                    const currentSearchQueryState = get()
                    const updatedQueryState =
                        typeof queryStateUpdate === 'function'
                            ? queryStateUpdate(currentSearchQueryState.queryState)
                            : queryStateUpdate

                    extensionCoreAPI
                        .emit({
                            type: 'sidebar_query_update',
                            proposedQueryState: {
                                queryState: updatedQueryState,
                                searchCaseSensitivity: currentSearchQueryState.searchCaseSensitivity,
                                searchPatternType: currentSearchQueryState.searchPatternType,
                                searchMode: currentSearchQueryState.searchMode,
                            },
                            currentQueryState: {
                                // Don't spread currentSearchQueryState as it contains un-clone-able functions.
                                queryState: currentSearchQueryState.queryState,
                                searchCaseSensitivity: currentSearchQueryState.searchCaseSensitivity,
                                searchPatternType: currentSearchQueryState.searchPatternType,
                                searchMode: currentSearchQueryState.searchMode,
                            },
                        })
                        .catch(error => {
                            // TODO surface to user
                            console.error('Error updating search query from Sourcegraph sidebar', error)
                        })

                    extensionCoreAPI.focusSearchPanel().catch(() => {
                        // noop.
                    })
                },
                submitSearch: (_submitSearchParameters, updates = []) => {
                    const previousSearchQueryState = get()
                    const updatedQuery = updateQuery(previousSearchQueryState.queryState.query, updates)
                    extensionCoreAPI
                        .streamSearch(updatedQuery, {
                            caseSensitive: previousSearchQueryState.searchCaseSensitivity,
                            patternType: previousSearchQueryState.searchPatternType,
                            version: LATEST_VERSION,
                            trace: undefined,
                        })
                        .catch(error => {
                            // TODO surface to user
                            console.error('Error submitting search from Sourcegraph sidebar', error)
                        })

                    extensionCoreAPI.focusSearchPanel().catch(() => {
                        // noop.
                    })
                },
            })),
        [extensionCoreAPI]
    )

    const searchQueryStateFromPanel = useObservable(
        useMemo(() => wrapRemoteObservable(extensionCoreAPI.observePanelQueryState()), [extensionCoreAPI])
    )

    useDeepCompareEffectNoCheck(() => {
        if (searchQueryStateFromPanel) {
            useSearchQueryState.setState({
                queryState: searchQueryStateFromPanel.queryState,
                searchCaseSensitivity: searchQueryStateFromPanel.searchCaseSensitivity,
                searchPatternType: searchQueryStateFromPanel.searchPatternType,
            })
        }
    }, [searchQueryStateFromPanel])

    const submitSearch = useSearchQueryState(state => state.submitSearch)
    const setQueryState = useSearchQueryState(state => state.setQueryState)

    const handleSidebarSearchSubmit = useCallback(
        (updates: QueryUpdate[]) =>
            submitSearch(
                {
                    historyOrNavigate: navigate,
                    location,
                    source: 'filter',
                },
                updates
            ),
        [navigate, location, submitSearch]
    )

    const onDynamicFilterClicked = useCallback(
        (value: string, kind?: string) => {
            platformContext.telemetryService.log('DynamicFilterClicked', { search_filter: { kind } })
            handleSidebarSearchSubmit([{ type: 'toggleSubquery', value }])
        },
        [handleSidebarSearchSubmit, platformContext.telemetryService]
    )

    const onSnippetClicked = useCallback(
        (value: string) => {
            platformContext.telemetryService.log('SearchSnippetClicked')
            handleSidebarSearchSubmit([{ type: 'toggleSubquery', value }])
        },
        [handleSidebarSearchSubmit, platformContext.telemetryService]
    )

    const repoFilters = useMemo(() => getFiltersOfKind(filters, FilterType.repo), [filters])

    return (
        <StickySearchSidebar onClose={() => setCollapsed(true)} className={styles.sidebarContainer}>
            <PersistSidebarStoreProvider>
                <SearchSidebarSection sectionId={SectionID.DYNAMIC_FILTERS} header="Dynamic Filters">
                    {getDynamicFilterLinks(
                        filters,
                        ['lang', 'file', 'utility'],
                        onDynamicFilterClicked,
                        (label, value) => `Filter by ${value}`,
                        (label, value) => value
                    )}
                </SearchSidebarSection>

                <SearchSidebarSection
                    sectionId={SectionID.REPOSITORIES}
                    header="Repositories"
                    searchOptions={{
                        ariaLabel: 'Find repositories',
                        noResultText: getRepoFilterNoResultText,
                    }}
                    minItems={1}
                >
                    {getRepoFilterLinks(repoFilters, onDynamicFilterClicked)}
                </SearchSidebarSection>

                <SearchSidebarSection
                    sectionId={SectionID.SEARCH_REFERENCE}
                    header="Search reference"
                    searchOptions={{
                        ariaLabel: 'Find filters',
                        // search reference should always preserve the filter
                        // (false is just an arbitrary but static value)
                        clearSearchOnChange: false,
                    }}
                >
                    {getSearchReferenceFactory({
                        telemetryService: platformContext.telemetryService,
                        setQueryState,
                    })}
                </SearchSidebarSection>

                <SearchSidebarSection sectionId={SectionID.SEARCH_SNIPPETS} header="Search snippets">
                    {getSearchSnippetLinks(settingsCascade, onSnippetClicked)}
                </SearchSidebarSection>

                <SearchSidebarSection sectionId={SectionID.QUICK_LINKS} header="Quicklinks">
                    {getQuickLinks(settingsCascade)}
                </SearchSidebarSection>
            </PersistSidebarStoreProvider>
        </StickySearchSidebar>
    )
})

const getRepoFilterNoResultText = (repoFilterLinks: ReactElement[]): ReactNode => (
    <span>
        None of the top {repoFilterLinks.length} repositories in your results match this filter. Try a{' '}
        <Code>repo:</Code> search in the main search bar instead.
    </span>
)
