import type { Decorator, Meta, StoryFn } from '@storybook/react'

import type { OrgSettingFields, UserSettingFields } from '@sourcegraph/shared/src/graphql-operations'
import { EMPTY_SETTINGS_CASCADE } from '@sourcegraph/shared/src/settings/settings'

import type { AuthenticatedUser } from '../../../auth'
import { WebStory } from '../../../components/WebStory'

import { CreateBatchChangePage } from './CreateBatchChangePage'

const decorator: Decorator = story => (
    <div className="p-3" style={{ height: '95vh', width: '100%' }}>
        {story()}
    </div>
)

const config: Meta = {
    title: 'web/batches/create/CreateBatchChangePage',
    decorators: [decorator],
    parameters: {
        chromatic: {
            disableSnapshot: false,
        },
    },
}

export default config

const MOCK_ORGANIZATION = {
    __typename: 'Org',
    name: 'acme-corp',
    displayName: 'ACME Corporation',
    id: 'acme-corp-id',
}

const mockAuthenticatedUser = {
    __typename: 'User',
    username: 'alice',
    displayName: 'alice',
    id: 'b',
    organizations: {
        nodes: [MOCK_ORGANIZATION],
    },
} as AuthenticatedUser

export const ExperimentalExecutionDisabled: StoryFn = () => (
    <WebStory>
        {props => (
            <CreateBatchChangePage
                {...props}
                headingElement="h1"
                settingsCascade={{
                    ...EMPTY_SETTINGS_CASCADE,
                    final: { experimentalFeatures: { batchChangesExecution: false } },
                }}
            />
        )}
    </WebStory>
)

ExperimentalExecutionDisabled.storyName = 'Experimental execution disabled'

const FIXTURE_ORG: OrgSettingFields = {
    __typename: 'Org',
    name: 'sourcegraph',
    displayName: 'Sourcegraph',
    id: 'a',
    viewerCanAdminister: true,
    settingsURL: null,
    latestSettings: null,
}

const FIXTURE_USER: UserSettingFields = {
    __typename: 'User',
    username: 'alice',
    displayName: 'alice',
    id: 'b',
    viewerCanAdminister: true,
    settingsURL: null,
    latestSettings: null,
}

export const ExperimentalExecutionEnabled: StoryFn = () => (
    <WebStory>
        {props => (
            <CreateBatchChangePage
                {...props}
                headingElement="h1"
                settingsCascade={{
                    ...EMPTY_SETTINGS_CASCADE,
                    subjects: [
                        { subject: FIXTURE_ORG, settings: { a: 1 }, lastID: 1 },
                        { subject: FIXTURE_USER, settings: { b: 2 }, lastID: 2 },
                    ],
                }}
            />
        )}
    </WebStory>
)

ExperimentalExecutionEnabled.storyName = 'Experimental execution enabled'

export const ExperimentalExecutionEnabledFromOrgNamespace: StoryFn = () => (
    <WebStory>
        {props => (
            <CreateBatchChangePage
                {...props}
                headingElement="h1"
                initialNamespaceID={MOCK_ORGANIZATION.id}
                settingsCascade={{
                    ...EMPTY_SETTINGS_CASCADE,
                    final: {
                        experimentalFeatures: { batchChangesExecution: true },
                    },
                    subjects: [
                        { subject: FIXTURE_ORG, settings: { a: 1 }, lastID: 1 },
                        { subject: FIXTURE_USER, settings: { b: 2 }, lastID: 2 },
                    ],
                }}
            />
        )}
    </WebStory>
)

ExperimentalExecutionEnabledFromOrgNamespace.storyName = 'Experimental execution enabled from org namespace'

export const ExperimentalExecutionEnabledFromUserNamespace: StoryFn = () => (
    <WebStory>
        {props => (
            <CreateBatchChangePage
                {...props}
                headingElement="h1"
                initialNamespaceID={mockAuthenticatedUser.id}
                settingsCascade={{
                    ...EMPTY_SETTINGS_CASCADE,
                    final: {
                        experimentalFeatures: { batchChangesExecution: true },
                    },
                    subjects: [
                        { subject: FIXTURE_ORG, settings: { a: 1 }, lastID: 1 },
                        { subject: FIXTURE_USER, settings: { b: 2 }, lastID: 2 },
                    ],
                }}
            />
        )}
    </WebStory>
)

ExperimentalExecutionEnabledFromUserNamespace.storyName = 'Experimental execution enabled from user namespace'
