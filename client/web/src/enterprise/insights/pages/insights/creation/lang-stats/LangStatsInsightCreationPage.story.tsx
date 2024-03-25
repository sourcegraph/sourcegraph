import type { MockedResponse } from '@apollo/client/testing'
import type { Meta, StoryFn } from '@storybook/react'
import delay from 'delay'
import { noop } from 'lodash'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../../../../../../components/WebStory'
import type { LangStatsInsightContentResult } from '../../../../../../graphql-operations'
import { GET_LANG_STATS_GQL } from '../../../../core/hooks/live-preview-insight'
import { useCodeInsightsLicenseState } from '../../../../stores'

import { LangStatsInsightCreationPage as LangStatsInsightCreationPageComponent } from './LangStatsInsightCreationPage'

const defaultStory: Meta = {
    title: 'web/insights/creation-ui/lang-stats/LangStatsInsightCreationPage',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
    parameters: {
        chromatic: {
            viewports: [576, 1440],
            disableSnapshot: false,
        },
    },
}

export default defaultStory

const LANG_STATS_MOCK: MockedResponse<LangStatsInsightContentResult> = {
    request: {
        query: getDocumentNode(GET_LANG_STATS_GQL),
        variables: {},
    },
    result: {
        data: {
            search: {
                results: { __typename: 'SearchResults', limitHit: false },
                stats: {
                    languages: [
                        { name: 'JavaScript', totalLines: 1000 },
                        { name: 'TypeScript', totalLines: 2000 },
                        { name: 'Markdown', totalLines: 500 },
                        { name: 'HTML', totalLines: 100 },
                        { name: 'CSS', totalLines: 3000 },
                        { name: 'Rust', totalLines: 5000 },
                        { name: 'Julia', totalLines: 90 },
                        { name: 'Go', totalLines: 3000 },
                    ],
                },
            },
        },
    },
}

export const LangStatsInsightCreationPage: StoryFn = () => {
    useCodeInsightsLicenseState.setState({ licensed: true, insightsLimit: null })

    return (
        <MockedTestProvider addTypename={true} mocks={[LANG_STATS_MOCK]}>
            <LangStatsInsightCreationPageComponent
                backUrl="/insights/create"
                onInsightCreateRequest={async () => {
                    await delay(1000)
                    throw new Error('Network error')
                }}
                onCancel={noop}
                onSuccessfulCreation={noop}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </MockedTestProvider>
    )
}
