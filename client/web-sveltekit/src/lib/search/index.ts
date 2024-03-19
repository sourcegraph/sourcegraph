import { KeywordKind, scanSearchQuery } from '$lib/shared'
import { parseSearchURL, type ParsedSearchURL } from '$lib/web'

import { filtersFromParams, type URLQueryFilter } from './dynamicFilters'

export interface ExtendedParsedSearchURL extends ParsedSearchURL {
    filters: URLQueryFilter[]
    /**
     * Original query + filters.
     */
    filteredQuery: string | undefined
}

export function parseExtendedSearchURL(url: URL): ExtendedParsedSearchURL {
    const parsedQuery = parseSearchURL(url.search)
    const filters = filtersFromParams(url.searchParams)

    let filteredQuery = parsedQuery.query

    if (filteredQuery && filters.length > 0) {
        // We need to wrap the query in parenthesis if it contains AND or OR operators to avoid
        // precedence issues.
        const result = scanSearchQuery(filteredQuery)
        if (result.type === 'success') {
            // Only append filters if the main query is valid
            if (
                result.term.some(
                    token =>
                        token.type === 'keyword' && (token.kind === KeywordKind.And || token.kind === KeywordKind.Or)
                )
            ) {
                filteredQuery = `(${filteredQuery})`
            }
            filteredQuery += ' ' + filters.map(filter => filter.value).join(' ')
        }
    }

    return { ...parsedQuery, filters, filteredQuery }
}
