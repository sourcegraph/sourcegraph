import { cleanup, fireEvent } from '@testing-library/react'
import delay from 'delay'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { AutomockGraphQLProvider, waitForNextApolloResponse } from '@sourcegraph/shared/src/testing/apollo'
import { setupMockServer } from '@sourcegraph/shared/src/testing/graphql/vitest'
import { type RenderWithBrandedContextResult, renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import {
    RepoRevisionSidebarSymbols,
    type RepoRevisionSidebarSymbolsProps,
    SYMBOLS_QUERY,
} from './RepoRevisionSidebarSymbols'

const location = {
    pathname: '/github.com/sourcegraph/sourcegraph@some-branch/-/blob/src/index.js',
}
const route = `${location.pathname}`

const sidebarProps: RepoRevisionSidebarSymbolsProps = {
    repoID: 'repo-id',
    revision: 'some-branch',
    activePath: 'src/index.js',
    onHandleSymbolClick: () => {},
}

const mockServer = setupMockServer()
const symbolsMock = mockServer.mockGraphql({
    query: SYMBOLS_QUERY,
    mocks: {
        SymbolConnection: () => ({
            nodes: [
                {
                    name: 'firstSymbol',
                    url: `${location.pathname}?L13:14`,
                },
            ],
        }),
    },
})

describe('RepoRevisionSidebarSymbols', () => {
    let renderResult: RenderWithBrandedContextResult
    afterEach(cleanup)

    beforeEach(async () => {
        mockServer.use(symbolsMock)
        renderResult = renderWithBrandedContext(
            <AutomockGraphQLProvider>
                <RepoRevisionSidebarSymbols {...sidebarProps} />
            </AutomockGraphQLProvider>,
            { route }
        )
        await waitForNextApolloResponse()
    })

    it('renders symbol correctly', () => {
        const symbol = renderResult.getByText('firstSymbol')
        expect(symbol).toBeVisible()
    })

    it('renders summary correctly', () => {
        expect(renderResult.getByText('1 symbol total')).toBeVisible()
    })

    it('clicking symbol updates route', async () => {
        expect(renderResult.locationRef.current?.search).toEqual('')

        const symbol = renderResult.getByText('firstSymbol')
        fireEvent.click(symbol)

        // We need to synchronously flush inside the event handler and since this is warning in
        // React 18, we've moved it to a setTimeout. This test needs to wait for this timeout to be
        // flushed
        await delay(0)

        expect(renderResult.locationRef.current?.search).toEqual('?L13:14')
    })
})
