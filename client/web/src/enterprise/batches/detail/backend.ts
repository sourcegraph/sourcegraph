import { map } from 'rxjs/operators'
import { dataOrThrowErrors, gql } from '../../../../../shared/src/graphql/graphql'
import { Observable } from 'rxjs'
import { diffStatFields, fileDiffFields } from '../../../backend/diff'
import {
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
    ChangesetCountsOverTimeFields,
    ChangesetCountsOverTimeResult,
    DeleteBatchChangeResult,
    ChangesetDiffResult,
    ChangesetDiffVariables,
    ReenqueueChangesetVariables,
    ReenqueueChangesetResult,
    ChangesetFields,
    DeleteBatchChangeVariables,
} from '../../../graphql-operations'
import { requestGraphQL } from '../../../backend/graphql'

const changesetsStatsFragment = gql`
    fragment ChangesetsStatsFields on ChangesetsStats {
        total
        closed
        deleted
        draft
        merged
        open
        unpublished
    }
`

const batchChangeFragment = gql`
    fragment BatchChangeFields on BatchChange {
        __typename
        id
        url
        name
        namespace {
            namespaceName
            url
        }
        description

        createdAt
        initialApplier {
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

        updatedAt
        closedAt
        viewerCanAdminister

        changesetsStats {
            ...ChangesetsStatsFields
        }

        currentSpec {
            originalInput
            supersedingBatchSpec {
                createdAt
                applyURL
            }
        }
    }

    ${changesetsStatsFragment}

    ${diffStatFields}
`

const changesetLabelFragment = gql`
    fragment ChangesetLabelFields on ChangesetLabel {
        color
        description
        text
    }
`

export const fetchBatchChangeByNamespace = (
    namespaceID: Scalars['ID'],
    batchChange: BatchChangeFields['name']
): Observable<BatchChangeFields | null> =>
    requestGraphQL<BatchChangeByNamespaceResult, BatchChangeByNamespaceVariables>(
        gql`
            query BatchChangeByNamespace($namespaceID: ID!, $batchChange: String!) {
                batchChange(namespace: $namespaceID, name: $batchChange) {
                    ...BatchChangeFields
                }
            }
            ${batchChangeFragment}
        `,
        { namespaceID, batchChange }
    ).pipe(
        map(dataOrThrowErrors),
        map(({ batchChange }) => {
            if (!batchChange) {
                return null
            }
            return batchChange
        })
    )

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
        externalID
        diffStat {
            ...DiffStatFields
        }
        createdAt
        updatedAt
        nextSyncAt
        currentSpec {
            id
            type
            description {
                __typename
                ... on GitBranchChangesetDescription {
                    headRef
                }
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

export const queryChangesets = ({
    batchChange,
    first,
    after,
    state,
    reviewState,
    checkState,
    onlyPublishedByThisBatchChange,
    search,
}: BatchChangeChangesetsVariables): Observable<
    (BatchChangeChangesetsResult['node'] & { __typename: 'BatchChange' })['changesets']
> =>
    requestGraphQL<BatchChangeChangesetsResult, BatchChangeChangesetsVariables>(
        gql`
            query BatchChangeChangesets(
                $batchChange: ID!
                $first: Int
                $after: String
                $state: ChangesetState
                $reviewState: ChangesetReviewState
                $checkState: ChangesetCheckState
                $onlyPublishedByThisBatchChange: Boolean
                $search: String
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
                        ) {
                            totalCount
                            pageInfo {
                                endCursor
                                hasNextPage
                            }
                            nodes {
                                ...ChangesetFields
                            }
                        }
                    }
                }
            }

            ${changesetFieldsFragment}
        `,
        {
            batchChange,
            first,
            after,
            state,
            reviewState,
            checkState,
            onlyPublishedByThisBatchChange,
            search,
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

export async function syncChangeset(changeset: Scalars['ID']): Promise<void> {
    const result = await requestGraphQL<SyncChangesetResult, SyncChangesetVariables>(
        gql`
            mutation SyncChangeset($changeset: ID!) {
                syncChangeset(changeset: $changeset) {
                    alwaysNil
                }
            }
        `,
        { changeset }
    ).toPromise()
    dataOrThrowErrors(result)
}

export async function reenqueueChangeset(changeset: Scalars['ID']): Promise<ChangesetFields> {
    return requestGraphQL<ReenqueueChangesetResult, ReenqueueChangesetVariables>(
        gql`
            mutation ReenqueueChangeset($changeset: ID!) {
                reenqueueChangeset(changeset: $changeset) {
                    ...ChangesetFields
                }
            }

            ${changesetFieldsFragment}
        `,
        { changeset }
    )
        .pipe(
            map(dataOrThrowErrors),
            map(data => data.reenqueueChangeset)
        )
        .toPromise()
}

// Because thats the name in the API:
// eslint-disable-next-line unicorn/prevent-abbreviations
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
    isLightTheme,
}: ExternalChangesetFileDiffsVariables): Observable<ExternalChangesetFileDiffsFields> =>
    requestGraphQL<ExternalChangesetFileDiffsResult, ExternalChangesetFileDiffsVariables>(
        gql`
            query ExternalChangesetFileDiffs(
                $externalChangeset: ID!
                $first: Int
                $after: String
                $isLightTheme: Boolean!
            ) {
                node(id: $externalChangeset) {
                    __typename
                    ...ExternalChangesetFileDiffsFields
                }
            }

            ${externalChangesetFileDiffsFields}
        `,
        { externalChangeset, first, after, isLightTheme }
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

export const queryChangesetCountsOverTime = ({
    batchChange,
}: ChangesetCountsOverTimeVariables): Observable<ChangesetCountsOverTimeFields[]> =>
    requestGraphQL<ChangesetCountsOverTimeResult, ChangesetCountsOverTimeVariables>(
        gql`
            query ChangesetCountsOverTime($batchChange: ID!) {
                node(id: $batchChange) {
                    __typename
                    ... on BatchChange {
                        changesetCountsOverTime {
                            ...ChangesetCountsOverTimeFields
                        }
                    }
                }
            }

            ${changesetCountsOverTimeFragment}
        `,
        { batchChange }
    ).pipe(
        map(dataOrThrowErrors),
        map(({ node }) => {
            if (!node) {
                throw new Error(`BatchChange with ID ${batchChange} does not exist`)
            }
            if (node.__typename !== 'BatchChange') {
                throw new Error(`The given ID is a ${node.__typename}, not a BatchChange`)
            }
            return node.changesetCountsOverTime
        })
    )

export async function deleteBatchChange(batchChange: Scalars['ID']): Promise<void> {
    const result = await requestGraphQL<DeleteBatchChangeResult, DeleteBatchChangeVariables>(
        gql`
            mutation DeleteBatchChange($batchChange: ID!) {
                deleteBatchChange(batchChange: $batchChange) {
                    alwaysNil
                }
            }
        `,
        { batchChange }
    ).toPromise()
    dataOrThrowErrors(result)
}

const changesetDiffFragment = gql`
    fragment ChangesetDiffFields on ExternalChangeset {
        currentSpec {
            description {
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
    return requestGraphQL<ChangesetDiffResult, ChangesetDiffVariables>(
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
    )
        .pipe(
            map(dataOrThrowErrors),
            map(({ node }) => {
                if (!node) {
                    throw new Error(`Changeset with ID ${changeset} does not exist`)
                } else if (node.__typename === 'HiddenExternalChangeset') {
                    throw new Error(`You do not have permission to view changeset ${changeset}`)
                } else if (node.__typename !== 'ExternalChangeset') {
                    throw new Error(`The given ID is a ${node.__typename}, not an ExternalChangeset`)
                }

                const commits = node.currentSpec?.description.commits
                if (!commits) {
                    throw new Error(`No commit available for changeset ID ${changeset}`)
                } else if (commits.length !== 1) {
                    throw new Error(`Unexpected number of commits on changeset ${changeset}: ${commits.length}`)
                }

                return commits[0].diff
            })
        )
        .toPromise()
}
