import { memo, type RefObject, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'

import { isEqual } from 'lodash'
import type { Layout, Layouts } from 'react-grid-layout'

import { BillingCategory, BillingProduct, TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps, TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { TelemetryRecorder } from '@sourcegraph/telemetry'
import { useLocalStorage } from '@sourcegraph/wildcard'

import type { Insight } from '../../core'
import { getTrackingTypeByInsightType } from '../../pings'
import { V2InsightType } from '../../pings/types'

import { SmartInsight } from './components/SmartInsight'
import { ViewGrid } from './components/view-grid/ViewGrid'
import { insightLayoutGenerator } from './utils/grid-layout-generator'

export interface GridApi {
    resetGridLayout: () => void
}

interface SmartInsightsViewGridProps extends TelemetryProps, TelemetryV2Props {
    id: string
    insights: Insight[]
    persistSizeAndOrder?: boolean
    className?: string
    onGridCreate?: (dashboardApi: GridApi) => void
}

/**
 * Renders grid of smart (stateful) insight card. These cards can independently extract and update
 * the insights settings (settings cascade subjects).
 */
export const SmartInsightsViewGrid = memo<SmartInsightsViewGridProps>(function SmartInsightsViewGrid(props) {
    const {
        id,
        insights,
        persistSizeAndOrder = true,
        telemetryService,
        telemetryRecorder,
        className,
        onGridCreate,
    } = props

    // eslint-disable-next-line no-restricted-syntax
    const [localStorageLayouts, setLocalStorageLayouts] = useLocalStorage<Layouts | null>(
        `insights.dashboard.${id}`,
        null
    )

    const [layouts, setLayouts] = useState<Layouts>({})
    const layoutsRef = useMutableValue(layouts)

    const [resizingView, setResizeView] = useState<Layout | null>(null)

    useLayoutEffect(() => {
        setLayouts(insightLayoutGenerator(insights, localStorageLayouts))

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [insights])

    useEffect(() => {
        onGridCreate?.({
            resetGridLayout: () => {
                // Reset local storage layout cache
                setLocalStorageLayouts(null)
                // Reset runtime calculated layout
                setLayouts(insightLayoutGenerator(insights, null))
            },
        })
    }, [insights, setLocalStorageLayouts, onGridCreate])

    const handleDragStart = useCallback(
        (item: Layout) => {
            trackUICustomization(telemetryService, telemetryRecorder, item, insights)
        },
        [telemetryService, telemetryRecorder, insights]
    )

    const handleDragStop = useCallback(() => {
        if (persistSizeAndOrder) {
            setLocalStorageLayouts(layoutsRef.current)
        }
    }, [layoutsRef, persistSizeAndOrder, setLocalStorageLayouts])

    const handleResizeStart = useCallback(
        (item: Layout) => {
            setResizeView(item)
            trackUICustomization(telemetryService, telemetryRecorder, item, insights)
        },
        [telemetryService, telemetryRecorder, insights]
    )

    const handleResizeStop = useCallback((): void => {
        setResizeView(null)

        if (persistSizeAndOrder) {
            setLocalStorageLayouts(layoutsRef.current)
        }
    }, [layoutsRef, persistSizeAndOrder, setLocalStorageLayouts])

    const handleLayoutChange = useCallback((currentLayout: Layout[], allLayouts: Layouts): void => {
        setLayouts(allLayouts)
    }, [])

    return (
        <ViewGrid
            layouts={layouts}
            className={className}
            onDragStart={handleDragStart}
            onDragStop={handleDragStop}
            onResizeStart={handleResizeStart}
            onResizeStop={handleResizeStop}
            onLayoutChange={handleLayoutChange}
        >
            {insights.map(insight => (
                <SmartInsight
                    key={insight.id}
                    insight={insight}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    resizing={resizingView?.i === insight.id}
                />
            ))}
        </ViewGrid>
    )
}, equalSmartGridProps)

function trackUICustomization(
    telemetryService: TelemetryService,
    telemetryRecorder: TelemetryRecorder<BillingCategory, BillingProduct>,
    item: Layout,
    insights: Insight[]
): void {
    try {
        const insight = insights.find(insight => item.i === insight.id)

        if (insight) {
            const insightType = getTrackingTypeByInsightType(insight.type)

            telemetryService.log('InsightUICustomization', { insightType }, { insightType })
            telemetryRecorder.recordEvent('insight.ui', 'customize', { metadata: { type: V2InsightType[insightType] } })
        }
    } catch {
        // noop
    }
}

function useMutableValue<T>(value: T): RefObject<T> {
    const valueRef = useRef<T>(value)
    valueRef.current = value
    return valueRef
}

/**
 * Custom props checker for the smart grid component.
 *
 * Ignore settings cascade change and insight body config changes to avoid
 * animations of grid item rerender and grid position items. In some cases (like insight
 * filters updating, we want to ignore insights from settings cascade).
 * But still trigger grid animation rerender if insight ordering or insight count
 * have been changed.
 */
function equalSmartGridProps(
    previousProps: SmartInsightsViewGridProps,
    nextProps: SmartInsightsViewGridProps
): boolean {
    const { insights: previousInsights, ...otherPrepProps } = previousProps
    const { insights: nextInsights, ...otherNextProps } = nextProps

    if (!isEqual(otherPrepProps, otherNextProps)) {
        return false
    }

    return isEqual(
        previousInsights.map(insight => insight.id),
        nextInsights.map(insight => insight.id)
    )
}
