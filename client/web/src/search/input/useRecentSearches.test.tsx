import React, { useState } from 'react'

import type { MockedResponse } from '@apollo/client/testing'
import { render, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'

import { getDocumentNode } from '@sourcegraph/http-client'
import type { RecentSearch } from '@sourcegraph/shared/src/settings/temporary/recentSearches'
import { MockTemporarySettings } from '@sourcegraph/shared/src/settings/temporary/testUtils'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import type { SearchHistoryEventLogsQueryResult } from '../../graphql-operations'

import { buildMockRecentSearches } from './recentSearches.test'
import { SEARCH_HISTORY_EVENT_LOGS_QUERY, useRecentSearches } from './useRecentSearches'

function buildMockEventLogs(items: number): SearchHistoryEventLogsQueryResult {
    return {
        currentUser: {
            __typename: 'User',
            recentSearchLogs: {
                nodes: Array.from({ length: items }, (_item, index) => ({
                    argument: `{"code_search": { "query_data": { "combined": "test${index}"}}}`,
                    timestamp: '2021-01-01T00:00:00Z',
                })),
            },
        },
    }
}

const Wrapper: React.FunctionComponent<{
    tempSettings: RecentSearch[]
    eventLogs: SearchHistoryEventLogsQueryResult
}> = ({ tempSettings, eventLogs }) => {
    const mockedEventLogs: MockedResponse[] = [
        {
            request: { query: getDocumentNode(SEARCH_HISTORY_EVENT_LOGS_QUERY), variables: { first: 20 } },
            result: { data: eventLogs },
        },
    ]

    return (
        <MockedTestProvider mocks={mockedEventLogs}>
            <MockTemporarySettings settings={{ 'search.input.recentSearches': tempSettings }}>
                <InnerWrapper />
            </MockTemporarySettings>
        </MockedTestProvider>
    )
}

const InnerWrapper: React.FunctionComponent<{}> = () => {
    const { recentSearches, addRecentSearch, state } = useRecentSearches()

    const [searchToAdd, setSearchToAdd] = useState('')

    return (
        <>
            <ul>
                {recentSearches?.map(recentSearch => (
                    <>
                        <li key={recentSearch.query}>{recentSearch.query}</li>
                    </>
                ))}
            </ul>
            <div data-testid="state">{state}</div>
            {/* eslint-disable-next-line react/forbid-elements*/}
            <input
                type="text"
                data-testid="input"
                value={searchToAdd}
                onInput={event => setSearchToAdd(event.currentTarget.value)}
            />
            <button type="button" data-testid="button" onClick={() => addRecentSearch(searchToAdd, 555, false)} />
        </>
    )
}

describe('recentSearches', () => {
    describe('useRecentSearches().recentSearches', () => {
        test('recent searches is empty array if no data in temp settings or event logs', async () => {
            const { queryAllByRole, getByTestId } = render(
                <Wrapper tempSettings={buildMockRecentSearches(0)} eventLogs={buildMockEventLogs(0)} />
            )

            await waitFor(() => expect(getByTestId('state')).toHaveTextContent('success'))

            const items = queryAllByRole('listitem').map(element => element.textContent)
            expect(items).toMatchInlineSnapshot('Array []')
        })

        test('recent searches is populated from event logs if no data in temp settings, with deduplication', async () => {
            const mockedEventLogs = buildMockEventLogs(2)
            const nodes = mockedEventLogs.currentUser?.recentSearchLogs?.nodes ?? []
            const mockedEventLogsWithDuplicates: SearchHistoryEventLogsQueryResult = {
                currentUser: {
                    __typename: 'User',
                    recentSearchLogs: {
                        nodes: [...nodes, ...nodes],
                    },
                },
            }

            const { queryAllByRole, getByTestId } = render(
                <Wrapper tempSettings={buildMockRecentSearches(0)} eventLogs={mockedEventLogsWithDuplicates} />
            )

            await waitFor(() => expect(getByTestId('state')).toHaveTextContent('success'))

            const items = queryAllByRole('listitem').map(element => element.textContent)
            expect(items).toMatchInlineSnapshot(`
                Array [
                  "test0",
                  "test1",
                ]
            `)
        })

        test('recent searches is populated from temp settings', async () => {
            const { queryAllByRole, getByTestId } = render(
                <Wrapper tempSettings={buildMockRecentSearches(4)} eventLogs={buildMockEventLogs(0)} />
            )

            await waitFor(() => expect(getByTestId('state')).toHaveTextContent('success'))

            const items = queryAllByRole('listitem').map(element => element.textContent)
            expect(items).toMatchInlineSnapshot(`
                Array [
                  "test0",
                  "test1",
                  "test2",
                  "test3",
                ]
            `)
        })
    })

    describe('useRecentSearches().addRecentSearch', () => {
        test('adding item to recent searches puts it at the top', async () => {
            const { queryAllByRole, getByTestId } = render(
                <Wrapper tempSettings={buildMockRecentSearches(4)} eventLogs={buildMockEventLogs(0)} />
            )

            await waitFor(() => expect(getByTestId('state')).toHaveTextContent('success'))

            userEvent.type(getByTestId('input'), 'test4')
            userEvent.click(getByTestId('button'))

            const items = queryAllByRole('listitem').map(element => element.textContent)
            expect(items).toMatchInlineSnapshot(`
                Array [
                  "test4",
                  "test0",
                  "test1",
                  "test2",
                  "test3",
                ]
            `)
        })

        test('adding an existing item to recent searches deduplicates it and puts it at the top', async () => {
            const { queryAllByRole, getByTestId } = render(
                <Wrapper tempSettings={buildMockRecentSearches(4)} eventLogs={buildMockEventLogs(0)} />
            )

            await waitFor(() => expect(getByTestId('state')).toHaveTextContent('success'))

            userEvent.type(getByTestId('input'), 'test2')
            userEvent.click(getByTestId('button'))

            const items = queryAllByRole('listitem').map(element => element.textContent)
            expect(items).toMatchInlineSnapshot(`
                    Array [
                      "test2",
                      "test0",
                      "test1",
                      "test3",
                    ]
                `)
        })

        test('adding an item beyond the limit of the list removes the last item', async () => {
            const { queryAllByRole, getByTestId } = render(
                <Wrapper tempSettings={buildMockRecentSearches(20)} eventLogs={buildMockEventLogs(0)} />
            )

            await waitFor(() => expect(getByTestId('state')).toHaveTextContent('success'))

            userEvent.type(getByTestId('input'), 'test20')
            userEvent.click(getByTestId('button'))

            const items = queryAllByRole('listitem').map(element => element.textContent)
            expect(items).toMatchInlineSnapshot(`
                Array [
                  "test20",
                  "test0",
                  "test1",
                  "test2",
                  "test3",
                  "test4",
                  "test5",
                  "test6",
                  "test7",
                  "test8",
                  "test9",
                  "test10",
                  "test11",
                  "test12",
                  "test13",
                  "test14",
                  "test15",
                  "test16",
                  "test17",
                  "test18",
                ]
            `)
        })
    })
})
