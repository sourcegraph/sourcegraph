import type { QueryResult, QueryTuple } from '@apollo/client'
import { EMPTY, lastValueFrom, type Observable } from 'rxjs'
import { expand, map, reduce } from 'rxjs/operators'

import { dataOrThrowErrors, gql, useLazyQuery, useQuery } from '@sourcegraph/http-client'

import { diffStatFields, fileDiffFields } from '../../../backend/diff'
import { requestGraphQL } from '../../../backend/graphql'
import type {
    BatchChangeChangesetsVariables,
    BatchChangeChangesetsResult,
    BatchChangeFields,
    BatchChangeByNamespaceResult,
    BatchChangeByNamespaceVariables,
    ExternalChangesetFileDiffsResult,
    ExternalChangesetFileDiffsVariables,
    ExternalChangesetFileDiffsFields,
    SyncChangesetResult,
    SyncChangesetVariables,
    Scalars,
    ChangesetCountsOverTimeVariables,
    ChangesetCountsOverTimeResult,
    DeleteBatchChangeResult,
    ChangesetDiffResult,
    ChangesetDiffVariables,
    ReenqueueChangesetVariables,
    ReenqueueChangesetResult,
    ChangesetFields,
    DeleteBatchChangeVariables,
    DetachChangesetsVariables,
    DetachChangesetsResult,
    ChangesetScheduleEstimateResult,
    ChangesetScheduleEstimateVariables,
    CreateChangesetCommentsResult,
    CreateChangesetCommentsVariables,
    AllChangesetIDsResult,
    AllChangesetIDsVariables,
    ChangesetIDConnectionFields,
    ReenqueueChangesetsResult,
    ReenqueueChangesetsVariables,
    MergeChangesetsResult,
    MergeChangesetsVariables,
    CloseChangesetsResult,
    CloseChangesetsVariables,
    PublishChangesetsResult,
    PublishChangesetsVariables,
    AvailableBulkOperationsVariables,
    AvailableBulkOperationsResult,
    BulkOperationType,
    GetChangesetsByIDsResult,
    GetChangesetsByIDsVariables,
} from '../../../graphql-operations'
import { VIEWER_BATCH_CHANGES_CODE_HOST_FRAGMENT } from '../MissingCredentialsAlert'

const changesetsStatsFragment = gql`
    fragment ChangesetsStatsFields on ChangesetsStats {
        __typename
        total
        closed
        deleted
        draft
        merged
        open
        unpublished
        archived
        isCompleted
        percentComplete
        failed
        retrying
        scheduled
        processing
    }
`

const bulkOperationFragment = gql`
    fragment BulkOperationFields on BulkOperation {
        __typename
        id
        type
        state
        progress
        errors {
            changeset {
                __typename
                ... on ExternalChangeset {
                    title
                    externalURL {
                        url
                    }
                    repository {
                        id
                        name
                        url
                    }
                }
            }
            error
        }
        initiator {
            username
            url
        }
        changesetCount
        createdAt
        finishedAt
    }
`

const batchChangeFragment = gql`
    fragment BatchChangeFields on BatchChange {
        __typename
        id
        url
        name
        namespace {
            __typename
            id
            namespaceName
            url
            ... on User {
                displayName
                username
            }
            ... on Org {
                displayName
                name
            }
        }
        description

        createdAt
        creator {
            username
            url
        }

        lastAppliedAt
        lastApplier {
            username
            url
        }

        diffStat {
            ...DiffStatFields
        }

        state

        updatedAt
        closedAt
        viewerCanAdminister

        changesetsStats {
            ...ChangesetsStatsFields
        }

        bulkOperations(first: 0) {
            __typename
            totalCount
        }

        activeBulkOperations: bulkOperations(first: 50, createdAfter: $createdAfter) {
            ...ActiveBulkOperationsConnectionFields
        }

        currentSpec {
            id
            originalInput
            description {
                __typename
                name
            }
            files {
                totalCount
                pageInfo {
                    endCursor
                    hasNextPage
                }
                nodes {
                    id
                    name
                    binary
                    byteSize
                    url
                }
            }
            source
            supersedingBatchSpec {
                createdAt
                applyURL
            }
            codeHostsWithoutWebhooks: viewerBatchChangesCodeHosts(first: 3, onlyWithoutWebhooks: true) {
                nodes {
                    externalServiceKind
                    externalServiceURL
                }
                pageInfo {
                    hasNextPage
                }
                totalCount
            }
            viewerBatchChangesCodeHosts(onlyWithoutCredential: true) {
                ...ViewerBatchChangesCodeHostsFields
            }
        }

        # TODO: We ought to be able to filter these by state, but because state is only computed
        # in the resolver and not persisted to the DB, it's currently expensive and messy to do so,
        # so for now we fetch the first 100 and count the active ones clientside.
        batchSpecs(first: 100) {
            nodes {
                state
            }
            pageInfo {
                hasNextPage
            }
        }
    }

    fragment ActiveBulkOperationsConnectionFields on BulkOperationConnection {
        __typename
        totalCount
        nodes {
            ...ActiveBulkOperationFields
        }
    }

    ${changesetsStatsFragment}

    ${diffStatFields}

    ${VIEWER_BATCH_CHANGES_CODE_HOST_FRAGMENT}

    fragment ActiveBulkOperationFields on BulkOperation {
        __typename
        id
        state
    }
`

const changesetLabelFragment = gql`
    fragment ChangesetLabelFields on ChangesetLabel {
        __typename
        color
        description
        text
    }
`

export const fetchBatchChangeByNamespace = (
    namespaceID: Scalars['ID'],
    batchChange: BatchChangeFields['name'],
    createdAfter: BatchChangeByNamespaceVariables['createdAfter']
): Observable<BatchChangeFields | null> =>
    requestGraphQL<BatchChangeByNamespaceResult, BatchChangeByNamespaceVariables>(
        gql`
            query BatchChangeByNamespace($namespaceID: ID!, $batchChange: String!, $createdAfter: DateTime!) {
                batchChange(namespace: $namespaceID, name: $batchChange) {
                    ...BatchChangeFields
                }
            }
            ${batchChangeFragment}
        `,
        { namespaceID, batchChange, createdAfter }
    ).pipe(
        map(dataOrThrowErrors),
        map(({ batchChange }) => {
            if (!batchChange) {
                return null
            }
            return batchChange
        })
    )

export const BATCH_CHANGE_BY_NAMESPACE = gql`
    query BatchChangeByNamespace($namespaceID: ID!, $batchChange: String!, $createdAfter: DateTime!) {
        batchChange(namespace: $namespaceID, name: $batchChange) {
            ...BatchChangeFields
        }
    }
    ${batchChangeFragment}
`

export const hiddenExternalChangesetFieldsFragment = gql`
    fragment HiddenExternalChangesetFields on HiddenExternalChangeset {
        __typename
        id
        createdAt
        updatedAt
        nextSyncAt
        state
    }
`
export const externalChangesetFieldsFragment = gql`
    fragment ExternalChangesetFields on ExternalChangeset {
        __typename
        id
        title
        body
        state
        reviewState
        checkState
        error
        syncerError
        labels {
            ...ChangesetLabelFields
        }
        repository {
            id
            name
            url
        }
        externalURL {
            url
        }
        forkNamespace
        externalID
        diffStat {
            ...DiffStatFields
        }
        createdAt
        updatedAt
        nextSyncAt
        commitVerification {
            ... on GitHubCommitVerification {
                verified
            }
        }
        currentSpec {
            id
            type
            description {
                __typename
                ... on GitBranchChangesetDescription {
                    baseRef
                    headRef
                }
            }
            forkTarget {
                pushUser
                namespace
            }
        }
    }

    ${diffStatFields}

    ${changesetLabelFragment}
`

export const changesetFieldsFragment = gql`
    fragment ChangesetFields on Changeset {
        __typename
        ... on HiddenExternalChangeset {
            ...HiddenExternalChangesetFields
        }
        ... on ExternalChangeset {
            ...ExternalChangesetFields
        }
    }

    ${hiddenExternalChangesetFieldsFragment}

    ${externalChangesetFieldsFragment}
`

export const CHANGESETS = gql`
    query BatchChangeChangesets(
        $batchChange: ID!
        $first: Int
        $after: String
        $state: ChangesetState
        $reviewState: ChangesetReviewState
        $checkState: ChangesetCheckState
        $onlyPublishedByThisBatchChange: Boolean
        $search: String
        $onlyArchived: Boolean
        $onlyClosable: Boolean
    ) {
        node(id: $batchChange) {
            __typename
            ... on BatchChange {
                changesets(
                    first: $first
                    after: $after
                    state: $state
                    reviewState: $reviewState
                    checkState: $checkState
                    onlyPublishedByThisBatchChange: $onlyPublishedByThisBatchChange
                    search: $search
                    onlyArchived: $onlyArchived
                    onlyClosable: $onlyClosable
                ) {
                    __typename
                    totalCount
                    pageInfo {
                        endCursor
                        hasNextPage
                    }
                    nodes {
                        # NOTE: Apollo typename resolution fails if we form a fragment on a union type (e.g. "on Changeset")
                        __typename
                        ... on HiddenExternalChangeset {
                            ...HiddenExternalChangesetFields
                        }
                        ... on ExternalChangeset {
                            ...ExternalChangesetFields
                        }
                    }
                }
            }
        }
    }

    ${hiddenExternalChangesetFieldsFragment}

    ${externalChangesetFieldsFragment}
`

// TODO: This has been superseded by CHANGESETS above, but the "Close" page still uses
// this older `requestGraphQL` one. The variables and result types are the same, so
// eventually this can just go away when we refactor the requests from the "Close" page.
export const queryChangesets = ({
    batchChange,
    first,
    after,
    state,
    onlyClosable,
    reviewState,
    checkState,
    onlyPublishedByThisBatchChange,
    search,
    onlyArchived,
}: BatchChangeChangesetsVariables): Observable<
    (BatchChangeChangesetsResult['node'] & { __typename: 'BatchChange' })['changesets']
> =>
    requestGraphQL<BatchChangeChangesetsResult, BatchChangeChangesetsVariables>(CHANGESETS, {
        batchChange,
        first,
        after,
        state,
        onlyClosable,
        reviewState,
        checkState,
        onlyPublishedByThisBatchChange,
        search,
        onlyArchived,
    }).pipe(
        map(dataOrThrowErrors),
        map(({ node }) => {
            if (!node) {
                throw new Error(`Batch change with ID ${batchChange} does not exist`)
            }
            if (node.__typename !== 'BatchChange') {
                throw new Error(`The given ID is a ${node.__typename}, not a BatchChange`)
            }
            return node.changesets
        })
    )

export async function syncChangeset(changeset: Scalars['ID']): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<SyncChangesetResult, SyncChangesetVariables>(
            gql`
                mutation SyncChangeset($changeset: ID!) {
                    syncChangeset(changeset: $changeset) {
                        alwaysNil
                    }
                }
            `,
            { changeset }
        )
    )
    dataOrThrowErrors(result)
}

export async function reenqueueChangeset(changeset: Scalars['ID']): Promise<ChangesetFields> {
    return lastValueFrom(
        requestGraphQL<ReenqueueChangesetResult, ReenqueueChangesetVariables>(
            gql`
                mutation ReenqueueChangeset($changeset: ID!) {
                    reenqueueChangeset(changeset: $changeset) {
                        ...ChangesetFields
                    }
                }

                ${changesetFieldsFragment}
            `,
            { changeset }
        ).pipe(
            map(dataOrThrowErrors),
            map(data => data.reenqueueChangeset)
        )
    )
}

// Because thats the name in the API:

export const gitRefSpecFields = gql`
    fragment GitRefSpecFields on GitRevSpec {
        __typename
        ... on GitObject {
            oid
        }
        ... on GitRef {
            target {
                oid
            }
        }
        ... on GitRevSpecExpr {
            object {
                oid
            }
        }
    }
`

export const externalChangesetFileDiffsFields = gql`
    fragment ExternalChangesetFileDiffsFields on ExternalChangeset {
        diff {
            __typename
            ... on RepositoryComparison {
                range {
                    base {
                        ...GitRefSpecFields
                    }
                    head {
                        ...GitRefSpecFields
                    }
                }
                fileDiffs(first: $first, after: $after) {
                    nodes {
                        ...FileDiffFields
                    }
                    totalCount
                    pageInfo {
                        hasNextPage
                        endCursor
                    }
                }
            }
            ... on PreviewRepositoryComparison {
                fileDiffs(first: $first, after: $after) {
                    nodes {
                        ...FileDiffFields
                    }
                    totalCount
                    pageInfo {
                        hasNextPage
                        endCursor
                    }
                }
            }
        }
    }

    ${fileDiffFields}

    ${gitRefSpecFields}
`

export const queryExternalChangesetWithFileDiffs = ({
    externalChangeset,
    first,
    after,
}: ExternalChangesetFileDiffsVariables): Observable<ExternalChangesetFileDiffsFields> =>
    requestGraphQL<ExternalChangesetFileDiffsResult, ExternalChangesetFileDiffsVariables>(
        gql`
            query ExternalChangesetFileDiffs($externalChangeset: ID!, $first: Int, $after: String) {
                node(id: $externalChangeset) {
                    __typename
                    ...ExternalChangesetFileDiffsFields
                }
            }

            ${externalChangesetFileDiffsFields}
        `,
        { externalChangeset, first, after }
    ).pipe(
        map(dataOrThrowErrors),
        map(({ node }) => {
            if (!node) {
                throw new Error(`Changeset with ID ${externalChangeset} does not exist`)
            }
            if (node.__typename !== 'ExternalChangeset') {
                throw new Error(`The given ID is a ${node.__typename}, not an ExternalChangeset`)
            }
            return node
        })
    )

const changesetCountsOverTimeFragment = gql`
    fragment ChangesetCountsOverTimeFields on ChangesetCounts {
        date
        merged
        closed
        draft
        openApproved
        openChangesRequested
        openPending
        total
    }
`

export const CHANGESET_COUNTS_OVER_TIME = gql`
    query ChangesetCountsOverTime($batchChange: ID!, $includeArchived: Boolean!) {
        node(id: $batchChange) {
            __typename
            ... on BatchChange {
                changesetCountsOverTime(includeArchived: $includeArchived) {
                    ...ChangesetCountsOverTimeFields
                }
            }
        }
    }

    ${changesetCountsOverTimeFragment}
`

export const useChangesetCountsOverTime = (
    batchChange: Scalars['ID'],
    includeArchived: boolean
): QueryResult<ChangesetCountsOverTimeResult, ChangesetCountsOverTimeVariables> =>
    useQuery<ChangesetCountsOverTimeResult, ChangesetCountsOverTimeVariables>(CHANGESET_COUNTS_OVER_TIME, {
        variables: {
            batchChange,
            includeArchived,
        },
        fetchPolicy: 'cache-and-network',
    })

export async function deleteBatchChange(batchChange: Scalars['ID']): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<DeleteBatchChangeResult, DeleteBatchChangeVariables>(
            gql`
                mutation DeleteBatchChange($batchChange: ID!) {
                    deleteBatchChange(batchChange: $batchChange) {
                        alwaysNil
                    }
                }
            `,
            { batchChange }
        )
    )
    dataOrThrowErrors(result)
}

const changesetDiffFragment = gql`
    fragment ChangesetDiffFields on ExternalChangeset {
        currentSpec {
            description {
                __typename
                ... on GitBranchChangesetDescription {
                    commits {
                        diff
                    }
                }
            }
        }
    }
`

export async function getChangesetDiff(changeset: Scalars['ID']): Promise<string> {
    return lastValueFrom(
        requestGraphQL<ChangesetDiffResult, ChangesetDiffVariables>(
            gql`
                query ChangesetDiff($changeset: ID!) {
                    node(id: $changeset) {
                        __typename
                        ...ChangesetDiffFields
                    }
                }

                ${changesetDiffFragment}
            `,
            { changeset }
        ).pipe(
            map(dataOrThrowErrors),
            map(({ node }) => {
                if (!node) {
                    throw new Error(`Changeset with ID ${changeset} does not exist`)
                } else if (node.__typename === 'HiddenExternalChangeset') {
                    throw new Error(`You do not have permission to view changeset ${changeset}`)
                } else if (node.__typename !== 'ExternalChangeset') {
                    throw new Error(`The given ID is a ${node.__typename}, not an ExternalChangeset`)
                }

                const commits =
                    node.currentSpec?.description?.__typename === 'GitBranchChangesetDescription' &&
                    node.currentSpec?.description.commits
                if (!commits) {
                    throw new Error(`No commit available for changeset ID ${changeset}`)
                } else if (commits.length !== 1) {
                    throw new Error(`Unexpected number of commits on changeset ${changeset}: ${commits.length}`)
                }

                return commits[0].diff
            })
        )
    )
}

const changesetScheduleEstimateFragment = gql`
    fragment ChangesetScheduleEstimateFields on ExternalChangeset {
        scheduleEstimateAt
    }
`

export async function getChangesetScheduleEstimate(changeset: Scalars['ID']): Promise<Scalars['DateTime'] | null> {
    return lastValueFrom(
        requestGraphQL<ChangesetScheduleEstimateResult, ChangesetScheduleEstimateVariables>(
            gql`
                query ChangesetScheduleEstimate($changeset: ID!) {
                    node(id: $changeset) {
                        __typename
                        ...ChangesetScheduleEstimateFields
                    }
                }

                ${changesetScheduleEstimateFragment}
            `,
            { changeset }
        ).pipe(
            map(dataOrThrowErrors),
            map(({ node }) => {
                if (!node) {
                    throw new Error(`Changeset with ID ${changeset} does not exist`)
                } else if (node.__typename === 'HiddenExternalChangeset') {
                    throw new Error(`You do not have permission to view changeset ${changeset}`)
                } else if (node.__typename !== 'ExternalChangeset') {
                    throw new Error(`The given ID is a ${node.__typename}, not an ExternalChangeset`)
                }

                return node.scheduleEstimateAt
            })
        )
    )
}

export async function detachChangesets(batchChange: Scalars['ID'], changesets: Scalars['ID'][]): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<DetachChangesetsResult, DetachChangesetsVariables>(
            gql`
                mutation DetachChangesets($batchChange: ID!, $changesets: [ID!]!) {
                    detachChangesets(batchChange: $batchChange, changesets: $changesets) {
                        id
                    }
                }
            `,
            { batchChange, changesets }
        )
    )
    dataOrThrowErrors(result)
}

export async function createChangesetComments(
    batchChange: Scalars['ID'],
    changesets: Scalars['ID'][],
    body: string
): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<CreateChangesetCommentsResult, CreateChangesetCommentsVariables>(
            gql`
                mutation CreateChangesetComments($batchChange: ID!, $changesets: [ID!]!, $body: String!) {
                    createChangesetComments(batchChange: $batchChange, changesets: $changesets, body: $body) {
                        id
                    }
                }
            `,
            { batchChange, changesets, body }
        )
    )
    dataOrThrowErrors(result)
}

export async function reenqueueChangesets(batchChange: Scalars['ID'], changesets: Scalars['ID'][]): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<ReenqueueChangesetsResult, ReenqueueChangesetsVariables>(
            gql`
                mutation ReenqueueChangesets($batchChange: ID!, $changesets: [ID!]!) {
                    reenqueueChangesets(batchChange: $batchChange, changesets: $changesets) {
                        id
                    }
                }
            `,
            { batchChange, changesets }
        )
    )
    dataOrThrowErrors(result)
}

export async function mergeChangesets(
    batchChange: Scalars['ID'],
    changesets: Scalars['ID'][],
    squash: boolean
): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<MergeChangesetsResult, MergeChangesetsVariables>(
            gql`
                mutation MergeChangesets($batchChange: ID!, $changesets: [ID!]!, $squash: Boolean!) {
                    mergeChangesets(batchChange: $batchChange, changesets: $changesets, squash: $squash) {
                        id
                    }
                }
            `,
            { batchChange, changesets, squash }
        )
    )
    dataOrThrowErrors(result)
}

export async function closeChangesets(batchChange: Scalars['ID'], changesets: Scalars['ID'][]): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<CloseChangesetsResult, CloseChangesetsVariables>(
            gql`
                mutation CloseChangesets($batchChange: ID!, $changesets: [ID!]!) {
                    closeChangesets(batchChange: $batchChange, changesets: $changesets) {
                        id
                    }
                }
            `,
            { batchChange, changesets }
        )
    )
    dataOrThrowErrors(result)
}

export async function publishChangesets(
    batchChange: Scalars['ID'],
    changesets: Scalars['ID'][],
    draft: boolean
): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<PublishChangesetsResult, PublishChangesetsVariables>(
            gql`
                mutation PublishChangesets($batchChange: ID!, $changesets: [ID!]!, $draft: Boolean!) {
                    publishChangesets(batchChange: $batchChange, changesets: $changesets, draft: $draft) {
                        id
                    }
                }
            `,
            { batchChange, changesets, draft }
        )
    )
    dataOrThrowErrors(result)
}

// We pass in the batchChange because this Query is configured to only fetch changesets with the given ID that
// belong to a specific batch change. This is because we only use this for exporting changesets in a particular
// batch change and we check if the user has permission to view/administer the Batch Change on the backend.
export const GET_CHANGESETS_BY_IDS_QUERY = gql`
    query GetChangesetsByIDs($batchChange: ID!, $changesets: [ID!]!) {
        getChangesetsByIDs(batchChange: $batchChange, changesets: $changesets) {
            nodes {
                ... on ExternalChangeset {
                    id
                    title
                    state
                    reviewState
                    externalURL {
                        url
                    }
                    repository {
                        name
                    }
                }
            }
        }
    }
`

export const useGetChangesetsByIDs = (
    batchChange: Scalars['ID'],
    changesets: Scalars['ID'][]
): QueryTuple<GetChangesetsByIDsResult, GetChangesetsByIDsVariables> =>
    useLazyQuery(GET_CHANGESETS_BY_IDS_QUERY, {
        variables: { batchChange, changesets },
    })

export const BULK_OPERATIONS = gql`
    query BatchChangeBulkOperations($batchChange: ID!, $first: Int, $after: String) {
        node(id: $batchChange) {
            __typename
            ... on BatchChange {
                bulkOperations(first: $first, after: $after) {
                    ...BulkOperationConnectionFields
                }
            }
        }
    }

    fragment BulkOperationConnectionFields on BulkOperationConnection {
        __typename
        totalCount
        pageInfo {
            hasNextPage
            endCursor
        }
        nodes {
            ...BulkOperationFields
        }
    }

    ${bulkOperationFragment}
`

export const queryAllChangesetIDs = ({
    batchChange,
    state,
    reviewState,
    checkState,
    onlyPublishedByThisBatchChange,
    search,
    onlyArchived,
}: Omit<AllChangesetIDsVariables, 'after'>): Observable<Scalars['ID'][]> => {
    const request = (after: string | null): Observable<ChangesetIDConnectionFields> =>
        requestGraphQL<AllChangesetIDsResult, AllChangesetIDsVariables>(
            gql`
                query AllChangesetIDs(
                    $batchChange: ID!
                    $after: String
                    $state: ChangesetState
                    $reviewState: ChangesetReviewState
                    $checkState: ChangesetCheckState
                    $onlyPublishedByThisBatchChange: Boolean
                    $search: String
                    $onlyArchived: Boolean
                ) {
                    node(id: $batchChange) {
                        __typename
                        ... on BatchChange {
                            changesets(
                                first: 10000
                                after: $after
                                state: $state
                                reviewState: $reviewState
                                checkState: $checkState
                                onlyPublishedByThisBatchChange: $onlyPublishedByThisBatchChange
                                search: $search
                                onlyArchived: $onlyArchived
                            ) {
                                ...ChangesetIDConnectionFields
                            }
                        }
                    }
                }

                fragment ChangesetIDConnectionFields on ChangesetConnection {
                    nodes {
                        __typename
                        id
                    }
                    pageInfo {
                        hasNextPage
                        endCursor
                    }
                }
            `,
            {
                batchChange,
                after,
                state,
                reviewState,
                checkState,
                onlyPublishedByThisBatchChange,
                search,
                onlyArchived,
            }
        ).pipe(
            map(dataOrThrowErrors),
            map(({ node }) => {
                if (!node) {
                    throw new Error(`Batch change with ID ${batchChange} does not exist`)
                }
                if (node.__typename !== 'BatchChange') {
                    throw new Error(`The given ID is a ${node.__typename}, not a BatchChange`)
                }
                return node.changesets
            })
        )

    return request(null).pipe(
        expand(connection => (connection.pageInfo.hasNextPage ? request(connection.pageInfo.endCursor) : EMPTY)),
        reduce(
            (previous, next) =>
                previous.concat(
                    next.nodes.filter(node => node.__typename === 'ExternalChangeset').map(node => node.id)
                ),
            [] as Scalars['ID'][]
        )
    )
}

export const queryAvailableBulkOperations = ({
    batchChange,
    changesets,
}: {
    batchChange: Scalars['ID']
    changesets: Scalars['ID'][]
}): Observable<BulkOperationType[]> =>
    requestGraphQL<AvailableBulkOperationsResult, AvailableBulkOperationsVariables>(
        gql`
            query AvailableBulkOperations($batchChange: ID!, $changesets: [ID!]!) {
                availableBulkOperations(batchChange: $batchChange, changesets: $changesets)
            }
        `,
        { batchChange, changesets }
    ).pipe(
        map(dataOrThrowErrors),
        map(item => item.availableBulkOperations)
    )
