import type { Dispatch, SetStateAction } from 'react'

import type { QueryTuple, MutationTuple, QueryResult } from '@apollo/client'
import { parse } from 'jsonc-parser'
import { type Observable, lastValueFrom } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError } from '@sourcegraph/common'
import { gql, dataOrThrowErrors, useMutation, useLazyQuery, useQuery } from '@sourcegraph/http-client'

import { requestGraphQL } from '../../backend/graphql'
import type {
    UpdateExternalServiceResult,
    UpdateExternalServiceVariables,
    Scalars,
    AddExternalServiceVariables,
    AddExternalServiceResult,
    DeleteExternalServiceVariables,
    DeleteExternalServiceResult,
    ExternalServicesVariables,
    ExternalServicesResult,
    ExternalServiceCheckConnectionByIdVariables,
    ExternalServiceCheckConnectionByIdResult,
    SyncExternalServiceResult,
    SyncExternalServiceVariables,
    ExternalServiceSyncJobsVariables,
    ExternalServiceSyncJobConnectionFields,
    ExternalServiceSyncJobsResult,
    CancelExternalServiceSyncVariables,
    CancelExternalServiceSyncResult,
    ListExternalServiceFields,
    ExternalServiceFields,
    ExternalServiceResult,
    ExternalServiceVariables,
} from '../../graphql-operations'
import {
    useShowMorePagination,
    type UseShowMorePaginationResult,
} from '../FilteredConnection/hooks/useShowMorePagination'

const RATE_LIMITER_STATE_FRAGMENT = gql`
    fragment RateLimiterStateFields on RateLimiterState {
        __typename
        currentCapacity
        burst
        limit
        interval
        lastReplenishment
        infinite
    }
`

export const externalServiceFragment = gql`
    ${RATE_LIMITER_STATE_FRAGMENT}
    fragment ExternalServiceFields on ExternalService {
        id
        kind
        displayName
        config
        warning
        lastSyncError
        rateLimiterState {
            ...RateLimiterStateFields
        }
        repoCount
        lastSyncAt
        nextSyncAt
        updatedAt
        createdAt
        creator {
            username
            url
        }
        lastUpdater {
            username
            url
        }
        webhookURL
        hasConnectionCheck
        unrestricted
    }
`

export const ADD_EXTERNAL_SERVICE = gql`
    mutation AddExternalService($input: AddExternalServiceInput!) {
        addExternalService(input: $input) {
            ...ExternalServiceFields
        }
    }

    ${externalServiceFragment}
`

export const useAddExternalService = (): MutationTuple<AddExternalServiceResult, AddExternalServiceVariables> =>
    useMutation<AddExternalServiceResult, AddExternalServiceVariables>(ADD_EXTERNAL_SERVICE)

export const UPDATE_EXTERNAL_SERVICE = gql`
    mutation UpdateExternalService($input: UpdateExternalServiceInput!) {
        updateExternalService(input: $input) {
            ...ExternalServiceFields
        }
    }
    ${externalServiceFragment}
`

export const useUpdateExternalService = (
    onCompleted: (result: UpdateExternalServiceResult) => void
): MutationTuple<UpdateExternalServiceResult, UpdateExternalServiceVariables> =>
    useMutation(UPDATE_EXTERNAL_SERVICE, { onCompleted })

export function updateExternalService(
    variables: UpdateExternalServiceVariables
): Promise<UpdateExternalServiceResult['updateExternalService']> {
    return lastValueFrom(
        requestGraphQL<UpdateExternalServiceResult, UpdateExternalServiceVariables>(
            UPDATE_EXTERNAL_SERVICE,
            variables
        ).pipe(
            map(dataOrThrowErrors),
            map(data => data.updateExternalService)
        )
    )
}

export async function deleteExternalService(externalService: Scalars['ID']): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<DeleteExternalServiceResult, DeleteExternalServiceVariables>(
            gql`
                mutation DeleteExternalService($externalService: ID!) {
                    deleteExternalService(externalService: $externalService) {
                        alwaysNil
                    }
                }
            `,
            { externalService }
        )
    )
    dataOrThrowErrors(result)
}

export const EXTERNAL_SERVICE_CHECK_CONNECTION_BY_ID = gql`
    query ExternalServiceCheckConnectionById($id: ID!) {
        node(id: $id) {
            __typename
            ... on ExternalService {
                id
                hasConnectionCheck
                checkConnection {
                    __typename
                    ... on ExternalServiceUnavailable {
                        suspectedReason
                    }
                }
            }
        }
    }
`

export const useExternalServiceCheckConnectionByIdLazyQuery = (
    id: string
): QueryTuple<ExternalServiceCheckConnectionByIdResult, ExternalServiceCheckConnectionByIdVariables> =>
    useLazyQuery<ExternalServiceCheckConnectionByIdResult, ExternalServiceCheckConnectionByIdVariables>(
        EXTERNAL_SERVICE_CHECK_CONNECTION_BY_ID,
        {
            variables: { id },
        }
    )

const EXTERNAL_SERVICE_SYNC_JOB_LIST_FIELDS_FRAGMENT = gql`
    fragment ExternalServiceSyncJobListFields on ExternalServiceSyncJob {
        __typename
        id
        state
        startedAt
        finishedAt
        failureMessage
        reposSynced
        repoSyncErrors
        reposAdded
        reposDeleted
        reposModified
        reposUnmodified
    }
`

const EXTERNAL_SERVICE_SYNC_JOB_CONNECTION_FIELDS_FRAGMENT = gql`
    ${EXTERNAL_SERVICE_SYNC_JOB_LIST_FIELDS_FRAGMENT}
    fragment ExternalServiceSyncJobConnectionFields on ExternalServiceSyncJobConnection {
        totalCount
        pageInfo {
            endCursor
            hasNextPage
        }
        nodes {
            ...ExternalServiceSyncJobListFields
        }
    }
`

export const EXTERNAL_SERVICE_SYNC_JOBS = gql`
    ${EXTERNAL_SERVICE_SYNC_JOB_CONNECTION_FIELDS_FRAGMENT}
    query ExternalServiceSyncJobs($first: Int, $externalService: ID!) {
        node(id: $externalService) {
            __typename
            ... on ExternalService {
                syncJobs(first: $first) {
                    ...ExternalServiceSyncJobConnectionFields
                }
            }
        }
    }
`

export const LIST_EXTERNAL_SERVICE_FRAGMENT = gql`
    ${EXTERNAL_SERVICE_SYNC_JOB_CONNECTION_FIELDS_FRAGMENT}
    ${RATE_LIMITER_STATE_FRAGMENT}
    fragment ListExternalServiceFields on ExternalService {
        id
        kind
        displayName
        rateLimiterState {
            ...RateLimiterStateFields
        }
        config
        warning
        lastSyncError
        repoCount
        lastSyncAt
        nextSyncAt
        updatedAt
        createdAt
        creator {
            username
            url
        }
        lastUpdater {
            username
            url
        }
        webhookURL
        hasConnectionCheck
        syncJobs(first: 1) {
            ...ExternalServiceSyncJobConnectionFields
        }
        unrestricted
    }
`

export const FETCH_EXTERNAL_SERVICE = gql`
    query ExternalService($id: ID!) {
        node(id: $id) {
            __typename
            ...ListExternalServiceFields
        }
    }
    ${LIST_EXTERNAL_SERVICE_FRAGMENT}
`

export const EXTERNAL_SERVICES = gql`
    query ExternalServices($first: Int, $after: String, $repo: ID) {
        externalServices(first: $first, after: $after, repo: $repo) {
            nodes {
                ...ListExternalServiceFields
            }
            totalCount
            pageInfo {
                endCursor
                hasNextPage
            }
        }
    }

    ${LIST_EXTERNAL_SERVICE_FRAGMENT}
`

export const EXTERNAL_SERVICE_IDS_AND_NAMES = gql`
    query ExternalServiceIDsAndNames {
        externalServices {
            nodes {
                id
                displayName
            }
        }
    }
`

export const useExternalServicesConnection = (
    vars: ExternalServicesVariables
): UseShowMorePaginationResult<ExternalServicesResult, ListExternalServiceFields> =>
    useShowMorePagination<ExternalServicesResult, ExternalServicesVariables, ListExternalServiceFields>({
        query: EXTERNAL_SERVICES,
        variables: { after: vars.after, first: vars.first ?? 10, repo: vars.repo },
        getConnection: result => {
            const { externalServices } = dataOrThrowErrors(result)
            return externalServices
        },
        options: {
            fetchPolicy: 'cache-and-network',
            pollInterval: 15000,
        },
    })

export const SYNC_EXTERNAL_SERVICE = gql`
    mutation SyncExternalService($id: ID!) {
        syncExternalService(id: $id) {
            alwaysNil
        }
    }
`

export function useSyncExternalService(): MutationTuple<SyncExternalServiceResult, SyncExternalServiceVariables> {
    return useMutation<SyncExternalServiceResult, SyncExternalServiceVariables>(SYNC_EXTERNAL_SERVICE)
}

export const CANCEL_EXTERNAL_SERVICE_SYNC = gql`
    mutation CancelExternalServiceSync($id: ID!) {
        cancelExternalServiceSync(id: $id) {
            alwaysNil
        }
    }
`

export function useCancelExternalServiceSync(): MutationTuple<
    CancelExternalServiceSyncResult,
    CancelExternalServiceSyncVariables
> {
    return useMutation<CancelExternalServiceSyncResult, CancelExternalServiceSyncVariables>(
        CANCEL_EXTERNAL_SERVICE_SYNC
    )
}

export function queryExternalServiceSyncJobs(
    variables: ExternalServiceSyncJobsVariables
): Observable<ExternalServiceSyncJobConnectionFields> {
    return requestGraphQL<ExternalServiceSyncJobsResult, ExternalServiceSyncJobsVariables>(
        EXTERNAL_SERVICE_SYNC_JOBS,
        variables
    ).pipe(
        map(({ data, errors }) => {
            if (errors) {
                throw createAggregateError(errors)
            }
            if (!data) {
                throw new Error('No data found')
            }
            if (!data.node) {
                throw new Error('External service not found')
            }
            if (data.node.__typename !== 'ExternalService') {
                throw new Error(`Node is a ${data.node.__typename}, not ExternalService`)
            }
            return data.node.syncJobs
        })
    )
}

export const getExternalService = (
    data?: ExternalServiceFields | null
): ExternalServiceFieldsWithConfig | undefined => {
    if (!data) {
        return undefined
    }
    const node: ExternalServiceFieldsWithConfig = data
    node.parsedConfig = parse(node.config) as ExternalServiceFieldsWithConfig['parsedConfig']
    return node
}

export const useFetchExternalService = (
    externalServiceID: string,
    setExternalService: Dispatch<SetStateAction<ExternalServiceFieldsWithConfig | undefined>>
): QueryResult<ExternalServiceResult, ExternalServiceVariables> =>
    useQuery<ExternalServiceResult, ExternalServiceVariables>(FETCH_EXTERNAL_SERVICE, {
        variables: { id: externalServiceID },
        notifyOnNetworkStatusChange: false,
        fetchPolicy: 'no-cache',
        onCompleted: (result: ExternalServiceResult) => {
            if (result?.node?.__typename !== 'ExternalService') {
                return
            }
            const data = getExternalService(result?.node)
            if (data) {
                setExternalService(data)
            }
        },
    })

export interface GitHubAppDetails {
    appID: number
    baseURL: string
    installationID: number
}

export interface ExternalServiceFieldsWithConfig extends ExternalServiceFields {
    parsedConfig?: {
        gitHubAppDetails?: GitHubAppDetails
        url: string
    }
}

export type RateLimiterState = NonNullable<ExternalServiceFields['rateLimiterState']>
