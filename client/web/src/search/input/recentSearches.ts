import { getGlobalSearchContextFilter } from '@sourcegraph/shared/src/search/query/query'
import { omitFilter } from '@sourcegraph/shared/src/search/query/transformer'
import type { RecentSearch } from '@sourcegraph/shared/src/settings/temporary/recentSearches'

const MAX_RECENT_SEARCHES = 20

export interface RecentSearchesManagerOptions {
    /**
     * This function is called when the manager made changes to the recent searches list.
     * It's the caller's responsibility to persist list.
     */
    persist: (searches: RecentSearch[]) => void
}

/**
 * Helper class to manage recent searches. It makes sure that searches are deduplicated and
 * sorted by recency. It also handles queueing added searches before the list of recent searches is
 * available.
 */
export class RecentSearchesManager {
    private recentSearches: RecentSearch[] | null = null
    private pendingSearches: RecentSearch[] = []

    constructor(private options: RecentSearchesManagerOptions) {}

    /**
     * Sets the recent searches list. This should be called when the list is finished loading.
     * If there are any pending searches, they will be added to the list.
     * and {@link RecentSearchesManagerOptions.persist} will be called.
     */
    public setRecentSearches(searches: RecentSearch[]): void {
        this.recentSearches = this.processPendingSearches(searches)
        if (this.recentSearches !== searches) {
            this.options.persist(this.recentSearches)
        }
    }

    /**
     * Adds non-empty queries. A  query is considered empty if it's an empty string or
     * only contains a context: filter.
     * If the search is being added after the list is finished loading, add it immediately.
     * If the search is being added before the list is finished loading, queue it to be
     * added after loading is complete.
     */
    public addRecentSearch(search: Omit<RecentSearch, 'timestamp'>): void {
        const searchContext = getGlobalSearchContextFilter(search.query)
        if (!searchContext || omitFilter(search.query, searchContext.filter).trim() !== '') {
            const recentSearch: RecentSearch = { ...search, timestamp: new Date().toISOString() }
            const currentRecentSearches = this.recentSearches
            if (currentRecentSearches !== null) {
                this.recentSearches = this.addOrMoveRecentSearchToTop(currentRecentSearches, recentSearch)
                if (this.recentSearches !== currentRecentSearches) {
                    this.options.persist(this.recentSearches)
                }
            } else {
                this.pendingSearches = this.pendingSearches.concat(recentSearch)
            }
        }
    }

    /**
     * Adds a new search to the top of the recent searches list.
     * If the search is already in the recent searches list, it moves it to the top.
     * If the list is full, the oldest search is removed.
     */
    private addOrMoveRecentSearchToTop(recentSearches: RecentSearch[], recentSearch: RecentSearch): RecentSearch[] {
        const newRecentSearches = recentSearches.filter(search => search.query !== recentSearch.query) || []
        newRecentSearches.unshift(recentSearch)
        // Truncate array if it's too long
        if (newRecentSearches.length > MAX_RECENT_SEARCHES) {
            newRecentSearches.splice(MAX_RECENT_SEARCHES)
        }
        return newRecentSearches
    }

    private processPendingSearches(recentSearches: RecentSearch[]): RecentSearch[] {
        if (this.pendingSearches.length > 0) {
            for (const search of this.pendingSearches) {
                recentSearches = this.addOrMoveRecentSearchToTop(recentSearches, search)
            }
            this.pendingSearches = []
        }
        return recentSearches
    }
}
