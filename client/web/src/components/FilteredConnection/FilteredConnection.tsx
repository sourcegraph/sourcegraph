import * as React from 'react'

import type * as H from 'history'
import { isEqual, uniq } from 'lodash'
import { useLocation, useNavigate, type NavigateFunction } from 'react-router-dom'
import { combineLatest, merge, of, Subject, Subscription, type Observable } from 'rxjs'
import {
    catchError,
    debounceTime,
    delay,
    distinctUntilChanged,
    filter,
    map,
    scan,
    share,
    skip,
    startWith,
    switchMap,
    takeUntil,
    tap,
} from 'rxjs/operators'

import { asError, isErrorLike, logger, type ErrorLike } from '@sourcegraph/common'

import {
    ConnectionNodes,
    type ConnectionNodesDisplayProps,
    type ConnectionNodesState,
    type ConnectionProps,
} from './ConnectionNodes'
import type { Connection, ConnectionQueryArguments } from './ConnectionType'
import { QUERY_KEY } from './constants'
import type { FilteredConnectionFilter, FilteredConnectionFilterValue } from './FilterControl'
import { ConnectionContainer, ConnectionError, ConnectionForm, ConnectionLoading } from './ui'
import type { ConnectionFormProps } from './ui/ConnectionForm'
import { getFilterFromURL, getUrlQuery, hasID, parseQueryInt } from './utils'

/**
 * Fields that belong in FilteredConnectionProps and that don't depend on the type parameters. These are the fields
 * that are most likely to be needed by callers, and it's simpler for them if they are in a parameter-less type.
 */
interface FilteredConnectionDisplayProps extends ConnectionNodesDisplayProps, ConnectionFormProps {
    navigate: NavigateFunction

    location: H.Location

    /** CSS class name for the root element. */
    className?: string

    /** CSS class name for the loader element. */
    loaderClassName?: string

    /** Whether to display it more compactly. */
    compact?: boolean

    /** Whether to display centered summary. */
    withCenteredSummary?: boolean

    /**
     * An observable that upon emission causes the connection to refresh the data (by calling queryConnection).
     *
     * In most cases, it's simpler to use updateOnChange.
     */
    updates?: Observable<void>

    /**
     * Refresh the data when this value changes. It is typically constructed as a key from the query args.
     */
    updateOnChange?: string

    /** The number of items to fetch, by default. */
    defaultFirst?: number

    /** Hides filters and search when the list of nodes is empty  */
    hideControlsWhenEmpty?: boolean

    /** Whether we will use the URL query string to reflect the filter and pagination state or not. */
    useURLQuery?: boolean

    /**
     * A subject that will force update the filtered connection's current search query, as if typed
     * by the user.
     */
    querySubject?: Subject<string>

    /** A function that generates an aria label given a node display name. */
    ariaLabelFunction?: (displayName: string) => string

    /**
     * Sets the aria-live attribute for the container around the nodes. This will announce updates to
     * the list to screen reader users (e.g. reading out nodes after they have finished loading).
     */
    ariaLive?: 'polite' | 'off'

    /**
     * A component that wraps around everything after the connection form. This is useful
     * for adding additional padding/background to the list, errors, or loading indicators.
     */
    contentWrapperComponent?: React.ComponentType<{ children: React.ReactNode }>
}

/**
 * Props for the FilteredConnection component.
 * @template C The GraphQL connection type, such as GQL.IRepositoryConnection.
 * @template N The node type of the GraphQL connection, such as GQL.IRepository (if C is GQL.IRepositoryConnection)
 * @template NP Props passed to `nodeComponent` in addition to `{ node: N }`
 * @template HP Props passed to `headComponent` in addition to `{ nodes: N[]; totalCount?: number | null }`.
 */
interface FilteredConnectionProps<C extends Connection<N>, N, NP = {}, HP = {}>
    extends ConnectionProps<N, NP, HP>,
        FilteredConnectionDisplayProps {
    /** Called to fetch the connection data to populate this component. */
    queryConnection: (args: FilteredConnectionQueryArguments) => Observable<C>

    /** Called when the queryConnection Observable emits. */
    onUpdate?: (value: C | ErrorLike | undefined, query: string, activeValues: FilteredConnectionArgs) => void

    /**
     * Set to true when the GraphQL response is expected to emit an `PageInfo.endCursor` value when
     * there is a subsequent page of results. This will request the next page of results and append
     * them onto the existing list of results instead of requesting twice as many results and
     * replacing the existing results.
     */
    cursorPaging?: boolean
}

/**
 * The arguments for the Props.queryConnection function.
 */
export interface FilteredConnectionQueryArguments extends ConnectionQueryArguments {}

interface FilteredConnectionState<C extends Connection<N>, N> extends ConnectionNodesState {
    activeFilterValues: Record<string, FilteredConnectionFilterValue['value'] | null>

    /** The fetched connection data or an error (if an error occurred). */
    connectionOrError?: C | ErrorLike

    /** The `PageInfo.endCursor` value from the previous request. */
    after?: string

    /**
     * The number of results that were visible from previous requests. The initial request of
     * a result set will load `visible` items, then will request `first` items on each subsequent
     * request. This has the effect of loading the correct number of visible results when a URL
     * is copied during pagination. This value is only useful with cursor-based paging.
     */
    visible?: number
}

/**
 * @deprecated Prefer using lower-level connection components exported from `./ui/index.ts`
 *
 * Check out usage examples:
 * 1. https://sourcegraph.com/github.com/sourcegraph/sourcegraph@4794d2ff1669a83bb15aa4e2ee8c448e53eae754/-/blob/client/web/src/team/list/TeamListPage.tsx?L106-148
 * 2. https://sourcegraph.com/github.com/sourcegraph/sourcegraph@4794d2ff1669a83bb15aa4e2ee8c448e53eae754/-/blob/client/web/src/repo/commits/RepositoryCommitsPage.tsx?L230-269
 * 3. https://sourcegraph.com/github.com/sourcegraph/sourcegraph@4794d2ff1669a83bb15aa4e2ee8c448e53eae754/-/blob/client/web/src/site-admin/SiteAdminPackagesPage.tsx?L340-381
 *
 * ------------------------------------------
 *
 * Displays a collection of items with filtering and pagination. It is called
 * "connection" because it is intended for use with GraphQL, which calls it that
 * (see http://graphql.org/learn/pagination/).
 * @template N The node type of the GraphQL connection, such as `GQL.IRepository` (if `C` is `GQL.IRepositoryConnection`)
 * @template NP Props passed to `nodeComponent` in addition to `{ node: N }`
 * @template HP Props passed to `headComponent` in addition to `{ nodes: N[]; totalCount?: number | null }`.
 * @template C The GraphQL connection type, such as `GQL.IRepositoryConnection`.
 */
export function FilteredConnection<N, NP = {}, HP = {}, C extends Connection<N> = Connection<N>>(
    props: Omit<FilteredConnectionProps<C, N, NP, HP>, 'location' | 'navigate'>
): JSX.Element | null {
    const location = useLocation()
    const navigate = useNavigate()

    return <InnerFilteredConnection<N, NP, HP, C> {...props} location={location} navigate={navigate} />
}

class InnerFilteredConnection<N, NP = {}, HP = {}, C extends Connection<N> = Connection<N>> extends React.PureComponent<
    FilteredConnectionProps<C, N, NP, HP>,
    FilteredConnectionState<C, N>
> {
    public static defaultProps: Partial<FilteredConnectionProps<any, any>> = {
        defaultFirst: 20,
        useURLQuery: true,
    }

    private queryInputChanges = new Subject<string>()
    private activeFilterValuesChanges = new Subject<Record<string, FilteredConnectionFilterValue['value'] | null>>()
    private showMoreClicks = new Subject<void>()
    private componentUpdates = new Subject<FilteredConnectionProps<C, N, NP, HP>>()
    private subscriptions = new Subscription()

    private filterRef: HTMLInputElement | null = null

    constructor(props: FilteredConnectionProps<C, N, NP, HP>) {
        super(props)

        const searchParameters = new URLSearchParams(this.props.location.search)

        // Note: in the initial state, do not set `after` from the URL, as this doesn't
        // track the number of results on the previous page. This makes the count look
        // broken when coming to a page in the middle of a set of results.
        //
        // For example:
        //   (1) come to page with first = 20
        //   (2) set first and after cursor in URL
        //   (3) reload page; will skip 20 results but will display (first 20 of X)
        //
        // Instead, we use `ConnectionStateCommon.visible` to load the correct number of
        // visible results on the initial request.

        this.state = {
            loading: true,
            query: (!this.props.hideSearch && this.props.useURLQuery && searchParameters.get(QUERY_KEY)) || '',
            activeFilterValues:
                (this.props.useURLQuery && getFilterFromURL(searchParameters, this.props.filters)) || {},
            first: (this.props.useURLQuery && parseQueryInt(searchParameters, 'first')) || this.props.defaultFirst!,
            visible: (this.props.useURLQuery && parseQueryInt(searchParameters, 'visible')) || 0,
        }
    }

    public componentDidMount(): void {
        const activeFilterValuesChanges = this.activeFilterValuesChanges.pipe(startWith(this.state.activeFilterValues))

        const queryChanges = (
            this.props.querySubject ? merge(this.queryInputChanges, this.props.querySubject) : this.queryInputChanges
        ).pipe(
            distinctUntilChanged(),
            tap(query => !this.props.hideSearch && this.setState({ query })),
            debounceTime(200),
            startWith(this.state.query)
        )

        /**
         * Emits `{ forceRefresh: false }` when loading a subsequent page (keeping the existing result set),
         * and emits `{ forceRefresh: true }` on all other refresh conditions (clearing the existing result set).
         */
        const refreshRequests = new Subject<{ forceRefresh: boolean }>()

        this.subscriptions.add(
            activeFilterValuesChanges
                .pipe(
                    tap(values => {
                        if (this.props.filters === undefined || this.props.onFilterSelect === undefined) {
                            return
                        }
                        for (const filter of this.props.filters) {
                            if (this.props.onFilterSelect) {
                                const value = values[filter.id]
                                if (value === undefined) {
                                    continue
                                }
                                this.props.onFilterSelect(filter, value)
                            }
                        }
                    })
                )
                .subscribe()
        )

        this.subscriptions.add(
            // Use this.activeFilterChanges not activeFilterChanges so that it doesn't trigger on the initial mount
            // (it doesn't need to).
            this.activeFilterValuesChanges.subscribe(values => {
                this.setState({ activeFilterValues: values })
            })
        )

        this.subscriptions.add(
            combineLatest([
                queryChanges,
                activeFilterValuesChanges,
                refreshRequests.pipe(startWith<{ forceRefresh: boolean }>({ forceRefresh: false })),
            ])
                .pipe(
                    // Track whether the query or the active order or filter changed
                    scan<
                        [
                            string,
                            Record<string, FilteredConnectionFilterValue['value'] | null> | undefined,
                            { forceRefresh: boolean }
                        ],
                        {
                            query: string
                            filterValues: Record<string, FilteredConnectionFilterValue['value'] | null> | undefined
                            shouldRefresh: boolean
                            queryCount: number
                        }
                    >(
                        (
                            { query, filterValues, queryCount },
                            [currentQuery, currentFilterValues, { forceRefresh }]
                        ) => ({
                            query: currentQuery,
                            filterValues: currentFilterValues,
                            shouldRefresh:
                                forceRefresh || query !== currentQuery || filterValues !== currentFilterValues,
                            queryCount: queryCount + 1,
                        }),
                        {
                            query: this.state.query,
                            filterValues: this.state.activeFilterValues,
                            shouldRefresh: false,
                            queryCount: 0,
                        }
                    ),
                    switchMap(({ query, filterValues, shouldRefresh, queryCount }) => {
                        const result = this.props
                            .queryConnection({
                                // If this is our first query and we were supplied a value for `visible`,
                                // load that many results. If we weren't given such a value or this is a
                                // subsequent request, only ask for one page of results.
                                first: (queryCount === 1 && this.state.visible) || this.state.first,
                                after: shouldRefresh ? undefined : this.state.after,
                                query,
                                ...(this.props.filters && filterValues
                                    ? this.buildArgs(this.props.filters, filterValues)
                                    : {}),
                            })
                            .pipe(
                                catchError(error => [asError(error)]),
                                map(
                                    (connectionOrError): PartialStateUpdate => ({
                                        connectionOrError,
                                        connectionQuery: query,
                                        loading: false,
                                    })
                                ),
                                share()
                            )

                        return (
                            shouldRefresh
                                ? merge(
                                      result,
                                      of({
                                          connectionOrError: undefined,
                                          loading: true,
                                      }).pipe(delay(250), takeUntil(result))
                                  )
                                : result
                        ).pipe(map(stateUpdate => ({ shouldRefresh, ...stateUpdate })))
                    }),
                    scan<PartialStateUpdate & { shouldRefresh: boolean }, PartialStateUpdate & { previousPage: N[] }>(
                        ({ previousPage }, { shouldRefresh, connectionOrError, ...rest }) => {
                            // Set temp variable in case we update its nodes. We cannot directly update connectionOrError.nodes as they are read-only props.
                            let temporaryConnection: C | ErrorLike | undefined = connectionOrError
                            let nodes: N[] = previousPage
                            let after: string | undefined

                            if (this.props.cursorPaging && connectionOrError && !isErrorLike(connectionOrError)) {
                                nodes = !shouldRefresh
                                    ? [...previousPage, ...connectionOrError.nodes]
                                    : connectionOrError.nodes
                                // Deduplicate any elements that occur between pages. This can happen as results are added during pagination.
                                nodes = [
                                    ...new Map(
                                        nodes.map((node, index) => [hasID(node) ? node.id : index, node])
                                    ).values(),
                                ]

                                temporaryConnection = { ...connectionOrError, nodes }

                                const pageInfo = temporaryConnection.pageInfo
                                after = pageInfo?.endCursor || undefined
                            }

                            return {
                                connectionOrError: temporaryConnection,
                                previousPage: nodes,
                                after,
                                ...rest,
                            }
                        },
                        {
                            previousPage: [],
                            after: undefined,
                            connectionOrError: undefined,
                            connectionQuery: undefined,
                            loading: true,
                        }
                    )
                )
                .subscribe(
                    ({ connectionOrError, previousPage, ...rest }) => {
                        if (this.props.useURLQuery) {
                            const { location, navigate } = this.props
                            const searchFragment = this.urlQuery({ visibleResultCount: previousPage.length })
                            const searchFragmentParams = new URLSearchParams(searchFragment)
                            searchFragmentParams.sort()

                            const oldParams = new URLSearchParams(location.search)
                            oldParams.sort()

                            if (!isEqual(Array.from(searchFragmentParams), Array.from(oldParams))) {
                                navigate(
                                    {
                                        search: searchFragment,
                                        hash: location.hash,
                                    },
                                    {
                                        replace: true,
                                        // Do not throw away flash messages
                                        state: location.state,
                                    }
                                )
                            }
                        }
                        if (this.props.onUpdate && this.props.filters) {
                            this.props.onUpdate(
                                connectionOrError,
                                this.state.query,
                                this.buildArgs(this.props.filters, this.state.activeFilterValues)
                            )
                        }
                        this.setState({ connectionOrError, ...rest })
                    },
                    error => logger.error(error)
                )
        )

        type PartialStateUpdate = Pick<
            FilteredConnectionState<C, N>,
            'connectionOrError' | 'connectionQuery' | 'loading' | 'after'
        >
        this.subscriptions.add(
            this.showMoreClicks
                .pipe(
                    map(() =>
                        // If we're doing cursor paging, we rely on the `endCursor` from the previous
                        // response's `PageInfo` object to make our next request. Otherwise, we'll
                        // fallback to our legacy 'request-more' paging technique and not supply a
                        // cursor to the subsequent request.
                        ({ first: this.props.cursorPaging ? this.state.first : this.state.first * 2 })
                    )
                )
                .subscribe(({ first }) =>
                    this.setState({ first, loading: true }, () => refreshRequests.next({ forceRefresh: false }))
                )
        )

        if (this.props.updates) {
            this.subscriptions.add(
                this.props.updates.subscribe(() => {
                    this.setState({ loading: true }, () => refreshRequests.next({ forceRefresh: true }))
                })
            )
        }

        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    distinctUntilChanged((a, b) => a.updateOnChange === b.updateOnChange),
                    filter(({ updateOnChange }) => updateOnChange !== undefined),
                    // Skip the very first emission as the FilteredConnection already fetches on component creation.
                    // Otherwise, 2 requests would be triggered immediately.
                    skip(1)
                )
                .subscribe(() => {
                    this.setState({ loading: true, connectionOrError: undefined }, () =>
                        refreshRequests.next({ forceRefresh: true })
                    )
                })
        )

        // Reload collection when the query callback changes.
        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    map(({ queryConnection }) => queryConnection),
                    distinctUntilChanged(),
                    skip(1), // prevent from triggering on initial mount
                    tap(() => {
                        if (this.props.autoFocus) {
                            this.focusFilter()
                        }
                    })
                )
                .subscribe(() =>
                    this.setState({ loading: true, connectionOrError: undefined }, () =>
                        refreshRequests.next({ forceRefresh: true })
                    )
                )
        )

        // React to location changes.
        this.subscriptions.add(
            this.componentUpdates
                .pipe(
                    map(({ location }) => location.search),
                    distinctUntilChanged(),
                    map(searchParams => new URLSearchParams(searchParams)),
                    map(searchParams => getFilterFromURL(searchParams, this.props.filters)),
                    // Map is compared by reference, so by default distinctUntilChanged
                    // will always return false for two maps. isEqual compares
                    // them by value.
                    distinctUntilChanged((prev, next) => isEqual(prev, next)),
                    skip(1)
                )
                .subscribe(newFilterValues => {
                    if (this.props.useURLQuery) {
                        this.activeFilterValuesChanges.next(newFilterValues)
                    }
                })
        )

        this.componentUpdates.next(this.props)
    }

    private urlQuery({
        first,
        query,
        filterValues,
        visibleResultCount,
    }: {
        first?: number
        query?: string
        filterValues?: Record<string, FilteredConnectionFilterValue['value'] | null>
        visibleResultCount?: number
    }): string {
        if (!first) {
            first = this.state.first
        }
        if (!query) {
            query = this.state.query
        }
        if (!filterValues) {
            filterValues = this.state.activeFilterValues
        }

        return getUrlQuery({
            query,
            first: {
                actual: first,
                // Always set through `defaultProps`
                default: this.props.defaultFirst!,
            },
            filterValues,
            visibleResultCount,
            search: this.props.location.search,
            filters: this.props.filters,
        })
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const errors: string[] = []
        if (isErrorLike(this.state.connectionOrError)) {
            errors.push(...uniq(this.state.connectionOrError.message.split('\n')))
        }
        if (
            this.state.connectionOrError &&
            !isErrorLike(this.state.connectionOrError) &&
            this.state.connectionOrError.error
        ) {
            errors.push(this.state.connectionOrError.error)
        }

        const inputPlaceholder = this.props.inputPlaceholder || `Search ${this.props.pluralNoun}...`

        const ContentWrapperComponent = this.props.contentWrapperComponent || React.Fragment

        return (
            <ConnectionContainer
                compact={this.props.compact}
                className={this.props.className}
                ariaLive={this.props.ariaLive}
            >
                {(!this.props.hideSearch || this.props.filters) && (
                    <ConnectionForm
                        ref={this.setFilterRef}
                        hideSearch={this.props.hideSearch}
                        showSearchFirst={this.props.showSearchFirst}
                        inputClassName={this.props.inputClassName}
                        inputPlaceholder={inputPlaceholder}
                        inputAriaLabel={this.props.inputAriaLabel || inputPlaceholder}
                        inputValue={this.state.query}
                        onInputChange={this.onChange}
                        autoFocus={this.props.autoFocus}
                        filters={this.props.filters}
                        onFilterSelect={this.onDidSelectFilterValue}
                        filterValues={this.state.activeFilterValues}
                        compact={this.props.compact}
                        formClassName={this.props.formClassName}
                    />
                )}

                <ContentWrapperComponent>
                    {errors.length > 0 && <ConnectionError errors={errors} compact={this.props.compact} />}

                    {this.state.connectionOrError && !isErrorLike(this.state.connectionOrError) && (
                        <ConnectionNodes
                            connection={this.state.connectionOrError}
                            loading={this.state.loading}
                            connectionQuery={this.state.connectionQuery}
                            first={this.state.first}
                            query={this.state.query}
                            noun={this.props.noun}
                            pluralNoun={this.props.pluralNoun}
                            listComponent={this.props.listComponent}
                            listClassName={this.props.listClassName}
                            summaryClassName={this.props.summaryClassName}
                            headComponent={this.props.headComponent}
                            headComponentProps={this.props.headComponentProps}
                            footComponent={this.props.footComponent}
                            showMoreClassName={this.props.showMoreClassName}
                            nodeComponent={this.props.nodeComponent}
                            nodeComponentProps={this.props.nodeComponentProps}
                            noShowMore={this.props.noShowMore}
                            noSummaryIfAllNodesVisible={this.props.noSummaryIfAllNodesVisible}
                            onShowMore={this.onClickShowMore}
                            emptyElement={this.props.emptyElement}
                            totalCountSummaryComponent={this.props.totalCountSummaryComponent}
                            withCenteredSummary={this.props.withCenteredSummary}
                            ariaLabelFunction={this.props.ariaLabelFunction}
                        />
                    )}

                    {this.state.loading && (
                        <ConnectionLoading compact={this.props.compact} className={this.props.loaderClassName} />
                    )}
                </ContentWrapperComponent>
            </ConnectionContainer>
        )
    }

    private setFilterRef = (element: HTMLInputElement | null): void => {
        this.filterRef = element
        if (element && this.props.autoFocus) {
            // TODO(sqs): The 30 msec delay is needed, or else the input is not
            // reliably focused. Find out why.
            setTimeout(() => element.focus(), 30)
        }
    }

    private focusFilter = (): void => {
        if (this.filterRef) {
            this.filterRef.focus()
        }
    }

    private onChange: React.ChangeEventHandler<HTMLInputElement> = event => {
        this.props.onInputChange?.(event)
        this.queryInputChanges.next(event.currentTarget.value)
    }

    private onDidSelectFilterValue = (
        filter: FilteredConnectionFilter,
        value: FilteredConnectionFilterValue['value'] | null
    ): void => {
        if (this.props.filters === undefined) {
            return
        }
        this.activeFilterValuesChanges.next({ ...this.state.activeFilterValues, [filter.id]: value })
    }

    private onClickShowMore = (): void => {
        this.showMoreClicks.next()
    }

    private buildArgs = buildFilterArgs
}

export const buildFilterArgs = (
    filters: FilteredConnectionFilter[],
    filterValues: Record<string, FilteredConnectionFilterValue['value'] | null>
): FilteredConnectionArgs => {
    let args: FilteredConnectionArgs = {}
    for (const [filterID, value] of Object.entries(filterValues)) {
        if (value === undefined) {
            continue
        }
        const filter = filters.find(f => f.id === filterID)
        if (filter) {
            const valueArgs = filter.values.find(v => v.value === value)?.args
            args = { ...args, ...valueArgs }
        }
    }
    return args
}

/**
 * Resets the `FilteredConnection` URL query string parameters to the defaults
 * @param parameters the current URL search parameters
 */
export const resetFilteredConnectionURLQuery = (parameters: URLSearchParams): void => {
    parameters.delete('visible')
    parameters.delete('first')
    parameters.delete('after')
}

export interface FilteredConnectionArgs {
    [name: string]: string | number | boolean
}
