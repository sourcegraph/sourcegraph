import { Position } from '@sourcegraph/extension-api-types'
import { DocumentSelector, TextDocument } from '@sourcegraph/shared/src/codeintel/legacy-extensions/api'

import { DocumentSelector, TextDocument } from '../../../codeintel/legacy-extensions/api'

import { match, offsetToPosition, positionToOffset, score } from './textDocument'

const FIXTURE_TEXT_DOCUMENT: Pick<TextDocument, 'uri' | 'languageId'> = { uri: 'file:///f', languageId: 'l' }

describe('match', () => {
    test('reports true if any selectors match', () => {
        expect(match([{ language: 'x' }, '*'], FIXTURE_TEXT_DOCUMENT)).toBeTruthy()
    })

    test('reports false if no selectors match', () => {
        expect(match(['x'], FIXTURE_TEXT_DOCUMENT)).toBe(false)
        expect(match([], FIXTURE_TEXT_DOCUMENT)).toBe(false)
    })

    test('supports iterators for the document selectors', () => {
        let index = 0
        const iterator = {
            [Symbol.iterator](): IterableIterator<DocumentSelector> {
                return {
                    [Symbol.iterator]: iterator[Symbol.iterator],
                    next: () => ({ value: ['*'], done: index++ === 1 }),
                }
            },
        }
        expect(match(iterator as IterableIterator<DocumentSelector>, FIXTURE_TEXT_DOCUMENT)).toBeTruthy()
    })
})

describe('score', () => {
    test('matches', () => {
        expect(score(['l'], new URL('file:///f'), 'l')).toBe(10)
        expect(score(['*'], new URL('file:///f'), 'l')).toBe(5)
        expect(score(['x'], new URL('file:///f'), 'l')).toBe(0)
        expect(score([{ scheme: 'file' }], new URL('file:///f'), 'l')).toBe(10)
        expect(score([{ scheme: '*' }], new URL('file:///f'), 'l')).toBe(5)
        expect(score([{ scheme: 'x' }], new URL('file:///f'), 'l')).toBe(0)
        expect(score([{ pattern: '**/*.txt' }], new URL('file:///f.txt'), 'l')).toBe(10)
        expect(score([{ pattern: '*.txt' }], new URL('file:///f.txt'), 'l')).toBe(10)
        expect(
            score([{ baseUri: 'git://repo?revision', pattern: '*.txt' }], new URL('git://repo?revision#f.txt'), 'l')
        ).toBe(10)
        expect(score([{ baseUri: 'file:///a/b', pattern: '*.txt' }], new URL('file:///a/b/c.txt'), 'l')).toBe(5)
        expect(
            score([{ baseUri: 'git://repo?revision', pattern: '**/*.txt' }], new URL('git://repo?revision#f.txt'), 'l')
        ).toBe(10)
        expect(score([{ baseUri: 'git://repo', pattern: '**/*.txt' }], new URL('git://repo?revision#f.txt'), 'l')).toBe(
            10
        )
        expect(score([{ baseUri: 'git://repo?revision' }], new URL('git://repo?revision#f.txt'), 'l')).toBe(5)
        expect(
            score(
                [{ pattern: '*.go' }],
                new URL('git://127.0.0.1-3434/repos/.git?51c44e6be08627c613f787032a2759162bf6f7c2#web/api.go'),
                'l'
            )
        ).toBe(5)
        expect(score([{ pattern: 'f.txt' }], new URL('file:///f.txt'), 'l')).toBe(10)
        expect(score([{ pattern: 'x' }], new URL('file:///f.txt'), 'l')).toBe(0)
        expect(score([{ pattern: 'f.txt', language: 'x' }], new URL('file:///f.txt'), 'l')).toBe(0)
        expect(score([{ language: 'x' }], new URL('file:///f.txt'), 'l')).toBe(0)
        expect(score([{ language: 'l' }], new URL('file:///f.txt'), 'l')).toBe(10)
        expect(score([{ language: '*' }], new URL('file:///f.txt'), 'l')).toBe(5)
        expect(score([{}], new URL('file:///f.txt'), 'l')).toBe(5)
    })
})

interface OffsetPositionTestCase {
    text: string
    offset: number
    pos: Position
}

const OFFSET_POSITION_COMMON_TESTS: OffsetPositionTestCase[] = [
    { text: 'ab\nc\nd', offset: 0, pos: { line: 0, character: 0 } },
    { text: 'ab\nc\nd', offset: 1, pos: { line: 0, character: 1 } },
    { text: 'ab\nc\nd', offset: 2, pos: { line: 0, character: 2 } },
    { text: 'ab\nc\nd', offset: 3, pos: { line: 1, character: 0 } },
    { text: 'ab\nc\nd', offset: 4, pos: { line: 1, character: 1 } },
    { text: 'ab\nc\nd', offset: 5, pos: { line: 2, character: 0 } },
    { text: 'ab\nc\nd', offset: 6, pos: { line: 2, character: 1 } },
    { text: 'ab\nc\nd\n', offset: 7, pos: { line: 3, character: 0 } },
    { text: '\n', offset: 0, pos: { line: 0, character: 0 } },
    { text: '\n', offset: 1, pos: { line: 1, character: 0 } },
    { text: '', offset: 0, pos: { line: 0, character: 0 } },
]

/** Shared test suite among multiple implementations of offset-to-position conversion functions. */
export const OFFSET_TO_POSITION_TESTS: OffsetPositionTestCase[] = [
    ...OFFSET_POSITION_COMMON_TESTS,
    { text: '', offset: 100, pos: { line: 0, character: 0 } },
    { text: 'a', offset: 100, pos: { line: 0, character: 1 } },
    { text: 'a\nb', offset: 100, pos: { line: 1, character: 1 } },
]

/** Shared test suite among multiple implementations of position-to-offset conversion functions. */
export const POSITION_TO_OFFSET_TESTS: OffsetPositionTestCase[] = [
    ...OFFSET_POSITION_COMMON_TESTS,
    { text: '', pos: { line: 100, character: 0 }, offset: 0 },
    { text: 'a', pos: { line: 100, character: 0 }, offset: 1 },
    { text: 'a\nb', pos: { line: 100, character: 0 }, offset: 3 },
]

describe('offsetToPosition', () => {
    for (const [index, { text, ...testCase }] of OFFSET_TO_POSITION_TESTS.entries()) {
        if (!('offset' in testCase)) {
            continue
        }
        test(index.toString(), () => expect(offsetToPosition(text, testCase.offset)).toEqual(testCase.pos))
    }
})

describe('positionToOffset', () => {
    for (const [index, { text, ...testCase }] of POSITION_TO_OFFSET_TESTS.entries()) {
        test(index.toString(), () => expect(positionToOffset(text, testCase.pos)).toEqual(testCase.offset))
    }
})
