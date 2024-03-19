import React from 'react'

import '@sourcegraph/shared/src/testing/mockReactVisibilitySensor'

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { EMPTY, NEVER, of } from 'rxjs'
import { spy, assert } from 'sinon'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { GitRefType, SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { SearchMode, SearchQueryStateStoreProvider } from '@sourcegraph/shared/src/search'
import type { AggregateStreamingSearchResults, Skipped } from '@sourcegraph/shared/src/search/stream'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import {
    COLLAPSABLE_SEARCH_RESULT,
    HIGHLIGHTED_FILE_LINES_REQUEST,
    MULTIPLE_SEARCH_RESULT,
    REPO_MATCH_RESULT,
    CHUNK_MATCH_RESULT,
    LINE_MATCH_RESULT,
} from '@sourcegraph/shared/src/testing/searchTestHelpers'
import { simulateMenuItemClick } from '@sourcegraph/shared/src/testing/simulateMenuItemClick'

import type { AuthenticatedUser } from '../../auth'
import { useNavbarQueryState } from '../../stores'
import * as helpers from '../helpers'

import { SearchResultsCacheProvider } from './SearchResultsCacheProvider'
import { generateMockedResponses } from './sidebar/Revisions.mocks'
import { StreamingSearchResults, type StreamingSearchResultsProps } from './StreamingSearchResults'

describe('StreamingSearchResults', () => {
    const streamingSearchResult = MULTIPLE_SEARCH_RESULT

    const defaultProps: StreamingSearchResultsProps = {
        telemetryService: NOOP_TELEMETRY_SERVICE,

        authenticatedUser: null,

        settingsCascade: {
            subjects: null,
            final: { experimentalFeatures: { newSearchResultFiltersPanel: false } },
        },
        platformContext: {
            settings: NEVER,
            requestGraphQL: () => EMPTY,
            sourcegraphURL: 'https://sourcegraph.com',
        } as any,

        streamSearch: () => of(MULTIPLE_SEARCH_RESULT),

        fetchHighlightedFileLineRanges: HIGHLIGHTED_FILE_LINES_REQUEST,
        isSourcegraphDotCom: false,
        searchContextsEnabled: true,
        searchAggregationEnabled: false,
        codeMonitoringEnabled: true,
        ownEnabled: true,
    }

    const revisionsMockResponses = generateMockedResponses(GitRefType.GIT_BRANCH, 5, 'github.com/golang/oauth2')

    function renderWrapper(component: React.ReactElement<StreamingSearchResultsProps>) {
        return render(
            <BrowserRouter>
                <MockedTestProvider mocks={revisionsMockResponses}>
                    <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                        <SearchResultsCacheProvider>{component}</SearchResultsCacheProvider>
                    </SearchQueryStateStoreProvider>
                </MockedTestProvider>
            </BrowserRouter>
        )
    }

    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
    const mockUser = {
        id: 'userID',
        username: 'username',
        emails: [{ email: 'user@me.com', isPrimary: true, verified: true }],
        siteAdmin: true,
    } as AuthenticatedUser

    beforeEach(() => {
        useNavbarQueryState.setState({
            searchCaseSensitivity: false,
            searchQueryFromURL: 'r:golang/oauth2 test f:travis',
        })
    })

    it('should call streaming search API with the right parameters from URL', async () => {
        useNavbarQueryState.setState({ searchCaseSensitivity: true, searchPatternType: SearchPatternType.regexp })
        const searchSpy = spy(defaultProps.streamSearch)

        renderWrapper(<StreamingSearchResults {...defaultProps} streamSearch={searchSpy} />)

        assert.calledOnce(searchSpy)
        const call = searchSpy.getCall(0)
        // We have to extract the query from the observable since we can't directly compare observables
        const receivedQuery = await call.args[0].toPromise()
        const receivedOptions = call.args[1]

        expect(receivedQuery).toEqual('r:golang/oauth2 test f:travis')
        expect(receivedOptions).toEqual({
            version: 'V3',
            patternType: SearchPatternType.regexp,
            caseSensitive: true,
            searchMode: SearchMode.SmartSearch,
            trace: undefined,
            chunkMatches: true,
            maxLineLen: 5 * 1024,
            featureOverrides: [],
            zoektSearchOptions: '',
        })
    })

    it('should render progress with data from API', () => {
        renderWrapper(<StreamingSearchResults {...defaultProps} />)

        // Dropdown not in doc for progress.skipped === []
        expect(screen.queryByTestId('streaming-progress-dropdown')).not.toBeInTheDocument()
        const expectedString = `${streamingSearchResult.progress.matchCount} results in ${(
            streamingSearchResult.progress.durationMs / 1000
        ).toFixed(2)}s`
        expect(screen.getAllByTestId('streaming-progress-count')[0]).toHaveTextContent(expectedString)
    })

    it('should expand and collapse results when event from infobar is triggered', async () => {
        renderWrapper(<StreamingSearchResults {...defaultProps} streamSearch={() => of(COLLAPSABLE_SEARCH_RESULT)} />)

        screen
            .getAllByTestId('file-search-result')
            .map(element => expect(element).toHaveAttribute('data-expanded', 'false'))

        userEvent.click(await screen.findByLabelText(/Open search actions menu/))
        simulateMenuItemClick(await screen.findByText(/Expand all/, { selector: '[role=menuitem]' }))

        screen
            .getAllByTestId('file-search-result')
            .map(element => expect(element).toHaveAttribute('data-expanded', 'true'))

        userEvent.click(await screen.findByLabelText(/Open search actions menu/))
        simulateMenuItemClick(await screen.findByText(/Collapse all/, { selector: '[role=menuitem]' }))

        screen
            .getAllByTestId('file-search-result')
            .map(element => expect(element).toHaveAttribute('data-expanded', 'false'))
    })

    it('should render correct components for file match and repository match', () => {
        const results: AggregateStreamingSearchResults = {
            ...streamingSearchResult,
            results: [CHUNK_MATCH_RESULT, REPO_MATCH_RESULT],
        }
        renderWrapper(<StreamingSearchResults {...defaultProps} streamSearch={() => of(results)} />)
        expect(screen.getAllByTestId('result-container').length).toBe(2)
        expect(screen.getByTestId('search-repo-result')).toBeVisible()

        expect(screen.getAllByTestId('result-container')[0]).toHaveAttribute('data-result-type', 'content')
        expect(screen.getAllByTestId('result-container')[1]).toHaveAttribute('data-result-type', 'repo')
    })

    it('should render correct components for file match using legacy line match format', () => {
        const results: AggregateStreamingSearchResults = {
            ...streamingSearchResult,
            results: [LINE_MATCH_RESULT],
        }

        renderWrapper(<StreamingSearchResults {...defaultProps} streamSearch={() => of(results)} />)
        expect(screen.getAllByTestId('result-container').length).toBe(1)

        expect(screen.getAllByTestId('result-container')[0]).toHaveAttribute('data-result-type', 'content')
    })

    it('should log view, query, and results fetched events', () => {
        const logSpy = spy()
        const logViewEventSpy = spy()
        const telemetryService = {
            ...NOOP_TELEMETRY_SERVICE,
            log: logSpy,
            logViewEvent: logViewEventSpy,
        }

        renderWrapper(<StreamingSearchResults {...defaultProps} telemetryService={telemetryService} />)

        assert.calledOnceWithExactly(logViewEventSpy, 'SearchResults')
        assert.calledWith(logSpy, 'SearchResultsQueried')
        assert.calledWith(logSpy, 'SearchResultsFetched')
    })

    // This test passes but it throws some internal happy-dom errors while
    // running. See thread  https://sourcegraph.slack.com/archives/C04MYFW01NV/p1705436143793999
    // you can find original problem issue https://github.com/sourcegraph/sourcegraph/issues/59700
    it.skip('should log events when clicking on search result', () => {
        const logSpy = spy()
        const telemetryService = {
            ...NOOP_TELEMETRY_SERVICE,
            log: logSpy,
        }

        renderWrapper(<StreamingSearchResults {...defaultProps} telemetryService={telemetryService} />)

        userEvent.click(screen.getAllByTestId('result-container')[0])
        assert.calledWith(logSpy, 'SearchResultClicked')
        assert.calledWith(logSpy, 'search.ranking.result-clicked', {
            index: 0,
            type: 'fileMatch',
            resultsLength: 3,
        })

        userEvent.click(screen.getAllByTestId('result-container')[2])
        assert.calledWith(logSpy, 'SearchResultClicked')
        assert.calledWith(logSpy, 'search.ranking.result-clicked', {
            index: 2,
            type: 'fileMatch',
            resultsLength: 3,
        })
    })

    it('should not show saved search modal on first load', () => {
        renderWrapper(<StreamingSearchResults {...defaultProps} />)
        expect(screen.queryByTestId('saved-search-modal')).not.toBeInTheDocument()
    })

    it('should open and close saved search modal if events trigger', async () => {
        renderWrapper(<StreamingSearchResults {...defaultProps} authenticatedUser={mockUser} />)
        userEvent.click(await screen.findByLabelText(/Open search actions menu/))
        simulateMenuItemClick(await screen.findByText(/Save search/, { selector: '[role=menuitem]' }))

        fireEvent.keyDown(await screen.findByText(/Save search query to:/), {
            key: 'Escape',
            code: 'Escape',
            keyCode: 27,
            charCode: 27,
        })

        expect(screen.queryByText(/Save search query to:/)).not.toBeInTheDocument()
    })

    it('should start a new search with added params when onSearchAgain event is triggered', async () => {
        const submitSearchMock = vi.spyOn(helpers, 'submitSearch').mockImplementation(() => {})
        const tests = [
            {
                parsedSearchQuery: 'r:golang/oauth2 test f:travis',
                skipReason: ['document-match-limit', 'excluded-archive', 'shard-timedout'] as Skipped['reason'][],
                additionalProperties: ['count:1000', 'archived:yes', 'timeout:2m'],
                want: 'r:golang/oauth2 test f:travis count:1000 archived:yes timeout:2m',
            },
            {
                parsedSearchQuery: 'r:golang/oauth2 test f:travis count:50',
                skipReason: ['document-match-limit', 'excluded-archive', 'shard-timedout'] as Skipped['reason'][],
                additionalProperties: ['count:1000', 'archived:yes', 'timeout:2m'],
                want: 'r:golang/oauth2 test f:travis count:1000 archived:yes timeout:2m',
            },
            {
                parsedSearchQuery: 'r:golang/oauth2 (foo count:1) or (bar count:2)',
                skipReason: ['document-match-limit', 'excluded-fork'] as Skipped['reason'][],
                additionalProperties: ['count:1000', 'fork:yes'],
                want: 'r:golang/oauth2 (foo count:1000) or (bar count:1000) fork:yes',
            },
        ]

        for (const [index, test] of tests.entries()) {
            cleanup()

            const results: AggregateStreamingSearchResults = {
                ...streamingSearchResult,
                progress: {
                    ...streamingSearchResult.progress,
                    skipped: test.additionalProperties.map((property, propertyIndex) => ({
                        reason: test.skipReason[propertyIndex],
                        message: property,
                        severity: 'info',
                        title: property,
                        suggested: {
                            title: property,
                            queryExpression: property,
                        },
                    })),
                },
            }

            useNavbarQueryState.setState({ searchQueryFromURL: test.parsedSearchQuery })

            renderWrapper(<StreamingSearchResults {...defaultProps} streamSearch={() => of(results)} />)

            userEvent.click((await screen.findAllByText(/results in/i))[0])
            const allChecks = await screen.findAllByTestId(/^streaming-progress-skipped-suggest-check/)

            for (const check of allChecks) {
                userEvent.click(check, undefined, { skipPointerEventsCheck: true })
            }

            userEvent.click(
                await screen.findByText(/modify and re-run/i, { selector: 'button[type=submit]' }),
                undefined,
                {
                    skipPointerEventsCheck: true,
                }
            )

            expect(helpers.submitSearch).toBeCalledTimes(index + 1)
            const args = submitSearchMock.mock.calls[index][0]
            expect(args.query).toBe(test.want)
        }
    })
})
