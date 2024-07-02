import type { Filter } from '$lib/shared'

import { parseExtendedSearchURL } from '..'
import { SearchCachePolicy, setCachePolicyInURL } from '../state'

/**
 * URLQueryFilter is the subset of a filter that is stored in the URL query
 * when a dynamic filter is selected. This is the minimum amount of state
 * necessary to render the selected filter before the backend streams back
 * any filters.
 */
export type URLQueryFilter = {
    kind: string
    label: string
    value: string
}

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
    setCachePolicyInURL(newURL, SearchCachePolicy.CacheFirst)

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
