import type { FC } from 'react'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { type Series, useDeepMemo, ErrorAlert } from '@sourcegraph/wildcard'

import {
    SeriesBasedChartTypes,
    SeriesChart,
    LivePreviewCard,
    LivePreviewHeader,
    LivePreviewLoading,
    LivePreviewChart,
    LivePreviewBlurBackdrop,
    LivePreviewBanner,
    LivePreviewLegend,
    SERIES_MOCK_CHART,
} from '../../../../../components'
import { DATA_SERIES_COLORS } from '../../../../../constants'
import {
    type SeriesWithStroke,
    useLivePreviewSeriesInsight,
    LivePreviewStatus,
} from '../../../../../core/hooks/live-preview-insight'
import { CodeInsightTrackType, useCodeInsightViewPings } from '../../../../../pings'

const createExampleDataSeries = (query: string): SeriesWithStroke[] => [
    {
        query,
        label: 'TODOs',
        generatedFromCaptureGroups: false,
        stroke: DATA_SERIES_COLORS.INDIGO,
    },
]

interface DynamicInsightPreviewProps extends TelemetryProps, TelemetryV2Props {
    disabled: boolean
    repositories: string[]
    query: string
    className?: string
}

export const DynamicInsightPreview: FC<DynamicInsightPreviewProps> = props => {
    const { disabled, repositories, query, className, telemetryService, telemetryRecorder } = props

    // Compare live insight settings with deep check to avoid unnecessary
    // search insight content fetching
    const settings = useDeepMemo({
        disabled,
        repoScope: { repositories },
        series: createExampleDataSeries(query),
        step: { months: 2 },
    })

    const { state } = useLivePreviewSeriesInsight({
        skip: disabled,
        ...settings,
    })

    const { trackMouseEnter, trackMouseLeave, trackDatumClicks } = useCodeInsightViewPings({
        telemetryService,
        insightType: CodeInsightTrackType.InProductLandingPageInsight,
        telemetryRecorder,
    })

    return (
        <LivePreviewCard className={className}>
            <LivePreviewHeader title="In-line TODO statements" />
            {state.status === LivePreviewStatus.Loading ? (
                <LivePreviewLoading>Loading code insight</LivePreviewLoading>
            ) : state.status === LivePreviewStatus.Error ? (
                <ErrorAlert error={state.error} />
            ) : (
                <LivePreviewChart>
                    {parent =>
                        state.status === LivePreviewStatus.Data ? (
                            <SeriesChart
                                type={SeriesBasedChartTypes.Line}
                                width={parent.width}
                                height={parent.height}
                                series={state.data}
                            />
                        ) : (
                            <>
                                <LivePreviewBlurBackdrop
                                    as={SeriesChart}
                                    type={SeriesBasedChartTypes.Line}
                                    width={parent.width}
                                    height={parent.height}
                                    onMouseEnter={trackMouseEnter}
                                    onMouseLeave={trackMouseLeave}
                                    onDatumClick={trackDatumClicks}
                                    // We cast to unknown here because ForwardReferenceComponent
                                    // doesn't support inferring as component with generic.
                                    series={SERIES_MOCK_CHART as Series<unknown>[]}
                                />
                                <LivePreviewBanner>
                                    The chart preview will be shown here once you have filled out the repositories and
                                    series fields.
                                </LivePreviewBanner>
                            </>
                        )
                    }
                </LivePreviewChart>
            )}

            {state.status === LivePreviewStatus.Data && <LivePreviewLegend series={state.data as Series<unknown>[]} />}
        </LivePreviewCard>
    )
}
