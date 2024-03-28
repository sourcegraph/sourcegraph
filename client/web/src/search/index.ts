import { escapeRegExp, memoize } from 'lodash'
import type { Location } from 'react-router-dom'
import { from, type Observable, of, type Subject } from 'rxjs'
import { distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators'

import { memoizeObservable } from '@sourcegraph/common'
import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { SearchMode } from '@sourcegraph/shared/src/search'
import { discreteValueAliases, escapeSpaces } from '@sourcegraph/shared/src/search/query/filters'
import { stringHuman } from '@sourcegraph/shared/src/search/query/printer'
import { FilterKind, findFilter, getGlobalSearchContextFilter } from '@sourcegraph/shared/src/search/query/query'
import { scanSearchQuery } from '@sourcegraph/shared/src/search/query/scanner'
import { createLiteral } from '@sourcegraph/shared/src/search/query/token'
import { omitFilter } from '@sourcegraph/shared/src/search/query/transformer'
import type { AggregateStreamingSearchResults, StreamSearchOptions } from '@sourcegraph/shared/src/search/stream'

export type { SearchAggregationProps } from './results/sidebar/search-aggregation-types'

/**
 * Parses the query out of the URL search params (the 'q' parameter). In non-interactive mode, if the 'q' parameter is not present, it
 * returns undefined. When parsing for interactive mode, each filter's individual query parameter
 * will be parsed and detected.
 *
 * @param query the URL query parameters
 *
 */
export function parseSearchURLQuery(query: string): string | undefined {
    const searchParameters = new URLSearchParams(query)
    return searchParameters.get('q') || undefined
}

/**
 * Parses the pattern type out of the URL search params (the 'patternType' parameter). If the 'pattern' parameter
 * is not present, or it is an invalid value, it returns undefined.
 */
export function parseSearchURLPatternType(query: string): SearchPatternType | undefined {
    const searchParameters = new URLSearchParams(query)
    const patternType = searchParameters.get('patternType')
    switch (patternType) {
        case SearchPatternType.literal:
        case SearchPatternType.standard:
        case SearchPatternType.regexp:
        case SearchPatternType.structural:
        case SearchPatternType.lucky:
        case SearchPatternType.codycontext:
        case SearchPatternType.keyword: {
            return patternType
        }
    }
    return undefined
}

export function parseSearchURLSearchMode(query: string): SearchMode {
    const defaultSearchMode = SearchMode.Precise
    const searchParameters = new URLSearchParams(query)
    const searchModeStr = searchParameters.get('sm')
    if (!searchModeStr) {
        return defaultSearchMode
    }

    const searchMode = parseInt(searchModeStr, 10)
    switch (searchMode) {
        case SearchMode.Precise:
        case SearchMode.SmartSearch: {
            return searchMode
        }
    }
    return defaultSearchMode
}

function searchURLIsCaseSensitive(query: string): boolean {
    const globalCase = findFilter(parseSearchURLQuery(query) || '', 'case', FilterKind.Global)
    if (globalCase?.value && globalCase.value.type === 'literal') {
        // if `case:` filter exists in the query, override the existing case: query param
        return discreteValueAliases.yes.includes(globalCase.value.value)
    }
    const searchParameters = new URLSearchParams(query)
    const caseSensitive = searchParameters.get('case')
    return discreteValueAliases.yes.includes(caseSensitive || '')
}

export interface ParsedSearchURL {
    query: string | undefined
    patternType: SearchPatternType | undefined
    caseSensitive: boolean
    searchMode: SearchMode
}

/**
 * parseSearchURL takes a URL's search querystring and returns
 * an object containing:
 * - the canonical, user-visible query (with `patternType` and `case` filters excluded),
 * - the effective pattern type, and
 * - the effective case sensitivity of the query.
 * - the search mode that defines general search behavior
 *
 * @param urlSearchQuery a URL's query string.
 */
export function parseSearchURL(
    urlSearchQuery: string,
    { appendCaseFilter = false }: { appendCaseFilter?: boolean } = {}
): ParsedSearchURL {
    let queryInput = parseSearchURLQuery(urlSearchQuery) || ''
    let patternTypeInput = parseSearchURLPatternType(urlSearchQuery)
    let caseSensitive = searchURLIsCaseSensitive(urlSearchQuery)
    const searchMode = parseSearchURLSearchMode(urlSearchQuery)

    const globalPatternType = findFilter(queryInput, 'patterntype', FilterKind.Global)
    if (globalPatternType?.value && globalPatternType.value.type === 'literal') {
        // Any `patterntype:` filter in the query should override the patternType= URL query parameter if it exists.
        queryInput = omitFilter(queryInput, globalPatternType)
        patternTypeInput = globalPatternType.value.value as SearchPatternType
    }

    let query = queryInput
    const { queryInput: newQuery, patternTypeInput: patternType } = literalSearchCompatibility({
        queryInput,
        patternTypeInput,
    })
    query = newQuery

    const globalCase = findFilter(query, 'case', FilterKind.Global)
    if (globalCase?.value && globalCase.value.type === 'literal') {
        // Any `case:` filter in the query should override the case= URL query parameter if it exists.
        query = omitFilter(query, globalCase)

        if (discreteValueAliases.yes.includes(globalCase.value.value)) {
            caseSensitive = true
        } else if (discreteValueAliases.no.includes(globalCase.value.value)) {
            caseSensitive = false
        }
    }

    if (appendCaseFilter) {
        // Invariant: If case:value was in the query, it is erased at this point. Add case:yes if needed.
        query = caseSensitive ? `${query} case:yes` : query
    }

    // For pattern types other than "keyword" or "regexp", we don't have UI elements to indicate which patternType
    // is active, hence we keep/add patternType: in those cases.
    if (patternType && !(patternType === SearchPatternType.keyword || patternType === SearchPatternType.regexp)) {
        // If structural search is enabled, then we have toggle for it in UI, and we don't have to append the // //
        // patternType: filter
        query =
            patternType === SearchPatternType.structural &&
            window.context?.experimentalFeatures?.structuralSearch === 'enabled'
                ? query
                : `${query} patternType:${patternType}`
    }

    return {
        query,
        patternType,
        caseSensitive,
        searchMode,
    }
}

export function repoFilterForRepoRevision(repoName: string, revision?: string): string {
    return `${escapeSpaces(`^${escapeRegExp(repoName)}$${revision ? `@${abbreviateOID(revision)}` : ''}`)}`
}

export function searchQueryForRepoRevision(repoName: string, revision?: string): string {
    return `repo:${repoFilterForRepoRevision(repoName, revision)} `
}

function abbreviateOID(oid: string): string {
    if (oid.length === 40) {
        return oid.slice(0, 7)
    }
    return oid
}

export function quoteIfNeeded(string: string): string {
    if (/[ "']/.test(string)) {
        return JSON.stringify(string)
    }
    return string
}

interface QueryCompatibility {
    queryInput: string
    patternTypeInput?: SearchPatternType
}

export function literalSearchCompatibility({ queryInput, patternTypeInput }: QueryCompatibility): QueryCompatibility {
    if (patternTypeInput === undefined || patternTypeInput !== SearchPatternType.literal) {
        return { queryInput, patternTypeInput }
    }
    const tokens = scanSearchQuery(queryInput, false, SearchPatternType.standard)
    if (tokens.type === 'error') {
        return { queryInput, patternTypeInput }
    }

    if (!tokens.term.find(token => token.type === 'pattern' && token.delimited)) {
        // If no /.../ pattern exists in this literal search, just return the query as-is.
        return { queryInput, patternTypeInput: SearchPatternType.standard }
    }

    const newQueryInput = stringHuman(
        tokens.term.map(token =>
            token.type === 'pattern' && token.delimited
                ? {
                      type: 'filter',
                      range: { start: 0, end: 0 },
                      field: createLiteral('content', { start: 0, end: 0 }, false),
                      value: createLiteral(`/${token.value}/`, { start: 0, end: 0 }, true),
                      negated: false /** if `NOT` was used on this pattern, it's already preserved */,
                  }
                : token
        )
    )

    return {
        queryInput: newQueryInput,
        patternTypeInput: SearchPatternType.standard,
    }
}

export interface SearchStreamingProps {
    streamSearch: (
        queryObservable: Observable<string>,
        options: StreamSearchOptions
    ) => Observable<AggregateStreamingSearchResults>
}

interface ParsedSearchURLAndContext extends ParsedSearchURL {
    /**
     * Search context extracted from query.
     */
    searchContextSpec: ReturnType<typeof getGlobalSearchContextFilter> | undefined
}

/**
 * getQueryStateFromLocation listens to history changes (via the location
 * observable) and extracts search query information while also handling and
 * validating search context information.
 * It returns an observable that emits the parsed query information, the
 * extracted search context (if any) and a processed version of the query
 * (without context filter).
 */
export function getQueryStateFromLocation({
    location,
    isSearchContextAvailable,
}: {
    location: Subject<Location>
    /**
     * Resolves to true if the provided search context exists for the user.
     */
    isSearchContextAvailable: (searchContextSpec: string) => Promise<boolean>
}): Observable<ParsedSearchURLAndContext> {
    // Memoized function to extract the `context:...` filter from a given
    // search query (avoids reparsing the query)
    const memoizedGetGlobalSearchContextSpec = memoize((query: string) => getGlobalSearchContextFilter(query))

    const memoizedIsSearchContextAvailable = memoizeObservable(
        (spec: string) =>
            spec
                ? // While we wait for the result of the `isSearchContextSpecAvailable` call, we assume the context is available
                  // to prevent flashing and moving content in the query bar. This optimizes for the most common use case where
                  // user selects a search context from the dropdown.
                  // See https://github.com/sourcegraph/sourcegraph/issues/19918 for more info.
                  from(isSearchContextAvailable(spec)).pipe(startWith(true), distinctUntilChanged())
                : of(false),
        spec => spec ?? ''
    )

    // This subscription handles updating the global query state store from
    // the URL.
    // Extract information from URL and validate search context if
    // available.
    return location.pipe(
        switchMap((location): Observable<{ parsedSearchURL: ParsedSearchURL; isSearchContextAvailable: boolean }> => {
            const parsedSearchURL = parseSearchURL(location.search)
            if (parsedSearchURL.query !== undefined) {
                return memoizedIsSearchContextAvailable(
                    memoizedGetGlobalSearchContextSpec(parsedSearchURL.query)?.spec ?? ''
                ).pipe(map(isSearchContextAvailable => ({ parsedSearchURL, isSearchContextAvailable })))
            }
            return of({ parsedSearchURL, isSearchContextAvailable: false })
        }),
        map(locationAndContextInformation => {
            const { parsedSearchURL, isSearchContextAvailable } = locationAndContextInformation
            const query = parsedSearchURL.query ?? ''
            const globalSearchContextSpec = isSearchContextAvailable
                ? memoizedGetGlobalSearchContextSpec(query)
                : undefined
            return { ...parsedSearchURL, searchContextSpec: globalSearchContextSpec }
        })
    )
}
