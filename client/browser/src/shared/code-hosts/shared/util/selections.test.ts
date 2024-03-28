import { describe, expect, test } from 'vitest'

import { lprToSelectionsZeroIndexed } from './selections'

/**
 * Asserts deep object equality using node's assert.deepEqual, except it (1) ignores differences in the
 * prototype (because that causes 2 object literals to fail the test) and (2) treats undefined properties as
 * missing.
 */
function assertDeepStrictEqual(actual: any, expected: any): void {
    actual = JSON.parse(JSON.stringify(actual))
    expected = JSON.parse(JSON.stringify(expected))
    expect(actual).toEqual(expected)
}

describe('lprToSelectionsZeroIndexed', () => {
    test('converts an LPR with only a start line', () => {
        assertDeepStrictEqual(
            lprToSelectionsZeroIndexed({
                line: 5,
            }),
            [
                {
                    start: {
                        line: 4,
                        character: 0,
                    },
                    end: {
                        line: 4,
                        character: 0,
                    },
                    anchor: {
                        line: 4,
                        character: 0,
                    },
                    active: {
                        line: 4,
                        character: 0,
                    },
                    isReversed: false,
                },
            ]
        )
    })

    test('converts an LPR with a line and a character', () => {
        assertDeepStrictEqual(
            lprToSelectionsZeroIndexed({
                line: 5,
                character: 45,
            }),
            [
                {
                    start: {
                        line: 4,
                        character: 44,
                    },
                    end: {
                        line: 4,
                        character: 44,
                    },
                    anchor: {
                        line: 4,
                        character: 44,
                    },
                    active: {
                        line: 4,
                        character: 44,
                    },
                    isReversed: false,
                },
            ]
        )
    })

    test('converts an LPR with a start and end line', () => {
        assertDeepStrictEqual(
            lprToSelectionsZeroIndexed({
                line: 12,
                endLine: 15,
            }),
            [
                {
                    start: {
                        line: 11,
                        character: 0,
                    },
                    end: {
                        line: 14,
                        character: 0,
                    },
                    anchor: {
                        line: 11,
                        character: 0,
                    },
                    active: {
                        line: 14,
                        character: 0,
                    },
                    isReversed: false,
                },
            ]
        )
    })

    test('converts an LPR with a start and end line and characters', () => {
        assertDeepStrictEqual(
            lprToSelectionsZeroIndexed({
                line: 12,
                character: 30,
                endLine: 15,
                endCharacter: 60,
            }),
            [
                {
                    start: {
                        line: 11,
                        character: 29,
                    },
                    end: {
                        line: 14,
                        character: 59,
                    },
                    anchor: {
                        line: 11,
                        character: 29,
                    },
                    active: {
                        line: 14,
                        character: 59,
                    },
                    isReversed: false,
                },
            ]
        )
    })
})
