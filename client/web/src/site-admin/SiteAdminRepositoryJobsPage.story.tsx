import type { Decorator, Meta, StoryFn } from '@storybook/react'
import { subHours, subMinutes, subSeconds } from 'date-fns'
import { MATCH_ANY_PARAMETERS, WildcardMockLink } from 'wildcard-mock-link'

import { getDocumentNode } from '@sourcegraph/http-client'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../components/WebStory'
import {
    GitserversResult,
    GlobalRepositoryJobsResult,
    RepositoryBackendType,
    RepositoryJobScheduleReason,
    RepositoryJobState,
    RepositoryJobType,
} from '../graphql-operations'

import { GLOBAL_REPOSITORY_JOBS } from './backend'
import { SiteAdminRepositoryJobsPage } from './SiteAdminRepositoryJobsPage'

const decorator: Decorator = story => <div className="p-3 container">{story()}</div>

const config: Meta = {
    title: 'web/site-admin/RepositoryJobs',
    decorators: [decorator],
}

export default config

const gitFetchOutput = `remote: Enumerating objects: 10452, done.
remote: Counting objects: 100% (19/19), done.
remote: Compressing objects: 100% (16/16), done.
remote: Total 10452 (delta 4), reused 8 (delta 1), pack-reused 10433
Receiving objects: 100% (10452/10452), 20.72 MiB | 18.15 MiB/s, done.
Resolving deltas: 100% (6625/6625), done.
From https://github.com/sourcegraph/src-cli
 * [new branch]      3.10                                         -> 3.10
 * [new branch]      3.43                                         -> 3.43
 * [new branch]      main                                         -> main
`

const mockData: GlobalRepositoryJobsResult = {
    __typename: 'Query',
    repositoryJobs: {
        __typename: 'RepositoryJobConnection',
        totalCount: 10,
        pageInfo: { endCursor: null, hasNextPage: false, __typename: 'PageInfo' },
        nodes: [
            {
                __typename: 'RepositoryJob',
                id: 'UmVwb3NpdG9yeUpvYjox',
                backend: RepositoryBackendType.GIT_CLI,
                executionLogs: null,
                failureMessage: null,
                finishedAt: null,
                placeInQueue: 4,
                queuedAt: new Date().toISOString(),
                repository: {
                    __typename: 'Repository',
                    id: 'UmVwb3NpdG9yeTox',
                    name: 'github.com/sourcegraph/sourcegraph',
                    url: '/github.com/sourcegraph/sourcegraph',
                },
                scheduleReason: RepositoryJobScheduleReason.SCHEDULED,
                startedAt: null,
                state: RepositoryJobState.QUEUED,
                type: RepositoryJobType.CLONE,
                executionDeadlineSeconds: null,
            },
            {
                __typename: 'RepositoryJob',
                id: 'UmVwb3NpdG9yeUpvYjo0',
                backend: RepositoryBackendType.GIT_CLI,
                executionLogs: null,
                failureMessage: 'Failed to fetch repository, invalid URL httsp://github.com/sourcegraph/sourcegraph',
                finishedAt: subMinutes(new Date(), 2).toISOString(),
                placeInQueue: null,
                queuedAt: subMinutes(new Date(), 7).toISOString(),
                repository: {
                    __typename: 'Repository',
                    id: 'UmVwb3NpdG9yeTox',
                    name: 'github.com/sourcegraph/sourcegraph',
                    url: '/github.com/sourcegraph/sourcegraph',
                },
                scheduleReason: RepositoryJobScheduleReason.SCHEDULED,
                startedAt: subMinutes(new Date(), 6).toISOString(),
                state: RepositoryJobState.FAILED,
                type: RepositoryJobType.FETCH,
                executionDeadlineSeconds: null,
            },
            {
                __typename: 'RepositoryJob',
                id: 'UmVwb3NpdG9yeUpvYjoy',
                backend: RepositoryBackendType.GIT_CLI,
                executionLogs: [
                    {
                        __typename: 'ExecutionLogEntry',
                        startTime: subMinutes(new Date(), 2).toISOString(),
                        command: ['git', 'repack', '--d', '-l', '--cruft'],
                        durationMilliseconds: 4321,
                        exitCode: null,
                        key: 'git.repack',
                        out: `Enumerating objects: 2276, done.
Counting objects: 100% (2276/2276), done.
Delta compression using up to 10 threads
Compressing objects: 100% (2223/2223), done.
Writing objects: 100% (2276/2276), done.
Selecting bitmap commits: 227, done.
Building bitmaps: 100% (107/107), done.
Total 2276 (delta 1418), reused 0 (delta 0), pack-reused 0
Enumerating cruft objects: 5443, done.
Traversing cruft objects: 5443, done.
Counting objects: 100% (5443/5443), done.
Delta compression using up to 10 threads
Compressing objects: 100% (5364/5364), done.
Writing objects: 100% (5443/5443), done.
Total 5443 (delta 1353), reused 0 (delta 0), pack-reused 0
Removing duplicate objects: 100% (256/256), done.`,
                    },
                ],
                failureMessage: null,
                finishedAt: null,
                placeInQueue: null,
                queuedAt: new Date().toISOString(),
                repository: {
                    __typename: 'Repository',
                    id: 'UmVwb3NpdG9yeTox',
                    name: 'github.com/sourcegraph/src-cli',
                    url: '/github.com/sourcegraph/src-cli',
                },
                scheduleReason: RepositoryJobScheduleReason.USER,
                startedAt: new Date().toISOString(),
                state: RepositoryJobState.PROCESSING,
                type: RepositoryJobType.JANITOR,
                executionDeadlineSeconds: 3600,
            },
            {
                __typename: 'RepositoryJob',
                id: 'UmVwb3NpdG9yeUpvYjoz',
                backend: RepositoryBackendType.GIT_CLI,
                executionLogs: [
                    {
                        __typename: 'ExecutionLogEntry',
                        startTime: subMinutes(new Date(), 7).toISOString(),
                        command: ['git', 'init', '--bare', '.'],
                        durationMilliseconds: 1234,
                        exitCode: 0,
                        key: 'git.init',
                        out: 'Initialized empty Git repository in /tmp/repo\n',
                    },
                    {
                        __typename: 'ExecutionLogEntry',
                        startTime: subMinutes(new Date(), 6).toISOString(),
                        command: [
                            'git',
                            'fetch',
                            '--progress',
                            '--prune',
                            'https://git:<redacted>@github.com/sourcegraph/src-cli.git',
                            '+refs/heads/*:refs/heads/*',
                        ],
                        durationMilliseconds: 83123,
                        exitCode: 0,
                        key: 'git.fetch',
                        out: gitFetchOutput,
                    },
                ],
                failureMessage: null,
                finishedAt: subMinutes(new Date(), 3).toISOString(),
                placeInQueue: null,
                queuedAt: subMinutes(new Date(), 10).toISOString(),
                repository: {
                    __typename: 'Repository',
                    id: 'UmVwb3NpdG9yeTox',
                    name: 'github.com/sourcegraph/sourcegraph',
                    url: '/github.com/sourcegraph/sourcegraph',
                },
                scheduleReason: RepositoryJobScheduleReason.USER,
                startedAt: subMinutes(new Date(), 7).toISOString(),
                state: RepositoryJobState.COMPLETED,
                type: RepositoryJobType.RECLONE,
                executionDeadlineSeconds: null,
            },
            {
                __typename: 'RepositoryJob',
                id: 'UmVwb3NpdG9yeUpvYjo1',
                backend: RepositoryBackendType.GIT_CLI,
                failureMessage: null,
                finishedAt: subMinutes(new Date(), 3).toISOString(),
                placeInQueue: null,
                queuedAt: subMinutes(new Date(), 10).toISOString(),
                repository: {
                    __typename: 'Repository',
                    id: 'UmVwb3NpdG9yeTox',
                    name: 'github.com/sourcegraph/sourcegraph',
                    url: '/github.com/sourcegraph/sourcegraph',
                },
                scheduleReason: RepositoryJobScheduleReason.USER,
                startedAt: subMinutes(new Date(), 7).toISOString(),
                state: RepositoryJobState.CANCELED,
                type: RepositoryJobType.FETCH,
                executionLogs: null,
                executionDeadlineSeconds: null,
            },
        ],
    },
}

const mocks = new WildcardMockLink([
    {
        request: {
            query: getDocumentNode(GLOBAL_REPOSITORY_JOBS),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: {
            data: mockData,
        },
        nMatches: Number.POSITIVE_INFINITY,
    },
])

export const GitserversPage: StoryFn = () => (
    <WebStory>
        {() => (
            <MockedTestProvider link={mocks}>
                <SiteAdminRepositoryJobsPage telemetryService={NOOP_TELEMETRY_SERVICE} />
            </MockedTestProvider>
        )}
    </WebStory>
)

GitserversPage.storyName = 'Site Admin Repository Jobs Page'
