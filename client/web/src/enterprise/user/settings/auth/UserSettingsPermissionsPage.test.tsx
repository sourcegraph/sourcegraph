import sinon from 'sinon'
import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider, waitForNextApolloResponse } from '@sourcegraph/shared/src/testing/apollo'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import {
    PermissionSource,
    PermissionsSyncJobPriority,
    PermissionsSyncJobReason,
    PermissionsSyncJobReasonGroup,
    CodeHostStatus,
    type PermissionsSyncJobsResult,
    type UserPermissionsInfoResult,
    PermissionsSyncJobState,
} from '../../../../graphql-operations'
import { PERMISSIONS_SYNC_JOBS_QUERY } from '../../../../site-admin/permissions-center/backend'

import { UserPermissionsInfoQuery } from './backend'
import { UserSettingsPermissionsPage } from './UserSettingsPermissionsPage'

const gqlUserID = 'VXNlcjox'

describe('UserSettingsPermissionsPage', () => {
    // mock current date time for consistent timestamps
    let sandbox: sinon.SinonSandbox

    beforeEach(() => {
        sandbox = sinon.createSandbox()
        const now = new Date('2023-08-08T12:25:12Z')
        sinon.useFakeTimers({
            now,
            shouldAdvanceTime: true,
            toFake: ['Date'],
        })
    })

    afterEach(() => {
        sandbox.restore()
    })

    test('empty state', async () => {
        const component = renderWithBrandedContext(
            <MockedTestProvider
                mocks={[
                    {
                        request: {
                            query: getDocumentNode(UserPermissionsInfoQuery),
                            variables: {
                                first: 20,
                                userID: gqlUserID,
                                query: '',
                                after: null,
                                before: null,
                                last: null,
                            },
                        },
                        result: EMPTY_USER_PERMISSIONS_RESPONSE,
                    },
                    {
                        request: {
                            query: getDocumentNode(PERMISSIONS_SYNC_JOBS_QUERY),
                            variables: {
                                first: 20,
                                reasonGroup: null,
                                state: null,
                                searchType: null,
                                query: '',
                                partial: false,
                                userID: 'VXNlcjox',
                                last: null,
                                after: null,
                                before: null,
                            },
                        },
                        result: EMPTY_SYNC_JOBS_RESPONSE,
                    },
                ]}
            >
                <UserSettingsPermissionsPage
                    user={{ id: gqlUserID, username: 'alice' }}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    telemetryRecorder={noOpTelemetryRecorder}
                />
            </MockedTestProvider>,
            {}
        )
        await waitForNextApolloResponse()
        await waitForNextApolloResponse()

        expect(component.asFragment()).toMatchSnapshot()
    })

    test('user alice with permissions', async () => {
        const component = renderWithBrandedContext(
            <MockedTestProvider
                mocks={[
                    {
                        request: {
                            query: getDocumentNode(UserPermissionsInfoQuery),
                            variables: {
                                first: 20,
                                userID: gqlUserID,
                                query: '',
                                after: null,
                                before: null,
                                last: null,
                            },
                        },
                        result: USER_PERMISSIONS_RESPONSE,
                    },
                    {
                        request: {
                            query: getDocumentNode(PERMISSIONS_SYNC_JOBS_QUERY),
                            variables: {
                                first: 20,
                                reasonGroup: null,
                                state: null,
                                searchType: null,
                                query: '',
                                partial: false,
                                userID: 'VXNlcjox',
                                last: null,
                                after: null,
                                before: null,
                            },
                        },
                        result: SYNC_JOBS_RESPONSE,
                    },
                ]}
            >
                <UserSettingsPermissionsPage
                    user={{ id: gqlUserID, username: 'alice' }}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    telemetryRecorder={noOpTelemetryRecorder}
                />
            </MockedTestProvider>,
            {}
        )
        await waitForNextApolloResponse()
        await waitForNextApolloResponse()

        expect(component.asFragment()).toMatchSnapshot()
    })
})

const EMPTY_SYNC_JOBS_RESPONSE: { data: PermissionsSyncJobsResult } = {
    data: {
        permissionsSyncJobs: {
            totalCount: 0,
            pageInfo: {
                hasNextPage: false,
                hasPreviousPage: false,
                startCursor: null,
                endCursor: null,
            },
            nodes: [],
        },
    },
}
const SYNC_JOBS_RESPONSE: { data: PermissionsSyncJobsResult } = {
    data: {
        permissionsSyncJobs: {
            totalCount: 6,
            pageInfo: {
                hasNextPage: false,
                hasPreviousPage: false,
                startCursor: '488155',
                endCursor: '488125',
            },
            nodes: [
                {
                    id: 'UGVybWlzc2lvbnNTeW5jSm9iOjQ4ODE1NQ==',
                    state: PermissionsSyncJobState.PROCESSING,
                    subject: {
                        __typename: 'User',
                        id: gqlUserID,
                        username: 'alice',
                        displayName: 'Alice Foo',
                        email: 'alice@foo.com',
                        avatarURL: null,
                    },
                    triggeredByUser: null,
                    reason: {
                        group: PermissionsSyncJobReasonGroup.SCHEDULE,
                        reason: PermissionsSyncJobReason.REASON_USER_OUTDATED_PERMS,
                        __typename: 'PermissionsSyncJobReasonWithGroup',
                    },
                    queuedAt: '2023-08-08T12:05:44Z',
                    startedAt: '2023-08-08T12:05:45Z',
                    finishedAt: null,
                    processAfter: null,
                    permissionsAdded: 0,
                    permissionsRemoved: 0,
                    permissionsFound: 0,
                    failureMessage: null,
                    cancellationReason: null,
                    ranForMs: 0,
                    numResets: 0,
                    numFailures: 0,
                    lastHeartbeatAt: '2023-08-08T12:05:45Z',
                    workerHostname: 'MacBook-Pro-10.local',
                    cancel: false,
                    priority: PermissionsSyncJobPriority.LOW,
                    noPerms: false,
                    invalidateCaches: false,
                    placeInQueue: null,
                    codeHostStates: [],
                    partialSuccess: false,
                    __typename: 'PermissionsSyncJob',
                },
                {
                    id: 'UGVybWlzc2lvbnNTeW5jSm9iOjQ4ODE1MA==',
                    state: PermissionsSyncJobState.COMPLETED,
                    subject: {
                        __typename: 'User',
                        id: gqlUserID,
                        username: 'alice',
                        displayName: 'Alice Foo',
                        email: 'alice@foo.com',
                        avatarURL: null,
                    },
                    triggeredByUser: null,
                    reason: {
                        group: PermissionsSyncJobReasonGroup.SCHEDULE,
                        reason: PermissionsSyncJobReason.REASON_USER_OUTDATED_PERMS,
                        __typename: 'PermissionsSyncJobReasonWithGroup',
                    },
                    queuedAt: '2023-08-08T12:23:29Z',
                    startedAt: '2023-08-08T12:23:31Z',
                    finishedAt: '2023-08-08T12:23:52Z',
                    processAfter: null,
                    permissionsAdded: 0,
                    permissionsRemoved: 0,
                    permissionsFound: 1,
                    failureMessage: null,
                    cancellationReason: null,
                    ranForMs: 3069,
                    numResets: 0,
                    numFailures: 0,
                    lastHeartbeatAt: '2023-08-08T12:23:52Z',
                    workerHostname: 'MacBook-Pro-10.local',
                    cancel: false,
                    priority: PermissionsSyncJobPriority.LOW,
                    noPerms: false,
                    invalidateCaches: false,
                    placeInQueue: null,
                    codeHostStates: [
                        {
                            providerID: 'https://github.com/',
                            providerType: 'github',
                            status: CodeHostStatus.SUCCESS,
                            message: 'FetchUserPerms',
                            __typename: 'CodeHostState',
                        },
                    ],
                    partialSuccess: false,
                    __typename: 'PermissionsSyncJob',
                },
                {
                    id: 'UGVybWlzc2lvbnNTeW5jSm9iOjQ4ODE0Mg==',
                    state: PermissionsSyncJobState.COMPLETED,
                    subject: {
                        __typename: 'User',
                        id: gqlUserID,
                        username: 'alice',
                        displayName: 'Alice Foo',
                        email: 'alice@foo.com',
                        avatarURL: null,
                    },
                    triggeredByUser: null,
                    reason: {
                        group: PermissionsSyncJobReasonGroup.SCHEDULE,
                        reason: PermissionsSyncJobReason.REASON_USER_OUTDATED_PERMS,
                        __typename: 'PermissionsSyncJobReasonWithGroup',
                    },
                    queuedAt: '2023-08-08T12:05:14Z',
                    startedAt: '2023-08-08T12:05:17Z',
                    finishedAt: '2023-08-08T12:05:19Z',
                    processAfter: null,
                    permissionsAdded: 0,
                    permissionsRemoved: 0,
                    permissionsFound: 1,
                    failureMessage: null,
                    cancellationReason: null,
                    ranForMs: 2887,
                    numResets: 0,
                    numFailures: 0,
                    lastHeartbeatAt: '2023-08-08T12:05:18Z',
                    workerHostname: 'MacBook-Pro-10.local',
                    cancel: false,
                    priority: PermissionsSyncJobPriority.LOW,
                    noPerms: false,
                    invalidateCaches: false,
                    placeInQueue: null,
                    codeHostStates: [
                        {
                            providerID: 'https://github.com/',
                            providerType: 'github',
                            status: CodeHostStatus.SUCCESS,
                            message: 'FetchUserPerms',
                            __typename: 'CodeHostState',
                        },
                    ],
                    partialSuccess: false,
                    __typename: 'PermissionsSyncJob',
                },
                {
                    id: 'UGVybWlzc2lvbnNTeW5jSm9iOjQ4ODEzOQ==',
                    state: PermissionsSyncJobState.COMPLETED,
                    subject: {
                        __typename: 'User',
                        id: gqlUserID,
                        username: 'alice',
                        displayName: 'Alice Foo',
                        email: 'alice@foo.com',
                        avatarURL: null,
                    },
                    triggeredByUser: null,
                    reason: {
                        group: PermissionsSyncJobReasonGroup.SCHEDULE,
                        reason: PermissionsSyncJobReason.REASON_USER_OUTDATED_PERMS,
                        __typename: 'PermissionsSyncJobReasonWithGroup',
                    },
                    queuedAt: '2023-08-08T12:04:59Z',
                    startedAt: '2023-08-08T12:05:01Z',
                    finishedAt: '2023-08-08T12:05:04Z',
                    processAfter: null,
                    permissionsAdded: 0,
                    permissionsRemoved: 0,
                    permissionsFound: 1,
                    failureMessage: null,
                    cancellationReason: null,
                    ranForMs: 2925,
                    numResets: 0,
                    numFailures: 0,
                    lastHeartbeatAt: '2023-08-08T12:05:01Z',
                    workerHostname: 'MacBook-Pro-10.local',
                    cancel: false,
                    priority: PermissionsSyncJobPriority.LOW,
                    noPerms: false,
                    invalidateCaches: false,
                    placeInQueue: null,
                    codeHostStates: [
                        {
                            providerID: 'https://github.com/',
                            providerType: 'github',
                            status: CodeHostStatus.SUCCESS,
                            message: 'FetchUserPerms',
                            __typename: 'CodeHostState',
                        },
                    ],
                    partialSuccess: false,
                    __typename: 'PermissionsSyncJob',
                },
                {
                    id: 'UGVybWlzc2lvbnNTeW5jSm9iOjQ4ODEzMg==',
                    state: PermissionsSyncJobState.COMPLETED,
                    subject: {
                        __typename: 'User',
                        id: gqlUserID,
                        username: 'alice',
                        displayName: 'Alice Foo',
                        email: 'alice@foo.com',
                        avatarURL: null,
                    },
                    triggeredByUser: null,
                    reason: {
                        group: PermissionsSyncJobReasonGroup.SCHEDULE,
                        reason: PermissionsSyncJobReason.REASON_USER_OUTDATED_PERMS,
                        __typename: 'PermissionsSyncJobReasonWithGroup',
                    },
                    queuedAt: '2023-08-08T12:04:44Z',
                    startedAt: '2023-08-08T12:04:45Z',
                    finishedAt: '2023-08-08T12:04:48Z',
                    processAfter: null,
                    permissionsAdded: 0,
                    permissionsRemoved: 0,
                    permissionsFound: 1,
                    failureMessage: null,
                    cancellationReason: null,
                    ranForMs: 2730,
                    numResets: 0,
                    numFailures: 0,
                    lastHeartbeatAt: '2023-08-08T12:04:45Z',
                    workerHostname: 'MacBook-Pro-10.local',
                    cancel: false,
                    priority: PermissionsSyncJobPriority.LOW,
                    noPerms: false,
                    invalidateCaches: false,
                    placeInQueue: null,
                    codeHostStates: [
                        {
                            providerID: 'https://github.com/',
                            providerType: 'github',
                            status: CodeHostStatus.SUCCESS,
                            message: 'FetchUserPerms',
                            __typename: 'CodeHostState',
                        },
                    ],
                    partialSuccess: false,
                    __typename: 'PermissionsSyncJob',
                },
                {
                    id: 'UGVybWlzc2lvbnNTeW5jSm9iOjQ4ODEyNQ==',
                    state: PermissionsSyncJobState.COMPLETED,
                    subject: {
                        __typename: 'User',
                        id: gqlUserID,
                        username: 'alice',
                        displayName: 'Alice Foo',
                        email: 'alice@foo.com',
                        avatarURL: null,
                    },
                    triggeredByUser: null,
                    reason: {
                        group: PermissionsSyncJobReasonGroup.SCHEDULE,
                        reason: PermissionsSyncJobReason.REASON_USER_OUTDATED_PERMS,
                        __typename: 'PermissionsSyncJobReasonWithGroup',
                    },
                    queuedAt: '2023-08-08T12:04:29Z',
                    startedAt: '2023-08-08T12:04:30Z',
                    finishedAt: '2023-08-08T12:04:33Z',
                    processAfter: null,
                    permissionsAdded: 0,
                    permissionsRemoved: 0,
                    permissionsFound: 1,
                    failureMessage: null,
                    cancellationReason: null,
                    ranForMs: 2695,
                    numResets: 0,
                    numFailures: 0,
                    lastHeartbeatAt: '2023-08-08T12:04:30Z',
                    workerHostname: 'MacBook-Pro-10.local',
                    cancel: false,
                    priority: PermissionsSyncJobPriority.LOW,
                    noPerms: false,
                    invalidateCaches: false,
                    placeInQueue: null,
                    codeHostStates: [
                        {
                            providerID: 'https://github.com/',
                            providerType: 'github',
                            status: CodeHostStatus.SUCCESS,
                            message: 'FetchUserPerms',
                            __typename: 'CodeHostState',
                        },
                    ],
                    partialSuccess: false,
                    __typename: 'PermissionsSyncJob',
                },
            ],
            __typename: 'PermissionsSyncJobsConnection',
        },
    },
}

const EMPTY_USER_PERMISSIONS_RESPONSE: { data: UserPermissionsInfoResult } = {
    data: {
        node: {
            __typename: 'User',
            permissionsInfo: {
                updatedAt: null,
                source: null,
                repositories: {
                    totalCount: 0,
                    nodes: [],
                    pageInfo: {
                        hasNextPage: false,
                        hasPreviousPage: false,
                        startCursor: null,
                        endCursor: null,
                    },
                },
            },
        },
    },
}

const USER_PERMISSIONS_RESPONSE: { data: UserPermissionsInfoResult } = {
    data: {
        node: {
            __typename: 'User',
            permissionsInfo: {
                updatedAt: '2023-08-08T12:23:52Z',
                source: PermissionSource.USER_SYNC,
                repositories: {
                    nodes: [
                        {
                            id: 'UmVwb3NpdG9yeTox',
                            reason: 'Unrestricted',
                            updatedAt: null,
                            repository: {
                                id: 'UmVwb3NpdG9yeTox',
                                name: 'github.com/hashicorp/errwrap',
                                url: '/github.com/hashicorp/errwrap',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeToy',
                            reason: 'Unrestricted',
                            updatedAt: null,
                            repository: {
                                id: 'UmVwb3NpdG9yeToy',
                                name: 'github.com/sourcegraph-testing/etcd',
                                url: '/github.com/sourcegraph-testing/etcd',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeToz',
                            reason: 'Unrestricted',
                            updatedAt: null,
                            repository: {
                                id: 'UmVwb3NpdG9yeToz',
                                name: 'github.com/sourcegraph-testing/tidb',
                                url: '/github.com/sourcegraph-testing/tidb',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeTo0',
                            reason: 'Unrestricted',
                            updatedAt: null,
                            repository: {
                                id: 'UmVwb3NpdG9yeTo0',
                                name: 'github.com/sourcegraph-testing/titan',
                                url: '/github.com/sourcegraph-testing/titan',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeTo1',
                            reason: 'Unrestricted',
                            updatedAt: null,
                            repository: {
                                id: 'UmVwb3NpdG9yeTo1',
                                name: 'github.com/sourcegraph-testing/zap',
                                url: '/github.com/sourcegraph-testing/zap',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeTo4',
                            reason: 'Site Admin',
                            updatedAt: '2022-07-08T10:42:25Z',
                            repository: {
                                id: 'UmVwb3NpdG9yeTo4',
                                name: 'github.com/alice/acme-destroy',
                                url: '/github.com/alice/acme-destroy',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeTo5',
                            reason: 'Explicit API',
                            updatedAt: '2023-04-01T03:17:43Z',
                            repository: {
                                id: 'UmVwb3NpdG9yeTo5',
                                name: 'gitlab.com/alice/acme-frontend',
                                url: '/gitlab.com/alice/acme-frontend',
                                externalRepository: {
                                    serviceType: 'gitlab',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                        {
                            id: 'UmVwb3NpdG9yeToxMQ==',
                            reason: 'Permissions Sync',
                            updatedAt: '2023-08-08T12:23:52Z',
                            repository: {
                                id: 'UmVwb3NpdG9yeToxMQ==',
                                name: 'github.com/alice/acme-api',
                                url: '/github.com/alice/acme-api',
                                externalRepository: {
                                    serviceType: 'github',
                                    __typename: 'ExternalRepository',
                                },
                                __typename: 'Repository',
                            },
                            __typename: 'PermissionsInfoRepositoryNode',
                        },
                    ],
                    totalCount: 8,
                    pageInfo: {
                        hasNextPage: false,
                        hasPreviousPage: false,
                        startCursor: 'github.com/hashicorp/errwrap',
                        endCursor: 'github.com/alice/acme-api',
                        __typename: 'ConnectionPageInfo',
                    },
                    __typename: 'PermissionsInfoRepositoriesConnection',
                },
                __typename: 'PermissionsInfo',
            },
        },
    },
}
