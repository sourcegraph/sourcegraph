import { useCallback, useContext, useState } from 'react'

import { type ErrorLike, logger } from '@sourcegraph/common'
import { BillingCategory, BillingProduct } from '@sourcegraph/shared/src/telemetry'
import { TelemetryRecorder } from '@sourcegraph/telemetry'

import { eventLogger } from '../../../tracking/eventLogger'
import { CodeInsightsBackendContext, type Insight, type InsightDashboard } from '../core'
import { getTrackingTypeByInsightType } from '../pings'
import { V2InsightType } from '../pings/types'

interface RemoveInsightInput {
    insight: Pick<Insight, 'id' | 'title' | 'type'>
    dashboard: Pick<InsightDashboard, 'id' | 'title'>
}

export interface useRemoveInsightFromDashboardAPI {
    remove: (insight: RemoveInsightInput) => Promise<void>
    loading: boolean
    error: ErrorLike | undefined
}

export function useRemoveInsightFromDashboard(
    telemetryRecorder: TelemetryRecorder<BillingCategory, BillingProduct>
): useRemoveInsightFromDashboardAPI {
    const { removeInsightFromDashboard } = useContext(CodeInsightsBackendContext)

    const [loading, setLoading] = useState<boolean>(false)
    const [error, setError] = useState<ErrorLike | undefined>()

    const handleRemove = useCallback(
        async (input: RemoveInsightInput) => {
            const { insight, dashboard } = input

            // Prevent double call if we already have ongoing request
            if (loading) {
                return
            }

            setLoading(true)
            setError(undefined)

            try {
                await removeInsightFromDashboard({
                    insightId: insight.id,
                    dashboardId: dashboard.id,
                }).toPromise()

                const insightType = getTrackingTypeByInsightType(insight.type)

                eventLogger.log('InsightRemovalFromDashboard', { insightType }, { insightType })
                telemetryRecorder.recordEvent('insight', 'removeFromDashboard', {
                    metadata: { insightType: V2InsightType[insightType] },
                })
            } catch (error) {
                // TODO [VK] Improve error UI for removing
                logger.error(error)
                setError(error)
            }
        },
        [loading, removeInsightFromDashboard, telemetryRecorder]
    )

    return { remove: handleRemove, loading, error }
}
