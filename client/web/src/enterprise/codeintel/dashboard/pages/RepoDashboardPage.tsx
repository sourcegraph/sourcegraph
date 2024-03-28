import { useCallback, useEffect, useMemo, useState } from 'react'

import { mdiCircleOffOutline } from '@mdi/js'
import { type Location, useLocation, useNavigate } from 'react-router-dom'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Alert,
    Container,
    ErrorAlert,
    Icon,
    Label,
    Link,
    LoadingSpinner,
    PageHeader,
    RadioButton,
    Select,
} from '@sourcegraph/wildcard'

import { DataSummary, type DataSummaryItem } from '../components/DataSummary'
import { DashboardTree } from '../components/tree/DashboardTree'
import { getIndexerKey, sanitizePath, getIndexRoot } from '../components/tree/util'
import { INDEX_COMPLETED_STATES, INDEX_FAILURE_STATES } from '../constants'
import { useRepoCodeIntelStatus } from '../hooks/useRepoCodeIntelStatus'

import styles from './RepoDashboardPage.module.scss'

export interface RepoDashboardPageProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    repo: { id: string; name: string }
    now?: () => Date
    indexingEnabled?: boolean
}

type ShowFilter = 'all' | 'indexes' | 'suggestions'
type IndexFilter = 'all' | 'success' | 'error'
type LanguageFilter = 'all' | string
interface DefaultFilterState {
    show: Extract<ShowFilter, 'all' | 'indexes'>
    indexState: IndexFilter
    language: LanguageFilter
}
interface SuggestionFilterState {
    show: Extract<ShowFilter, 'suggestions'>
    language: LanguageFilter
}
export type FilterState = SuggestionFilterState | DefaultFilterState

/**
 * Build a valid FilterState
 * Used to allow other pages to easily link to a configured dashboard
 **/
export const buildParamsFromFilterState = (filterState: FilterState): URLSearchParams => {
    const params = new URLSearchParams()

    if (filterState.show === 'suggestions') {
        params.set('show', 'suggestions')
    } else {
        params.set('show', filterState.show)
        params.set('indexState', filterState.indexState)
    }

    params.set('language', filterState.language)

    return params
}

/**
 * Parse search parameters and build a valid FilterState.
 * Used to manage the state of the dashboard
 */
const buildFilterStateFromParams = ({ search }: Location, indexingEnabled: boolean): FilterState => {
    const queryParameters = new URLSearchParams(search)

    const show = queryParameters.get('show') || (indexingEnabled ? 'all' : 'indexes')
    const language = queryParameters.get('language') || 'all'

    if (show === 'suggestions') {
        return {
            show,
            language,
        }
    }

    return {
        show: show as DefaultFilterState['show'],
        language,
        indexState: (queryParameters.get('indexState') || 'all') as IndexFilter,
    }
}

export const RepoDashboardPage: React.FunctionComponent<RepoDashboardPageProps> = ({
    telemetryService,
    repo,
    authenticatedUser,
    now,
    indexingEnabled = window.context?.codeIntelAutoIndexingEnabled,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryService.logPageView('CodeIntelRepoDashboard')
        telemetryRecorder.recordEvent('repo.codeIntel.dashboard', 'view')
    }, [telemetryService, telemetryRecorder])

    const location = useLocation()
    const navigate = useNavigate()

    const { data, loading, error } = useRepoCodeIntelStatus({ repository: repo.name })

    const [filterState, setFilterState] = useState<FilterState>(buildFilterStateFromParams(location, indexingEnabled))

    useEffect(() => {
        setFilterState(buildFilterStateFromParams(location, indexingEnabled))
    }, [location, indexingEnabled])

    const handleFilterChange = useCallback(
        (value: string, paramKey: keyof SuggestionFilterState | keyof DefaultFilterState) => {
            const queryParameters = new URLSearchParams(location.search)
            queryParameters.set(paramKey, value)
            navigate({ search: queryParameters.toString() }, { replace: true })
        },
        [location.search, navigate]
    )

    const indexes = useMemo(() => {
        if (!data) {
            return []
        }
        return data.summary.recentActivity
    }, [data])

    const suggestedIndexers = useMemo(() => {
        if (!data) {
            return []
        }

        return data.summary.availableIndexers
            .flatMap(({ rootsWithKeys, indexer }) =>
                rootsWithKeys.map(({ root, comparisonKey }) => ({ root, comparisonKey, ...indexer }))
            )
            .filter(
                ({ root, key }) =>
                    !indexes.some(index => getIndexRoot(index) === sanitizePath(root) && getIndexerKey(index) === key)
            )
    }, [data, indexes])

    const languageKeys = new Set([...indexes.map(getIndexerKey), ...suggestedIndexers.map(indexer => indexer.key)])

    const summaryItems = useMemo((): DataSummaryItem[] => {
        if (!indexes || !suggestedIndexers) {
            return []
        }

        const numCompletedIndexes = indexes.filter(index => INDEX_COMPLETED_STATES.has(index.state)).length
        const numFailedIndexes = indexes.filter(index => INDEX_FAILURE_STATES.has(index.state)).length
        const numUnconfiguredProjects = suggestedIndexers.length

        return [
            {
                label: 'Successfully indexed projects',
                value: <>{numCompletedIndexes}</>,
                valueClassName: 'text-success',
                to: `?${buildParamsFromFilterState({
                    show: 'indexes',
                    indexState: 'success',
                    language: 'all',
                }).toString()}`,
            },
            {
                label: 'Projects with errors',
                value: <>{numFailedIndexes}</>,
                className: styles.summaryItemThin,
                valueClassName: 'text-danger',
                to: `?${buildParamsFromFilterState({
                    show: 'indexes',
                    indexState: 'error',
                    language: 'all',
                }).toString()}`,
            },
            ...(indexingEnabled
                ? [
                      {
                          label: 'Configurable projects',
                          value: <>{numUnconfiguredProjects}</>,
                          valueClassName: 'text-primary',
                          to: `?${buildParamsFromFilterState({
                              show: 'suggestions',
                              language: 'all',
                          }).toString()}`,
                      },
                  ]
                : [
                      {
                          label: 'Auto-indexing is disabled',
                          value: (
                              <Icon size="sm" aria-label="Auto-indexing is disabled" svgPath={mdiCircleOffOutline} />
                          ),
                          valueClassName: 'text-muted',
                      },
                  ]),
        ]
    }, [indexes, suggestedIndexers, indexingEnabled])

    if (loading && !data) {
        return <LoadingSpinner />
    }

    if (error) {
        return <ErrorAlert prefix="Failed to load code intelligence summary for repository" error={error} />
    }

    return (
        <>
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: (
                            <>
                                Code intelligence summary for <RepoLink repoName={repo.name} to={null} />
                            </>
                        ),
                    },
                ]}
                description="View the latest indexes and suggestions for this repository."
                className="mb-3"
                actions={
                    authenticatedUser?.siteAdmin && (
                        <Link to="/site-admin/code-graph/dashboard">View global dashboard</Link>
                    )
                }
            />
            {data && (
                <>
                    <Alert variant={data.commitGraph.stale ? 'primary' : 'success'} aria-live="off">
                        {data.commitGraph.stale ? (
                            <>
                                Repository commit graph is currently stale and is queued to be refreshed. Refreshing the
                                commit graph updates which uploads are visible from which commits.
                            </>
                        ) : (
                            <>Repository commit graph is currently up to date.</>
                        )}{' '}
                        {data.commitGraph.updatedAt && (
                            <>
                                Last refreshed <Timestamp date={data.commitGraph.updatedAt} now={now} />.
                            </>
                        )}
                    </Alert>

                    <Container className={styles.summaryContainer}>
                        <DataSummary items={summaryItems} className={styles.summary} />
                        <div>
                            <div className="text-muted">
                                <small className="d-block">
                                    {data.summary.lastIndexScan ? (
                                        <>
                                            This repository was scanned for auto-indexing{' '}
                                            <Timestamp date={data.summary.lastIndexScan} />.
                                        </>
                                    ) : (
                                        <>This repository has never been scanned for auto-indexing.</>
                                    )}
                                </small>
                                <small className="d-block">
                                    {data.summary.lastUploadRetentionScan ? (
                                        <>
                                            The indexes of this repository were last considered for expiration{' '}
                                            <Timestamp date={data.summary.lastUploadRetentionScan} />.
                                        </>
                                    ) : (
                                        <> The indexes of this repository have never been considered for expiration.</>
                                    )}
                                </small>
                            </div>
                        </div>
                    </Container>

                    <Container className="my-3">
                        <div className="d-flex justify-content-end">
                            <div className={styles.filterContainer}>
                                {indexingEnabled && (
                                    <div>
                                        <Label className={styles.radioGroup}>
                                            Show:
                                            <RadioButton
                                                name="show-filter"
                                                id="show-all"
                                                value="all"
                                                checked={filterState.show === 'all'}
                                                onChange={event => handleFilterChange(event.target.value, 'show')}
                                                label="All"
                                                wrapperClassName="ml-2 mr-3"
                                            />
                                            <RadioButton
                                                name="show-filter"
                                                id="show-indexes"
                                                value="indexes"
                                                checked={filterState.show === 'indexes'}
                                                onChange={event => handleFilterChange(event.target.value, 'show')}
                                                label="Indexes"
                                                wrapperClassName="mr-3"
                                            />
                                            <RadioButton
                                                name="show-filter"
                                                id="show-suggestions"
                                                value="suggestions"
                                                checked={filterState.show === 'suggestions'}
                                                onChange={event => handleFilterChange(event.target.value, 'show')}
                                                label="Suggestions"
                                            />
                                        </Label>
                                    </div>
                                )}
                                <div className="d-flex">
                                    <Select
                                        id="language-filter"
                                        label="Language:"
                                        value={filterState.language}
                                        onChange={event => handleFilterChange(event.target.value, 'language')}
                                        className="d-flex align-items-center mb-0"
                                        selectClassName={styles.select}
                                        labelClassName="mb-0 mr-2"
                                        isCustomStyle={true}
                                    >
                                        <option value="all">All</option>
                                        {[...languageKeys].sort().map(key => (
                                            <option key={key} value={key}>
                                                {key}
                                            </option>
                                        ))}
                                    </Select>
                                    {'indexState' in filterState && (
                                        <Select
                                            id="index-filter"
                                            label="Indexing:"
                                            value={filterState.indexState}
                                            onChange={event => handleFilterChange(event.target.value, 'indexState')}
                                            className="d-flex align-items-center mb-0 ml-3"
                                            selectClassName={styles.select}
                                            labelClassName="mb-0 mr-2"
                                            isCustomStyle={true}
                                        >
                                            <option value="all">Most recent attempt</option>
                                            <option value="success">Most recent success</option>
                                            <option value="error">Most recent failure</option>
                                        </Select>
                                    )}
                                </div>
                            </div>
                        </div>

                        <DashboardTree indexes={indexes} suggestedIndexers={suggestedIndexers} filter={filterState} />
                    </Container>
                </>
            )}
        </>
    )
}
