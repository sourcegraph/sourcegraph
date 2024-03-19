import type { MockedResponse } from '@apollo/client/testing'
import type { Decorator, StoryFn, Meta } from '@storybook/react'

import { getDocumentNode } from '@sourcegraph/http-client'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../../components/WebStory'
import type {
    CheckMirrorRepositoryConnectionResult,
    CheckMirrorRepositoryConnectionVariables,
    SettingsAreaRepositoryFields,
    SettingsAreaRepositoryResult,
    SettingsAreaRepositoryVariables,
} from '../../graphql-operations'
import { CHECK_MIRROR_REPOSITORY_CONNECTION } from '../../site-admin/backend'

import { FETCH_SETTINGS_AREA_REPOSITORY_GQL } from './backend'
import { RepoSettingsMirrorPage } from './RepoSettingsMirrorPage'

const decorator: Decorator = story => <div className="p-3 container web-content">{story()}</div>

const config: Meta = {
    title: 'web/repo/settings/RepoSettingsMirrorPage',
    decorators: [decorator],
}

export default config

const repo: SettingsAreaRepositoryFields = {
    externalServices: { __typename: 'ExternalServiceConnection', nodes: [] },
    id: 'github.com/sourcegraph/sourcegraph',
    name: 'sourcegraph/sourcegraph',
    isPrivate: false,
    metadata: [],
    url: 'github.com/sourcegraph/sourcegraph',
    mirrorInfo: {
        __typename: 'MirrorRepositoryInfo',
        cloneInProgress: true,
        cloned: false,
        cloneProgress:
            'remote: Resolving deltas: 100% (10/10), completed with 10 local objects. remote: Resolving deltas: 100% (10/10), completed with 10 local objects. remote: Resolving deltas: 100% (10/10), completed with 10 local objects.',
        corruptionLogs: [],
        isCorrupted: false,
        lastError: null,
        lastSyncOutput: null,
        remoteURL: 'https://github.com/sourcegraph/sourcegraph',
        shard: 'gitserver-1',
        updatedAt: '2021-01-19T13:45:59Z',
        updateSchedule: {
            __typename: 'UpdateSchedule',
            due: '2021-01-19T13:45:59Z',
            index: 2,
            total: 100,
        },
        updateQueue: {
            __typename: 'UpdateQueue',
            index: 100,
            total: 1000,
            updating: true,
        },
    },
    viewerCanAdminister: true,
    __typename: 'Repository',
}

const FETCH_SETTINGS_AREA_REPOSITORY_MOCK: MockedResponse<
    SettingsAreaRepositoryResult,
    SettingsAreaRepositoryVariables
> = {
    request: {
        query: getDocumentNode(FETCH_SETTINGS_AREA_REPOSITORY_GQL),
        variables: {
            name: 'sourcegraph/sourcegraph',
        },
    },
    result: {
        data: {
            __typename: 'Query',
            repository: repo,
        },
    },
}

const CHECK_MIRROR_REPOSITORY_CONNECTION_MOCK: MockedResponse<
    CheckMirrorRepositoryConnectionResult,
    CheckMirrorRepositoryConnectionVariables
> = {
    request: {
        query: getDocumentNode(CHECK_MIRROR_REPOSITORY_CONNECTION),
        variables: {
            repository: 'github.com/sourcegraph/sourcegraph',
        },
    },
    result: {
        data: {
            __typename: 'Mutation',
            checkMirrorRepositoryConnection: {
                __typename: 'CheckMirrorRepositoryConnectionResult',
                error: null,
            },
        },
    },
}

export const CloneInProgress: StoryFn = args => (
    <WebStory>
        {() => (
            <MockedTestProvider mocks={[FETCH_SETTINGS_AREA_REPOSITORY_MOCK, CHECK_MIRROR_REPOSITORY_CONNECTION_MOCK]}>
                <RepoSettingsMirrorPage disablePolling={true} repo={repo} />
            </MockedTestProvider>
        )}
    </WebStory>
)
