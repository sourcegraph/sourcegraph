import * as assert from 'assert'

import * as sinon from 'sinon'

import * as scip from '../../scip'
import * as sourcegraph from '../api'
import { cStyleComment } from '../language-specs/comments'
import { LanguageSpec, Result } from '../language-specs/language-spec'
import { Providers, SourcegraphProviders } from '../providers'
import { API, SearchResult } from '../util/api'
import { observableFromAsyncIterator } from '../util/ix'

import { createProviders } from './providers'

const spec: LanguageSpec = {
    stylized: 'Lang',
    languageID: 'lang',
    fileExts: [],
    commentStyles: [cStyleComment],
    identCharPattern: /./,
    filterDefinitions: <T extends Result>(results: T[]) => results.filter(result => result.file !== '/f.ts'),
}

const textDocument1 = {
    uri: 'git://sourcegraph.test/repo?rev#foo.ts',
    languageId: 'typescript',
    text: undefined,
}

const textDocument2 = {
    uri: 'git://sourcegraph.test/repo%20with%20spaces?rev#/foo.ts',
    languageId: 'typescript',
    text: undefined,
}

const position = new scip.Position(3, 1)
const range1 = scip.Range.fromNumbers(2, 3, 4, 5)
const range2 = scip.Range.fromNumbers(3, 4, 5, 6)
const range3 = scip.Range.fromNumbers(4, 5, 6, 7)

const searchResult1 = {
    file: { path: '/a.ts', commit: { oid: 'rev1' } },
    repository: { name: 'repo1' },
    symbols: [
        {
            name: 'sym1',
            fileLocal: false,
            kind: 'class',
            location: { resource: { path: '/b.ts' }, range: range1 },
        },
    ],
    lineMatches: [],
}

const searchResult2 = {
    file: { path: '/c.ts', commit: { oid: 'rev2' } },
    repository: { name: 'repo2' },
    symbols: [
        {
            name: 'sym2',
            fileLocal: false,
            kind: 'class',
            location: { resource: { path: '/d.ts' }, range: range2 },
        },
    ],
    lineMatches: [],
}

const searchResult3 = {
    file: { path: '/e.ts', commit: { oid: 'rev3' } },
    repository: { name: 'repo3' },
    symbols: [
        {
            name: 'sym3',
            fileLocal: false,
            kind: 'class',
            location: { resource: { path: '/f.ts' }, range: range3 },
        },
    ],
    lineMatches: [],
}

const makeNoopPromise = <T>() =>
    new Promise<T>(() => {
        /* block forever */
    })

describe('search providers', () => {
    let tick = 0
    let clock: sinon.SinonFakeTimers | undefined

    /**
     * This creates mocks for the default process timers that will tick
     * 5s ahead every 100ms. This is because there is no good place to
     * call clock.tick explicitly in these tests, and it doesn't hurt
     * (our assertions) to fast forward all time while these tests are
     * running.
     */
    beforeEach(() => {
        tick++
        const currentTick = tick

        const schedule = () => {
            if (tick === currentTick) {
                if (clock) {
                    clock.tick(5000)
                }

                setTimeout(schedule, 100)
            }
        }

        setTimeout(schedule, 100)
        clock = sinon.useFakeTimers()
    })

    afterEach(() => {
        if (clock) {
            clock.restore()
        }

        tick++
    })

    const newAPIWithStubResolveRepo = ({
        isFork = false,
        isArchived = false,
        id = 1,
    }: {
        isFork?: boolean
        isArchived?: boolean
        id?: number
    } = {}): API => {
        const api = new API()

        const stubResolveRepo = sinon.stub(api, 'resolveRepo')
        stubResolveRepo.callsFake(repo => Promise.resolve({ name: repo, isFork, isArchived, id }))

        const stubHasLocalCodeIntelField = sinon.stub(api, 'hasLocalCodeIntelField')
        stubHasLocalCodeIntelField.callsFake(() => Promise.resolve(true))

        const stubFindSymbol = sinon.stub(api, 'findLocalSymbol')
        stubFindSymbol.callsFake(() => Promise.resolve(undefined))

        const stubFetchSymbolInfo = sinon.stub(api, 'fetchSymbolInfo')
        stubFetchSymbolInfo.callsFake(() => Promise.resolve(undefined))

        return api
    }

    describe('definition provider', () => {
        it('should correctly parse result', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.resolves([searchResult1])

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(await gatherValues(createProviders(spec, {}, api).definition(textDocument1, position)), [
                [{ uri: 'git://repo1?rev1#/b.ts', range: range1 }],
            ])

            assert.strictEqual(searchStub.callCount, 1)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])
        })

        it('should correctly format repositories with spaces', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.resolves([searchResult1])

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(
                await gatherValues(createProviders(spec, {}, api).definition({ ...textDocument2 }, position)),
                [[{ uri: 'git://repo1?rev1#/b.ts', range: range1 }]]
            )

            assert.strictEqual(searchStub.callCount, 1)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo\\ with\\ spaces$@rev',
                'type:symbol',
            ])
        })

        it('should fallback to remote definition', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake((searchQuery: string) =>
                Promise.resolve(searchQuery.includes('-repo') ? [searchResult1] : [])
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(await gatherValues(createProviders(spec, {}, api).definition(textDocument1, position)), [
                [{ uri: 'git://repo1?rev1#/b.ts', range: range1 }],
            ])

            assert.strictEqual(searchStub.callCount, 2)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:symbol',
            ])
        })

        it('should apply definition filter', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.resolves([searchResult1, searchResult2, searchResult3])

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(await gatherValues(createProviders(spec, {}, api).definition(textDocument1, position)), [
                [
                    { uri: 'git://repo1?rev1#/b.ts', range: range1 },
                    { uri: 'git://repo2?rev2#/d.ts', range: range2 },
                ],
            ])

            assert.strictEqual(searchStub.callCount, 1)
        })

        it('should fallback to index-only queries', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake(
                (searchQuery: string): Promise<SearchResult[]> =>
                    searchQuery.includes('index:only') ? Promise.resolve([searchResult1]) : makeNoopPromise()
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            const values = gatherValues(createProviders(spec, {}, api).definition(textDocument1, position))

            assert.deepEqual(await values, [[{ uri: 'git://repo1?rev1#b.ts', range: range1 }]])

            assert.strictEqual(searchStub.callCount, 2)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$',
                'type:symbol',
                'index:only',
            ])
        })

        it('should fallback to index-only remote definition definition', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake(
                (searchQuery: string): Promise<SearchResult[]> =>
                    searchQuery.includes('-repo')
                        ? searchQuery.includes('index:only')
                            ? Promise.resolve([searchResult1])
                            : makeNoopPromise()
                        : Promise.resolve([])
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(await gatherValues(createProviders(spec, {}, api).definition(textDocument1, position)), [
                [{ uri: 'git://repo1?rev1#/b.ts', range: range1 }],
            ])

            assert.strictEqual(searchStub.callCount, 3)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:symbol',
            ])
            assertQuery(searchStub.thirdCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:symbol',
                'index:only',
            ])
        })

        it('should search forks in same repo if repo is a fork', async () => {
            const api = newAPIWithStubResolveRepo({ isFork: true })
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake((searchQuery: string) =>
                Promise.resolve(searchQuery.includes('-repo') ? [searchResult1] : [])
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(await gatherValues(createProviders(spec, {}, api).definition(textDocument1, position)), [
                [{ uri: 'git://repo1?rev1#/b.ts', range: range1 }],
            ])

            assert.strictEqual(searchStub.callCount, 2)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'fork:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:symbol',
            ])
        })
    })

    describe('references provider', () => {
        it('should correctly parse result', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake((searchQuery: string) =>
                Promise.resolve(searchQuery.includes('-repo') ? [searchResult2] : [searchResult1])
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(
                await gatherValues(
                    createProviders(spec, {}, api).references(textDocument1, position, {
                        includeDeclaration: false,
                    })
                ),
                [
                    [
                        { uri: 'git://repo1?rev1#b.ts', range: range1 },
                        { uri: 'git://repo2?rev2#d.ts', range: range2 },
                    ],
                ]
            )

            assert.strictEqual(searchStub.callCount, 2)
            assertQuery(searchStub.firstCall.args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:file',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:file',
            ])
        })

        it('should correctly format repositories with spaces', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake((searchQuery: string) =>
                Promise.resolve(searchQuery.includes('-repo') ? [searchResult2] : [searchResult1])
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(
                await gatherValues(
                    createProviders(spec, {}, api).references({ ...textDocument2 }, position, {
                        includeDeclaration: false,
                    })
                ),
                [
                    [
                        { uri: 'git://repo1?rev1#/b.ts', range: range1 },
                        { uri: 'git://repo2?rev2#/d.ts', range: range2 },
                    ],
                ]
            )

            assert.strictEqual(searchStub.callCount, 2)
            assertQuery(searchStub.firstCall.args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo\\ with\\ spaces$@rev',
                'type:file',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo\\ with\\ spaces$',
                'type:file',
            ])
        })

        it('should fallback to index-only queries', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake(
                (searchQuery: string): Promise<SearchResult[]> =>
                    searchQuery.includes('index:only')
                        ? searchQuery.includes('-repo')
                            ? Promise.resolve([searchResult2])
                            : Promise.resolve([searchResult1])
                        : makeNoopPromise()
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(
                await gatherValues(
                    createProviders(spec, {}, api).references(textDocument1, position, {
                        includeDeclaration: false,
                    })
                ),
                [
                    [
                        { uri: 'git://repo1?rev1#b.ts', range: range1 },
                        { uri: 'git://repo2?rev2#d.ts', range: range2 },
                    ],
                ]
            )

            assert.strictEqual(searchStub.callCount, 4)
            assertQuery(searchStub.getCall(0).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:file',
            ])
            assertQuery(searchStub.getCall(1).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:file',
            ])
            assertQuery(searchStub.getCall(2).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$',
                'type:file',
                'index:only',
            ])
            assertQuery(searchStub.getCall(3).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:file',
                'index:only',
            ])
        })

        it('should search forks in same repo if repo is a fork', async () => {
            const api = newAPIWithStubResolveRepo({ isFork: true })
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake(
                (searchQuery: string): Promise<SearchResult[]> =>
                    searchQuery.includes('index:only')
                        ? searchQuery.includes('-repo')
                            ? Promise.resolve([searchResult2])
                            : Promise.resolve([searchResult1])
                        : makeNoopPromise()
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.resolves('\n\n\nfoobar\n')

            assert.deepEqual(
                await gatherValues(
                    createProviders(spec, {}, api).references(textDocument1, position, {
                        includeDeclaration: false,
                    })
                ),
                [
                    [
                        { uri: 'git://repo1?rev1#b.ts', range: range1 },
                        { uri: 'git://repo2?rev2#d.ts', range: range2 },
                    ],
                ]
            )

            assert.strictEqual(searchStub.callCount, 4)
            assertQuery(searchStub.getCall(0).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'fork:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:file',
            ])
            assertQuery(searchStub.getCall(1).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:file',
            ])
            assertQuery(searchStub.getCall(2).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'fork:yes',
                'index:only',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$',
                'type:file',
            ])
            assertQuery(searchStub.getCall(3).args[0], [
                '\\bfoobar\\b',
                'count:500',
                'case:yes',
                'patternType:regexp',
                '-repo:^sourcegraph.test/repo$',
                'type:file',
                'index:only',
            ])
        })
    })

    /** Create providers with the definition provider fed into itself. */
    const recurProviders = (api: API): Providers => {
        const recur: Partial<SourcegraphProviders> = {}
        const providers = createProviders(spec, recur, api)
        recur.definition = {
            provideDefinition: (textDocument_: sourcegraph.TextDocument, position_: sourcegraph.Position) =>
                observableFromAsyncIterator(() => providers.definition(textDocument_, position_)),
        }

        return providers
    }

    describe('hover provider', () => {
        it('should correctly parse result', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.resolves([searchResult1])
            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.onFirstCall().resolves('\n\n\nfoobar\n')
            getFileContentStub.onSecondCall().resolves('text\n// simple docstring\ndef')

            assert.deepEqual(await gatherValues(recurProviders(api).hover(textDocument1, position)), [
                {
                    contents: {
                        kind: 'markdown',
                        value: '```lang\ndef\n```\n\n---\n\nsimple docstring',
                    },
                },
            ])

            assert.strictEqual(searchStub.callCount, 1)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])

            assert.strictEqual(getFileContentStub.callCount, 2)
            assert.deepEqual(getFileContentStub.secondCall.args, ['repo1', 'rev1', '/b.ts'])
        })

        it('should fallback to index-only queries', async () => {
            const api = newAPIWithStubResolveRepo()
            const searchStub = sinon.stub(api, 'search')
            searchStub.callsFake((searchQuery: string) =>
                searchQuery.includes('index:only') ? Promise.resolve([searchResult1]) : makeNoopPromise()
            )

            const getFileContentStub = sinon.stub(api, 'getFileContent')
            getFileContentStub.onFirstCall().resolves('\n\n\nfoobar\n')
            getFileContentStub.onSecondCall().resolves('text\n// simple docstring\ndef')

            assert.deepEqual(await gatherValues(recurProviders(api).hover(textDocument1, position)), [
                {
                    contents: {
                        kind: 'markdown',
                        value: '```lang\ndef\n```\n\n---\n\nsimple docstring',
                    },
                },
            ])

            assert.strictEqual(searchStub.callCount, 2)
            assertQuery(searchStub.firstCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$@rev',
                'type:symbol',
            ])
            assertQuery(searchStub.secondCall.args[0], [
                '^foobar$',
                'count:50',
                'case:yes',
                'patternType:regexp',
                'repo:^sourcegraph.test/repo$',
                'type:symbol',
                'index:only',
            ])

            assert.strictEqual(getFileContentStub.callCount, 2)
            assert.deepEqual(getFileContentStub.secondCall.args, ['repo1', 'rev1', '/b.ts'])
        })
    })
})

function assertQuery(searchQuery: string, expectedTerms: string[]): void {
    // Split terms in a way that preserved escaped spaces
    const actualTerms = searchQuery.split(/(?<!\\) /).filter(part => !!part)
    actualTerms.sort()
    expectedTerms.sort()
    assert.deepEqual(actualTerms, expectedTerms)
}

async function gatherValues<T>(generator: AsyncGenerator<T>): Promise<T[]> {
    const values: T[] = []
    for await (const value of generator) {
        values.push(value)
    }
    return values
}
