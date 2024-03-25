import { forwardRef, useContext, useState, type HTMLAttributes } from 'react'

import { useMergeRefs } from 'use-callback-ref'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Link, ParentSize, ErrorAlert } from '@sourcegraph/wildcard'

import type { LangStatsInsight } from '../../../../core'
import { useLazyLivePreviewLangStatsInsight } from '../../../../core/hooks/live-preview-insight'
import { LazyQueryStatus } from '../../../../hooks/use-parallel-requests/use-parallel-request'
import { getTrackingTypeByInsightType, useCodeInsightViewPings } from '../../../../pings'
import {
    CategoricalBasedChartTypes,
    CategoricalChart,
    InsightCard,
    InsightCardBanner,
    InsightCardHeader,
    InsightCardLoading,
} from '../../../views'
import { useInsightData } from '../../hooks/use-insight-data'
import { InsightContextMenu } from '../insight-context-menu/InsightContextMenu'
import { InsightContext } from '../InsightContext'

import styles from './LangStatsInsightCard.module.scss'

interface BuiltInInsightProps extends TelemetryProps, TelemetryV2Props, HTMLAttributes<HTMLElement> {
    insight: LangStatsInsight
    resizing: boolean
}

export const LangStatsInsightCard = forwardRef<HTMLElement, BuiltInInsightProps>((props, ref) => {
    const { insight, resizing, telemetryService, telemetryRecorder, children, ...attributes } = props

    const { currentDashboard } = useContext(InsightContext)
    const cardRef = useMergeRefs([ref])

    const { lazyQuery } = useLazyLivePreviewLangStatsInsight({
        repository: insight.repository,
        otherThreshold: insight.otherThreshold,
    })

    const { state, isVisible } = useInsightData(lazyQuery, cardRef)

    // Visual line chart settings
    const [zeroYAxisMin, setZeroYAxisMin] = useState(false)

    const { trackDatumClicks, trackMouseLeave, trackMouseEnter } = useCodeInsightViewPings({
        telemetryService,
        telemetryRecorder,
        insightType: getTrackingTypeByInsightType(insight.type),
    })

    return (
        <InsightCard
            {...attributes}
            ref={cardRef}
            data-testid={`insight-card.${insight.id}`}
            aria-label={`${insight.title} insight`}
            role="listitem"
            onMouseEnter={trackMouseEnter}
            onMouseLeave={trackMouseLeave}
        >
            <InsightCardHeader
                title={
                    <Link
                        to={`${window.location.origin}/insights/${insight.id}`}
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        {insight.title}
                    </Link>
                }
            >
                {isVisible && (
                    <InsightContextMenu
                        insight={insight}
                        currentDashboard={currentDashboard}
                        zeroYAxisMin={zeroYAxisMin}
                        onToggleZeroYAxisMin={() => setZeroYAxisMin(!zeroYAxisMin)}
                        telemetryRecorder={telemetryRecorder}
                    />
                )}
            </InsightCardHeader>
            {resizing ? (
                <InsightCardBanner>Resizing</InsightCardBanner>
            ) : state.status === LazyQueryStatus.Loading || !isVisible ? (
                <InsightCardLoading>Loading code insight</InsightCardLoading>
            ) : state.status === LazyQueryStatus.Error ? (
                <ErrorAlert error={state.error} />
            ) : (
                <ParentSize className={styles.chartContainer}>
                    {parent => (
                        <CategoricalChart
                            type={CategoricalBasedChartTypes.Pie}
                            width={parent.width}
                            height={parent.height}
                            locked={insight.isFrozen}
                            className={styles.chart}
                            onDatumLinkClick={trackDatumClicks}
                            {...state.data}
                        />
                    )}
                </ParentSize>
            )}
            {
                // Passing children props explicitly to render any top-level content like
                // resize-handler from the react-grid-layout library
                isVisible && children
            }
        </InsightCard>
    )
})
