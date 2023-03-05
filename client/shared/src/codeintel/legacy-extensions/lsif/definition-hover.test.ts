import * as assert from 'assert'

import * as sinon from 'sinon'

import { QueryGraphQLFn } from '../util/graphql'

import { GenericLSIFResponse } from './api'
import { definitionAndHoverForPosition, DefinitionAndHoverResponse } from './definition-hover'
import { makeEnvelope, resource1, resource2, resource3, range1, range2, range3, document, position } from './util.test'

describe('definitionAndHoverForPosition', () => {
    it('should correctly parse result', async () => {
        const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(() =>
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

        assert.deepEqual(await definitionAndHoverForPosition(document, position, queryGraphQLFn), {
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
        })
    })

    it('should deal with empty definition payload', async () => {
        const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(() =>
            makeEnvelope({
                hover: {
                    markdown: { text: 'foo' },
                    range: range1,
                },
            })
        )

        assert.deepEqual(await definitionAndHoverForPosition(document, position, queryGraphQLFn), {
            definition: null,
            hover: {
                contents: {
                    value: 'foo',
                    kind: 'markdown',
                },
                range: range1,
            },
        })
    })

    it('should deal with empty hover payload', async () => {
        const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(() =>
            makeEnvelope({
                definitions: {
                    nodes: [
                        { resource: resource1, range: range1 },
                        { resource: resource2, range: range2 },
                        { resource: resource3, range: range3 },
                    ],
                },
            })
        )

        assert.deepEqual(await definitionAndHoverForPosition(document, position, queryGraphQLFn), {
            definition: [
                { uri: 'git://repo1?deadbeef1#a.ts', range: range1 },
                { uri: 'git://repo2?deadbeef2#b.ts', range: range2 },
                { uri: 'git://repo3?deadbeef3#c.ts', range: range3 },
            ],
            hover: null,
        })
    })

    it('should deal with empty payload', async () => {
        const queryGraphQLFn = sinon.spy<QueryGraphQLFn<GenericLSIFResponse<DefinitionAndHoverResponse | null>>>(() =>
            makeEnvelope()
        )

        assert.deepEqual(await definitionAndHoverForPosition(document, position, queryGraphQLFn), null)
    })
})
