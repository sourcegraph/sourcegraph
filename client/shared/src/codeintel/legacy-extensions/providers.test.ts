import * as assert from 'assert'

import { Observable } from 'rxjs'
import * as sinon from 'sinon'

import * as scip from '../scip'

import * as sourcegraph from './api'
import * as indicators from './indicators'
import {
    clearReferenceResultCache,
    createDefinitionProvider,
    createDocumentHighlightProvider,
    createHoverProvider,
    createReferencesProvider,
} from './providers'
import { API } from './util/api'

const textDocument: sourcegraph.TextDocument = {
    uri: 'https://sourcegraph.test/repo@rev/-/raw/foo.ts',
    languageId: 'typescript',
    text: undefined,
}

const position = new scip.Position(10, 5)
const range1 = scip.Range.fromNumbers(1, 2, 3, 4)
const range2 = scip.Range.fromNumbers(5, 6, 7, 8)

const location1: sourcegraph.Location = { uri: 'http://test/1', range: range1 }
const location2: sourcegraph.Location = { uri: 'http://test/2', range: range1 }
const location3: sourcegraph.Location = { uri: 'http://test/3', range: range1 }
const location4: sourcegraph.Location = { uri: 'http://test/4', range: range1 }
const location5: sourcegraph.Location = { uri: 'http://test/2', range: range2 } // overlapping URI
const location6: sourcegraph.Location = { uri: 'http://test/3', range: range2 } // overlapping URI
const location7: sourcegraph.Location = { uri: 'http://test/4', range: range2 } // overlapping URI

const hover1: sourcegraph.Hover = { contents: { value: 'test1' } }
const hover2: sourcegraph.Hover = { contents: { value: 'test2' } }
const hover3: sourcegraph.Hover = { contents: { value: 'test3' } }

const trimGitHubPrefix = (url: string) =>
    Promise.resolve({
        id: 5,
        name: url.slice('github.com/'.length),
        isFork: false,
        isArchived: false,
    })

const makeStubAPI = (): API => {
    const api = new API()
    const stub = sinon.stub(api, 'resolveRepo')
    stub.callsFake(trimGitHubPrefix)
    return api
}

describe('createDefinitionProvider', () => {
    it('uses precise definitions as source of truth', async () => {
        const result = createDefinitionProvider(
            () => Promise.resolve({ definition: [location1, location2], hover: null }),
            () => asyncGeneratorFromValues([location3]),
            undefined,
            undefined,
            undefined,
            makeStubAPI()
        ).provideDefinition(textDocument, position) as Observable<sourcegraph.Definition>

        assert.deepStrictEqual(await gatherValues(result), [
            { ...location1, aggregableBadges: [indicators.preciseBadge] },
            { ...location2, aggregableBadges: [indicators.preciseBadge] },
        ])
    })

    it('falls back to search when precise results are not found', async () => {
        const result = createDefinitionProvider(
            () => Promise.resolve(null),
            () => asyncGeneratorFromValues([location1]),
            undefined,
            undefined,
            undefined,
            makeStubAPI()
        ).provideDefinition(textDocument, position) as Observable<sourcegraph.Definition>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...location1,
                badge: indicators.impreciseBadge,
                aggregableBadges: [indicators.searchBasedBadge],
            },
        ])
    })
})

describe('createReferencesProvider', () => {
    beforeEach(clearReferenceResultCache)

    it('uses precise definitions as source of truth', async () => {
        const result = createReferencesProvider(
            () =>
                asyncGeneratorFromValues([
                    [location1, location2],
                    [location1, location2, location3],
                ]),
            () => asyncGeneratorFromValues([]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideReferences(textDocument, position, {
            includeDeclaration: false,
        }) as Observable<sourcegraph.Badged<sourcegraph.Location>[]>

        assert.deepStrictEqual(await gatherValues(result), [
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
            ],
        ])
    })

    it('supplements precise results with search results', async () => {
        const result = createReferencesProvider(
            () =>
                asyncGeneratorFromValues([
                    [location1, location2],
                    [location1, location2, location3],
                ]),
            () => asyncGeneratorFromValues([[location4]]),
            undefined,
            undefined,
            makeStubAPI(),
            () => true
        ).provideReferences(textDocument, position, {
            includeDeclaration: false,
        }) as Observable<sourcegraph.Badged<sourcegraph.Location>[]>

        assert.deepStrictEqual(await gatherValues(result), [
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
                {
                    ...location4,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
            ],
        ])
    })

    it('supplements precise results with non-overlapping search results', async () => {
        const result = createReferencesProvider(
            () =>
                asyncGeneratorFromValues([
                    [location1, location2],
                    [location1, location2, location3],
                ]),
            () => asyncGeneratorFromValues([[location4], [location4, location5, location6, location7]]),
            undefined,
            undefined,
            makeStubAPI(),
            () => true
        ).provideReferences(textDocument, position, {
            includeDeclaration: false,
        }) as Observable<sourcegraph.Badged<sourcegraph.Location>[]>

        assert.deepStrictEqual(await gatherValues(result), [
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
                {
                    ...location4,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
                {
                    ...location4,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
                {
                    ...location7,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
            ],
        ])
    })

    it('supplements precise results with search results (disabled)', async () => {
        const result = createReferencesProvider(
            () =>
                asyncGeneratorFromValues([
                    [location1, location2],
                    [location1, location2, location3],
                ]),
            () => asyncGeneratorFromValues([[location4], [location4, location5, location6, location7]]),
            undefined,
            undefined,
            makeStubAPI(),
            () => false
        ).provideReferences(textDocument, position, {
            includeDeclaration: false,
        }) as Observable<sourcegraph.Badged<sourcegraph.Location>[]>

        assert.deepStrictEqual(await gatherValues(result), [
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
            ],
        ])
    })

    it('supplements precise results with search results (toggled)', async () => {
        const mixedResults = createReferencesProvider(
            () =>
                asyncGeneratorFromValues([
                    [location1, location2],
                    [location1, location2, location3],
                ]),
            () => asyncGeneratorFromValues([[location4], [location4, location5, location6, location7]]),
            undefined,
            undefined,
            makeStubAPI(),
            () => true
        ).provideReferences(textDocument, position, {
            includeDeclaration: false,
        }) as Observable<sourcegraph.Badged<sourcegraph.Location>[]>

        assert.deepStrictEqual(await gatherValues(mixedResults), [
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
                {
                    ...location4,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
            ],
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
                {
                    ...location4,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
                {
                    ...location7,
                    badge: indicators.impreciseBadge,
                    aggregableBadges: [indicators.searchBasedBadge],
                },
            ],
        ])

        const preciseResults = createReferencesProvider(
            () =>
                asyncGeneratorFromValues([
                    [location1, location2],
                    [location1, location2, location3],
                ]),
            () => asyncGeneratorFromValues([[location4], [location4, location5, location6, location7]]),
            undefined,
            undefined,
            makeStubAPI(),
            () => false
        ).provideReferences(textDocument, position, {
            includeDeclaration: false,
        }) as Observable<sourcegraph.Badged<sourcegraph.Location>[]>

        // Should immediately return all precise results from previous call
        assert.deepStrictEqual(await gatherValues(preciseResults), [
            [
                { ...location1, aggregableBadges: [indicators.preciseBadge] },
                { ...location2, aggregableBadges: [indicators.preciseBadge] },
                { ...location3, aggregableBadges: [indicators.preciseBadge] },
            ],
        ])
    })
})

describe('createHoverProvider', () => {
    it('uses precise definitions as source of truth', async () => {
        const searchDefinitionProvider = sinon.spy(() => asyncGeneratorFromValues([]))

        const result = createHoverProvider(
            () => Promise.resolve({ definition: [location1], hover: hover1 }),
            searchDefinitionProvider,
            () => asyncGeneratorFromValues([hover2]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideHover(textDocument, position) as Observable<sourcegraph.Badged<sourcegraph.Hover>>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...hover1,
                aggregableBadges: [indicators.preciseBadge],
            },
        ])

        // Search providers not called at all
        assert.strictEqual(searchDefinitionProvider.called, false)
    })

    it('tags partially precise results', async () => {
        const searchDefinitionProvider = sinon.spy(() => asyncGeneratorFromValues([[location1]]))

        const result = createHoverProvider(
            () => Promise.resolve({ definition: [], hover: hover1 }),
            searchDefinitionProvider,
            () => asyncGeneratorFromValues([hover2]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideHover(textDocument, position) as Observable<sourcegraph.Badged<sourcegraph.Hover>>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...hover1,
                aggregableBadges: [indicators.partialHoverNoDefinitionBadge],
            },
        ])

        // Search providers called to determine if there's search hover text
        assert.strictEqual(searchDefinitionProvider.called, true)
    })

    it('does not tag partially precise results without search definition', async () => {
        const result = createHoverProvider(
            () => Promise.resolve({ definition: [], hover: hover1 }),
            () => asyncGeneratorFromValues([]),
            () => asyncGeneratorFromValues([hover2]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideHover(textDocument, position) as Observable<sourcegraph.Badged<sourcegraph.Hover>>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...hover1,
                aggregableBadges: [indicators.preciseBadge],
            },
        ])
    })

    it('falls back to search when precise results are not found', async () => {
        const result = createHoverProvider(
            () => Promise.resolve(null),
            () => asyncGeneratorFromValues([]),
            () => asyncGeneratorFromValues([hover3]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideHover(textDocument, position) as Observable<sourcegraph.Badged<sourcegraph.Hover>>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...hover3,
                aggregableBadges: [indicators.searchBasedBadge],
            },
        ])
    })

    it('alerts search results correctly with experimental LSIF support', async () => {
        const result = createHoverProvider(
            () => Promise.resolve(null),
            () => asyncGeneratorFromValues([]),
            () => asyncGeneratorFromValues([hover1]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideHover(textDocument, position) as Observable<sourcegraph.Badged<sourcegraph.Hover>>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...hover1,
                aggregableBadges: [indicators.searchBasedBadge],
            },
        ])
    })

    it('alerts search results correctly with robust LSIF support', async () => {
        const result = createHoverProvider(
            () => Promise.resolve(null),
            () => asyncGeneratorFromValues([]),
            () => asyncGeneratorFromValues([hover1]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideHover(textDocument, position) as Observable<sourcegraph.Badged<sourcegraph.Hover>>

        assert.deepStrictEqual(await gatherValues(result), [
            {
                ...hover1,
                aggregableBadges: [indicators.searchBasedBadge],
            },
        ])
    })
})

describe('createDocumentHighlightProvider', () => {
    it('uses precise document highlights', async () => {
        const result = createDocumentHighlightProvider(
            () => asyncGeneratorFromValues([[{ range: range1 }]]),
            () => asyncGeneratorFromValues([[{ range: range2 }]]),
            undefined,
            undefined,
            makeStubAPI()
        ).provideDocumentHighlights(textDocument, position) as Observable<sourcegraph.DocumentHighlight[]>

        assert.deepStrictEqual(await gatherValues(result), [[{ range: range1 }]])
    })
})

async function* asyncGeneratorFromValues<P>(source: P[]): AsyncGenerator<P, void, undefined> {
    await Promise.resolve()

    for (const value of source) {
        yield value
    }
}

async function gatherValues<T>(observable: Observable<T>): Promise<T[]> {
    const values: T[] = []
    await new Promise<void>(complete => observable.subscribe({ next: value => values.push(value), complete }))
    return values
}
