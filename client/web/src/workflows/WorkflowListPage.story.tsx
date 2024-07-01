import { ComponentProps } from 'react'

import type { Meta, StoryFn } from '@storybook/react'

import { MockedStoryProvider } from '@sourcegraph/shared/src/stories'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../components/WebStory'

import { MOCK_REQUESTS } from './mocks'
import { WorkflowListPage } from './WorkflowListPage'

const config: Meta = {
    title: 'web/workflows/WorkflowListPage',
    component: WorkflowListPage,
    decorators: [story => <div className="container mt-5">{story()}</div>],
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const commonProps: ComponentProps<typeof WorkflowListPage> = {
    namespace: {
        __typename: 'User',
        username: 'alice',
        namespaceName: 'alice',
        displayName: 'Alice',
        id: '',
        url: '',
    },
    telemetryRecorder: noOpTelemetryRecorder,
}

export const Default: StoryFn = () => (
    <WebStory>
        {webProps => (
            <MockedStoryProvider mocks={MOCK_REQUESTS}>
                <WorkflowListPage {...webProps} {...commonProps} />
            </MockedStoryProvider>
        )}
    </WebStory>
)
