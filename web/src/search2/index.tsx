/** The options that describe a search */
export interface SearchOptions {
    /** The query entered by the user */
    query: string

    /** The query provided by the active scope */
    scopeQuery: string
}

/**
 * Builds a URL query for given SearchOptions (without leading `?`)
 */
export function buildSearchURLQuery(options: SearchOptions): string {
    const searchParams = new URLSearchParams()
    searchParams.set('q', options.query)
    searchParams.set('sq', options.scopeQuery || '')
    return searchParams.toString().replace(/%2F/g, '/').replace(/%3A/g, ':')
}

/**
 * Parses the SearchOptions out of URL search params
 */
export function parseSearchURLQuery(query: string): SearchOptions {
    const searchParams = new URLSearchParams(query)
    return {
        query: searchParams.get('q') || '',
        scopeQuery: searchParams.get('sq') || '',
    }
}

/**
 * Returns whether the two sets of search options are equal.
 */
export function searchOptionsEqual(a: SearchOptions, b: SearchOptions): boolean {
    return a.query === b.query && a.scopeQuery === b.scopeQuery
}
