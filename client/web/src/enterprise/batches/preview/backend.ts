import { diffStatFields } from '../../../backend/diff'
import { gql, dataOrThrowErrors } from '../../../../../shared/src/graphql/graphql'
import {
    Scalars,
    CreateBatchChangeVariables,
    CreateBatchChangeResult,
    ApplyBatchChangeResult,
    ApplyBatchChangeVariables,
    BatchSpecByIDResult,
    BatchSpecByIDVariables,
    BatchSpecFields,
} from '../../../graphql-operations'
import { Observable } from 'rxjs'
import { map } from 'rxjs/operators'
import { requestGraphQL } from '../../../backend/graphql'

export const viewerBatchChangesCodeHostsFragment = gql`
    fragment ViewerBatchChangesCodeHostsFields on BatchChangesCodeHostConnection {
        totalCount
        nodes {
            externalServiceURL
            externalServiceKind
        }
    }
`

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
        applyPreview {
            stats {
                close
                detach
                import
                publish
                publishDraft
                push
                reopen
                undraft
                update

                added
                modified
                removed
            }
        }
        supersedingBatchSpec {
            ...SupersedingBatchSpecFields
        }
        viewerBatchChangesCodeHosts(onlyWithoutCredential: true) {
            ...ViewerBatchChangesCodeHostsFields
        }
    }

    ${viewerBatchChangesCodeHostsFragment}

    ${diffStatFields}

    ${supersedingBatchSpecFragment}
`

export const fetchBatchSpecById = (batchSpec: Scalars['ID']): Observable<BatchSpecFields | null> =>
    requestGraphQL<BatchSpecByIDResult, BatchSpecByIDVariables>(
        gql`
            query BatchSpecByID($batchSpec: ID!) {
                node(id: $batchSpec) {
                    __typename
                    ... on BatchSpec {
                        ...BatchSpecFields
                    }
                }
            }
            ${batchSpecFragment}
        `,
        { batchSpec }
    ).pipe(
        map(dataOrThrowErrors),
        map(({ node }) => {
            if (!node) {
                return null
            }
            if (node.__typename !== 'BatchSpec') {
                throw new Error(`The given ID is a ${node.__typename}, not a BatchSpec`)
            }
            return node
        })
    )

export const createBatchChange = ({
    batchSpec,
}: CreateBatchChangeVariables): Promise<CreateBatchChangeResult['createBatchChange']> =>
    requestGraphQL<CreateBatchChangeResult, CreateBatchChangeVariables>(
        gql`
            mutation CreateBatchChange($batchSpec: ID!) {
                createBatchChange(batchSpec: $batchSpec) {
                    id
                    url
                }
            }
        `,
        { batchSpec }
    )
        .pipe(
            map(dataOrThrowErrors),
            map(data => data.createBatchChange)
        )
        .toPromise()

export const applyBatchChange = ({
    batchSpec,
    batchChange,
}: ApplyBatchChangeVariables): Promise<ApplyBatchChangeResult['applyBatchChange']> =>
    requestGraphQL<ApplyBatchChangeResult, ApplyBatchChangeVariables>(
        gql`
            mutation ApplyBatchChange($batchSpec: ID!, $batchChange: ID!) {
                applyBatchChange(batchSpec: $batchSpec, ensureBatchChange: $batchChange) {
                    id
                    url
                }
            }
        `,
        { batchSpec, batchChange }
    )
        .pipe(
            map(dataOrThrowErrors),
            map(data => data.applyBatchChange)
        )
        .toPromise()
