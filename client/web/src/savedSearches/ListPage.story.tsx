import type { ComponentProps } from 'react'

import type { Meta, StoryFn } from '@storybook/react'

import { MockedStoryProvider } from '@sourcegraph/shared/src/stories'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import type { AuthenticatedUser } from '../auth'
import { WebStory } from '../components/WebStory'

import { MOCK_REQUESTS } from './graphql.mocks'
import { ListPage } from './ListPage'

const config: Meta = {
    title: 'web/savedSearches/ListPage',
    component: ListPage,
    decorators: [story => <div className="container mt-5">{story()}</div>],
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

// eslint-disable-next-line @typescript-eslint/consistent-type-assertions
const mockUser = {
    id: 'u',
    namespaceName: 'alice',
    organizations: {
        nodes: [
            { id: 'o1', namespaceName: 'org1' },
            { id: 'o2', namespaceName: 'org2' },
        ],
    },
} as AuthenticatedUser

const commonProps: ComponentProps<typeof ListPage> = {
    namespace: {
        __typename: 'User',
        username: 'alice',
        namespaceName: 'alice',
        displayName: 'Alice',
        id: '',
        url: '',
    },
    authenticatedUser: mockUser,
    telemetryRecorder: noOpTelemetryRecorder,
}

export const Default: StoryFn = () => (
    <WebStory>
        {webProps => (
            <MockedStoryProvider mocks={MOCK_REQUESTS}>
                <ListPage {...webProps} {...commonProps} />
            </MockedStoryProvider>
        )}
    </WebStory>
)
