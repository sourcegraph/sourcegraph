import * as assert from 'assert'

import * as sinon from 'sinon'

import * as scip from '../../scip'
import { QueryGraphQLFn } from '../util/graphql'

import { GenericLSIFResponse } from './api'
import { DefinitionResponse, DefinitionAndHoverResponse } from './definition-hover'
import { createGraphQLProviders as createProviders, searchStencil } from './providers'
import { ReferencesResponse, MAX_REFERENCE_PAGE_REQUESTS } from './references'
import { makeStencilFn } from './stencil'
import {
    gatherValues,
    makeEnvelope,
    resource0,
    resource1,
    resource2,
    resource3,
    range1,
    range2,
    range3,
    range4,
    range5,
    range6,
    document,
    position,
    stencil1,
} from './util.test'

describe('graphql providers', () => {
    describe('combined definition and hover provider', () => {
        it('should use result from window', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(
                () => makeEnvelope(null)
            )

            const getBulkLocalIntelligence = Promise.resolve(() =>
                Promise.resolve({
                    range: range1,
                    definitions: () => [
                        { uri: 'git://repo1?deadbeef1#a.ts', range: range1 },
                        { uri: 'git://repo2?deadbeef2#b.ts', range: range2 },
                        { uri: 'git://repo3?deadbeef3#c.ts', range: range3 },
                    ],
                    hover: {
                        markdown: { text: 'foo' },
                        range: range1,
                    },
                })
            )

            assert.deepEqual(
                await createProviders(
                    queryGraphQLFn,
                    makeStencilFn(stencil1),
                    getBulkLocalIntelligence
                ).definitionAndHover(document, position),
                {
                    definition: [
                        { uri: 'git://repo1?deadbeef1#a.ts', range: range1 },
                        { uri: 'git://repo2?deadbeef2#b.ts', range: range2 },
                        { uri: 'git://repo3?deadbeef3#c.ts', range: range3 },
                    ],
                    hover: {
                        contents: {
                            value: 'foo',
                            kind: 'markdown',
                        },
                        range: range1,
                    },
                }
            )
        })

        it('should correctly parse result', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(
                () =>
                    makeEnvelope({
                        definitions: {
                            nodes: [
                                { resource: resource1, range: range1 },
                                { resource: resource2, range: range2 },
                                { resource: resource3, range: range3 },
                            ],
                        },
                        hover: {
                            markdown: { text: 'foo' },
                            range: range1,
                        },
                    })
            )

            assert.deepEqual(
                await createProviders(queryGraphQLFn, makeStencilFn(stencil1)).definitionAndHover(document, position),
                {
                    definition: [
                        { uri: 'git://repo1?deadbeef1#a.ts', range: range1 },
                        { uri: 'git://repo2?deadbeef2#b.ts', range: range2 },
                        { uri: 'git://repo3?deadbeef3#c.ts', range: range3 },
                    ],
                    hover: {
                        contents: {
                            value: 'foo',
                            kind: 'markdown',
                        },
                        range: range1,
                    },
                }
            )
        })

        it('should deal with empty payload', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(
                () => makeEnvelope()
            )

            assert.deepEqual(
                await createProviders(queryGraphQLFn, makeStencilFn(stencil1)).definitionAndHover(document, position),
                null
            )
        })
    })

    describe('references provider', () => {
        it('should use result from window', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(() =>
                makeEnvelope({
                    references: {
                        nodes: [
                            { resource: resource1, range: range1 },
                            { resource: resource2, range: range2 },
                            { resource: resource3, range: range3 },
                        ],
                        pageInfo: {},
                    },
                })
            )

            const getBulkLocalIntelligence = Promise.resolve(() =>
                Promise.resolve({
                    range: range1,
                    references: () => [
                        { uri: 'git://repo1?deadbeef1#d.ts', range: range1 },
                        { uri: 'git://repo2?deadbeef2#e.ts', range: range2 },
                        { uri: 'git://repo3?deadbeef3#f.ts', range: range3 },
                    ],
                })
            )

            const values = await gatherValues(
                createProviders(queryGraphQLFn, makeStencilFn(stencil1), getBulkLocalIntelligence).references(
                    document,
                    position,
                    {
                        includeDeclaration: false,
                    }
                )
            )

            assert.deepStrictEqual(values, [
                [
                    { uri: 'git://repo1?deadbeef1#d.ts', range: range1 },
                    { uri: 'git://repo2?deadbeef2#e.ts', range: range2 },
                    { uri: 'git://repo3?deadbeef3#f.ts', range: range3 },
                ],
                [
                    { ...{ uri: 'git://repo1?deadbeef1#a.ts', range: range1 } },
                    { ...{ uri: 'git://repo2?deadbeef2#b.ts', range: range2 } },
                    { ...{ uri: 'git://repo3?deadbeef3#c.ts', range: range3 } },
                ],
            ])
        })

        it('should correctly parse result', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(() =>
                makeEnvelope({
                    references: {
                        nodes: [
                            { resource: resource1, range: range1 },
                            { resource: resource2, range: range2 },
                            { resource: resource3, range: range3 },
                        ],
                        pageInfo: {},
                    },
                })
            )

            assert.deepEqual(
                await gatherValues(
                    createProviders(queryGraphQLFn, makeStencilFn(stencil1)).references(document, position, {
                        includeDeclaration: false,
                    })
                ),
                [
                    [
                        { uri: 'git://repo1?deadbeef1#a.ts', range: range1 },
                        { uri: 'git://repo2?deadbeef2#b.ts', range: range2 },
                        { uri: 'git://repo3?deadbeef3#c.ts', range: range3 },
                    ],
                ]
            )
        })

        it('should deal with empty payload', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(() =>
                makeEnvelope()
            )

            assert.deepEqual(
                await gatherValues(
                    createProviders(queryGraphQLFn, makeStencilFn(stencil1)).references(document, position, {
                        includeDeclaration: false,
                    })
                ),
                []
            )
        })

        it('should paginate results', async () => {
            const stub = sinon.stub<
                Parameters<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>,
                ReturnType<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>
            >()
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(stub)

            stub.onCall(0).returns(
                makeEnvelope({
                    references: {
                        nodes: [{ resource: resource1, range: range1 }],
                        pageInfo: { endCursor: 'page2' },
                    },
                })
            )
            stub.onCall(1).returns(
                makeEnvelope({
                    references: {
                        nodes: [{ resource: resource2, range: range2 }],
                        pageInfo: { endCursor: 'page3' },
                    },
                })
            )
            stub.onCall(2).returns(
                makeEnvelope({
                    references: {
                        nodes: [{ resource: resource3, range: range3 }],
                        pageInfo: {},
                    },
                })
            )

            const location1 = { uri: 'git://repo1?deadbeef1#a.ts', range: range1 }
            const location2 = { uri: 'git://repo2?deadbeef2#b.ts', range: range2 }
            const location3 = { uri: 'git://repo3?deadbeef3#c.ts', range: range3 }

            assert.deepEqual(
                await gatherValues(
                    createProviders(queryGraphQLFn, makeStencilFn(stencil1)).references(document, position, {
                        includeDeclaration: false,
                    })
                ),
                [[location1], [location1, location2], [location1, location2, location3]]
            )

            assert.strictEqual(queryGraphQLFn.getCall(0).args[1]?.after, undefined)
            assert.strictEqual(queryGraphQLFn.getCall(1).args[1]?.after, 'page2')
            assert.strictEqual(queryGraphQLFn.getCall(2).args[1]?.after, 'page3')
        })

        it('should not page results indefinitely', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(() =>
                makeEnvelope({
                    references: {
                        nodes: [{ resource: resource1, range: range1 }],
                        pageInfo: { endCursor: 'infinity' },
                    },
                })
            )

            const location = { uri: 'git://repo1?deadbeef1#a.ts', range: range1 }

            const values = [[location]]
            for (let index = 1; index < MAX_REFERENCE_PAGE_REQUESTS; index++) {
                const lastCopy = [...values[values.length - 1]]
                lastCopy.push(location)
                values.push(lastCopy)
            }

            assert.deepEqual(
                await gatherValues(
                    createProviders(queryGraphQLFn, makeStencilFn(stencil1)).references(document, position, {
                        includeDeclaration: false,
                    })
                ),
                values
            )

            assert.strictEqual(queryGraphQLFn.callCount, MAX_REFERENCE_PAGE_REQUESTS)
        })
    })

    describe('document highlights provider', () => {
        it('should use result from window', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionResponse | null>>>(() =>
                makeEnvelope(null)
            )

            const getBulkLocalIntelligence = Promise.resolve(() =>
                Promise.resolve({
                    range: range1,
                    references: () => [
                        { uri: 'git://repo?rev#foo.ts', range: range1 },
                        { uri: 'git://repo?rev#bar.ts', range: range2 },
                        { uri: 'git://repo?rev#foo.ts', range: range3 },
                        { uri: 'git://repo?rev#baz.ts', range: range4 },
                        { uri: 'git://repo?rev#foo.ts', range: range5 },
                        { uri: 'git://repo?rev#baz.ts', range: range6 },
                    ],
                })
            )

            assert.deepEqual(
                await gatherValues(
                    createProviders(
                        queryGraphQLFn,
                        makeStencilFn(stencil1),
                        getBulkLocalIntelligence
                    ).documentHighlights(document, position)
                ),
                [[{ range: range1 }, { range: range3 }, { range: range5 }]]
            )
        })

        it('should correctly parse result', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(() =>
                makeEnvelope({
                    references: {
                        nodes: [
                            { resource: resource0, range: range1 },
                            { resource: resource1, range: range2 },
                            { resource: resource0, range: range3 },
                            { resource: resource2, range: range4 },
                            { resource: resource0, range: range5 },
                            { resource: resource3, range: range6 },
                        ],
                        pageInfo: {},
                    },
                })
            )
            assert.deepEqual(
                await gatherValues(
                    createProviders(queryGraphQLFn, makeStencilFn(stencil1)).documentHighlights(document, position)
                ),
                [[{ range: range1 }, { range: range3 }, { range: range5 }]]
            )
        })

        it('should deal with empty payload', async () => {
            const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<ReferencesResponse | null>>>(() =>
                makeEnvelope()
            )
            assert.deepEqual(
                await gatherValues(
                    createProviders(queryGraphQLFn, makeStencilFn(stencil1)).documentHighlights(document, position)
                ),
                [null]
            )
        })
    })

    describe('stencil', () => {
        it('should detect a hit', async () => {
            assert.deepEqual(await searchStencil(document.uri, position, makeStencilFn(stencil1)), 'hit')
        })

        it('should detect a miss', async () => {
            const bogusPosition = new scip.Position(1000, 0)
            assert.deepEqual(await searchStencil(document.uri, bogusPosition, makeStencilFn(stencil1)), 'miss')
        })
    })
})
