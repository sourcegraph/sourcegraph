import {
    type FC,
    type HTMLAttributes,
    type PropsWithChildren,
    Suspense,
    useCallback,
    useEffect,
    useLayoutEffect,
    useMemo,
    useRef,
} from 'react'

import { mdiClose } from '@mdi/js'
import classNames from 'classnames'
import type { Observable } from 'rxjs'

import { StreamingProgress, StreamingSearchResultsList, useSearchResultState } from '@sourcegraph/branded'
import type { FetchFileParameters } from '@sourcegraph/shared/src/backend/file'
import type { FilePrefetcher } from '@sourcegraph/shared/src/components/PrefetchableFile'
import { HighlightResponseFormat, type SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type {
    QueryState,
    QueryStateUpdate,
    QueryUpdate,
    SearchMode,
    SearchPatternTypeMutationProps,
    SearchPatternTypeProps,
} from '@sourcegraph/shared/src/search'
import {
    type AggregateStreamingSearchResults,
    type ContentMatch,
    getFileMatchUrl,
    type PathMatch,
    type StreamSearchOptions,
} from '@sourcegraph/shared/src/search/stream'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE, type TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { Button, H2, H4, Icon, Link, Panel, useLocalStorage, useScrollManager } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../auth'
import { fetchBlob } from '../../../../repo/blob/backend'
import { isSearchJobsEnabled } from '../../../../search-jobs/utility'
import { buildSearchURLQueryFromQueryState } from '../../../../stores'
import { GettingStartedTour } from '../../../../tour/GettingStartedTour'
import { DidYouMean } from '../../../suggestion/DidYouMean'
import { SmartSearch } from '../../../suggestion/SmartSearch'
import { SearchFiltersSidebar } from '../../sidebar/SearchFiltersSidebar'
import { AggregationUIMode, SearchAggregationResult } from '../aggregation'
import { SearchFiltersPanel, SearchFiltersTabletButton } from '../filters-panel/SearchFiltersPanel'
import { SearchResultsInfoBar } from '../search-results-info-bar/SearchResultsInfoBar'
import { SearchAlert } from '../SearchAlert'
import { UnownedResultsAlert } from '../UnownedResultsAlert'
import { isSmartSearchAlert } from '../utils'

import { useIsNewSearchFiltersEnabled } from './use-new-search-filters'

import styles from './NewSearchContent.module.scss'

const LazySideBlob = lazyComponent(() => import('../../../../codeintel/SideBlob'), 'SideBlob')

/**
 * At the moment search result preview panel supports only
 * blob-like type of search results to preview.
 */
type SearchResultPreview = ContentMatch | PathMatch

interface NewSearchContentProps
    extends TelemetryProps,
        SettingsCascadeProps,
        PlatformContextProps,
        SearchPatternTypeProps,
        SearchPatternTypeMutationProps {
    submittedURLQuery: string
    queryState: QueryState
    liveQuery: string
    allExpanded: boolean
    searchMode: SearchMode
    trace: boolean
    searchContextsEnabled: boolean
    results: AggregateStreamingSearchResults | undefined
    showAggregationPanel: boolean
    selectedSearchContextSpec: string | undefined
    aggregationUIMode: AggregationUIMode
    caseSensitive: boolean
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    enableRepositoryMetadata: boolean
    options: StreamSearchOptions
    codeMonitoringEnabled: boolean
    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
    onNavbarQueryChange: (queryState: QueryStateUpdate) => void
    onSearchSubmit: (updates: QueryUpdate[], updatedSearchQuery?: string) => void
    onQuerySubmit: (newQuery: string, updatedQuerySearch: string) => void
    onExpandAllResultsToggle: () => void
    onSearchAgain: (additionalFilters: string[]) => void
    onDisableSmartSearch: () => void
    onTogglePatternType: (patternType: SearchPatternType) => void
    onLogSearchResultClick: (index: number, type: string, resultsLength: number) => void
}

export const NewSearchContent: FC<NewSearchContentProps> = props => {
    const {
        submittedURLQuery,
        liveQuery,
        queryState,
        allExpanded,
        trace,
        patternType,
        searchContextsEnabled,
        results,
        showAggregationPanel,
        selectedSearchContextSpec,
        aggregationUIMode,
        settingsCascade,
        telemetryService,
        fetchHighlightedFileLineRanges,
        caseSensitive,
        authenticatedUser,
        isSourcegraphDotCom,
        enableRepositoryMetadata,
        codeMonitoringEnabled,
        options,
        platformContext,
        onNavbarQueryChange,
        onSearchSubmit,
        onQuerySubmit,
        onExpandAllResultsToggle,
        onSearchAgain,
        onDisableSmartSearch,
        onTogglePatternType,
        onLogSearchResultClick,
    } = props

    const telemetryRecorder = platformContext.telemetryRecorder

    const submittedURLQueryRef = useRef(submittedURLQuery)
    const containerRef = useRef<HTMLDivElement>(null)
    const { previewBlob, clearPreview } = useSearchResultState()

    const newFiltersEnabled = useIsNewSearchFiltersEnabled()
    const [sidebarCollapsed, setSidebarCollapsed] = useLocalStorage('search.sidebar.collapsed', true)

    useScrollManager('SearchResultsContainer', containerRef)

    // Clean up hook, close the preview panel if search result page
    // have been closed/unmount
    useEffect(clearPreview, [clearPreview])

    // File preview clean up hook, close the preview panel every time when we
    // re-search / re-submit the query.
    useLayoutEffect(() => {
        if (submittedURLQuery !== submittedURLQueryRef.current) {
            submittedURLQueryRef.current = submittedURLQuery
            clearPreview()
        }
    }, [submittedURLQuery, clearPreview])

    const prefetchFile: FilePrefetcher = useCallback(
        params =>
            fetchBlob({
                ...params,
                format: HighlightResponseFormat.JSON_SCIP,
            }),
        []
    )

    const handleFilterPanelQueryChange = useCallback(
        (updatedQuery: string, updatedSearchURLQuery?: string): void => {
            onSearchSubmit([{ type: 'replaceQuery', value: updatedQuery }], updatedSearchURLQuery)
        },
        [onSearchSubmit]
    )

    const handleFilterPanelClose = useCallback(() => {
        clearPreview()
        telemetryService.log('SearchFilePreviewClose', {}, {})
        telemetryRecorder.recordEvent('search.filePreview', 'close')
    }, [telemetryService, clearPreview, telemetryRecorder])

    return (
        <div className={classNames(styles.root, { [styles.rootWithNewFilters]: newFiltersEnabled })}>
            {newFiltersEnabled && (
                <SearchFiltersPanel
                    query={submittedURLQuery}
                    filters={results?.filters}
                    withCountAllFilter={isSearchLimitHit(results)}
                    isFilterLoadingComplete={results?.state === 'complete'}
                    className={styles.newFilters}
                    onQueryChange={handleFilterPanelQueryChange}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                />
            )}

            {!newFiltersEnabled && !sidebarCollapsed && (
                <SearchFiltersSidebar
                    as={NewSearchSidebarWrapper}
                    liveQuery={liveQuery}
                    submittedURLQuery={submittedURLQuery}
                    patternType={patternType}
                    filters={results?.filters}
                    showAggregationPanel={showAggregationPanel}
                    selectedSearchContextSpec={selectedSearchContextSpec}
                    aggregationUIMode={aggregationUIMode}
                    settingsCascade={settingsCascade}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    caseSensitive={caseSensitive}
                    className={classNames(styles.filters)}
                    setSidebarCollapsed={setSidebarCollapsed}
                    onNavbarQueryChange={onNavbarQueryChange}
                    onSearchSubmit={onSearchSubmit}
                />
            )}

            <SearchResultsInfoBar
                query={submittedURLQuery}
                patternType={patternType}
                results={results}
                options={options}
                allExpanded={allExpanded}
                caseSensitive={caseSensitive}
                enableCodeMonitoring={codeMonitoringEnabled}
                sidebarCollapsed={!!sidebarCollapsed}
                setSidebarCollapsed={setSidebarCollapsed}
                authenticatedUser={authenticatedUser}
                sourcegraphURL={platformContext.sourcegraphURL}
                isSourcegraphDotCom={isSourcegraphDotCom}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                className={styles.infobar}
                onExpandAllResultsToggle={onExpandAllResultsToggle}
                onShowMobileFiltersChanged={setSidebarCollapsed}
                onTogglePatternType={onTogglePatternType}
                stats={
                    <>
                        <StreamingProgress
                            showTrace={trace}
                            query={`${submittedURLQuery} patterntype:${patternType}`}
                            progress={results?.progress || { durationMs: 0, matchCount: 0, skipped: [] }}
                            state={results?.state || 'loading'}
                            onSearchAgain={onSearchAgain}
                            isSearchJobsEnabled={isSearchJobsEnabled()}
                            telemetryService={props.telemetryService}
                            telemetryRecorder={telemetryRecorder}
                        />
                        {newFiltersEnabled && <SearchFiltersTabletButton />}
                    </>
                }
            />

            <div className={styles.content} ref={containerRef}>
                {aggregationUIMode === AggregationUIMode.SearchPage && (
                    <SearchAggregationResult
                        query={submittedURLQuery}
                        patternType={patternType}
                        caseSensitive={caseSensitive}
                        aria-label="Aggregation results panel"
                        onQuerySubmit={onQuerySubmit}
                        telemetryService={telemetryService}
                        telemetryRecorder={telemetryRecorder}
                        className="m-3"
                    />
                )}

                {aggregationUIMode !== AggregationUIMode.SearchPage && (
                    <div className={styles.contentMetaInfo}>
                        <DidYouMean
                            telemetryService={props.telemetryService}
                            telemetryRecorder={telemetryRecorder}
                            query={submittedURLQuery}
                            patternType={patternType}
                            caseSensitive={caseSensitive}
                            selectedSearchContextSpec={props.selectedSearchContextSpec}
                            className="m-2"
                        />

                        {results?.alert?.kind && isSmartSearchAlert(results.alert.kind) && (
                            <SmartSearch
                                alert={results?.alert}
                                onDisableSmartSearch={onDisableSmartSearch}
                                className="m-2"
                            />
                        )}

                        <GettingStartedTour.Info className="m-2" isSourcegraphDotCom={props.isSourcegraphDotCom} />

                        {results?.alert && (!results?.alert.kind || !isSmartSearchAlert(results.alert.kind)) && (
                            <div className={classNames(styles.alertArea, 'm-2')}>
                                {results?.alert?.kind === 'unowned-results' ? (
                                    <UnownedResultsAlert
                                        alertTitle={results.alert.title}
                                        alertDescription={results.alert.description}
                                        queryState={queryState}
                                        patternType={patternType}
                                        caseSensitive={caseSensitive}
                                        selectedSearchContextSpec={props.selectedSearchContextSpec}
                                    />
                                ) : (
                                    <SearchAlert
                                        alert={results.alert}
                                        caseSensitive={caseSensitive}
                                        patternType={patternType}
                                    />
                                )}
                            </div>
                        )}
                    </div>
                )}

                {aggregationUIMode !== AggregationUIMode.SearchPage && (
                    <StreamingSearchResultsList
                        telemetryService={telemetryService}
                        telemetryRecorder={telemetryRecorder}
                        platformContext={platformContext}
                        settingsCascade={settingsCascade}
                        searchContextsEnabled={searchContextsEnabled}
                        fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        enableRepositoryMetadata={enableRepositoryMetadata}
                        results={results}
                        allExpanded={allExpanded}
                        executedQuery={location.search}
                        prefetchFileEnabled={true}
                        prefetchFile={prefetchFile}
                        enableKeyboardNavigation={true}
                        showQueryExamplesOnNoResultsPage={true}
                        queryState={queryState}
                        buildSearchURLQueryFromQueryState={buildSearchURLQueryFromQueryState}
                        selectedSearchContextSpec={selectedSearchContextSpec}
                        logSearchResultClicked={onLogSearchResultClick}
                        queryExamplesPatternType={patternType}
                    />
                )}
            </div>

            {previewBlob && (
                <FilePreviewPanel
                    blobInfo={previewBlob}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    onClose={handleFilterPanelClose}
                />
            )}
        </div>
    )
}

const isSearchLimitHit = (results?: AggregateStreamingSearchResults): boolean => {
    if (results?.state !== 'complete') {
        return false
    }

    return results?.progress.skipped.some(skipped => skipped.reason.includes('-limit'))
}

interface NewSearchSidebarWrapper extends HTMLAttributes<HTMLElement> {
    onClose: () => void
}

const NewSearchSidebarWrapper: FC<PropsWithChildren<NewSearchSidebarWrapper>> = props => {
    const { children, className, onClose, ...attributes } = props

    return (
        <div
            {...attributes}
            aria-label="Search dynamic filters panel"
            className={classNames(styles.filters, className)}
        >
            <header className={styles.filtersHeader}>
                <H4 as={H2} className="mb-0">
                    Filters
                </H4>
                <Button variant="icon" aria-label="Close" onClick={onClose}>
                    <Icon aria-hidden={true} svgPath={mdiClose} />
                </Button>
            </header>
            <div className={styles.filtersContent}>{children}</div>
        </div>
    )
}

interface FilePreviewPanelProps extends TelemetryProps, TelemetryV2Props {
    blobInfo: SearchResultPreview
    onClose: () => void
}

const FilePreviewPanel: FC<FilePreviewPanelProps> = props => {
    const { blobInfo, onClose, telemetryService, telemetryRecorder } = props

    const staticHighlights = useMemo(() => {
        if (blobInfo.type === 'path') {
            return []
        }
        return blobInfo.chunkMatches?.flatMap(chunkMatch => chunkMatch.ranges)
    }, [blobInfo])

    useEffect(() => {
        telemetryService.logViewEvent('SearchFilePreview')
        telemetryRecorder.recordEvent('search.filePreview', 'view')
    }, [telemetryService, telemetryRecorder])

    return (
        <Panel
            defaultSize={300}
            minSize={256}
            maxSize={600}
            position="right"
            storageKey="file preview"
            ariaLabel="File sidebar"
            className={classNames(styles.preview)}
        >
            <header className={styles.previewHeader}>
                <H4 as={H2} className="mb-0">
                    File preview
                </H4>
                <Button variant="icon" aria-label="Close" onClick={onClose}>
                    <Icon aria-hidden={true} svgPath={mdiClose} />
                </Button>
            </header>

            <small className={styles.previewFileLink}>
                <Link to={getFileMatchUrl(blobInfo)}>{blobInfo.path}</Link>
            </small>

            <Suspense fallback={null}>
                <LazySideBlob
                    repository={blobInfo.repository}
                    file={blobInfo.path}
                    commitID={blobInfo.commit ?? ''}
                    wrapLines={false}
                    navigateToLineOnAnyClick={false}
                    className={styles.previewContent}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    telemetryRecorder={telemetryRecorder}
                    staticHighlightRanges={staticHighlights}
                />
            </Suspense>
        </Panel>
    )
}
