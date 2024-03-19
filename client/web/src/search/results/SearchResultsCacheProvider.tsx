import React, {
    createContext,
    createRef,
    type MutableRefObject,
    useContext,
    useEffect,
    useMemo,
    useRef,
    useState,
} from 'react'

import { isEqual } from 'lodash'
import { useNavigationType, useLocation } from 'react-router-dom'
import { merge, of } from 'rxjs'
import { last, share, tap, throttleTime } from 'rxjs/operators'

import { type URLQueryFilter, serializeURLQueryFilters, mergeQueryAndFilters } from '@sourcegraph/branded'
import type { AggregateStreamingSearchResults, StreamSearchOptions } from '@sourcegraph/shared/src/search/stream'
import type { TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useObservable } from '@sourcegraph/wildcard'

import type { SearchStreamingProps } from '..'

interface CachedResults {
    query: string
    options: StreamSearchOptions
    cache: {
        // Preserve collection of results by filter query sub-key,
        // means that when filter change we don't run additional
        // request if we had results for the set of filters before.
        [filterQuery: string]: AggregateStreamingSearchResults | undefined
    }
}

const SearchResultsCacheContext = createContext<MutableRefObject<CachedResults | null>>(createRef())

interface CachedSearchResultsInput {
    /** Search query */
    query: string

    /**
     * Filter query, different from the search query since new filters
     * don't modify the main search query
     */
    urlFilters: URLQueryFilter[]

    /**
     * Options to pass on to `streamSeach`.
     * MUST be wrapped in `useMemo` for this to work.
     */
    options: StreamSearchOptions

    /** Search function to call the backend with. */
    streamSearch: SearchStreamingProps['streamSearch']
    telemetryService: TelemetryService
}

/**
 * Returns the cached value if the options have not changed.
 * Otherwise, executes a new search and caches the value once
 * the search completes.
 *
 * @returns Search results, either from cache or from running a new search
 * (updated as new streaming results come in).
 */
export function useCachedSearchResults(props: CachedSearchResultsInput): AggregateStreamingSearchResults | undefined {
    const { query, urlFilters: selectedFilters, options, streamSearch, telemetryService } = props
    const cachedResults = useContext(SearchResultsCacheContext)

    const location = useLocation()
    const navigationType = useNavigationType()
    const [queryTimestamp, setQueryTimestamp] = useState<number | undefined>()

    const results = useObservable(
        useMemo(() => {
            const isCachedQuery = query === cachedResults.current?.query
            const isCachedOptions = isEqual(options, cachedResults.current?.options)
            const filterCacheKey = serializeURLQueryFilters(selectedFilters) ?? ''

            // If query and options have not changed, return cached value
            if (isCachedQuery && isCachedOptions && cachedResults.current?.cache[filterCacheKey]) {
                const cacheHit = cachedResults.current?.cache[filterCacheKey]
                return of(cacheHit)
            }
            const saveToCache = (results: AggregateStreamingSearchResults): void => {
                const previousCache = isCachedQuery && isCachedOptions ? cachedResults.current?.cache ?? {} : {}
                cachedResults.current = { query, options, cache: { ...previousCache, [filterCacheKey]: results } }
            }

            const extendQueryWithFilters = mergeQueryAndFilters(query, selectedFilters)
            const stream = streamSearch(of(extendQueryWithFilters), options).pipe(share())

            // If the throttleTime option `trailing` is set, we will return the
            // final value, but it also removes the guarantee that the output events
            // are a minimum of 200ms apart. If it's unset, we might throw away
            // some trailing events. This is a fundamental issue with throttleTime,
            // and is discussed extensively in github issues. Instead, we just manually
            // merge throttleTime with only leading values and the final value.
            // See: https://github.com/ReactiveX/rxjs/issues/5732
            return merge(stream.pipe(throttleTime(500)), stream.pipe(last(), tap(saveToCache)))

            // We also need to pass `queryTimestamp` to the dependency array, because
            // it's used in the `useEffect` below to reset the cache if a new search
            // is made with the same query. Otherwise, the new search will not be executed.
            // eslint-disable-next-line react-hooks/exhaustive-deps
        }, [query, selectedFilters, options, streamSearch, cachedResults, queryTimestamp])
    )

    // Reset cached results if a new search is made with the same query
    // (e.g. to force refresh when the search button is clicked).
    // The query timestamp is set when the search is submitted in `helpers.tsx` -> `submitSearch()`.
    // Since the location state is of `any` type, we need to disable the eslint warnings.
    /* eslint-disable @typescript-eslint/no-unsafe-member-access */
    useEffect(() => {
        if (cachedResults && location.state?.queryTimestamp !== queryTimestamp && navigationType === 'REPLACE') {
            cachedResults.current = null
            setQueryTimestamp(location.state?.queryTimestamp)
        }
    }, [location.state?.queryTimestamp, queryTimestamp, navigationType, cachedResults])
    /* eslint-enable @typescript-eslint/no-unsafe-member-access */

    // In case of back/forward navigation, log if the cache is being used.
    useEffect(() => {
        const cacheExists = query === cachedResults.current?.query && isEqual(options, cachedResults.current?.options)

        if (navigationType === 'POP') {
            telemetryService.log('SearchResultsCacheRetrieved', { cacheHit: cacheExists }, { cacheHit: cacheExists })
        }
        // Only log on first render
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    return results
}

export const SearchResultsCacheProvider: React.FunctionComponent<React.PropsWithChildren<{}>> = ({ children }) => {
    const cachedResultsState = useRef<CachedResults | null>(null)

    return (
        <SearchResultsCacheContext.Provider value={cachedResultsState}>{children}</SearchResultsCacheContext.Provider>
    )
}
