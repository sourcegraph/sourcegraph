import React, { useState } from 'react'

import classNames from 'classnames'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ParentSize, ErrorAlert } from '@sourcegraph/wildcard'

import {
    CategoricalBasedChartTypes,
    CategoricalChart,
    InsightCard,
    InsightCardHeader,
    InsightCardLoading,
} from '../../../../../components'
import type { LangStatsInsight } from '../../../../../core'
import { LivePreviewStatus, useLivePreviewLangStatsInsight } from '../../../../../core/hooks/live-preview-insight'
import { getTrackingTypeByInsightType, useCodeInsightViewPings } from '../../../../../pings'
import { StandaloneInsightContextMenu } from '../context-menu/StandaloneInsightContextMenu'

import styles from './StandaloneLangStatsInsight.module.scss'

interface StandaloneLangStatsInsightProps extends TelemetryProps, TelemetryV2Props {
    insight: LangStatsInsight
    className?: string
}

export function StandaloneLangStatsInsight(props: StandaloneLangStatsInsightProps): React.ReactElement {
    const { insight, telemetryService, telemetryRecorder, className } = props

    const { state } = useLivePreviewLangStatsInsight(insight)

    // Visual line chart settings
    const [zeroYAxisMin, setZeroYAxisMin] = useState(false)

    const { trackDatumClicks, trackMouseLeave, trackMouseEnter } = useCodeInsightViewPings({
        telemetryService,
        telemetryRecorder,
        insightType: getTrackingTypeByInsightType(insight.type),
    })

    return (
        <InsightCard
            data-testid={`insight-card.${insight.id}`}
            className={classNames(styles.chart, className)}
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
            {state.status === LivePreviewStatus.Loading || state.status === LivePreviewStatus.Intact ? (
                <InsightCardLoading>Loading code insight</InsightCardLoading>
            ) : state.status === LivePreviewStatus.Error ? (
                <ErrorAlert error={state.error} />
            ) : (
                <ParentSize className={styles.chartParentSize}>
                    {parent => (
                        <CategoricalChart
                            type={CategoricalBasedChartTypes.Pie}
                            width={parent.width}
                            height={parent.height}
                            locked={insight.isFrozen}
                            onDatumLinkClick={trackDatumClicks}
                            {...state.data}
                        />
                    )}
                </ParentSize>
            )}
        </InsightCard>
    )
}
