import { mdiCodeBraces, mdiFileOutline, mdiFunction, mdiPlusMinus, mdiSourceCommit, mdiSourceFork } from '@mdi/js'

import type { Filter } from '@sourcegraph/shared/src/search/stream'

import { parseExtendedSearchURL } from '..'
import { USE_CLIENT_CACHE_QUERY_PARAMETER } from '../constants'

export type SectionItem = Omit<Filter, 'count'> & {
    count?: Filter['count']
    selected: boolean
}

/**
 * URLQueryFilter is the subset of a filter that is stored in the URL query
 * when a dynamic filter is selected. This is the minimum amount of state
 * necessary to render the selected filter before the backend streams back
 * any filters.
 */
export type URLQueryFilter = Pick<Filter, 'kind' | 'label' | 'value'>

const DYNAMIC_FILTER_URL_QUERY_KEY = 'df'

function deserializeURLFilter(input: string): URLQueryFilter {
    const [kind, label, value] = JSON.parse(input) as [Filter['kind'], string, string]
    return { kind, label, value }
}

function serializeURLFilter(input: URLQueryFilter): string {
    return JSON.stringify([input.kind, input.label, input.value])
}

export function updateFilterInURL(url: URL, filter: URLQueryFilter, remove: boolean): URL {
    let selectedFilters = filtersFromParams(url.searchParams)
    if (remove) {
        selectedFilters = selectedFilters.filter(
            selectedFilter => selectedFilter.kind !== filter.kind || selectedFilter.value != filter.value
        )
    } else {
        if (filter.kind === 'type') {
            selectedFilters = selectedFilters.filter(selectedFilter => selectedFilter.kind !== 'type')
        }
        selectedFilters = selectedFilters.concat([filter])
    }

    const newURL = new URL(url.toString())
    newURL.searchParams.delete(DYNAMIC_FILTER_URL_QUERY_KEY)
    selectedFilters
        .map(serializeURLFilter)
        .forEach(selectedFilter => newURL.searchParams.append(DYNAMIC_FILTER_URL_QUERY_KEY, selectedFilter))
    newURL.searchParams.set(USE_CLIENT_CACHE_QUERY_PARAMETER, '')

    return newURL
}

export function moveFiltersToQuery(url: URL): URL {
    const extendedSearchURL = parseExtendedSearchURL(url)
    if (!extendedSearchURL.filteredQuery) {
        return url
    }
    const newURL = new URL(url)
    newURL.searchParams.delete(DYNAMIC_FILTER_URL_QUERY_KEY)
    newURL.searchParams.set('q', extendedSearchURL.filteredQuery)
    return newURL
}

export function resetFilters(url: URL): URL {
    const newURL = new URL(url)
    newURL.searchParams.delete(DYNAMIC_FILTER_URL_QUERY_KEY)
    return newURL
}

export function filtersFromParams(params: URLSearchParams): URLQueryFilter[] {
    return params.getAll(DYNAMIC_FILTER_URL_QUERY_KEY).map(deserializeURLFilter)
}

export const staticTypeFilters: URLQueryFilter[] = [
    { kind: 'type', label: 'Code', value: 'type:file' },
    { kind: 'type', label: 'Repositories', value: 'type:repo' },
    { kind: 'type', label: 'Paths', value: 'type:path' },
    { kind: 'type', label: 'Symbols', value: 'type:symbol' },
    { kind: 'type', label: 'Commits', value: 'type:commit' },
    { kind: 'type', label: 'Diffs', value: 'type:diff' },
]

export const typeFilterIcons: Record<string, string> = {
    Code: mdiCodeBraces,
    Repositories: mdiSourceFork,
    Paths: mdiFileOutline,
    Symbols: mdiFunction,
    Commits: mdiSourceCommit,
    Diffs: mdiPlusMinus,
}

export type FilterGroups = Record<Filter['kind'], SectionItem[]>

export function groupFilters(streamFilters: Filter[], selectedFilters: URLQueryFilter[]): FilterGroups {
    const groupedFilters: FilterGroups = {
        type: [],
        repo: [],
        lang: [],
        utility: [],
        author: [],
        file: [],
        'commit date': [],
        'symbol type': [],
    }
    for (const selectedFilter of selectedFilters) {
        const streamFilter = streamFilters.find(
            streamFilter => streamFilter.kind === selectedFilter.kind && streamFilter.value === selectedFilter.value
        )
        groupedFilters[selectedFilter.kind].push({
            value: selectedFilter.value,
            label: selectedFilter.label,
            kind: selectedFilter.kind,
            selected: true,
            // Use count and exhaustiveness from the stream filter if it exists
            count: streamFilter?.count,
            exhaustive: streamFilter?.exhaustive || false,
        })
    }
    for (const filter of streamFilters) {
        if (groupedFilters[filter.kind].some(existingFilter => existingFilter.value === filter.value)) {
            // Skip any filters that were already added by the seleced loop above
            continue
        }
        groupedFilters[filter.kind].push({
            ...filter,
            selected: false,
        })
    }
    return groupedFilters
}
