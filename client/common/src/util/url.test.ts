import { describe, expect, it } from 'vitest'

import { encodeURIPathComponent, LineOrPositionOrRange, SourcegraphURL } from './url'

describe('SourcegraphURL', () => {
    describe('from', () => {
        describe('string input', () => {
            it.each`
                input                                                     | expected
                ${'https://sourcegraph.com/some/path?some=param#L1'}      | ${'https://sourcegraph.com/some/path?some=param#L1'}
                ${'https://sourcegraph.com:3443/some/path?some=param#L1'} | ${'https://sourcegraph.com:3443/some/path?some=param#L1'}
                ${'/some/path?some=param#L1'}                             | ${'/some/path?some=param#L1'}
                ${'?some=param#L1'}                                       | ${'?some=param#L1'}
                ${'#L1'}                                                  | ${'#L1'}
            `('$input => $expected', ({ input, expected }) => {
                expect(SourcegraphURL.from(input).toString()).toBe(expected)
            })
        })

        it('accepts a URL object', () => {
            expect(SourcegraphURL.from(new URL('https://sourcegraph.com/some/path?some=param#L1')).toString()).toBe(
                'https://sourcegraph.com/some/path?some=param#L1'
            )
        })

        it('accepts URLSearchParams', () => {
            expect(SourcegraphURL.from(new URLSearchParams('some=param')).search).toBe('?some=param')
        })

        describe('location object', () => {
            it.each`
                pathname        | search           | hash     | expected
                ${'/some/path'} | ${'?some=param'} | ${'#L1'} | ${'/some/path?some=param#L1'}
                ${'/some/path'} | ${'some=param'}  | ${'L1'}  | ${'/some/path?some=param#L1'}
                ${'/some/path'} | ${''}            | ${''}    | ${'/some/path'}
                ${''}           | ${'?some=param'} | ${'#L1'} | ${'?some=param#L1'}
                ${''}           | ${''}            | ${'#L1'} | ${'#L1'}
                ${'/some/path'} | ${''}            | ${'#L1'} | ${'/some/path#L1'}
            `(
                '{pathname: $pathname, search: $search, hash: $hash} => $expected',
                ({ pathname, search, hash, expected }) => {
                    expect(SourcegraphURL.from({ pathname, search, hash }).toString()).toBe(expected)
                }
            )
        })
    })

    describe('get lineRange', () => {
        it.each`
            input         | expected
            ${'L1'}       | ${{ line: 1 }}
            ${'L1:0'}     | ${{ line: 1 }}
            ${'L1:1'}     | ${{ line: 1, character: 1 }}
            ${'L1-2'}     | ${{ line: 1, endLine: 2 }}
            ${'L1:1-2:2'} | ${{ line: 1, character: 1, endLine: 2, endCharacter: 2 }}
            ${'L1:1-1:1'} | ${{ line: 1, character: 1 }}
        `('$input => $expected', ({ input, expected }) => {
            // Search parameter position
            expect(SourcegraphURL.from(`/some/path?${input}`).lineRange).toEqual(expected)
            // Hash position
            expect(SourcegraphURL.from(`/some/path#${input}`).lineRange).toEqual(expected)
        })

        it.each`
            input             | message
            ${'L-1'}          | ${'invalid line number'}
            ${'L0'}           | ${'invalid line number'}
            ${'L1:1-1'}       | ${'invalid position-line range'}
            ${'L1-2:1'}       | ${'invalid line-position range'}
            ${'L1:1-2:1-3:1'} | ${'multiple ranges'}
        `('$input ($message) => {}', ({ input }) => {
            // Search parameter position
            expect(SourcegraphURL.from(`/some/path?${input}`).lineRange).toEqual({})
            // Hash position
            expect(SourcegraphURL.from(`/some/path#${input}`).lineRange).toEqual({})
        })
    })

    describe('setLineRange', () => {
        it.each`
            input                     | lpr                                                         | expected
            ${'/path'}                | ${{ line: 24, character: 24 }}                              | ${'/path?L24:24'}
            ${'/path'}                | ${{ line: 12, endLine: 56 }}                                | ${'/path?L12-56'}
            ${'/path'}                | ${{ line: 12, character: 3, endLine: 56, endCharacter: 1 }} | ${'/path?L12:3-56:1'}
            ${'/path'}                | ${{ line: 12, character: 0, endLine: 56, endCharacter: 0 }} | ${'/path?L12-56'}
            ${'/path?test=test'}      | ${{ line: 24, character: 24 }}                              | ${'/path?L24:24&test=test'}
            ${'/path?L1:1'}           | ${{ line: 24, character: 24 }}                              | ${'/path?L24:24'}
            ${'/path?L1:1&test=test'} | ${{}}                                                       | ${'/path?test=test'}
            ${'?'}                    | ${{ line: 24, character: 24 }}                              | ${'?L24:24'}
            ${'?'}                    | ${{ line: 24, endLine: 56 }}                                | ${'?L24-56'}
            ${'?'}                    | ${{ line: 12, character: 3, endLine: 56, endCharacter: 1 }} | ${'?L12:3-56:1'}
            ${'?'}                    | ${{ line: 12, character: 0, endLine: 56, endCharacter: 0 }} | ${'?L12-56'}
            ${'?test=test'}           | ${{ line: 24, character: 24 }}                              | ${'?L24:24&test=test'}
            ${'?L1:1'}                | ${{ line: 24, character: 24 }}                              | ${'?L24:24'}
            ${'?L1:1&test=test'}      | ${{}}                                                       | ${'?test=test'}
            ${'?L1:1'}                | ${{}}                                                       | ${''}
            ${'?L1:1'}                | ${null}                                                     | ${''}
        `('$input => $expected', ({ input, lpr, expected }) => {
            expect(SourcegraphURL.from(input).setLineRange(lpr).toString()).toBe(expected)
        })
    })

    describe('get viewState', () => {
        it.each`
            input                               | expected
            ${'/path#tab=references'}           | ${'references'}
            ${'/path#test=test&tab=references'} | ${'references'}
            ${'/path?test=test#tab=references'} | ${'references'}
            ${'/path?L1:1#tab=references'}      | ${'references'}
        `('$input => $expected', ({ input, expected }) => {
            expect(SourcegraphURL.from(input).viewState).toBe(expected)
        })
    })

    describe('setViewState', () => {
        it.each`
            input                     | viewState        | expected
            ${'/path'}                | ${'references'}  | ${'/path#tab=references'}
            ${'/path#test=test'}      | ${'references'}  | ${'/path#test=test&tab=references'}
            ${'/path#tab=references'} | ${'definitions'} | ${'/path#tab=definitions'}
            ${'/path?test=test'}      | ${'references'}  | ${'/path?test=test#tab=references'}
            ${'/path?L1:1'}           | ${'references'}  | ${'/path?L1:1#tab=references'}
        `('$input => $expected', ({ input, viewState, expected }) => {
            expect(SourcegraphURL.from(input).setViewState(viewState).toString()).toBe(expected)
        })
    })
})

describe('encodeURIPathComponent', () => {
    it('encodes all special characters except slashes and the plus sign', () => {
        expect(encodeURIPathComponent('hello world+/+some_special_characters_:_#_?_%_@')).toBe(
            'hello%20world+/+some_special_characters_%3A_%23_%3F_%25_%40'
        )
    })
})

describe('parse legacy hash', () => {
    function parseHash(hash: string): LineOrPositionOrRange & { viewState?: string } {
        const url = SourcegraphURL.from({ pathname: '', hash })
        return { ...url.lineRange, viewState: url.viewState }
    }

    it('parses empty hash', () => {
        expect(parseHash('')).toEqual({})
    })

    it('parses unexpectedly formatted hash', () => {
        expect(parseHash('L-53')).toEqual({})
        expect(parseHash('L53:')).toEqual({})
        expect(parseHash('L1:2-')).toEqual({})
        expect(parseHash('L1:2-3')).toEqual({})
        expect(parseHash('L1:2-3:')).toEqual({})
        expect(parseHash('L1:-3:')).toEqual({})
        expect(parseHash('L1:-3:4')).toEqual({})
        expect(parseHash('L1-2:3')).toEqual({})
        expect(parseHash('L1-2:')).toEqual({})
        expect(parseHash('L1:-2')).toEqual({})
        expect(parseHash('L1:2--3:4')).toEqual({})
        expect(parseHash('L53:a')).toEqual({})
    })

    it('parses hash with leading octothorpe', () => {
        expect(parseHash('#L1')).toEqual({ line: 1 })
    })

    it('parses hash with line', () => {
        expect(parseHash('L1')).toEqual({ line: 1 })
    })

    it('parses hash with line and character', () => {
        expect(parseHash('L1:1')).toEqual({ line: 1, character: 1 })
    })

    it('parses hash with range', () => {
        expect(parseHash('L1-2')).toEqual({ line: 1, endLine: 2 })
        expect(parseHash('L1:2-3:4')).toEqual({ line: 1, character: 2, endLine: 3, endCharacter: 4 })
        expect(parseHash('L47-L55')).toEqual({ line: 47, endLine: 55 })
        expect(parseHash('L34:2-L38:3')).toEqual({ line: 34, character: 2, endLine: 38, endCharacter: 3 })
    })

    it('parses hash with references', () => {
        expect(parseHash('$references')).toEqual({ viewState: 'references' })
        expect(parseHash('L1:1$references')).toEqual({ line: 1, character: 1, viewState: 'references' })
    })
    it('parses modern hash with references', () => {
        expect(parseHash('tab=references')).toEqual({ viewState: 'references' })
        expect(parseHash('L1:1&tab=references')).toEqual({ line: 1, character: 1, viewState: 'references' })
    })
})
