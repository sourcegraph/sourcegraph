import { type ComponentProps } from 'react'

import type { Meta, StoryFn } from '@storybook/react'

import { MockedStoryProvider } from '@sourcegraph/shared/src/stories'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../components/WebStory'

import { CreateForm } from './CreateForm'
import { MOCK_REQUESTS } from './graphql.mocks'

const config: Meta = {
    title: 'web/savedSearches/CreateForm',
    component: CreateForm,
    decorators: [story => <div className="container mt-5">{story()}</div>],
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const commonProps: ComponentProps<typeof CreateForm> = {
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
                <CreateForm {...webProps} {...commonProps} />
            </MockedStoryProvider>
        )}
    </WebStory>
)
