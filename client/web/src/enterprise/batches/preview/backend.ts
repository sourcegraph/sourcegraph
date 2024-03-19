import { lastValueFrom, type Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'

import { diffStatFields } from '../../../backend/diff'
import { requestGraphQL } from '../../../backend/graphql'
import type {
    CreateBatchChangeVariables,
    CreateBatchChangeResult,
    ApplyBatchChangeResult,
    ApplyBatchChangeVariables,
    QueryApplyPreviewStatsVariables,
    QueryApplyPreviewStatsResult,
    ApplyPreviewStatsFields,
} from '../../../graphql-operations'
import { VIEWER_BATCH_CHANGES_CODE_HOST_FRAGMENT } from '../MissingCredentialsAlert'

const supersedingBatchSpecFragment = gql`
    fragment SupersedingBatchSpecFields on BatchSpec {
        createdAt
        applyURL
    }
`

export const batchSpecFragment = gql`
    fragment BatchSpecFields on BatchSpec {
        id
        description {
            name
            description
        }
        appliesToBatchChange {
            id
            name
            url
        }
        createdAt
        creator {
            username
            url
        }
        expiresAt
        namespace {
            namespaceName
            url
        }
        viewerCanAdminister
        diffStat {
            ...DiffStatFields
        }
        originalInput
        applyPreview {
            stats {
                archive
            }
            totalCount
        }
        supersedingBatchSpec {
            ...SupersedingBatchSpecFields
        }
        viewerBatchChangesCodeHosts(onlyWithoutCredential: true) {
            ...ViewerBatchChangesCodeHostsFields
        }
    }

    ${VIEWER_BATCH_CHANGES_CODE_HOST_FRAGMENT}

    ${diffStatFields}

    ${supersedingBatchSpecFragment}
`

export const BATCH_SPEC_BY_ID = gql`
    query BatchSpecByID($batchSpec: ID!) {
        node(id: $batchSpec) {
            __typename
            ... on BatchSpec {
                ...BatchSpecFields
            }
        }
    }
    ${batchSpecFragment}
`

export const queryApplyPreviewStats = ({
    batchSpec,
    publicationStates,
}: QueryApplyPreviewStatsVariables): Observable<ApplyPreviewStatsFields['stats']> =>
    requestGraphQL<QueryApplyPreviewStatsResult, QueryApplyPreviewStatsVariables>(
        gql`
            query QueryApplyPreviewStats($batchSpec: ID!, $publicationStates: [ChangesetSpecPublicationStateInput!]) {
                node(id: $batchSpec) {
                    __typename
                    ... on BatchSpec {
                        id
                        applyPreview(publicationStates: $publicationStates) {
                            ...ApplyPreviewStatsFields
                        }
                    }
                }
            }

            fragment ApplyPreviewStatsFields on ChangesetApplyPreviewConnection {
                stats {
                    close
                    detach
                    archive
                    import
                    publish
                    publishDraft
                    push
                    reopen
                    undraft
                    update
                    reattach

                    added
                    modified
                    removed
                }
            }
        `,
        { batchSpec, publicationStates }
    ).pipe(
        map(dataOrThrowErrors),
        map(({ node }) => {
            if (!node) {
                throw new Error(`BatchSpec with ID ${batchSpec} does not exist`)
            }
            if (node.__typename !== 'BatchSpec') {
                throw new Error(`The given ID is a ${node.__typename}, not a BatchSpec`)
            }
            return node.applyPreview.stats
        })
    )

export const createBatchChange = ({
    batchSpec,
    publicationStates,
}: CreateBatchChangeVariables): Promise<CreateBatchChangeResult['createBatchChange']> =>
    lastValueFrom(
        requestGraphQL<CreateBatchChangeResult, CreateBatchChangeVariables>(
            gql`
                mutation CreateBatchChange($batchSpec: ID!, $publicationStates: [ChangesetSpecPublicationStateInput!]) {
                    createBatchChange(batchSpec: $batchSpec, publicationStates: $publicationStates) {
                        id
                        url
                    }
                }
            `,
            { batchSpec, publicationStates }
        ).pipe(
            map(dataOrThrowErrors),
            map(data => data.createBatchChange)
        )
    )

export const applyBatchChange = ({
    batchSpec,
    batchChange,
    publicationStates,
}: ApplyBatchChangeVariables): Promise<ApplyBatchChangeResult['applyBatchChange']> =>
    lastValueFrom(
        requestGraphQL<ApplyBatchChangeResult, ApplyBatchChangeVariables>(
            gql`
                mutation ApplyBatchChange(
                    $batchSpec: ID!
                    $batchChange: ID!
                    $publicationStates: [ChangesetSpecPublicationStateInput!]
                ) {
                    applyBatchChange(
                        batchSpec: $batchSpec
                        ensureBatchChange: $batchChange
                        publicationStates: $publicationStates
                    ) {
                        id
                        url
                    }
                }
            `,
            { batchSpec, batchChange, publicationStates }
        ).pipe(
            map(dataOrThrowErrors),
            map(data => data.applyBatchChange)
        )
    )
