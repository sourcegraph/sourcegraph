import { type FC, useCallback, useMemo, useRef, useState } from 'react'

import { mdiChevronDoubleDown, mdiChevronDoubleUp, mdiOpenInNew, mdiThumbDown, mdiThumbUp } from '@mdi/js'
import classNames from 'classnames'
import { useLocation, useNavigate } from 'react-router-dom'

import { Toggle } from '@sourcegraph/branded/src/components/Toggle'
import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import type { CaseSensitivityProps, SearchPatternTypeProps } from '@sourcegraph/shared/src/search'
import { FilterKind, findFilter } from '@sourcegraph/shared/src/search/query/query'
import type { AggregateStreamingSearchResults, StreamSearchOptions } from '@sourcegraph/shared/src/search/stream'
import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Alert,
    Button,
    createRectangle,
    FeedbackPrompt,
    H3,
    Icon,
    Link,
    Popover,
    PopoverContent,
    PopoverTrigger,
    Position,
    ProductStatusBadge,
    Text,
    useSessionStorage,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../auth'
import {
    canWriteBatchChanges,
    NO_ACCESS_BATCH_CHANGES_WRITE,
    NO_ACCESS_SOURCEGRAPH_COM,
} from '../../../../batches/utils'
import { useHandleSubmitFeedback } from '../../../../hooks'
import { SavedSearchModal } from '../../../../savedSearches/SavedSearchModal'
import { eventLogger } from '../../../../tracking/eventLogger'
import { DOTCOM_URL } from '../../../../tracking/util'
import { SearchResultsCsvExportModal } from '../../export/SearchResultsCsvExportModal'
import { AggregationUIMode, useAggregationUIMode } from '../aggregation'
import { SearchActionsMenu } from '../SearchActionsMenu'

import {
    type CreateAction,
    getBatchChangeCreateAction,
    getCodeMonitoringCreateAction,
    getInsightsCreateAction,
    getSearchContextCreateAction,
} from './createActions'
import { NewStarsIcon } from './NewStarsIcon'

import styles from './SearchResultsInfoBar.module.scss'

// Adds padding to the popover content to add some space between the trigger
// button and the content
const KEYWORD_SEARCH_POPOVER_PADDING = createRectangle(0, 0, 0, 2)

export interface SearchResultsInfoBarProps
    extends TelemetryProps,
        SearchPatternTypeProps,
        Pick<CaseSensitivityProps, 'caseSensitive'> {
    /** The currently authenticated user or null */
    authenticatedUser: Pick<
        AuthenticatedUser,
        'id' | 'displayName' | 'emails' | 'permissions' | 'organizations' | 'username'
    > | null

    enableCodeInsights?: boolean
    enableCodeMonitoring: boolean

    /** The search query and results */
    query?: string
    options: StreamSearchOptions
    results?: AggregateStreamingSearchResults

    batchChangesEnabled?: boolean
    /** Whether running batch changes server-side is enabled */
    batchChangesExecutionEnabled?: boolean

    // Expand all feature
    allExpanded: boolean
    onExpandAllResultsToggle: () => void

    className?: string

    stats: JSX.Element

    onShowMobileFiltersChanged?: (show: boolean) => void

    sidebarCollapsed: boolean
    setSidebarCollapsed: (collapsed: boolean) => void

    isSourcegraphDotCom: boolean
    patternType: SearchPatternType
    sourcegraphURL: string

    showKeywordSearchToggle: boolean
    onTogglePatternType: (patternType: SearchPatternType) => void
}

/**
 * The info bar shown over the search results list that displays metadata
 * and a few actions like expand all and save query
 */
export const SearchResultsInfoBar: FC<SearchResultsInfoBarProps> = props => {
    const {
        query,
        patternType,
        authenticatedUser,
        results,
        options,
        sourcegraphURL,
        onTogglePatternType,
        telemetryService,
    } = props

    const popoverRef = useRef<HTMLDivElement>(null)
    const navigate = useNavigate()
    const newFiltersEnabled = useExperimentalFeatures(features => features.newSearchResultFiltersPanel)

    const [aggregationUIMode, setAggregationUIMode] = useAggregationUIMode()
    const [showSavedSearchModal, setShowSavedSearchModal] = useState(false)
    const [showCsvExportModal, setShowCsvExportModal] = useState(false)

    const globalTypeFilter = useMemo(
        () => (props.query ? findFilter(props.query, 'type', FilterKind.Global)?.value?.value : undefined),
        [props.query]
    )

    const canCreateMonitorFromQuery = useMemo(
        () => globalTypeFilter === 'diff' || globalTypeFilter === 'commit',
        [globalTypeFilter]
    )

    const canCreateBatchChanges: boolean | string = useMemo(() => {
        if (globalTypeFilter === 'diff' || globalTypeFilter === 'commit') {
            return 'Batch changes cannot be created from searches with type:diff or type:commit'
        }
        if (props.isSourcegraphDotCom) {
            return NO_ACCESS_SOURCEGRAPH_COM
        }
        if (!props.batchChangesEnabled || !props.batchChangesExecutionEnabled) {
            return false
        }
        if (!canWriteBatchChanges(props.authenticatedUser)) {
            return NO_ACCESS_BATCH_CHANGES_WRITE
        }
        return true
    }, [
        globalTypeFilter,
        props.isSourcegraphDotCom,
        props.batchChangesEnabled,
        props.batchChangesExecutionEnabled,
        props.authenticatedUser,
    ])

    // When adding a new create action check and update the $collapse-breakpoint in CreateActions.module.scss.
    // The collapse breakpoint indicates at which window size we hide the buttons and show the collapsed menu instead.
    const createActions = useMemo(
        () =>
            [
                getBatchChangeCreateAction(props.query, props.patternType, canCreateBatchChanges),
                getSearchContextCreateAction(props.query, props.authenticatedUser),
                getInsightsCreateAction(props.query, props.patternType, window.context?.codeInsightsEnabled),
            ].filter((button): button is CreateAction => button !== null),
        [props.authenticatedUser, props.patternType, props.query, canCreateBatchChanges]
    )

    // The create code monitor action is separated from the rest of the actions, because we use the
    // <ExperimentalActionButton /> component instead of a regular (button) link, and it has a tour attached.
    const createCodeMonitorAction = useMemo(
        () => getCodeMonitoringCreateAction(props.query, props.patternType, props.enableCodeMonitoring),
        [props.enableCodeMonitoring, props.patternType, props.query]
    )

    // Show/hide mobile filters menu
    const [showMobileFilters, setShowMobileFilters] = useState(false)
    const onShowMobileFiltersClicked = (): void => {
        const newShowFilters = !showMobileFilters
        setShowMobileFilters(newShowFilters)
        props.onShowMobileFiltersChanged?.(newShowFilters)
    }

    const location = useLocation()
    const isPrivateInstance = window.location.host !== DOTCOM_URL.href
    const refFromCodySearch = new URLSearchParams(location.search).get('ref') === 'cody-search'
    const [codySearchInputString] = useSessionStorage<string>('cody-search-input', '')
    const codySearchInput: { input?: string; translatedQuery?: string } = JSON.parse(codySearchInputString || '{}')
    const [codyFeedback, setCodyFeedback] = useState<null | boolean>(null)

    const collectCodyFeedback = (positive: boolean): void => {
        if (codyFeedback !== null) {
            return
        }

        eventLogger.log(
            'web:codySearch:feedbackSubmitted',
            !isPrivateInstance ? { ...codySearchInput, positive } : null,
            !isPrivateInstance ? { ...codySearchInput, positive } : null
        )
        setCodyFeedback(positive)
    }

    const onSaveQueryModalClose = useCallback(() => {
        setShowSavedSearchModal(false)
        telemetryService.log('SavedQueriesToggleCreating', { queries: { creating: false } })
    }, [telemetryService])

    const handleKeywordSearchToggle = useCallback(() => {
        telemetryService.log('ToggleKeywordPatternType', { currentStatus: patternType === SearchPatternType.keyword })
        onTogglePatternType(patternType)
    }, [onTogglePatternType, patternType, telemetryService])

    const [feedbackModalOpen, setFeedbackModalOpen] = useState(false)

    const { handleSubmitFeedback } = useHandleSubmitFeedback({
        routeMatch: location.pathname,
        textPrefix: '[Source: keyword search] ',
    })

    const feedbackPromptInitialValue =
        props.isSourcegraphDotCom && query !== undefined ? '<Feedback here>\n\nQuery: ' + query : undefined

    return (
        <aside
            role="region"
            aria-label="Search results information"
            className={classNames(props.className, styles.searchResultsInfoBar)}
            data-testid="results-info-bar"
        >
            {feedbackModalOpen ? (
                <FeedbackPrompt
                    onSubmit={handleSubmitFeedback}
                    modal={true}
                    openByDefault={true}
                    authenticatedUser={
                        props.authenticatedUser
                            ? {
                                  username: props.authenticatedUser.username || '',
                                  email: props.authenticatedUser.emails.find(email => email.isPrimary)?.email || '',
                              }
                            : null
                    }
                    onClose={() => setFeedbackModalOpen(false)}
                    initialValue={feedbackPromptInitialValue}
                />
            ) : null}
            {refFromCodySearch && codySearchInput.input && codySearchInput.translatedQuery === props.query ? (
                <Alert variant="info" className={styles.codyFeedbackAlert}>
                    Sourcegraph converted "<strong>{codySearchInput.input}</strong>" to "
                    <strong>{codySearchInput.translatedQuery}</strong>".{' '}
                    <small>
                        <Link target="blank" to="/help/code_search/reference/queries">
                            Complete query reference{' '}
                            <Icon role="img" aria-label="Open in a new tab" svgPath={mdiOpenInNew} />
                        </Link>
                    </small>
                    {codyFeedback === null ? (
                        <>
                            <Text className="my-2">Was this helpful?</Text>
                            <div>
                                <Button
                                    variant="secondary"
                                    outline={true}
                                    size="sm"
                                    onClick={() => collectCodyFeedback(true)}
                                >
                                    <Icon aria-hidden={true} className="mr-1" svgPath={mdiThumbUp} />
                                    Yes
                                </Button>
                                <Button
                                    className="ml-2"
                                    variant="secondary"
                                    outline={true}
                                    size="sm"
                                    onClick={() => collectCodyFeedback(false)}
                                >
                                    <Icon aria-hidden={true} className="mr-1" svgPath={mdiThumbDown} />
                                    No
                                </Button>
                            </div>
                        </>
                    ) : (
                        <Text className="my-2">
                            <strong>Thanks for your feedback!</strong>
                        </Text>
                    )}
                </Alert>
            ) : null}
            <div className={styles.row}>
                {props.stats}

                <div className={styles.expander} />

                {props.showKeywordSearchToggle && (
                    <div ref={popoverRef} className={styles.toggleWrapper}>
                        <span className="mr-1">
                            <NewStarsIcon aria-hidden={true} />
                        </span>

                        <Popover>
                            <PopoverTrigger
                                as={Button}
                                type="button"
                                className="p-0"
                                data-testid="dropdown-toggle"
                                data-test-tooltip-content="Learn more about the new search language."
                            >
                                Keyword search
                            </PopoverTrigger>
                            <PopoverContent
                                target={popoverRef.current}
                                position={Position.bottomEnd}
                                className={styles.popoverContent}
                                targetPadding={KEYWORD_SEARCH_POPOVER_PADDING}
                            >
                                <div>
                                    <H3 className="d-flex align-items-center">
                                        About keyword search
                                        <ProductStatusBadge status="beta" className="ml-2" />
                                    </H3>
                                    <Text>
                                        The new search behavior ANDs terms together instead of searching literally by
                                        default. To search literally, wrap the query in quotes.
                                    </Text>
                                    <Text>
                                        <Link
                                            to="https://sourcegraph.com/docs/code-search/queries#keyword-search-default"
                                            target="_blank"
                                            rel="noopener noreferrer"
                                        >
                                            Read the docs
                                        </Link>{' '}
                                        to learn more.
                                    </Text>
                                    <Button
                                        className={styles.feedbackButton}
                                        onClick={() => setFeedbackModalOpen(true)}
                                    >
                                        Send feedback
                                    </Button>
                                </div>
                            </PopoverContent>
                        </Popover>

                        <Toggle
                            value={props.patternType === SearchPatternType.keyword}
                            onToggle={handleKeywordSearchToggle}
                            title="Enable search language update"
                            className="mr-2"
                        />
                    </div>
                )}

                <ul className="nav align-items-center">
                    <SearchActionsMenu
                        authenticatedUser={props.authenticatedUser}
                        createActions={createActions}
                        createCodeMonitorAction={createCodeMonitorAction}
                        canCreateMonitor={canCreateMonitorFromQuery}
                        results={props.results}
                        allExpanded={props.allExpanded}
                        onExpandAllResultsToggle={props.onExpandAllResultsToggle}
                        onSaveQueryClick={() => setShowSavedSearchModal(true)}
                        onExportCsvClick={() => setShowCsvExportModal(true)}
                    />
                </ul>

                {!newFiltersEnabled && (
                    <Button
                        className={classNames(
                            'd-flex align-items-center d-lg-none',
                            styles.filtersButton,
                            showMobileFilters && 'active'
                        )}
                        aria-pressed={showMobileFilters}
                        onClick={onShowMobileFiltersClicked}
                        outline={true}
                        variant="secondary"
                        size="sm"
                        aria-label={`${showMobileFilters ? 'Hide' : 'Show'} filters`}
                    >
                        Filters
                        <Icon
                            aria-hidden={true}
                            className="ml-2"
                            svgPath={showMobileFilters ? mdiChevronDoubleUp : mdiChevronDoubleDown}
                        />
                    </Button>
                )}

                {!newFiltersEnabled && props.sidebarCollapsed && (
                    <Button
                        className={classNames('align-items-center d-none d-lg-flex', styles.filtersButton)}
                        onClick={() => props.setSidebarCollapsed(false)}
                        outline={true}
                        variant="secondary"
                        size="sm"
                        aria-label="Show filters sidebar"
                    >
                        Filters
                        <Icon aria-hidden={true} className="ml-2" svgPath={mdiChevronDoubleDown} />
                    </Button>
                )}
                {newFiltersEnabled && (
                    <Button
                        size="sm"
                        variant="secondary"
                        outline={true}
                        aria-label="Show aggregation results"
                        className="align-items-center d-lg-flex"
                        onClick={() =>
                            setAggregationUIMode(
                                aggregationUIMode === AggregationUIMode.SearchPage
                                    ? AggregationUIMode.Sidebar
                                    : AggregationUIMode.SearchPage
                            )
                        }
                    >
                        {aggregationUIMode === AggregationUIMode.SearchPage ? 'Hide' : 'Show'} aggregation results
                    </Button>
                )}
            </div>

            {showSavedSearchModal && (
                <SavedSearchModal
                    navigate={navigate}
                    patternType={patternType}
                    query={query}
                    authenticatedUser={authenticatedUser}
                    onDidCancel={onSaveQueryModalClose}
                />
            )}
            {showCsvExportModal && (
                <SearchResultsCsvExportModal
                    query={query}
                    options={options}
                    results={results}
                    sourcegraphURL={sourcegraphURL}
                    telemetryService={telemetryService}
                    onClose={() => setShowCsvExportModal(false)}
                />
            )}
        </aside>
    )
}
