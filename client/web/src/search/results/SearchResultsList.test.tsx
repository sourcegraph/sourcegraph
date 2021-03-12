import { createBrowserHistory } from 'history'
import { range } from 'lodash'
import * as React from 'react'
import { BrowserRouter } from 'react-router-dom'
import { cleanup, getAllByTestId, getByTestId, queryByTestId, render } from '@testing-library/react'
import _VisibilitySensor from 'react-visibility-sensor'
import sinon from 'sinon'
import * as GQL from '../../../../shared/src/graphql/schema'
import { NOOP_TELEMETRY_SERVICE } from '../../../../shared/src/telemetry/telemetryService'
import {
    extensionsController,
    HIGHLIGHTED_FILE_LINES_REQUEST,
    MULTIPLE_SEARCH_REQUEST,
    RESULT,
    SEARCH_REQUEST,
} from '../../../../shared/src/util/searchTestHelpers'
import { SearchResultsList, SearchResultsListProps } from './SearchResultsList'
import { NEVER, of } from 'rxjs'
import { SearchPatternType } from '../../../../shared/src/graphql-operations'

let VISIBILITY_CHANGED_CALLBACKS: ((isVisible: boolean) => void)[] = []

class MockVisibilitySensor extends React.Component<{ onChange?: (isVisible: boolean) => void }> {
    constructor(props: { onChange?: (isVisible: boolean) => void }) {
        super(props)
        if (props.onChange) {
            VISIBILITY_CHANGED_CALLBACKS.push(props.onChange)
        }
    }

    public render(): JSX.Element {
        return <>{this.props.children}</>
    }

    public componentWillUnmount(): void {
        if (this.props.onChange) {
            VISIBILITY_CHANGED_CALLBACKS.splice(VISIBILITY_CHANGED_CALLBACKS.indexOf(this.props.onChange), 1)
        }
    }
}

jest.mock('react-visibility-sensor', (): typeof _VisibilitySensor => ({ children, onChange }) => (
    <MockVisibilitySensor onChange={onChange}>{children}</MockVisibilitySensor>
))

describe('SearchResultsList', () => {
    /**
     * Simulates "scrolling" to the end of the search results,
     * by triggering all the visibility changed callbacks with
     * visibility: `true`.
     */
    const scrollToBottom = (): void => {
        for (const onChange of VISIBILITY_CHANGED_CALLBACKS) {
            onChange(true)
        }
    }

    const mockResults = ({
        resultCount,
        limitHit,
    }: {
        resultCount: number
        limitHit: boolean
    }): GQL.ISearchResults => ({
        ...MULTIPLE_SEARCH_REQUEST(),
        limitHit,
        resultCount,
        approximateResultCount: `${resultCount}`,
        results: range(resultCount).map(index => ({
            ...RESULT,
            file: {
                ...RESULT.file,
                url: `${index}`,
            },
        })),
    })

    afterEach(() => {
        VISIBILITY_CHANGED_CALLBACKS = []
    })

    afterAll(cleanup)

    const history = createBrowserHistory()
    history.replace({ search: 'q=r:golang/oauth2+test+f:travis' })

    const defaultProps: SearchResultsListProps = {
        location: history.location,
        history,
        authenticatedUser: null,
        isSourcegraphDotCom: false,
        deployType: 'dev',

        resultsOrError: SEARCH_REQUEST(),
        onShowMoreResultsClick: sinon.spy(),

        allExpanded: true,
        onExpandAllResultsToggle: sinon.spy(),

        showSavedQueryModal: false,
        onSavedQueryModalClose: sinon.spy(),
        onSaveQueryClick: sinon.spy(),

        fetchHighlightedFileLineRanges: HIGHLIGHTED_FILE_LINES_REQUEST,

        isLightTheme: true,
        settingsCascade: {
            subjects: null,
            final: null,
        },
        extensionsController: { executeCommand: sinon.spy(), extHostAPI: extensionsController.extHostAPI },
        platformContext: { forceUpdateTooltip: sinon.spy(), settings: NEVER },
        telemetryService: NOOP_TELEMETRY_SERVICE,
        parsedSearchQuery: 'r:golang/oauth2 test f:travis',
        patternType: SearchPatternType.regexp,
        setPatternType: sinon.spy(),
        caseSensitive: false,
        setCaseSensitivity: sinon.spy(),

        versionContext: undefined,
        selectedSearchContextSpec: 'global',

        navbarSearchQueryState: { query: '' },

        shouldDisplayPerformanceWarning: () => of(false),
        enableCodeMonitoring: false,
    }

    it('displays loading text when results is undefined', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...defaultProps} resultsOrError={undefined} />
            </BrowserRouter>
        )

        expect(queryByTestId(container, 'loading-container')).toBeVisible()
    })

    it('shows error message when the search GraphQL request returns an error', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...defaultProps} resultsOrError={{ message: 'test error', name: 'TestError' }} />
            </BrowserRouter>
        )
        expect(getByTestId(container, 'search-results-list-error')).toBeVisible()
    })

    it('renders the search results info bar when there are results', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...defaultProps} />
            </BrowserRouter>
        )
        expect(getByTestId(container, 'results-info-bar')).toBeVisible()
    })

    it('renders one search result', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...defaultProps} />
            </BrowserRouter>
        )
        expect(getByTestId(container, 'result-container')).toBeVisible()
        expect(getAllByTestId(container, 'result-container').length).toBe(1)
    })

    it('does not display the loading indicator or error message if there are results', () => {
        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...defaultProps} />
            </BrowserRouter>
        )
        expect(queryByTestId(container, 'loading-container')).not.toBeInTheDocument()
        expect(queryByTestId(container, 'search-results-list-error')).not.toBeInTheDocument()
    })

    it('renders correct number of search results if there are multiple', () => {
        const props = { ...defaultProps, resultsOrError: MULTIPLE_SEARCH_REQUEST() }
        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        expect(getAllByTestId(container, 'result-container')).toHaveLength(3)
    })

    it('displays "Show More" when the limit is hit', () => {
        const props = {
            ...defaultProps,
            resultsOrError: mockResults({ resultCount: 31, limitHit: true }),
        }
        const { container, rerender } = render(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        scrollToBottom()
        rerender(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        expect(getByTestId(container, 'search-show-more-button')).toBeVisible()
    })

    it('does not display "Show More" if the limit isn\'t hit', () => {
        const props = {
            ...defaultProps,
            resultsOrError: mockResults({ resultCount: 40, limitHit: false }),
        }
        const { container, rerender } = render(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        scrollToBottom()
        rerender(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        expect(queryByTestId(container, 'search-show-more-button')).toBe(null)
    })

    it('shows "show more" when new results with limitHit=true are received, even if it previously received results with limitHit=false', () => {
        // Rel: https://github.com/sourcegraph/sourcegraph/issues/4564
        // Display search results with the limit hit, simulate scrolling and click 'show more'.
        const showMore = sinon.spy()
        const props = {
            ...defaultProps,
            resultsOrError: mockResults({ resultCount: 2, limitHit: true }),
            onShowMoreResultsClick: showMore,
        }
        const { container, rerender } = render(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        scrollToBottom()
        rerender(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        getByTestId(container, 'search-show-more-button').click()
        sinon.assert.calledOnce(showMore)

        // Render new results without a limit.
        // Simulate scrolling and verify show more button isn't present.
        rerender(
            <BrowserRouter>
                <SearchResultsList
                    {...defaultProps}
                    resultsOrError={mockResults({ resultCount: 4, limitHit: false })}
                />
            </BrowserRouter>
        )
        scrollToBottom()
        expect(queryByTestId(container, 'search-show-more-button')).toBe(null)

        // Re-render with original props and simulate scrolling,
        // Expect "show more" button to be present.
        rerender(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )
        scrollToBottom()
        expect(getByTestId(container, 'search-show-more-button')).toBeVisible()
    })

    it('does not add filters to query in search suggestions link', () => {
        const resultsOrError = mockResults({ resultCount: 0, limitHit: false })
        resultsOrError.alert = {
            __typename: 'SearchAlert',
            title: 'Test title',
            description: 'Test description',
            proposedQueries: [
                {
                    __typename: 'SearchQueryDescription',
                    description: 'test',
                    query: 'repo:test1|test2',
                },
            ],
        }

        const props = {
            ...defaultProps,
            resultsOrError,
        }

        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )

        const link = getByTestId(container, 'proposed-query-link') as HTMLAnchorElement
        expect(link).toBeVisible()
        expect(link.href).toStrictEqual('http://localhost/search?q=repo:test1%7Ctest2&patternType=regexp')
    })

    it('shows both alerts and results if both are present', () => {
        const resultsOrError = mockResults({ resultCount: 1, limitHit: false })
        resultsOrError.alert = {
            __typename: 'SearchAlert',
            title: 'Test title',
            description: 'Test description',
            proposedQueries: [
                {
                    __typename: 'SearchQueryDescription',
                    description: 'test',
                    query: 'repo:test1|test2',
                },
            ],
        }

        const props = {
            ...defaultProps,
            resultsOrError,
        }

        const { container } = render(
            <BrowserRouter>
                <SearchResultsList {...props} />
            </BrowserRouter>
        )

        const link = getByTestId(container, 'proposed-query-link') as HTMLAnchorElement
        const result = getByTestId(container, 'result-container')

        expect(link).toBeVisible()
        expect(result).toBeVisible()
        expect(link.compareDocumentPosition(result)).toStrictEqual(link.DOCUMENT_POSITION_FOLLOWING)
    })
})
