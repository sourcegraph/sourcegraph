import PuzzleIcon from 'mdi-react/PuzzleIcon'
import React, { useContext, useMemo } from 'react'

import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { haveInitialExtensionsLoaded } from '@sourcegraph/shared/src/api/features'
import { ExtensionsControllerProps } from '@sourcegraph/shared/src/extensions/controller'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useObservable } from '@sourcegraph/shared/src/util/useObservable'

import { CodeInsightsIcon, InsightsViewGrid } from '../../../../../../../components'
import { InsightsApiContext } from '../../../../../../../core/backend/api-provider'
import { EmptyInsightDashboard } from '../empty-insight-dashboard/EmptyInsightDashboard'

interface DashboardInsightsProps extends ExtensionsControllerProps, TelemetryProps {
    /**
     * Dashboard specific insight ids.
     */
    insightIds?: string[]
}

export const DashboardInsights: React.FunctionComponent<DashboardInsightsProps> = props => {
    const { telemetryService, extensionsController, insightIds } = props
    const { getInsightCombinedViews } = useContext(InsightsApiContext)

    const views = useObservable(
        useMemo(() => getInsightCombinedViews(extensionsController?.extHostAPI, insightIds), [
            insightIds,
            extensionsController,
            getInsightCombinedViews,
        ])
    )

    // Ensures that we don't show a misleading empty state when extensions haven't loaded yet.
    const areExtensionsReady = useObservable(
        useMemo(() => haveInitialExtensionsLoaded(props.extensionsController.extHostAPI), [props.extensionsController])
    )

    if (!areExtensionsReady) {
        return (
            <div className="d-flex justify-content-center align-items-center pt-5">
                <LoadingSpinner />
                <span className="mx-2">Loading Sourcegraph extensions</span>
                <PuzzleIcon className="icon-inline" />
            </div>
        )
    }

    if (views === undefined) {
        return (
            <div className="d-flex justify-content-center align-items-center pt-5">
                <LoadingSpinner />
                <span className="mx-2">Loading code insights</span>
                <CodeInsightsIcon className="icon-inline" />
            </div>
        )
    }

    return (
        <div>
            {views.length > 0 ? (
                <InsightsViewGrid views={views} hasContextMenu={true} telemetryService={telemetryService} />
            ) : (
                <EmptyInsightDashboard />
            )}
        </div>
    )
}
