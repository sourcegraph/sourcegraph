import { gql } from '@sourcegraph/http-client'

import type { BlobContentResult, BlobContentVariables } from '../graphql-operations'

import { requestGraphQLFromVSCode } from './requestGraphQl'

const blobContentQuery = gql`
    query BlobContent($repository: String!, $revision: String!, $path: String!) {
        repository(name: $repository) {
            commit(rev: $revision) {
                blob(path: $path) {
                    content
                    binary
                    byteSize
                }
            }
        }
    }
`

export interface FileContents {
    content: Uint8Array
    isBinary: boolean
    byteSize: number
}

export async function getBlobContent(variables: BlobContentVariables): Promise<FileContents | undefined> {
    const result = await requestGraphQLFromVSCode<BlobContentResult, BlobContentVariables>(blobContentQuery, variables)

    const blob = result.data?.repository?.commit?.blob
    if (blob) {
        return {
            content: new TextEncoder().encode(blob.content),
            isBinary: blob.binary,
            byteSize: blob.byteSize,
        }
    }
    return undefined
}
