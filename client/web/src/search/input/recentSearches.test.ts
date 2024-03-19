import { test, describe, expect } from 'vitest'

import type { RecentSearch } from '@sourcegraph/shared/src/settings/temporary/recentSearches'

import { RecentSearchesManager } from './recentSearches'

export function buildMockRecentSearches(items: number): RecentSearch[] {
    return Array.from({ length: items }, (_item, index) => ({
        query: `test${index}`,
        resultCount: 555,
        limitHit: false,
        timestamp: '2021-01-01T00:00:00Z',
    }))
}

describe('addRecentSearch', () => {
    test('adding item while recent searches are not set queues it', () => {
        let recentSearches: readonly RecentSearch[] = []
        const manager = new RecentSearchesManager({ persist: searches => (recentSearches = searches) })
        manager.addRecentSearch({ query: 'test5', resultCount: 555, limitHit: false })

        expect(recentSearches).toEqual([])

        manager.setRecentSearches(buildMockRecentSearches(3))
        expect(recentSearches.map(search => search.query)).toMatchInlineSnapshot(`
          Array [
            "test5",
            "test0",
            "test1",
            "test2",
          ]
        `)
    })

    test('adding item to recent search puts it at the top', () => {
        let recentSearches: readonly RecentSearch[] = []
        const manager = new RecentSearchesManager({ persist: searches => (recentSearches = searches) })
        const searches = buildMockRecentSearches(3)
        manager.setRecentSearches(searches)
        manager.addRecentSearch({ query: 'test3', resultCount: 555, limitHit: false })
        expect(recentSearches.map(search => search.query)).toMatchInlineSnapshot(`
          Array [
            "test3",
            "test0",
            "test1",
            "test2",
          ]
        `)
    })

    test('adding an existing item to recent searches deduplicates it and puts it at the top', () => {
        let recentSearches: readonly RecentSearch[] = []
        const manager = new RecentSearchesManager({ persist: searches => (recentSearches = searches) })
        const searches = buildMockRecentSearches(3)
        manager.setRecentSearches(searches)
        manager.addRecentSearch({ query: 'test1', resultCount: 555, limitHit: false })
        expect(recentSearches.map(search => search.query)).toMatchInlineSnapshot(`
          Array [
            "test1",
            "test0",
            "test2",
          ]
        `)
    })

    test('adding an item beyond the limit of the list removes the last item', () => {
        let recentSearches: readonly RecentSearch[] = []
        const manager = new RecentSearchesManager({ persist: searches => (recentSearches = searches) })
        const searches = buildMockRecentSearches(20)
        manager.setRecentSearches(searches)
        manager.addRecentSearch({ query: 'test21', resultCount: 555, limitHit: false })
        expect(recentSearches.map(search => search.query)).toMatchInlineSnapshot(`
          Array [
            "test21",
            "test0",
            "test1",
            "test2",
            "test3",
            "test4",
            "test5",
            "test6",
            "test7",
            "test8",
            "test9",
            "test10",
            "test11",
            "test12",
            "test13",
            "test14",
            "test15",
            "test16",
            "test17",
            "test18",
          ]
        `)
    })
})
