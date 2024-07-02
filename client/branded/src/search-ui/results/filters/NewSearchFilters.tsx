import { type FC, type ReactNode, useEffect, useCallback, useMemo } from 'react'

import { shortcutDisplayName } from '@sourcegraph/shared/src/keyboardShortcuts'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import { FilterType, resolveFilter } from '@sourcegraph/shared/src/search/query/filters'
import { findFilters } from '@sourcegraph/shared/src/search/query/query'
import { scanSearchQuery, succeedScan } from '@sourcegraph/shared/src/search/query/scanner'
import type { Filter as QueryFilter } from '@sourcegraph/shared/src/search/query/token'
import { omitFilter } from '@sourcegraph/shared/src/search/query/transformer'
import { TELEMETRY_FILTER_TYPES, type Filter } from '@sourcegraph/shared/src/search/stream'
import { useSettings } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, H1, H3, Icon, Tooltip } from '@sourcegraph/wildcard'

import {
    authorFilter,
    commitDateFilter,
    languageFilter,
    repoFilter,
    SearchDynamicFilter,
    symbolFilter,
} from './components/dynamic-filter/SearchDynamicFilter'
import { SearchFilterSkeleton } from './components/filter-skeleton/SearchFilterSkeleton'
import { FilterTypeList } from './components/filter-type-list/FilterTypeList'
import { FiltersDocFooter } from './components/filters-doc-footer/FiltersDocFooter'
import { ArrowBendIcon } from './components/Icons'
import { mergeQueryAndFilters, type URLQueryFilter, useUrlFilters } from './hooks'
import { FilterKind, type SearchTypeFilter, SEARCH_TYPES_TO_FILTER_TYPES, DYNAMIC_FILTER_KINDS } from './types'

import styles from './NewSearchFilters.module.scss'

interface NewSearchFiltersProps extends TelemetryProps, TelemetryV2Props {
    query: string
    filters?: Filter[]
    withCountAllFilter: boolean
    isFilterLoadingComplete: boolean
    onQueryChange: (nextQuery: string, updatedSearchURLQuery?: string) => void
    children?: ReactNode
}

export const NewSearchFilters: FC<NewSearchFiltersProps> = ({
    query,
    filters,
    withCountAllFilter,
    isFilterLoadingComplete,
    onQueryChange,
    children,
    telemetryService,
    telemetryRecorder,
}) => {
    const [selectedFilters, setSelectedFilters, serializeFiltersURL] = useUrlFilters()

    const hasNoFilters = useMemo(() => {
        const dynamicFilters = filters?.filter(filter => DYNAMIC_FILTER_KINDS.includes(filter.kind as FilterKind)) ?? []
        const selectedDynamicFilters = selectedFilters.filter(filter =>
            DYNAMIC_FILTER_KINDS.includes(filter.kind as FilterKind)
        )

        return dynamicFilters.length === 0 && selectedDynamicFilters.length === 0
    }, [filters, selectedFilters])

    // Observe query and selectedFilters change and reset filter type in URL filters
    // if original search box query already has explicit type filter
    useEffect(() => {
        if (queryHasTypeFilter(query) && selectedFilters.some(filter => filter.kind === FilterKind.Type)) {
            setSelectedFilters(selectedFilters.filter(filter => filter.kind !== FilterKind.Type))
        }
    }, [selectedFilters, query, setSelectedFilters])

    const handleFilterTypeClick = useCallback(
        (filter: URLQueryFilter, remove: boolean): void => {
            telemetryService.log('SearchFiltersTypeClick', { filterType: filter.label }, { filterType: filter.label })
            telemetryRecorder.recordEvent('search.filters.type', 'click', {
                metadata: { filterKind: TELEMETRY_FILTER_TYPES[filter.kind] },
            })
            if (remove) {
                setSelectedFilters(
                    selectedFilters.filter(
                        selectedFilter => selectedFilter.kind !== 'type' || selectedFilter.label !== filter.label
                    )
                )
            } else {
                const relevantFilters = omitImpossibleFilters(selectedFilters, filter.label as SearchTypeFilter)
                setSelectedFilters([
                    ...relevantFilters.filter(relevantFilters => relevantFilters.kind !== 'type'),
                    filter,
                ])
            }
        },
        [selectedFilters, setSelectedFilters, telemetryService, telemetryRecorder]
    )

    const handleFilterChange = useCallback(
        (filterKind: FilterKind, filters: URLQueryFilter[]) => {
            setSelectedFilters(filters)
            telemetryService.log('SearchFiltersSelectFilter', { filterKind }, { filterKind })
            telemetryRecorder.recordEvent('search.filters', 'select', {
                metadata: { filterKind: TELEMETRY_FILTER_TYPES[filterKind] },
            })
        },
        [setSelectedFilters, telemetryService, telemetryRecorder]
    )

    const handleApplyButtonFilters = (): void => {
        onQueryChange(mergeQueryAndFilters(query, selectedFilters), serializeFiltersURL([]))
        telemetryService.log('SearchFiltersApplyFiltersClick')
        telemetryRecorder.recordEvent('search.filters', 'apply')
    }

    const onAddFilterToQuery = (filter: string): void => {
        onQueryChange(`${query} ${filter}`)
    }

    const settings = useSettings()
    const snippetFilters = settings?.['search.scopes']?.map(
        (scope): Filter => ({
            label: scope.name,
            value: scope.value,
            count: 0,
            exhaustive: true,
            kind: 'snippet' as any,
        })
    )

    return (
        <div className={styles.scrollWrapper}>
            <div className={styles.filterPanelHeader}>
                <H3 as={H1} className="px-2 py-1">
                    Filter results
                </H3>
                {selectedFilters.length !== 0 && (
                    <div className={styles.resetButton}>
                        <Shortcut held={['Alt']} ordered={['Backspace']} onMatch={() => setSelectedFilters([])} />
                        <Button variant="link" size="sm" onClick={() => setSelectedFilters([])} className="p-0 m-0">
                            Reset all
                            <kbd className={styles.keybind}>{shortcutDisplayName('Alt+Backspace')}</kbd>
                        </Button>
                    </div>
                )}
            </div>
            <div className={styles.filters}>
                {!queryHasTypeFilter(query) && (
                    <FilterTypeList
                        backendFilters={filters ?? []}
                        selectedFilters={selectedFilters}
                        onClick={handleFilterTypeClick}
                    />
                )}
                {hasNoFilters && !isFilterLoadingComplete && (
                    <>
                        <SearchFilterSkeleton />
                        <SearchFilterSkeleton />
                        <SearchFilterSkeleton />
                    </>
                )}

                <SearchDynamicFilter
                    title="By repository"
                    filterKind={FilterKind.Repository}
                    filters={filters}
                    selectedFilters={selectedFilters}
                    renderItem={repoFilter}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SearchDynamicFilter
                    title="By language"
                    filterKind={FilterKind.Language}
                    filters={filters}
                    selectedFilters={selectedFilters}
                    renderItem={languageFilter}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SearchDynamicFilter
                    title="By symbol kind"
                    filterKind={FilterKind.SymbolKind}
                    filters={filters}
                    selectedFilters={selectedFilters}
                    renderItem={symbolFilter}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SearchDynamicFilter
                    title="By author"
                    filterKind={FilterKind.Author}
                    filters={filters}
                    selectedFilters={selectedFilters}
                    renderItem={authorFilter}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SearchDynamicFilter
                    title="By commit date"
                    filterKind={FilterKind.CommitDate}
                    filters={filters}
                    selectedFilters={selectedFilters}
                    renderItem={commitDateFilter}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SearchDynamicFilter
                    title="By file"
                    filterKind={FilterKind.File}
                    filters={filters}
                    selectedFilters={selectedFilters}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SearchDynamicFilter
                    title="Snippets"
                    filterKind={FilterKind.Snippet}
                    filters={snippetFilters}
                    selectedFilters={selectedFilters}
                    renderItem={commitDateFilter}
                    onSelectedFilterChange={handleFilterChange}
                    onAddFilterToQuery={onAddFilterToQuery}
                />

                <SyntheticCountFilter
                    query={query}
                    isLimitHit={withCountAllFilter}
                    onQueryChange={onQueryChange}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                />
            </div>

            <FiltersDocFooter />

            <footer className={styles.actions}>
                {selectedFilters.length > 0 && (
                    <Tooltip
                        placement="right"
                        content="Moves all your applied filters from this panel into the query bar at the top and resets selected options from this panel."
                    >
                        <Button variant="secondary" outline={true} onClick={handleApplyButtonFilters}>
                            Move filters to the query
                            <Icon as={ArrowBendIcon} aria-hidden={true} className={styles.moveIcon} />
                        </Button>
                    </Tooltip>
                )}

                {children}
            </footer>
        </div>
    )
}

function queryHasTypeFilter(query: string): boolean {
    const tokens = scanSearchQuery(query)
    if (tokens.type !== 'success') {
        return false
    }
    const filters = tokens.term.filter((token): token is QueryFilter => token.type === 'filter')
    return filters.some(filter => filter.field.value === 'type')
}

const STATIC_COUNT_FILTER: Filter[] = [
    {
        // Since backend filters don't support count filter
        // this means that Filter type doesn't have this as possible kind value
        // It's okay to cast it since it's handled in one place below in SyntheticCountFilter
        kind: 'count' as any,
        label: 'Show all matches',
        count: 0,
        exhaustive: true,
        value: 'count:all',
    },
]

interface SyntheticCountFilterProps extends TelemetryProps, TelemetryV2Props {
    query: string
    isLimitHit: boolean
    onQueryChange: (query: string) => void
}

/**
 * Client-based count filter, usually filters are generated on the backend
 * based on a given query and results, count filter isn't supported by our
 * backend at the moment, so it's synthetically included on the client.
 *
 * It scans original search query and detects count filter if it catches
 * count:all filter. As user enables this filter in the filter panel it
 * changes the original query by adding count:all to the end.
 */
const SyntheticCountFilter: FC<SyntheticCountFilterProps> = props => {
    const { query, isLimitHit, onQueryChange, telemetryService, telemetryRecorder } = props

    const selectedCountFilter = useMemo<Filter[]>(() => {
        const tokens = scanSearchQuery(query)

        if (tokens.type === 'success') {
            const filters = tokens.term.filter((token): token is QueryFilter => token.type === 'filter')
            const countFilters = filters.filter(filter => resolveFilter(filter.field.value)?.type === 'count')

            if (countFilters.length === 0 || countFilters.length > 1) {
                return []
            }

            return STATIC_COUNT_FILTER
        }

        return []
    }, [query])

    const handleCountAllFilter = (filterKind: FilterKind, countFilters: URLQueryFilter[]): void => {
        telemetryService.log('SearchFiltersSelectFilter', { filterKind }, { filterKind })
        telemetryRecorder.recordEvent('search.filters', 'select', {
            metadata: { filterKind: TELEMETRY_FILTER_TYPES[filterKind] },
        })

        if (countFilters.length > 0) {
            onQueryChange(`${query} count:all`)
        } else {
            const filters = findFilters(succeedScan(query), FilterType.count)
            const nextQuery = filters.reduce((query, filter) => omitFilter(query, filter), query)

            onQueryChange(nextQuery)
        }
    }

    // Hide count all filter if search is already exhaustive
    if (selectedCountFilter.length === 0 && !isLimitHit) {
        return null
    }

    return (
        <SearchDynamicFilter
            filterKind={FilterKind.Count}
            filters={STATIC_COUNT_FILTER}
            selectedFilters={selectedCountFilter}
            renderItem={commitDateFilter}
            onSelectedFilterChange={handleCountAllFilter}
            onAddFilterToQuery={() => {}}
        />
    )
}

function omitImpossibleFilters(filters: URLQueryFilter[], searchType: SearchTypeFilter): URLQueryFilter[] {
    const searchTypePossibleFilters = SEARCH_TYPES_TO_FILTER_TYPES[searchType]
    return filters.filter(filter => searchTypePossibleFilters.includes(filter.kind))
}
