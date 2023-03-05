import { LineOrPositionOrRange } from '@sourcegraph/common'

import { testCodeHostMountGetters, testToolbarMountGetter } from '../shared/codeHostTestUtils'

import { bitbucketServerCodeHost, getToolbarMount, parseHash } from './codeHost'

describe('bitbucketServerCodeHost', () => {
    testCodeHostMountGetters(bitbucketServerCodeHost, {
        getViewContextOnSourcegraphMount: `${__dirname}/__fixtures__/browse.html`,
    })
    describe('getToolbarMount()', () => {
        testToolbarMountGetter(`${__dirname}/__fixtures__/code-views/pull-request/split/modified.html`, getToolbarMount)
    })
})

describe('parseHash', () => {
    const entries: [string, LineOrPositionOrRange][] = [
        ['#1', { line: 1 }],
        ['#1-5', { line: 1, endLine: 5 }],
        ['#1:5', {}],
        ['#1a', {}],
    ]

    for (const [hash, expectedValue] of entries) {
        test(`given "${hash}" as argument returns ${JSON.stringify(expectedValue)}`, () => {
            expect(parseHash(hash)).toEqual(expectedValue)
        })
    }
})
