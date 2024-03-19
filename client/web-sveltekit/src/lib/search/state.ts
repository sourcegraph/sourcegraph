import { writable, type Readable } from 'svelte/store'

import { SearchPatternType } from '$lib/graphql-operations'
import { buildSearchURLQuery, type Settings } from '$lib/shared'
import { defaultSearchModeFromSettings, defaultPatternTypeFromSettings } from '$lib/web'

import { USE_CLIENT_CACHE_QUERY_PARAMETER } from './constants'

// Defined in @sourcegraph/shared/src/search/searchQueryState.tsx
export enum SearchMode {
    Precise = 0,
    SmartSearch = 1 << 0,
}

type Update<T> = T | ((value: T) => T)

interface Options {
    readonly caseSensitive: boolean
    readonly regularExpression: boolean
    readonly patternType: SearchPatternType
    readonly searchMode: SearchMode
    readonly query: string
    readonly searchContext: string
}

type QuerySettings = Pick<
    Settings,
    'search.defaultCaseSensitive' | 'search.defaultPatternType' | 'search.defaultMode'
> | null
export type QueryOptions = Pick<Options, 'patternType' | 'caseSensitive' | 'searchMode' | 'searchContext'>

export class QueryState {
    private defaultCaseSensitive = false
    private defaultPatternType = SearchPatternType.keyword
    private defaultSearchMode = SearchMode.Precise
    private defaultQuery = ''
    private defaultSearchContext = 'global'

    private constructor(public readonly options: Partial<Options>, public settings: QuerySettings) {}

    public static init(options: Partial<Options>, settings: QuerySettings): QueryState {
        return new QueryState(options, settings)
    }

    public get caseSensitive(): boolean {
        return this.options.caseSensitive ?? this.settings?.['search.defaultCaseSensitive'] ?? this.defaultCaseSensitive
    }

    public get patternType(): SearchPatternType {
        return (
            this.options.patternType ??
            (this.settings ? defaultPatternTypeFromSettings({ final: this.settings, subjects: [] }) : null) ??
            this.defaultPatternType
        )
    }

    public get searchMode(): SearchMode {
        return (
            // {final: this.settings, subjects} is a workaround to make our
            // settings representation work with defaultSearchModeFromSettings
            this.options.searchMode ??
            (this.settings ? defaultSearchModeFromSettings({ final: this.settings, subjects: [] }) : null) ??
            this.defaultSearchMode
        )
    }

    public get query(): string {
        return this.options.query ?? this.defaultQuery
    }

    public get searchContext(): string {
        return this.options.searchContext ?? this.defaultSearchContext
    }

    public setQuery(newQuery: Update<string>): QueryState {
        const query = typeof newQuery === 'function' ? newQuery(this.query) : newQuery
        return new QueryState({ ...this.options, query }, this.settings)
    }

    public setCaseSensitive(caseSensitive: Update<boolean>): QueryState {
        return new QueryState(
            {
                ...this.options,
                caseSensitive: typeof caseSensitive === 'function' ? caseSensitive(this.caseSensitive) : caseSensitive,
            },
            this.settings
        )
    }

    public setPatternType(patternType: Update<SearchPatternType>): QueryState {
        return new QueryState(
            {
                ...this.options,
                patternType: typeof patternType === 'function' ? patternType(this.patternType) : patternType,
            },
            this.settings
        )
    }

    public setMode(mode: SearchMode): QueryState {
        return new QueryState({ ...this.options, searchMode: mode }, this.settings)
    }

    public setSettings(settings: QuerySettings): QueryState {
        return new QueryState(this.options, settings)
    }
}

export interface QueryStateStore extends Readable<QueryState> {
    setQuery(update: Update<string>): void
    setCaseSensitive(update: Update<boolean>): void
    setPatternType(update: Update<SearchPatternType>): void
    setSettings(settings: QuerySettings): void
    setMode(mode: SearchMode): void
    set(options: Partial<Options>): void
}

export function queryStateStore(initial: Partial<Options> = {}, settings: QuerySettings): QueryStateStore {
    const { subscribe, update } = writable<QueryState>(QueryState.init(initial, settings))
    return {
        subscribe,
        setQuery(newQuery) {
            update(state => state.setQuery(newQuery))
        },
        setCaseSensitive(caseSensitive) {
            update(state => state.setCaseSensitive(caseSensitive))
        },
        setPatternType(patternType) {
            update(state => state.setPatternType(patternType))
        },
        setSettings(settings) {
            update(state => state.setSettings(settings))
        },
        setMode(mode) {
            update(state => state.setMode(mode))
        },
        set(options: Partial<Options>) {
            update(state => QueryState.init({ ...state.options, ...options }, state.settings))
        },
    }
}

/**
 * getQueryURL builds a /search URL from the given query state.
 * If enforceCache is true the in-memory query cache will be used when available.
 *
 * @param queryState The query state to build the URL from.
 * @param enforceCache Whether to enforce the use of the in-memory query cache.
 */
export function getQueryURL(
    queryState: Pick<QueryState, 'searchMode' | 'query' | 'caseSensitive' | 'patternType' | 'searchContext'>,
    enforceCache = false
): URL {
    let url = new URL('/search', location.href)
    url.search = buildSearchURLQuery(
        queryState.query,
        queryState.patternType,
        queryState.caseSensitive,
        queryState.searchContext,
        queryState.searchMode
    )
    if (enforceCache) {
        url.searchParams.append(USE_CLIENT_CACHE_QUERY_PARAMETER, '')
    }
    return url
}
