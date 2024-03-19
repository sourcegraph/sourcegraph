import { getContext, setContext } from 'svelte'

import type { QueryStateStore } from '$lib/search/state'
import type { ContentMatch, PathMatch, SearchMatch, SymbolMatch } from '$lib/shared'

interface SearchResultsContext {
    isExpanded(match: SearchMatch): boolean
    setExpanded(match: SearchMatch, expanded: boolean): void
    setPreview(result: PathMatch | ContentMatch | SymbolMatch | null): void
    queryState: QueryStateStore
}

const CONTEXT_KEY = {}

export function getSearchResultsContext(): SearchResultsContext {
    return getContext(CONTEXT_KEY)
}

export function setSearchResultsContext(context: SearchResultsContext): SearchResultsContext {
    return setContext(CONTEXT_KEY, context)
}
