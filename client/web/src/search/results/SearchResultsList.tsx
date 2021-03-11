import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import * as H from 'history'
import { isEqual } from 'lodash'
import FileIcon from 'mdi-react/FileIcon'
import SearchIcon from 'mdi-react/SearchIcon'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'
import * as React from 'react'
import { Link } from 'react-router-dom'
import { Observable, Subject, Subscription } from 'rxjs'
import { debounceTime, distinctUntilChanged, filter, first, map, skip, skipUntil } from 'rxjs/operators'
import {
    parseSearchURLQuery,
    PatternTypeProps,
    CaseSensitivityProps,
    ParsedSearchQueryProps,
    SearchContextProps,
} from '..'
import { FetchFileParameters } from '../../../../shared/src/components/CodeExcerpt'
import { FileMatch } from '../../../../shared/src/components/FileMatch'
import { displayRepoName } from '../../../../shared/src/components/RepoFileLink'
import { VirtualList } from '../../../../shared/src/components/VirtualList'
import { ExtensionsControllerProps } from '../../../../shared/src/extensions/controller'
import * as GQL from '../../../../shared/src/graphql/schema'
import { PlatformContextProps } from '../../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { isDefined, hasProperty } from '../../../../shared/src/util/types'
import { SearchResult } from '../../components/SearchResult'
import { SavedSearchModal } from '../../savedSearches/SavedSearchModal'
import { ThemeProps } from '../../../../shared/src/theme'
import { eventLogger } from '../../tracking/eventLogger'
import { SearchResultsInfoBar } from './SearchResultsInfoBar'
import { ErrorAlert } from '../../components/alerts'
import { VersionContextProps } from '../../../../shared/src/search/util'
import { DeployType } from '../../jscontext'
import { AuthenticatedUser } from '../../auth'
import { SearchResultTypeTabs } from './SearchResultTypeTabs'
import { QueryState } from '../helpers'
import { PerformanceWarningAlert } from '../../site/PerformanceWarningAlert'
import { SearchResultsStats } from './SearchResultsStats'
import { SearchAlert } from './SearchAlert'
import { CodeMonitoringProps } from '../../enterprise/code-monitoring'

const isSearchResults = (value: unknown): value is GQL.ISearchResults =>
    typeof value === 'object' &&
    value !== null &&
    hasProperty('__typename')(value) &&
    value.__typename === 'SearchResults'

export interface SearchResultsListProps
    extends ExtensionsControllerProps<'executeCommand' | 'services'>,
        PlatformContextProps<'forceUpdateTooltip' | 'settings'>,
        TelemetryProps,
        SettingsCascadeProps,
        ThemeProps,
        Pick<ParsedSearchQueryProps, 'parsedSearchQuery'>,
        PatternTypeProps,
        CaseSensitivityProps,
        VersionContextProps,
        Pick<CodeMonitoringProps, 'enableCodeMonitoring'>,
        Pick<SearchContextProps, 'selectedSearchContextSpec'> {
    location: H.Location
    history: H.History
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    deployType: DeployType

    // Result list
    resultsOrError?: GQL.ISearchResults | ErrorLike
    onShowMoreResultsClick?: () => void
    navbarSearchQueryState: QueryState

    /* Called when the first result has fully loaded. */
    onFirstResultLoad?: () => void

    // Expand all feature
    allExpanded: boolean
    onExpandAllResultsToggle: () => void

    // Saved queries
    showSavedQueryButton?: boolean
    showSavedQueryModal: boolean
    onSavedQueryModalClose: () => void
    onSaveQueryClick: () => void

    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
    shouldDisplayPerformanceWarning: (deployType: DeployType) => Observable<boolean>
}

interface State {
    resultsShown: number
    visibleItems: Set<number>
    didScrollToItem: boolean
    /** Map from repo name to display name */
    fileMatchRepoDisplayNames: ReadonlyMap<string, string>
    displayPerformanceWarning: boolean
}

export class SearchResultsList extends React.PureComponent<SearchResultsListProps, State> {
    /** Emits when a result was either scrolled into or out of the page */
    private visibleItemChanges = new Subject<{ isVisible: boolean; index: number }>()
    private nextItemVisibilityChange = (isVisible: boolean, index: number): void =>
        this.visibleItemChanges.next({ isVisible, index })

    /** Emits with the index of the first visible result on the page */
    private firstVisibleItems = new Subject<number>()

    /** Reference to the current scrollable list element */
    private scrollableElementRef: HTMLElement | null = null
    private setScrollableElementRef = (reference: HTMLElement | null): void => {
        this.scrollableElementRef = reference
    }

    /** Emits with the <VirtualList> elements */
    private virtualListContainerElements = new Subject<HTMLElement | null>()
    private nextVirtualListContainerElement = (reference: HTMLElement | null): void =>
        this.virtualListContainerElements.next(reference)

    private jumpToTopClicks = new Subject<void>()
    private nextJumpToTopClick = (): void => this.jumpToTopClicks.next()

    private componentUpdates = new Subject<SearchResultsListProps>()

    private subscriptions = new Subscription()

    constructor(props: SearchResultsListProps) {
        super(props)

        this.state = {
            resultsShown: this.getCheckpoint() + 15,
            visibleItems: new Set<number>(),
            didScrollToItem: false,
            fileMatchRepoDisplayNames: new Map<string, string>(),
            displayPerformanceWarning: false,
        }

        // Handle items that have become visible
        this.subscriptions.add(
            this.visibleItemChanges
                .pipe(filter(({ isVisible, index }) => isVisible && !this.state.visibleItems.has(index)))
                .subscribe(({ index }) => {
                    this.setState(({ visibleItems }) => {
                        visibleItems.add(index)

                        return {
                            visibleItems: new Set(visibleItems),
                        }
                    })
                })
        )

        // Handle items that are no longer visible
        this.subscriptions.add(
            this.visibleItemChanges
                .pipe(filter(({ isVisible, index }) => !isVisible && this.state.visibleItems.has(index)))
                .subscribe(({ index }) => {
                    this.setState(({ visibleItems }) => {
                        visibleItems.delete(index)

                        return {
                            visibleItems: new Set(visibleItems),
                        }
                    })
                })
        )

        /** Emits when the first visible items has changed */
        const firstVisibleItemChanges = this.firstVisibleItems.pipe(
            // No need to update when unchanged
            distinctUntilChanged(),
            // Wait a little so we don't update while scrolling
            debounceTime(250)
        )

        //  Update the `at` query param with the latest first visible item
        this.subscriptions.add(
            firstVisibleItemChanges
                // Skip page load
                .pipe(skip(1))
                .subscribe(checkpoint => this.setCheckpoint(checkpoint))
        )

        // Remove the "Jump to top" button when the user starts scrolling
        this.subscriptions.add(
            this.visibleItemChanges
                .pipe(
                    // We know the user has scrolled when the first visible item has changed
                    skipUntil(firstVisibleItemChanges),
                    // Ignore items being scrolled out due to result items expanding as they load
                    filter(({ isVisible }) => isVisible),
                    // No need to keep firing this
                    first()
                )
                .subscribe(() =>
                    this.setState({
                        didScrollToItem: false,
                    })
                )
        )

        // Scroll the list to the item specified by the `at` query param
        this.subscriptions.add(
            this.virtualListContainerElements
                .pipe(
                    filter(isDefined),
                    // Only on page load
                    first(),
                    map(container => ({ container, checkpoint: this.getCheckpoint() })),
                    // Don't scroll to the first item
                    filter(({ checkpoint }) => checkpoint > 0)
                )
                .subscribe(({ container, checkpoint }) => {
                    let itemToScrollTo = container.children.item(checkpoint)

                    // Handle edge case where user manually sets the checkpoint to greater than the number of results
                    if (itemToScrollTo === null) {
                        const lastIndex = container.children.length - 1

                        itemToScrollTo = container.children.item(lastIndex)

                        this.setCheckpoint(lastIndex)
                    }

                    // It seems unlikely, but still possbile for 'scrollableElementRef' to be null here.
                    // It might be possible for the 'onRef' callback of 'VirtualList' to be triggered
                    // (which would kick off this pipeline) BEFORE the 'ref' callback for the
                    // 'search-results-list' div is executed (which would cause this conditional to be met).
                    // We'll log the error and gracefully exit for now, but we might need to re-evaluate our strategy
                    // if we see this error in production.
                    //
                    // If this case occurs, the page will not automatically scroll to the list item
                    // on page load.
                    if (this.scrollableElementRef === null) {
                        console.error('scrollableElement ref was null when trying to scroll to a list item')
                        return
                    }

                    const scrollable = this.scrollableElementRef

                    const scrollTo =
                        itemToScrollTo!.getBoundingClientRect().top - scrollable.getBoundingClientRect().top

                    scrollable.scrollTop = scrollTo

                    this.setState({ didScrollToItem: true })
                })
        )

        // Scroll to the top when "Jump to top" is clicked
        this.subscriptions.add(
            this.jumpToTopClicks.subscribe(() => {
                // this.scrollableElementRef will never be null here. 'jumpToTopClicks'
                // only emits events when the "Jump to Top" anchor tag is clicked, which can
                // never occur before that element is rendered (the 'ref' callback for
                // 'search-results-list' would have already been called at this point).
                const scrollable = this.scrollableElementRef!

                scrollable.scrollTop = 0
                this.setState({ didScrollToItem: false })
            })
        )

        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    distinctUntilChanged((a, b) => isEqual(a, b)),
                    map(({ resultsOrError }) => resultsOrError),
                    filter(isDefined),
                    filter((resultsOrError): resultsOrError is GQL.ISearchResults => !isErrorLike(resultsOrError)),
                    map(({ results }) => results),
                    map((results): GQL.IFileMatch[] =>
                        results.filter((result): result is GQL.IFileMatch => result.__typename === 'FileMatch')
                    )
                )
                .subscribe(fileMatches => {
                    const fileMatchRepoDisplayNames = new Map<string, string>()
                    for (const {
                        repository: { name },
                    } of fileMatches) {
                        const displayName = displayRepoName(name)
                        fileMatchRepoDisplayNames.set(name, displayName)
                    }

                    const displayNameCounts = new Map<string, number>()
                    for (const displayName of fileMatchRepoDisplayNames.values()) {
                        displayNameCounts.set(displayName, (displayNameCounts.get(displayName) || 0) + 1)
                    }

                    for (const [displayName, count] of displayNameCounts.entries()) {
                        if (count > 1) {
                            for (const [name, displayName1] of fileMatchRepoDisplayNames) {
                                if (displayName === displayName1) {
                                    fileMatchRepoDisplayNames.set(name, name)
                                }
                            }
                        }
                    }

                    this.setState({ fileMatchRepoDisplayNames })
                })
        )
    }

    public componentDidMount(): void {
        this.componentUpdates.next(this.props)

        this.subscriptions.add(
            this.props
                .shouldDisplayPerformanceWarning(this.props.deployType)
                .subscribe(displayPerformanceWarning => this.setState({ displayPerformanceWarning }))
        )
    }

    public componentDidUpdate(): void {
        const lowestIndex = [...this.state.visibleItems].reduce((low, index) => Math.min(index, low), Infinity)

        this.firstVisibleItems.next(lowestIndex)

        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): React.ReactNode {
        const parsedQuery = parseSearchURLQuery(this.props.location.search)

        return (
            <>
                {this.state.didScrollToItem && (
                    <div className="search-results-list__jump-to-top">
                        Scrolled to result {this.getCheckpoint()} based on URL.&nbsp;
                        <a href="#" onClick={this.nextJumpToTopClick}>
                            Jump to top.
                        </a>
                    </div>
                )}

                <div className="search-results-list" ref={this.setScrollableElementRef}>
                    {/* Saved Queries Form */}
                    {this.props.showSavedQueryModal && (
                        <SavedSearchModal
                            {...this.props}
                            query={parsedQuery}
                            authenticatedUser={this.props.authenticatedUser}
                            onDidCancel={this.props.onSavedQueryModalClose}
                        />
                    )}

                    {this.props.resultsOrError === undefined ? (
                        <div className="text-center mt-2" data-testid="loading-container">
                            <LoadingSpinner className="icon-inline" />
                        </div>
                    ) : isErrorLike(this.props.resultsOrError) ? (
                        /* GraphQL, network, query syntax error */
                        <ErrorAlert
                            className="m-2"
                            data-testid="search-results-list-error"
                            error={this.props.resultsOrError}
                        />
                    ) : (
                        (() => {
                            const results = this.props.resultsOrError

                            return (
                                <>
                                    <div className="d-lg-flex mb-2 align-items-end flex-wrap">
                                        <SearchResultTypeTabs
                                            {...this.props}
                                            query={this.props.navbarSearchQueryState.query}
                                            className="search-results-list__tabs"
                                        />

                                        <SearchResultsInfoBar
                                            {...this.props}
                                            query={parsedQuery}
                                            resultsFound={results.results.length > 0}
                                            className="border-bottom flex-grow-1"
                                            stats={
                                                <SearchResultsStats
                                                    results={results}
                                                    onShowMoreResultsClick={this.props.onShowMoreResultsClick}
                                                />
                                            }
                                        />
                                    </div>

                                    {!results.alert && this.state.displayPerformanceWarning && (
                                        <PerformanceWarningAlert />
                                    )}

                                    {/* Server-provided help message */}
                                    {results.alert && (
                                        <SearchAlert
                                            alert={results.alert}
                                            caseSensitive={this.props.caseSensitive}
                                            patternType={this.props.patternType}
                                            versionContext={this.props.versionContext}
                                        >
                                            {' '}
                                            {results.timedout.length > 0 &&
                                                results.timedout.length === results.repositoriesCount &&
                                                /* All repositories timed out. */
                                                this.renderRecommendations(
                                                    ['dev', 'docker-container'].includes(window.context.deployType)
                                                        ? [
                                                              <>
                                                                  Upgrade to Sourcegraph Enterprise for a highly
                                                                  scalable Docker Compose or Kubernetes cluster
                                                                  deployment option.
                                                              </>,
                                                              window.context.likelyDockerOnMac
                                                                  ? 'Use Docker Machine instead of Docker for Mac for better performance on macOS.'
                                                                  : 'Contact your Sourcegraph administrator if you are seeing timeouts regularly, as more CPU, memory, or disk resources may need to be provisioned.',
                                                          ]
                                                        : []
                                                )}
                                        </SearchAlert>
                                    )}

                                    {/* Results */}
                                    <VirtualList<GQL.SearchResult>
                                        itemsToShow={this.state.resultsShown}
                                        onShowMoreItems={this.onBottomHit(results.results.length)}
                                        onVisibilityChange={this.nextItemVisibilityChange}
                                        items={results.results}
                                        itemKey={this.itemKey}
                                        itemProps={undefined}
                                        renderItem={this.renderResult}
                                        containment={this.scrollableElementRef || undefined}
                                        onRef={this.nextVirtualListContainerElement}
                                    />

                                    {/*
                                        Show more button

                                        We only show this button at the bottom of the page when the
                                        user has scrolled completely to the bottom of the virtual
                                        list (i.e. when resultsShown is results.length).

                                        Note however that when the bottom is hit, this.onBottomHit
                                        is called to asynchronously update resultsShown to add 10
                                        which means there is a race condition in which e.g.
                                        results.length == 30 && resultsShown == 40 so we use >=
                                        comparison below.
                                    */}
                                    {results.limitHit &&
                                        this.state.resultsShown >= results.results.length &&
                                        this.props.onShowMoreResultsClick && (
                                            <button
                                                type="button"
                                                className="btn btn-secondary btn-block"
                                                data-testid="search-show-more-button"
                                                onClick={this.props.onShowMoreResultsClick}
                                            >
                                                Show more
                                            </button>
                                        )}

                                    {results.matchCount === 0 && !results.alert && (
                                        <div className="alert alert-info d-flex m-2">
                                            <h3 className="m-0">
                                                <SearchIcon className="icon-inline" /> No results
                                            </h3>
                                        </div>
                                    )}
                                </>
                            )
                        })()
                    )}

                    <div className="pb-4" />
                    {this.props.resultsOrError !== undefined && (
                        <Link className="mb-4 p-3" to="/help/code_search">
                            Learn more about our search syntax.
                        </Link>
                    )}
                </div>
            </>
        )
    }

    /**
     * Renders the given recommendations in a list if multiple, otherwise returns the first one or undefined
     */
    private renderRecommendations(recommendations: React.ReactNode[]): React.ReactNode {
        if (recommendations.length <= 1) {
            return recommendations[0]
        }
        return (
            <>
                <h4>Recommendations:</h4>
                <ul>
                    {recommendations.map((recommendation, index) => (
                        <li key={index}>{recommendation}</li>
                    ))}
                </ul>
            </>
        )
    }

    private renderResult = (result: GQL.GenericSearchResultInterface | GQL.IFileMatch): JSX.Element => {
        switch (result.__typename) {
            case 'FileMatch':
                return (
                    <FileMatch
                        location={this.props.location}
                        eventLogger={eventLogger}
                        icon={result.lineMatches && result.lineMatches.length > 0 ? SourceRepositoryIcon : FileIcon}
                        result={result}
                        onSelect={this.logEvent}
                        expanded={false}
                        showAllMatches={false}
                        isLightTheme={this.props.isLightTheme}
                        allExpanded={this.props.allExpanded}
                        fetchHighlightedFileLineRanges={this.props.fetchHighlightedFileLineRanges}
                        repoDisplayName={this.state.fileMatchRepoDisplayNames.get(result.repository.name)}
                        settingsCascade={this.props.settingsCascade}
                    />
                )
        }
        return <SearchResult result={result} isLightTheme={this.props.isLightTheme} history={this.props.history} />
    }

    private itemKey = (item: GQL.GenericSearchResultInterface | GQL.IFileMatch): string => {
        if (item.__typename === 'FileMatch') {
            return `file:${item.file.url}`
        }
        return item.url
    }

    /** onBottomHit increments the amount of results to be shown when we have scrolled to the bottom of the list. */
    private onBottomHit = (limit: number) => (): void =>
        this.setState(({ resultsShown }) => ({
            resultsShown: Math.min(limit, resultsShown + 10),
        }))

    /**
     * getCheckpoint gets the location from the hash in the URL. It is used to scroll to the result on page load of the given URL.
     */
    private getCheckpoint(): number {
        const checkpoint = parseInt(this.props.location.hash.slice(1), 10) || 0

        // If checkpoint is `0`, remove it.
        if (checkpoint === 0) {
            this.setCheckpoint(0) // `setCheckpoint` removes the hash when it is 0
        }

        return checkpoint
    }

    /** setCheckpoint sets the hash in the URL. It will be used to scroll to the result on page load of the given URL. */
    private setCheckpoint = (checkpoint: number): void => {
        if (!isSearchResults(this.props.resultsOrError) || this.props.resultsOrError.limitHit) {
            return
        }

        const { hash, ...location } = this.props.location

        let newHash = ''
        if (checkpoint > 0) {
            newHash = `#${checkpoint}`
        }

        this.props.history.replace({
            ...location,
            hash: newHash,
        })
    }

    private logEvent = (): void => eventLogger.log('SearchResultClicked')
}
