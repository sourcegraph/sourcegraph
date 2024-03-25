import { useCallback, useRef } from 'react'

import { useDebouncedCallback } from 'use-debounce'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { V2InsightType, type CodeInsightTrackType } from './types'

interface UseCodeInsightViewPingsInput extends TelemetryProps, TelemetryV2Props {
    /**
     * View tracking type is used to send a proper pings event (InsightHover, InsightDataPointClick)
     * with view type as a tracking variable.
     */
    insightType: CodeInsightTrackType
}

interface PingHandlers {
    trackMouseEnter: () => void
    trackMouseLeave: () => void
    trackDatumClicks: () => void
    trackFilterChanges: () => void
}

/**
 * Shared logic for tracking insight related ping events on the insight card component.
 */
export function useCodeInsightViewPings(props: UseCodeInsightViewPingsInput): PingHandlers {
    const { insightType, telemetryService, telemetryRecorder } = props
    const timeoutID = useRef<number>()

    const trackMouseEnter = useCallback(() => {
        // Set timer to increase confidence that the user meant to interact with the
        // view, as opposed to accidentally moving past it. If the mouse leaves
        // the view quickly, clear the timeout for logging the event
        timeoutID.current = window.setTimeout(() => {
            telemetryService.log('InsightHover', { insightType }, { insightType })
            telemetryRecorder.recordEvent('insight', 'hover', {
                metadata: { insightType: V2InsightType[insightType] },
            })
        }, 500)
    }, [insightType, telemetryService, telemetryRecorder])

    const trackMouseLeave = useCallback(() => {
        window.clearTimeout(timeoutID.current)
    }, [])

    const trackDatumClicks = useCallback(() => {
        telemetryService.log('InsightDataPointClick', { insightType }, { insightType })
        telemetryRecorder.recordEvent('insight.dataPoint', 'click', {
            metadata: { insightType: V2InsightType[insightType] },
        })
    }, [insightType, telemetryService, telemetryRecorder])

    const trackFilterChanges = useDebouncedCallback(() => {
        telemetryService.log('InsightFiltersChange', { insightType }, { insightType })
        telemetryRecorder.recordEvent('insight.filters', 'change', {
            metadata: { insightType: V2InsightType[insightType] },
        })
    }, 1000)

    return {
        trackDatumClicks,
        trackMouseEnter,
        trackMouseLeave,
        trackFilterChanges,
    }
}
