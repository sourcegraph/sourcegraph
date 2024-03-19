import type { MockedResponse } from '@apollo/client/testing'
import type { Decorator, StoryFn, Meta } from '@storybook/react'

import { getDocumentNode } from '@sourcegraph/http-client'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../../../components/WebStory'
import type {
    RepositoryRecordedCommandsResult,
    RepositoryRecordedCommandsVariables,
    SettingsAreaRepositoryFields,
} from '../../../graphql-operations'

import { REPOSITORY_RECORDED_COMMANDS_QUERY } from './backend'
import { RepoSettingsLogsPage } from './RepoSettingsLogsPage'

const decorator: Decorator = story => <div className="p-3 container web-content">{story()}</div>

const config: Meta = {
    title: 'web/repo/settings/RepoSettingsLogsPage',
    decorators: [decorator],
}

export default config

const REPOSITORY_RECORDED_COMMANDS_MOCK: MockedResponse<
    RepositoryRecordedCommandsResult,
    RepositoryRecordedCommandsVariables
> = {
    request: {
        query: getDocumentNode(REPOSITORY_RECORDED_COMMANDS_QUERY),
        variables: {
            id: 'github.com/sourcegraph/sourcegraph',
            limit: 40,
            offset: 0,
        },
    },
    result: {
        data: {
            __typename: 'Query',
            node: {
                __typename: 'Repository',
                isRecordingEnabled: true,
                recordedCommands: {
                    __typename: 'RecordedCommandConnection',
                    totalCount: 2,
                    pageInfo: { hasNextPage: false, __typename: 'PageInfo' },
                    nodes: [
                        {
                            __typename: 'RecordedCommand',
                            command: 'git rev-parse HEAD',
                            dir: '/data/repos/sourcegraph/sourcegraph',
                            duration: 1200,
                            isSuccess: true,
                            output: '0123456789abcdef',
                            path: '/data/repos/sourcegraph/sourcegraph/.git/HEAD',
                            start: '2021-05-03T18:30:00Z',
                        },
                        {
                            __typename: 'RecordedCommand',
                            command: 'git symbolic-ref HEAD',
                            dir: '/data/repos/sourcegraph/sourcegraph',
                            duration: 1200,
                            isSuccess: false,
                            output: 'refs/heads/main',
                            path: '/data/repos/sourcegraph/sourcegraph/.git/HEAD',
                            start: '2021-05-03T18:30:00Z',
                        },
                    ],
                },
            },
        },
    },
}

const REPOSITORY_RECORDED_COMMANDS_DISABLED_MOCK: MockedResponse<
    RepositoryRecordedCommandsResult,
    RepositoryRecordedCommandsVariables
> = {
    request: {
        query: getDocumentNode(REPOSITORY_RECORDED_COMMANDS_QUERY),
        variables: {
            id: 'github.com/sourcegraph/sourcegraph',
            limit: 40,
            offset: 0,
        },
    },
    result: {
        data: {
            __typename: 'Query',
            node: {
                __typename: 'Repository',
                isRecordingEnabled: false,
                recordedCommands: {
                    __typename: 'RecordedCommandConnection',
                    totalCount: 0,
                    pageInfo: { hasNextPage: false, __typename: 'PageInfo' },
                    nodes: [],
                },
            },
        },
    },
}

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
        cloneProgress: 'remote: Resolving deltas: 100% (10/10), completed with 10 local objects.',
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

export const WithLogs: StoryFn = args => (
    <WebStory>
        {() => (
            <MockedTestProvider mocks={[REPOSITORY_RECORDED_COMMANDS_MOCK]}>
                <RepoSettingsLogsPage repo={repo} />
            </MockedTestProvider>
        )}
    </WebStory>
)

export const Disabled: StoryFn = args => (
    <WebStory>
        {() => (
            <MockedTestProvider mocks={[REPOSITORY_RECORDED_COMMANDS_DISABLED_MOCK]}>
                <RepoSettingsLogsPage repo={repo} />
            </MockedTestProvider>
        )}
    </WebStory>
)
