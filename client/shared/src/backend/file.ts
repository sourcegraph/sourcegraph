import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError, memoizeObservable } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'

import {
    type HighlightedFileResult,
    type HighlightedFileVariables,
    type HighlightLineRange,
    HighlightResponseFormat,
} from '../graphql-operations'
import type { PlatformContext } from '../platform/context'
import { makeRepoGitURI } from '../util/url'

/*
    Highlighted file result query doesn't support `format` on Sourcegraph versions older than 3.43.
    As we don't have feature detection implemented for the VSCode extensions yet,
    we omit `format` variable for this query if it comes from the VSCode extension.
*/
type RequestVariables = Omit<HighlightedFileVariables, 'format'> & { format?: HighlightedFileVariables['format'] }

const IS_VSCE = typeof window !== 'undefined' && typeof (window as any).acquireVsCodeApi === 'function'

const HIGHLIGHTED_FILE_QUERY = gql`
    query HighlightedFile(
        $repoName: String!
        $commitID: String!
        $filePath: String!
        $disableTimeout: Boolean!
        $ranges: [HighlightLineRange!]!
        $format: HighlightResponseFormat!
    ) {
        repository(name: $repoName) {
            commit(rev: $commitID) {
                file(path: $filePath) {
                    isDirectory
                    richHTML
                    highlight(disableTimeout: $disableTimeout, format: $format) {
                        aborted
                        lineRanges(ranges: $ranges)
                    }
                }
            }
        }
    }
`

const VSCE_HIGHLIGHTED_FILE_QUERY = gql`
    query HighlightedFileVSCE(
        $repoName: String!
        $commitID: String!
        $filePath: String!
        $disableTimeout: Boolean!
        $ranges: [HighlightLineRange!]!
    ) {
        repository(name: $repoName) {
            commit(rev: $commitID) {
                file(path: $filePath) {
                    isDirectory
                    richHTML
                    highlight(disableTimeout: $disableTimeout) {
                        aborted
                        lineRanges(ranges: $ranges)
                    }
                }
            }
        }
    }
`

export interface FetchFileParameters {
    repoName: string
    commitID: string
    filePath: string
    disableTimeout?: boolean
    ranges: HighlightLineRange[]
    format?: HighlightResponseFormat
}

/**
 * Fetches the specified highlighted file line ranges (`FetchFileParameters.ranges`) and returns
 * them as a list of ranges, each describing a list of lines in the form of HTML table '<tr>...</tr>'.
 */
export const fetchHighlightedFileLineRanges = memoizeObservable(
    ({
        platformContext,
        format = HighlightResponseFormat.HTML_HIGHLIGHT,
        ...context
    }: FetchFileParameters & {
        platformContext: Pick<PlatformContext, 'requestGraphQL'>
    }): Observable<string[][]> => {
        let request = HIGHLIGHTED_FILE_QUERY
        const variables: RequestVariables = {
            ...context,
            format,
            disableTimeout: Boolean(context.disableTimeout),
        }

        if (IS_VSCE) {
            /*
                Highlighted file result query doesn't support `format` on Sourcegraph versions older than 3.43.
                As we don't have feature detection implemented for the VSCode extensions yet,
                we omit `format` variable for this query if it comes from the VSCode extension.
            */
            request = VSCE_HIGHLIGHTED_FILE_QUERY
            delete variables.format
        }

        return platformContext
            .requestGraphQL<HighlightedFileResult, RequestVariables>({
                request,
                variables,
                mightContainPrivateInfo: true,
            })
            .pipe(
                map(({ data, errors }) => {
                    if (!data?.repository?.commit?.file?.highlight) {
                        throw createAggregateError(errors)
                    }
                    const file = data.repository.commit.file
                    if (file.isDirectory) {
                        return []
                    }
                    return file.highlight.lineRanges
                })
            )
    },
    context =>
        makeRepoGitURI(context) +
        `?disableTimeout=${String(context.disableTimeout)}&ranges=${context.ranges
            .map(range => `${range.startLine}:${range.endLine}`)
            .join(',')}&format=${context.format}`
)
