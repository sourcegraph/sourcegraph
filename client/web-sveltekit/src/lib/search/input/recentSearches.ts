import { onDestroy } from 'svelte'
import { type Readable, derived } from 'svelte/store'

import type { RecentSearch } from '@sourcegraph/shared/src/settings/temporary/recentSearches'
import { RecentSearchesManager } from '@sourcegraph/web/src/search/input/recentSearches'

import { temporarySetting } from '$lib/temporarySettings'

interface RecentSearchesStore extends Readable<RecentSearch[] | null> {
    addRecentSearch: (search: Omit<RecentSearch, 'timestamp'>) => void
}

/**
 * Creates a store for recent searches. Must be called during component initialization.
 */
export function createRecentSearchesStore(): RecentSearchesStore {
    const recentSearches = temporarySetting('search.input.recentSearches', [])
    const manager = new RecentSearchesManager({
        persist: recentSearches.setValue,
    })

    onDestroy(
        recentSearches.subscribe($recentSearches => {
            if ($recentSearches.loading === false && $recentSearches.data) {
                manager.setRecentSearches($recentSearches.data)
            }
        })
    )

    return {
        ...derived(recentSearches, $recentSearches => (!$recentSearches.loading && $recentSearches.data) || null),
        addRecentSearch: (search: Omit<RecentSearch, 'timestamp'>) => {
            manager.addRecentSearch(search)
        },
    }
}
