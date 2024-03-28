import { type Observable, of } from 'rxjs'
import { map } from 'rxjs/operators'

import { memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { makeRepoGitURI, type UIRange } from '@sourcegraph/shared/src/util/url'

import { requestGraphQL } from '../../backend/graphql'
import type { DefinitionFields } from '../../graphql-operations'

const buildRangeKey = (range: UIRange): string => {
    const { start, end } = range
    return `L${start.line}C${start.character}L${end.line}C${end.character}`
}

export const DefinitionFieldsFragment = gql`
    fragment DefinitionFields on Location {
        resource {
            path
            repository {
                name
            }
            commit {
                oid
            }
        }
        range {
            start {
                line
                character
            }
            end {
                line
                character
            }
        }
    }
`

interface FetchDefinitionsResult {
    repository: {
        commit: {
            blob: {
                lsif: {
                    [key: string]: {
                        nodes: DefinitionFields[]
                    }
                }
            }
        }
    }
}

interface FetchDefinitionsVariables {
    repoName: string
    revision: string
    filePath: string
}

interface FetchDefinitionsFromRangesOptions {
    repoName: string
    revision: string
    filePath: string
    ranges: UIRange[]
}

export interface DefinitionResponse {
    range: UIRange
    definition: DefinitionFields | null
}

/**
 * Fetches definitions for the given ranges.
 *
 * Note: This currently works by batching multiple definition queries into a single request through GQL query aliases.
 * It should only be used for ranges that are known to contain definition information ahead of time (e.g. through a `stencil` API call).
 *
 * TODO: Introduce a new backend API that can fetch definitions for multiple ranges in a single request.
 * GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/43179
 */
export const fetchDefinitionsFromRanges = memoizeObservable(
    (options: FetchDefinitionsFromRangesOptions): Observable<DefinitionResponse[] | null> => {
        const { repoName, revision, filePath, ranges } = options

        if (ranges.length < 1) {
            return of(null)
        }

        const result = requestGraphQL<FetchDefinitionsResult, FetchDefinitionsVariables>(
            `
        query Definitions(
            $repoName: String!
            $revision: String!
            $filePath: String!
        ) {
            repository(name: $repoName) {
                commit(rev: $revision) {
                    blob(path: $filePath) {
                        lsif {
                            ${ranges.map(
                                range => `
                                ${buildRangeKey(range)}: definitions(line: ${range.start.line}, character: ${
                                    range.start.character
                                }) {
                                    nodes {
                                        ...DefinitionFields
                                    }
                                }`
                            )}
                        }
                    }
                }
            }
        }

        ${DefinitionFieldsFragment}
    `,
            {
                repoName,
                revision,
                filePath,
            }
        ).pipe(
            map(dataOrThrowErrors),
            map(data => {
                if (!data.repository?.commit) {
                    throw new Error('Commit not found')
                }

                const lsif = data.repository.commit.blob?.lsif

                if (!lsif) {
                    return null
                }

                const definitions = ranges.map(range => {
                    const key = buildRangeKey(range)
                    return {
                        range: { start: range.start, end: range.end },
                        definition: lsif[key]?.nodes[0] ?? null,
                    }
                })

                return definitions
            })
        )

        return result
    },
    options => {
        const { repoName, revision, filePath, ranges } = options
        return `${makeRepoGitURI({ repoName, revision, filePath })}?${ranges.map(buildRangeKey).join(',')}`
    }
)
