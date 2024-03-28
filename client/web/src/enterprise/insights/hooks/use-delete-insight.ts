import { useCallback, useContext, useState } from 'react'

import { lastValueFrom } from 'rxjs'

import { type ErrorLike, logger } from '@sourcegraph/common'
import { BillingCategory, BillingProduct } from '@sourcegraph/shared/src/telemetry'
import { TelemetryRecorder } from '@sourcegraph/telemetry'

import { eventLogger } from '../../../tracking/eventLogger'
import { CodeInsightsBackendContext, type Insight } from '../core'
import { getTrackingTypeByInsightType } from '../pings'
import { V2InsightType } from '../pings/types'

type DeletionInsight = Pick<Insight, 'id' | 'type'>

export interface UseDeleteInsightAPI {
    delete: (insight: DeletionInsight) => Promise<void>
    loading: boolean
    error: ErrorLike | undefined
}

/**
 * Returns delete handler that deletes insight from all subject settings and from all dashboards
 * that include this insight.
 */
export function useDeleteInsight(
    telemetryRecorder: TelemetryRecorder<BillingCategory, BillingProduct>
): UseDeleteInsightAPI {
    const { deleteInsight } = useContext(CodeInsightsBackendContext)

    const [loading, setLoading] = useState<boolean>(false)
    const [error, setError] = useState<ErrorLike | undefined>()

    const handleDelete = useCallback(
        async (insight: DeletionInsight) => {
            // Prevent double call if we already have ongoing request
            if (loading) {
                return
            }

            setLoading(true)
            setError(undefined)

            try {
                await lastValueFrom(deleteInsight(insight.id), { defaultValue: undefined })
                const insightType = getTrackingTypeByInsightType(insight.type)

                eventLogger.log('InsightRemoval', { insightType }, { insightType })
                telemetryRecorder.recordEvent('insight', 'delete', {
                    metadata: { insightType: V2InsightType[insightType] },
                })
            } catch (error) {
                // TODO [VK] Improve error UI for deleting
                logger.error(error)
                setError(error)
            }
        },
        [loading, deleteInsight, telemetryRecorder]
    )

    return { delete: handleDelete, loading, error }
}
