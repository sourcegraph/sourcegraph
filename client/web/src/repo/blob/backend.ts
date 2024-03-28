import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { makeRepoGitURI } from '@sourcegraph/shared/src/util/url'

import { requestGraphQL } from '../../backend/graphql'
import {
    type BlobFileFields,
    type BlobResult,
    type BlobVariables,
    HighlightResponseFormat,
} from '../../graphql-operations'

/**
 * Makes sure that default values are applied consistently for the cache key and the `fetchBlob` function.
 */
const applyDefaultValuesToFetchBlobOptions = ({
    disableTimeout = false,
    format = HighlightResponseFormat.HTML_HIGHLIGHT,
    startLine = null,
    endLine = null,
    visibleIndexID = null,
    scipSnapshot = false,
    ...options
}: FetchBlobOptions): Required<FetchBlobOptions> => ({
    ...options,
    disableTimeout,
    format,
    startLine,
    endLine,
    visibleIndexID,
    scipSnapshot,
})

function fetchBlobCacheKey(options: FetchBlobOptions): string {
    const { disableTimeout, format, scipSnapshot, visibleIndexID } = applyDefaultValuesToFetchBlobOptions(options)

    return `${makeRepoGitURI(
        options
    )}?disableTimeout=${disableTimeout}&=${format}&snap=${scipSnapshot}&visible=${visibleIndexID}`
}

interface FetchBlobOptions {
    repoName: string
    revision: string
    filePath: string
    disableTimeout?: boolean
    format?: HighlightResponseFormat
    startLine?: number | null
    endLine?: number | null
    scipSnapshot?: boolean
    visibleIndexID?: string | null
}

export const fetchBlob = memoizeObservable(
    (
        options: FetchBlobOptions
    ): Observable<
        (BlobFileFields & { snapshot?: { offset: number; data: string; additional: string[] | null }[] | null }) | null
    > => {
        const {
            repoName,
            revision,
            filePath,
            disableTimeout,
            format,
            startLine,
            endLine,
            scipSnapshot,
            visibleIndexID,
        } = applyDefaultValuesToFetchBlobOptions(options)

        return requestGraphQL<BlobResult, BlobVariables>(
            gql`
                query Blob(
                    $repoName: String!
                    $revision: String!
                    $filePath: String!
                    $disableTimeout: Boolean!
                    $format: HighlightResponseFormat!
                    $startLine: Int
                    $endLine: Int
                    $snapshot: Boolean!
                    $visibleIndexID: ID!
                ) {
                    repository(name: $repoName) {
                        commit(rev: $revision) {
                            __typename
                            ...GitCommitFieldsWithFileAndBlob
                        }
                        changelist(cid: $revision) {
                            __typename
                            cid
                            commit {
                                ...GitCommitFieldsWithFileAndBlob
                            }
                        }
                    }
                }

                fragment GitCommitFieldsWithFileAndBlob on GitCommit {
                    oid
                    file(path: $filePath) {
                        ...BlobFileFields
                    }
                    blob(path: $filePath) @include(if: $snapshot) {
                        lsif {
                            snapshot(indexID: $visibleIndexID) {
                                offset
                                data
                                additional
                            }
                        }
                    }
                }

                fragment BlobFileFields on File2 {
                    __typename
                    content(startLine: $startLine, endLine: $endLine)
                    richHTML(startLine: $startLine, endLine: $endLine)
                    highlight(
                        disableTimeout: $disableTimeout
                        format: $format
                        startLine: $startLine
                        endLine: $endLine
                    ) {
                        aborted
                        lsif
                    }
                    totalLines
                    ... on GitBlob {
                        lfs {
                            byteSize
                        }
                        externalURLs {
                            url
                            serviceKind
                        }
                    }
                    languages
                }
            `,
            {
                repoName,
                revision,
                filePath,
                disableTimeout,
                format,
                startLine,
                endLine,
                snapshot: scipSnapshot,
                visibleIndexID: visibleIndexID ?? '',
            }
        ).pipe(
            map(dataOrThrowErrors),
            map(data => {
                const commit = data.repository?.commit || data.repository?.changelist?.commit

                if (!commit) {
                    throw new Error('Commit not found')
                }

                if (commit.file === null) {
                    return null
                }

                return {
                    ...commit.file,
                    snapshot: commit?.blob?.lsif?.snapshot,
                }
            })
        )
    },
    fetchBlobCacheKey
)
