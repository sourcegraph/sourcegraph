import { map } from 'rxjs/operators'
import { dataOrThrowErrors, gql } from '../../../../../shared/src/graphql/graphql'
import { Observable } from 'rxjs'
import {
    BatchChangesVariables,
    BatchChangesResult,
    BatchChangesByNamespaceResult,
    BatchChangesByNamespaceVariables,
    AreBatchChangesLicensedResult,
    AreBatchChangesLicensedVariables,
} from '../../../graphql-operations'
import { requestGraphQL } from '../../../backend/graphql'

const listBatchChangeFragment = gql`
    fragment ListBatchChange on BatchChange {
        id
        url
        name
        namespace {
            namespaceName
            url
        }
        description
        createdAt
        closedAt
        changesetsStats {
            open
            closed
            merged
        }
    }
`

export interface ListBatchChangesResult {
    batchChanges: BatchChangesResult['batchChanges']
    totalCount: number
}

export const queryBatchChanges = ({
    first,
    after,
    state,
    viewerCanAdminister,
}: Partial<BatchChangesVariables>): Observable<ListBatchChangesResult> =>
    requestGraphQL<BatchChangesResult, BatchChangesVariables>(
        gql`
            query BatchChanges($first: Int, $after: String, $state: BatchChangeState, $viewerCanAdminister: Boolean) {
                batchChanges(first: $first, after: $after, state: $state, viewerCanAdminister: $viewerCanAdminister) {
                    nodes {
                        ...ListBatchChange
                    }
                    pageInfo {
                        endCursor
                        hasNextPage
                    }
                    totalCount
                }
                allBatchChanges: batchChanges(first: 0) {
                    totalCount
                }
            }

            ${listBatchChangeFragment}
        `,
        {
            first: first ?? null,
            after: after ?? null,
            state: state ?? null,
            viewerCanAdminister: viewerCanAdminister ?? null,
        }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => ({
            batchChanges: data.batchChanges,
            totalCount: data.allBatchChanges.totalCount,
        }))
    )

export const queryBatchChangesByNamespace = ({
    namespaceID,
    first,
    after,
    state,
    viewerCanAdminister,
}: BatchChangesByNamespaceVariables): Observable<ListBatchChangesResult> =>
    requestGraphQL<BatchChangesByNamespaceResult, BatchChangesByNamespaceVariables>(
        gql`
            query BatchChangesByNamespace(
                $namespaceID: ID!
                $first: Int
                $after: String
                $state: BatchChangeState
                $viewerCanAdminister: Boolean
            ) {
                node(id: $namespaceID) {
                    __typename
                    ... on User {
                        batchChanges(
                            first: $first
                            after: $after
                            state: $state
                            viewerCanAdminister: $viewerCanAdminister
                        ) {
                            ...BatchChangesFields
                        }
                        allBatchChanges: batchChanges(first: 0) {
                            totalCount
                        }
                    }
                    ... on Org {
                        batchChanges(
                            first: $first
                            after: $after
                            state: $state
                            viewerCanAdminister: $viewerCanAdminister
                        ) {
                            ...BatchChangesFields
                        }
                        allBatchChanges: batchChanges(first: 0) {
                            totalCount
                        }
                    }
                }
            }

            fragment BatchChangesFields on BatchChangeConnection {
                nodes {
                    ...ListBatchChange
                }
                pageInfo {
                    endCursor
                    hasNextPage
                }
                totalCount
            }

            ${listBatchChangeFragment}
        `,
        { first, after, state, viewerCanAdminister, namespaceID }
    ).pipe(
        map(dataOrThrowErrors),
        map(data => {
            if (!data.node) {
                throw new Error('Namespace not found')
            }

            if (data.node.__typename !== 'Org' && data.node.__typename !== 'User') {
                throw new Error(`Requested node is a ${data.node.__typename}, not a User or Org`)
            }
            return {
                batchChanges: data.node.batchChanges,
                totalCount: data.node.allBatchChanges.totalCount,
            }
        })
    )

export function areBatchChangesLicensed(): Observable<boolean> {
    return requestGraphQL<AreBatchChangesLicensedResult, AreBatchChangesLicensedVariables>(
        gql`
            query AreBatchChangesLicensed {
                campaigns: enterpriseLicenseHasFeature(feature: "campaigns")
                batchChanges: enterpriseLicenseHasFeature(feature: "batch-changes")
            }
        `
    ).pipe(
        map(dataOrThrowErrors),
        map(data => data.campaigns || data.batchChanges)
    )
}
