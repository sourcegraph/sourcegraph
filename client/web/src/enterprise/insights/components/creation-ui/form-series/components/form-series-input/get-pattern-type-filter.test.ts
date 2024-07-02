import { describe, expect, test } from 'vitest'

import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'

import { getQueryPatternTypeFilter } from './get-pattern-type-filter'

describe('getQueryPatternTypeFilter', () => {
    const defaultPatternType = SearchPatternType.keyword

    test('returns defaultPatternType when query does not contain patterntype filter', () => {
        const query = 'test query'
        const result = getQueryPatternTypeFilter(query, defaultPatternType)
        expect(result).toBe(defaultPatternType)
    })

    test('returns correct patternType when query contains patterntype filter', () => {
        const query = 'patterntype:regexp'
        const result = getQueryPatternTypeFilter(query, defaultPatternType)
        expect(result).toBe(SearchPatternType.regexp)
    })

    test('returns defaultPatternType when query contains unknown patterntype filter', () => {
        const query = 'patterntype:unknown'
        const result = getQueryPatternTypeFilter(query, defaultPatternType)
        expect(result).toBe(defaultPatternType)
    })
})
