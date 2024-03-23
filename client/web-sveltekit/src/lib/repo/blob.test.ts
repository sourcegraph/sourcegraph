import { describe, test, expect } from 'vitest'

import { updateSearchParamsWithLineInformation } from './blob'

describe('updateSearchParamsWithLineInformation', () => {
    test.each`
        range                        | expected
        ${{ line: 5 }}               | ${'?L5'}
        ${{ line: 5, character: 3 }} | ${'?L5'}
        ${{ line: 5, endLine: 7 }}   | ${'?L5-7'}
    `('$range -> $expected', ({ range, expected }) => {
        expect(updateSearchParamsWithLineInformation(new URLSearchParams(), range)).toBe(expected)
    })

    test('replace existing line information', () => {
        expect(updateSearchParamsWithLineInformation(new URLSearchParams('L1'), { line: 2 })).toBe('?L2')
    })

    test('preserve other parameters', () => {
        expect(updateSearchParamsWithLineInformation(new URLSearchParams('existing=param'), { line: 2 })).toBe(
            '?L2&existing=param'
        )
    })
})
