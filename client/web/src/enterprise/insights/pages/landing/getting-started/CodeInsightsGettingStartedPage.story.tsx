import type { MockedResponse } from '@apollo/client/testing'
import type { Meta } from '@storybook/react'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../../../../../components/WebStory'
import type { GetExampleRepositoryResult } from '../../../../../graphql-operations'

import { CodeInsightsGettingStartedPage } from './CodeInsightsGettingStartedPage'
import { GET_EXAMPLE_REPOSITORY } from './components/dynamic-code-insight-example/DynamicCodeInsightExample'

const config: Meta = {
    title: 'web/insights/getting-started/CodeInsightsGettingStartedPage',
    decorators: [
        story => (
            <div className="container py-5">
                <WebStory>{() => story()}</WebStory>
            </div>
        ),
    ],
}

export default config

const FirstExampleRepositoryMock: MockedResponse<GetExampleRepositoryResult> = {
    request: { query: getDocumentNode(GET_EXAMPLE_REPOSITORY) },
    result: {
        data: {
            firstRepo: {
                results: {
                    repositories: [{ name: 'github.com/first-repo-url' }],
                },
            },
            todoRepo: {
                results: {
                    repositories: [],
                },
            },
        },
    },
}

export const CodeInsightsGettingStartedPageStory = () => (
    <MockedTestProvider mocks={[FirstExampleRepositoryMock]}>
        <CodeInsightsGettingStartedPage
            telemetryService={NOOP_TELEMETRY_SERVICE}
            telemetryRecorder={noOpTelemetryRecorder}
        />
    </MockedTestProvider>
)
