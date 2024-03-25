import { type HTMLAttributes, forwardRef, useEffect } from 'react'

import { useMergeRefs } from 'use-callback-ref'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useSearchParameters } from '@sourcegraph/wildcard'

import { type Insight, isBackendInsight } from '../../../core'

import { BackendInsightView } from './backend-insight/BackendInsight'
import { LangStatsInsightCard } from './lang-stats-insight-card/LangStatsInsightCard'
import { ViewGridItem } from './view-grid/ViewGrid'

export interface SmartInsightProps extends TelemetryProps, TelemetryV2Props, HTMLAttributes<HTMLElement> {
    insight: Insight
    resizing?: boolean
}

/**
 * Render smart insight with ф(gql or extension api) fetcher and independent mutation
 * actions.
 */
export const SmartInsight = forwardRef<HTMLElement, SmartInsightProps>((props, reference) => {
    const { insight, resizing = false, telemetryService, telemetryRecorder, children, ...attributes } = props

    const mergedReference = useMergeRefs([reference])
    const search = useSearchParameters()

    useEffect(() => {
        const insightIdToBeFocused = search.get('focused')
        const element = mergedReference.current

        if (element && insightIdToBeFocused === insight.id) {
            // Schedule card focus in the next frame in order to wait
            // until dashboard rendering is complete
            requestAnimationFrame(() => {
                element.focus()
            })
        }
    }, [insight.id, mergedReference, search])

    return (
        <ViewGridItem id={insight.id} ref={mergedReference} {...attributes}>
            {isBackendInsight(insight) ? (
                <BackendInsightView
                    insight={insight}
                    resizing={resizing}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                >
                    {children}
                </BackendInsightView>
            ) : (
                <LangStatsInsightCard
                    insight={insight}
                    resizing={resizing}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                >
                    {children}
                </LangStatsInsightCard>
            )}
        </ViewGridItem>
    )
})

SmartInsight.displayName = 'SmartInsight'
