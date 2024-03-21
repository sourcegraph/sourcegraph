import { gql } from '@sourcegraph/http-client'

import type { RepositoryMetadataResult, RepositoryMetadataVariables } from '../graphql-operations'

import { requestGraphQLFromVSCode } from './requestGraphQl'

const repositoryMetadataQuery = gql`
    query RepositoryMetadata($repositoryName: String!) {
        repositoryRedirect(name: $repositoryName) {
            ... on Repository {
                id
                mirrorInfo {
                    cloneInProgress
                    cloneProgress
                    cloned
                }
                commit(rev: "") {
                    oid
                    abbreviatedOID
                    tree(path: "") {
                        url
                    }
                }
                defaultBranch {
                    abbrevName
                }
            }
            ... on Redirect {
                url
            }
        }
    }
`

export interface RepositoryMetadata {
    repositoryId: string
    defaultOID?: string
    defaultAbbreviatedOID?: string
    defaultBranch?: string
}

export async function getRepositoryMetadata(
    variables: RepositoryMetadataVariables
): Promise<RepositoryMetadata | undefined> {
    const result = await requestGraphQLFromVSCode<RepositoryMetadataResult, RepositoryMetadataVariables>(
        repositoryMetadataQuery,
        variables
    )
    if (result.data?.repositoryRedirect?.__typename === 'Repository') {
        return {
            repositoryId: result.data.repositoryRedirect.id,
            defaultOID: result.data.repositoryRedirect.commit?.oid,
            defaultAbbreviatedOID: result.data.repositoryRedirect.commit?.abbreviatedOID,
            defaultBranch: result.data.repositoryRedirect.defaultBranch?.abbrevName,
        }
    }
    // v1 Debt: surface error to user.
    return undefined
}
