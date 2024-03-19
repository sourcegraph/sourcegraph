import {
    type FC,
    type ReactNode,
    type ReactElement,
    useCallback,
    useMemo,
    forwardRef,
    type HTMLAttributes,
    type ComponentType,
    type PropsWithChildren,
} from 'react'

import { mdiInformationOutline } from '@mdi/js'

import {
    StickySearchSidebar,
    SearchSidebarSection,
    getDynamicFilterLinks,
    getRepoFilterLinks,
    getSearchSnippetLinks,
    getSearchReferenceFactory,
    getSearchTypeLinks,
    getFiltersOfKind,
    useLastRepoName,
    PersistSidebarStoreProvider,
} from '@sourcegraph/branded'
import type { QueryStateUpdate, QueryUpdate } from '@sourcegraph/shared/src/search'
import { FilterType } from '@sourcegraph/shared/src/search/query/filters'
import type { Filter } from '@sourcegraph/shared/src/search/stream'
import { type SettingsCascadeProps, useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { SectionID } from '@sourcegraph/shared/src/settings/temporary/searchSidebar'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Code, Tooltip, Icon } from '@sourcegraph/wildcard'

import type { SearchPatternType } from '../../../graphql-operations'
import { buildSearchURLQueryFromQueryState } from '../../../stores'
import { AggregationUIMode, GroupResultsPing } from '../components/aggregation'

import { getRevisions } from './Revisions'
import { SearchAggregations } from './SearchAggregations'

interface GenericSidebarProps extends HTMLAttributes<HTMLElement> {
    onClose: () => void
}

export interface SearchFiltersSidebarProps extends TelemetryProps, SettingsCascadeProps, HTMLAttributes<HTMLElement> {
    as?: ComponentType<PropsWithChildren<GenericSidebarProps>>
    liveQuery: string
    submittedURLQuery: string
    patternType: SearchPatternType
    caseSensitive: boolean
    filters?: Filter[]
    showAggregationPanel?: boolean
    selectedSearchContextSpec?: string
    aggregationUIMode?: AggregationUIMode
    onNavbarQueryChange: (queryState: QueryStateUpdate) => void
    onSearchSubmit: (updates: QueryUpdate[], updatedSearchQuery?: string) => void
    setSidebarCollapsed: (collapsed: boolean) => void
}

export const SearchFiltersSidebar = forwardRef<HTMLElement, PropsWithChildren<SearchFiltersSidebarProps>>(props => {
    const {
        as: Component = StickySearchSidebar,
        liveQuery,
        submittedURLQuery,
        caseSensitive,
        patternType,
        filters,
        showAggregationPanel,
        selectedSearchContextSpec,
        aggregationUIMode,
        onNavbarQueryChange,
        onSearchSubmit,
        setSidebarCollapsed,
        telemetryService,
        settingsCascade,
        children,
        ...attributes
    } = props

    // Settings
    const { enableSearchAggregations, proactiveSearchAggregations } = useExperimentalFeatures(features => ({
        enableSearchAggregations: features.searchResultsAggregations ?? true,
        proactiveSearchAggregations: features.proactiveSearchResultsAggregations ?? true,
    }))

    // Derived state
    const repoFilters = useMemo(() => getFiltersOfKind(filters, FilterType.repo), [filters])
    const repoName = useLastRepoName(liveQuery, repoFilters)

    const onDynamicFilterClicked = useCallback(
        (value: string, kind?: string) => {
            telemetryService.log('DynamicFilterClicked', { search_filter: { kind } })
            onSearchSubmit([{ type: 'toggleSubquery', value }])
        },
        [telemetryService, onSearchSubmit]
    )

    const onSnippetClicked = useCallback(
        (value: string) => {
            telemetryService.log('SearchSnippetClicked')
            onSearchSubmit([{ type: 'toggleSubquery', value }])
        },
        [telemetryService, onSearchSubmit]
    )

    const handleAggregationBarLinkClick = useCallback(
        (query: string, updatedSearchQuery: string): void => {
            onSearchSubmit([{ type: 'replaceQuery', value: query }], updatedSearchQuery)
        },
        [onSearchSubmit]
    )

    const handleGroupedByToggle = useCallback(
        (open: boolean): void => {
            telemetryService.log(open ? GroupResultsPing.ExpandSidebarSection : GroupResultsPing.CollapseSidebarSection)
        },
        [telemetryService]
    )

    return (
        <Component {...attributes} onClose={() => setSidebarCollapsed(true)}>
            <PersistSidebarStoreProvider>
                {children}

                {showAggregationPanel &&
                    enableSearchAggregations &&
                    aggregationUIMode === AggregationUIMode.Sidebar && (
                        <SearchSidebarSection
                            sectionId={SectionID.GROUPED_BY}
                            header="Group results by"
                            postHeader={<CustomAggregationHeading telemetryService={props.telemetryService} />}
                            // SearchAggregations content contains component that makes a few API network requests
                            // in order to prevent these calls if this section is collapsed we turn off force render
                            // for collapse section component
                            forcedRender={false}
                            onToggle={handleGroupedByToggle}
                        >
                            <SearchAggregations
                                query={submittedURLQuery}
                                patternType={patternType}
                                proactive={proactiveSearchAggregations}
                                caseSensitive={caseSensitive}
                                telemetryService={telemetryService}
                                onQuerySubmit={handleAggregationBarLinkClick}
                            />
                        </SearchSidebarSection>
                    )}

                <SearchSidebarSection sectionId={SectionID.SEARCH_TYPES} header="Search Types">
                    {getSearchTypeLinks({
                        query: liveQuery,
                        onNavbarQueryChange,
                        selectedSearchContextSpec,
                        buildSearchURLQueryFromQueryState,
                        forceButton: false,
                    })}
                </SearchSidebarSection>

                <SearchSidebarSection sectionId={SectionID.LANGUAGES} header="Languages" minItems={2}>
                    {getDynamicFilterLinks(filters, ['lang'], onDynamicFilterClicked, label => `Search ${label} files`)}
                </SearchSidebarSection>

                <SearchSidebarSection
                    sectionId={SectionID.REPOSITORIES}
                    header="Repositories"
                    searchOptions={{ ariaLabel: 'Find repositories', noResultText: getRepoFilterNoResultText }}
                    minItems={2}
                >
                    {getRepoFilterLinks(repoFilters, onDynamicFilterClicked)}
                </SearchSidebarSection>

                <SearchSidebarSection sectionId={SectionID.FILE_TYPES} header="File types">
                    {getDynamicFilterLinks(filters, ['file'], onDynamicFilterClicked)}
                </SearchSidebarSection>
                <SearchSidebarSection sectionId={SectionID.OTHER} header="Other">
                    {getDynamicFilterLinks(filters, ['utility'], onDynamicFilterClicked)}
                </SearchSidebarSection>

                {repoName && (
                    <SearchSidebarSection
                        sectionId={SectionID.REVISIONS}
                        header="Revisions"
                        searchOptions={{
                            ariaLabel: 'Find revisions',
                            clearSearchOnChange: repoName,
                        }}
                    >
                        {getRevisions({ repoName, onFilterClick: onSearchSubmit })}
                    </SearchSidebarSection>
                )}

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
                    {getSearchReferenceFactory({ telemetryService, setQueryState: onNavbarQueryChange })}
                </SearchSidebarSection>

                <SearchSidebarSection sectionId={SectionID.SEARCH_SNIPPETS} header="Search snippets">
                    {getSearchSnippetLinks(settingsCascade, onSnippetClicked)}
                </SearchSidebarSection>
            </PersistSidebarStoreProvider>
        </Component>
    )
})

const getRepoFilterNoResultText = (repoFilterLinks: ReactElement[]): ReactNode => (
    <span>
        None of the top {repoFilterLinks.length} repositories in your results match this filter. Try a{' '}
        <Code>repo:</Code> search in the main search bar instead.
    </span>
)

const CustomAggregationHeading: FC<TelemetryProps> = ({ telemetryService }) => (
    <Tooltip content="Aggregation is based on results with no count limitation (count:all).">
        <Icon
            aria-label="(Aggregation is based on results with no count limitation (count:all).)"
            size="md"
            svgPath={mdiInformationOutline}
            onMouseEnter={() => telemetryService.log(GroupResultsPing.InfoIconHover)}
        />
    </Tooltip>
)
