import type { Meta, StoryFn } from '@storybook/react'
import { of } from 'rxjs'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../../../../../components/WebStory'
import { CodeInsightsBackendStoryMock } from '../../../CodeInsightsBackendStoryMock'
import type { CodeInsightsGqlBackend } from '../../../core/backend/gql-backend/code-insights-gql-backend'
import { InsightsDashboardOwnerType } from '../../../core/types'
import { useCodeInsightsLicenseState } from '../../../stores'

import { InsightsDashboardCreationPage } from './InsightsDashboardCreationPage'

const config: Meta = {
    title: 'web/insights/InsightsDashboardCreationPage',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
    parameters: {
        chromatic: {
            viewports: [576, 1440],
            disableSnapshot: false,
        },
    },
}

export default config

const codeInsightsBackend: Partial<CodeInsightsGqlBackend> = {
    getDashboardOwners: () =>
        of([
            { type: InsightsDashboardOwnerType.Personal, id: '001', title: 'Personal' },
            { type: InsightsDashboardOwnerType.Organization, id: '002', title: 'Organization 1' },
            { type: InsightsDashboardOwnerType.Organization, id: '003', title: 'Organization 2' },
            { type: InsightsDashboardOwnerType.Global, id: '004', title: 'Global' },
        ]),
}

export const InsightsDashboardCreationLicensed: StoryFn = () => {
    useCodeInsightsLicenseState.setState({ licensed: true, insightsLimit: null })

    return (
        <CodeInsightsBackendStoryMock mocks={codeInsightsBackend}>
            <InsightsDashboardCreationPage
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </CodeInsightsBackendStoryMock>
    )
}

export const InsightsDashboardCreationUnlicensed: StoryFn = () => {
    useCodeInsightsLicenseState.setState({ licensed: false, insightsLimit: 2 })

    return (
        <CodeInsightsBackendStoryMock mocks={codeInsightsBackend}>
            <InsightsDashboardCreationPage
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </CodeInsightsBackendStoryMock>
    )
}
