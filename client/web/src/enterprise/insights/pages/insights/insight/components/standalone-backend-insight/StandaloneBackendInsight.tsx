import React, { useContext, useRef, useState, useReducer } from 'react'

import classNames from 'classnames'
import { useHistory } from 'react-router'
import VisibilitySensor from 'react-visibility-sensor'

import { asError } from '@sourcegraph/common'
import { useQuery } from '@sourcegraph/http-client'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Card, CardBody, useDebounce, useDeepMemo } from '@sourcegraph/wildcard'

import { useFeatureFlag } from '../../../../../../../featureFlags/useFeatureFlag'
import {
    GetInsightViewResult,
    GetInsightViewVariables,
    InsightViewFiltersInput,
    SeriesDisplayOptionsInput,
} from '../../../../../../../graphql-operations'
import { InsightCard, InsightCardHeader, InsightCardLoading } from '../../../../../components'
import { FORM_ERROR, FormChangeEvent, SubmissionErrors } from '../../../../../components/form/hooks/useForm'
import {
    DrillDownInsightFilters,
    FilterSectionVisualMode,
    DrillDownInsightCreationForm,
    DrillDownFiltersStep,
    BackendInsightChart,
    BackendInsightErrorAlert,
    DrillDownFiltersFormValues,
    DrillDownInsightCreationFormValues,
} from '../../../../../components/insights-view-grid/components/backend-insight/components'
import { useSeriesToggle } from '../../../../../components/insights-view-grid/components/backend-insight/components/backend-insight-chart/use-series-toggle'
import {
    ALL_INSIGHTS_DASHBOARD,
    BackendInsight,
    CodeInsightsBackendContext,
    DEFAULT_SERIES_DISPLAY_OPTIONS,
    InsightFilters,
    InsightType,
} from '../../../../../core'
import { BackendInsightData } from '../../../../../core/backend/code-insights-backend-types'
import { GET_INSIGHT_VIEW_GQL } from '../../../../../core/backend/gql-backend'
import { createBackendInsightData } from '../../../../../core/backend/gql-backend/methods/get-backend-insight-data/deserializators'
import { insightPollingInterval } from '../../../../../core/backend/gql-backend/utils/insight-polling'
import { getTrackingTypeByInsightType, useCodeInsightViewPings } from '../../../../../pings'
import { StandaloneInsightContextMenu } from '../context-menu/StandaloneInsightContextMenu'

import styles from './StandaloneBackendInsight.module.scss'

interface StandaloneBackendInsight extends TelemetryProps {
    insight: BackendInsight
    className?: string
}

function wasEverVisible(previouslyVisible: boolean, currentVisibility: boolean): boolean {
    return previouslyVisible || currentVisibility
}

export const StandaloneBackendInsight: React.FunctionComponent<StandaloneBackendInsight> = props => {
    const { telemetryService, insight, className } = props
    const history = useHistory()
    const { createInsight, updateInsight } = useContext(CodeInsightsBackendContext)
    const { toggle, isSeriesSelected, isSeriesHovered, setHoveredId } = useSeriesToggle()
    const [wasVisble, dispatchVisibilityChange] = useReducer(wasEverVisible, false)
    const [insightData, setInsightData] = useState<BackendInsightData | undefined>()
    const [disablePolling] = useFeatureFlag('disable-insight-polling')
    const pollingInterval = disablePolling ? 0 : insightPollingInterval(insight)

    // Visual line chart settings
    const [zeroYAxisMin, setZeroYAxisMin] = useState(false)
    const [step, setStep] = useState(DrillDownFiltersStep.Filters)

    // Original insight filters values that are stored in setting subject with insight
    // configuration object, They are updated  whenever the user clicks update/save button
    const [originalInsightFilters, setOriginalInsightFilters] = useState(insight.filters)
    const insightCardReference = useRef<HTMLDivElement>(null)

    // Live valid filters from filter form. They are updated whenever the user is changing
    // filter value in filters fields.
    const [filters, setFilters] = useState<InsightFilters>(originalInsightFilters)
    const [filterVisualMode, setFilterVisialMode] = useState<FilterSectionVisualMode>(FilterSectionVisualMode.Preview)
    const debouncedFilters = useDebounce(useDeepMemo<InsightFilters>(filters), 500)

    const [seriesDisplayOptions, setSeriesDisplayOptions] = useState(insight.seriesDisplayOptions)

    const filterInput: InsightViewFiltersInput = {
        includeRepoRegex: debouncedFilters.includeRepoRegexp,
        excludeRepoRegex: debouncedFilters.excludeRepoRegexp,
        searchContexts: [debouncedFilters.context],
    }
    const displayInput: SeriesDisplayOptionsInput = {
        limit: seriesDisplayOptions?.limit,
        sortOptions: seriesDisplayOptions?.sortOptions,
    }

    const { error, loading, stopPolling } = useQuery<GetInsightViewResult, GetInsightViewVariables>(
        GET_INSIGHT_VIEW_GQL,
        {
            variables: { id: insight.id, filters: filterInput, seriesDisplayOptions: displayInput },
            fetchPolicy: 'cache-and-network',
            pollInterval: pollingInterval,
            skip: !wasVisble,
            onCompleted: data => {
                const parsedData = createBackendInsightData(insight, data.insightViews.nodes[0])
                if (!parsedData.isFetchingHistoricalData) {
                    stopPolling()
                }
                setInsightData(parsedData)
            },
        }
    )

    const { trackMouseLeave, trackMouseEnter, trackDatumClicks } = useCodeInsightViewPings({
        telemetryService,
        insightType: getTrackingTypeByInsightType(insight.type),
    })

    const handleFilterChange = (event: FormChangeEvent<DrillDownFiltersFormValues>): void => {
        if (event.valid) {
            setFilters(event.values)
        }
    }

    const handleFilterSave = async (filters: InsightFilters): Promise<SubmissionErrors> => {
        try {
            await updateInsight({ insightId: insight.id, nextInsightData: { ...insight, filters } }).toPromise()
            setOriginalInsightFilters(filters)
            telemetryService.log('CodeInsightsSearchBasedFilterUpdating')
        } catch (error) {
            return { [FORM_ERROR]: asError(error) }
        }

        return
    }

    const handleInsightFilterCreation = async (
        values: DrillDownInsightCreationFormValues
    ): Promise<SubmissionErrors> => {
        try {
            await createInsight({
                insight: {
                    ...insight,
                    title: values.insightName,
                    filters,
                },
                dashboard: null,
            }).toPromise()

            setOriginalInsightFilters(filters)
            history.push(`/insights/dashboard${ALL_INSIGHTS_DASHBOARD.id}`)
            telemetryService.log('CodeInsightsSearchBasedFilterInsightCreation')
        } catch (error) {
            return { [FORM_ERROR]: asError(error) }
        }

        return
    }

    return (
        <div className={classNames(className, styles.root)}>
            <Card as={CardBody} className={styles.filters}>
                {step === DrillDownFiltersStep.Filters && (
                    <DrillDownInsightFilters
                        initialValues={filters}
                        originalValues={originalInsightFilters}
                        visualMode={filterVisualMode}
                        onVisualModeChange={setFilterVisialMode}
                        showSeriesDisplayOptions={insight.type === InsightType.CaptureGroup}
                        onFiltersChange={handleFilterChange}
                        onFilterSave={handleFilterSave}
                        onCreateInsightRequest={() => setStep(DrillDownFiltersStep.ViewCreation)}
                        originalSeriesDisplayOptions={DEFAULT_SERIES_DISPLAY_OPTIONS}
                        onSeriesDisplayOptionsChange={setSeriesDisplayOptions}
                    />
                )}

                {step === DrillDownFiltersStep.ViewCreation && (
                    <DrillDownInsightCreationForm
                        onCreateInsight={handleInsightFilterCreation}
                        onCancel={() => setStep(DrillDownFiltersStep.Filters)}
                    />
                )}
            </Card>

            <InsightCard
                ref={insightCardReference}
                data-testid={`insight-standalone-card.${insight.id}`}
                className={styles.chart}
                onMouseEnter={trackMouseEnter}
                onMouseLeave={trackMouseLeave}
            >
                <InsightCardHeader title={insight.title}>
                    <StandaloneInsightContextMenu
                        insight={insight}
                        zeroYAxisMin={zeroYAxisMin}
                        onToggleZeroYAxisMin={setZeroYAxisMin}
                    />
                </InsightCardHeader>

                <VisibilitySensor active={true} onChange={dispatchVisibilityChange} partialVisibility={true}>
                    {loading || !wasVisble || !insightData ? (
                        <InsightCardLoading>Loading code insight</InsightCardLoading>
                    ) : error ? (
                        <BackendInsightErrorAlert error={error} />
                    ) : (
                        <BackendInsightChart
                            {...insightData}
                            locked={insight.isFrozen}
                            zeroYAxisMin={zeroYAxisMin}
                            isSeriesSelected={isSeriesSelected}
                            isSeriesHovered={isSeriesHovered}
                            onDatumClick={trackDatumClicks}
                            onLegendItemClick={toggle}
                            setHoveredId={setHoveredId}
                        />
                    )}
                </VisibilitySensor>
            </InsightCard>
        </div>
    )
}
