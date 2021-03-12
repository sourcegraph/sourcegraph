import { createBrowserHistory } from 'history'
import React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { cleanup, getAllByTestId, getByTestId, render, waitFor } from '@testing-library/react'
import { noop, NEVER } from 'rxjs'
import sinon from 'sinon'
import {
    extensionsController,
    HIGHLIGHTED_FILE_LINES_REQUEST,
    NOOP_SETTINGS_CASCADE,
    OBSERVABLE_SEARCH_REQUEST,
} from '../../../../shared/src/util/searchTestHelpers'
import { SearchResults, SearchResultsProps } from './SearchResults'
import { SearchPatternType } from '../../graphql-operations'

describe('SearchResults', () => {
    afterAll(cleanup)

    const history = createBrowserHistory()
    history.replace({ search: 'q=r:golang/oauth2+test+f:travis&patternType=regexp' })

    const defaultProps: SearchResultsProps = {
        authenticatedUser: null,
        location: history.location,
        history,
        navbarSearchQueryState: { query: '' },
        fetchHighlightedFileLineRanges: HIGHLIGHTED_FILE_LINES_REQUEST,
        searchRequest: OBSERVABLE_SEARCH_REQUEST,
        isLightTheme: true,
        settingsCascade: NOOP_SETTINGS_CASCADE,
        extensionsController,
        isSourcegraphDotCom: false,
        platformContext: { forceUpdateTooltip: sinon.spy(), settings: NEVER },
        telemetryService: { log: noop, logViewEvent: noop },
        deployType: 'dev',
        patternType: SearchPatternType.regexp,
        parsedSearchQuery: 'r:golang/oauth2 test f:travis',
        caseSensitive: false,
        setPatternType: sinon.spy(),
        setCaseSensitivity: sinon.spy(),
        versionContext: undefined,
        selectedSearchContextSpec: 'global',
        setVersionContext: () => Promise.resolve(undefined),
        availableVersionContexts: undefined,
        previousVersionContext: 'sg-last-version-context',
        enableCodeMonitoring: false,
    }

    it('calls the search request once', () => {
        render(
            <BrowserRouter>
                <SearchResults {...defaultProps} />
            </BrowserRouter>
        )
        expect(OBSERVABLE_SEARCH_REQUEST.calledOnce)
    })

    it('displays exactly one filter bar and one repositories filter bar', async () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResults {...defaultProps} />
            </BrowserRouter>
        )
        await waitFor(() => getByTestId(container, 'filters-bar'))
        expect(getAllByTestId(container, 'filters-bar').length).toBe(1)
        expect(getAllByTestId(container, 'repo-filters-bar').length).toBe(1)
    })

    it('displays correct number of non-repository filters', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResults {...defaultProps} />
            </BrowserRouter>
        )
        const filtersBar = getByTestId(container, 'filters-bar')
        expect(getAllByTestId(filtersBar, 'filter-chip').length).toBe(2)
    })

    it('displays correct number of repository filters', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResults {...defaultProps} />
            </BrowserRouter>
        )
        const repoFiltersBar = getByTestId(container, 'repo-filters-bar')
        expect(getAllByTestId(repoFiltersBar, 'filter-chip').length).toBe(1)
    })
})
