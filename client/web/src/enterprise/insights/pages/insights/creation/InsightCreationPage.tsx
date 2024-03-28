import { type FC, useContext } from 'react'

import { useNavigate } from 'react-router-dom'
import { lastValueFrom } from 'rxjs'

import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { CodeInsightsBackendContext, type CreationInsightInput } from '../../../core'
import { useQueryParameters } from '../../../hooks'
import { encodeDashboardIdQueryParam } from '../../../routers.constant'

import { CaptureGroupCreationPage } from './capture-group'
import { ComputeInsightCreationPage } from './compute'
import { LangStatsInsightCreationPage } from './lang-stats'
import { SearchInsightCreationPage } from './search-insight'

export enum InsightCreationPageType {
    LangStats = 'lang-stats',
    Compute = 'compute',
    Search = 'search-based',
    CaptureGroup = 'capture-group',
}

interface InsightCreateEvent {
    insight: CreationInsightInput
}

interface InsightCreationPageProps extends TelemetryProps, TelemetryV2Props {
    mode: InsightCreationPageType
}

export const InsightCreationPage: FC<InsightCreationPageProps> = props => {
    const { mode, telemetryService, telemetryRecorder } = props

    const navigate = useNavigate()
    const { createInsight } = useContext(CodeInsightsBackendContext)
    const { dashboardId } = useQueryParameters(['dashboardId'])

    const codeInsightsCompute = useExperimentalFeatures(features => features.codeInsightsCompute)

    const handleInsightCreateRequest = async (event: InsightCreateEvent): Promise<unknown> => {
        const { insight } = event

        return lastValueFrom(createInsight({ insight, dashboardId: dashboardId ?? null }), { defaultValue: undefined })
    }

    const handleInsightSuccessfulCreation = (): void => {
        if (!dashboardId) {
            // Navigate to the dashboard page with new created dashboard
            navigate('/insights/all')

            return
        }

        navigate(`/insights/dashboards/${dashboardId}`)
    }

    const handleCancel = (): void => {
        if (!dashboardId) {
            navigate('/insights/all')

            return
        }

        navigate(`/insights/dashboards/${dashboardId}`)
    }

    const backCreateUrl = encodeDashboardIdQueryParam('/insights/create', dashboardId)

    if (mode === InsightCreationPageType.CaptureGroup) {
        return (
            <CaptureGroupCreationPage
                backUrl={backCreateUrl}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                onInsightCreateRequest={handleInsightCreateRequest}
                onSuccessfulCreation={handleInsightSuccessfulCreation}
                onCancel={handleCancel}
            />
        )
    }

    if (mode === InsightCreationPageType.Search) {
        return (
            <SearchInsightCreationPage
                backUrl={backCreateUrl}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                onInsightCreateRequest={handleInsightCreateRequest}
                onSuccessfulCreation={handleInsightSuccessfulCreation}
                onCancel={handleCancel}
            />
        )
    }

    if (codeInsightsCompute && mode === InsightCreationPageType.Compute) {
        return (
            <ComputeInsightCreationPage
                backUrl={backCreateUrl}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                onInsightCreateRequest={handleInsightCreateRequest}
                onSuccessfulCreation={handleInsightSuccessfulCreation}
                onCancel={handleCancel}
            />
        )
    }

    return (
        <LangStatsInsightCreationPage
            backUrl={backCreateUrl}
            telemetryService={telemetryService}
            telemetryRecorder={telemetryRecorder}
            onInsightCreateRequest={handleInsightCreateRequest}
            onSuccessfulCreation={handleInsightSuccessfulCreation}
            onCancel={handleCancel}
        />
    )
}
