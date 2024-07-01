import { ComponentProps } from 'react'

import type { Meta, StoryFn } from '@storybook/react'

import { MockedStoryProvider } from '@sourcegraph/shared/src/stories'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../components/WebStory'

import { MOCK_REQUESTS } from './mocks'
import { WorkflowCreateForm } from './WorkflowCreateForm'

const config: Meta = {
    title: 'web/workflows/WorkflowCreateForm',
    component: WorkflowCreateForm,
    decorators: [story => <div className="container mt-5">{story()}</div>],
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const commonProps: ComponentProps<typeof WorkflowCreateForm> = {
    isSourcegraphDotCom: false,
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
                <WorkflowCreateForm {...webProps} {...commonProps} />
            </MockedStoryProvider>
        )}
    </WebStory>
)
