import { from, type Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError, memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'

import type { ResolveRawRepoNameResult, TreeEntriesResult, TreeFields } from '../graphql-operations'
import type { PlatformContext } from '../platform/context'
import { type AbsoluteRepoFile, makeRepoGitURI, type RepoSpec } from '../util/url'

import { CloneInProgressError, RepoNotFoundError } from './errors'

/**
 * @returns Observable that emits the `rawRepoName`. Errors with a `CloneInProgressError` if the repo is still being cloned.
 */
export const resolveRawRepoName = memoizeObservable(
    ({
        requestGraphQL,
        repoName,
    }: Pick<RepoSpec, 'repoName'> & Pick<PlatformContext, 'requestGraphQL'>): Observable<string> =>
        from(
            requestGraphQL<ResolveRawRepoNameResult>({
                request: gql`
                    query ResolveRawRepoName($repoName: String!) {
                        repository(name: $repoName) {
                            uri
                            mirrorInfo {
                                cloned
                            }
                        }
                    }
                `,
                variables: { repoName },
                mightContainPrivateInfo: true,
            })
        ).pipe(
            map(dataOrThrowErrors),
            map(({ repository }) => {
                if (!repository) {
                    throw new RepoNotFoundError(repoName)
                }
                if (!repository.mirrorInfo.cloned) {
                    throw new CloneInProgressError(repoName)
                }
                return repository.uri
            })
        ),
    ({ repoName }) => repoName
)

export const fetchTreeEntries = memoizeObservable(
    ({
        requestGraphQL,
        ...args
    }: AbsoluteRepoFile & { first?: number } & Pick<PlatformContext, 'requestGraphQL'>): Observable<TreeFields> =>
        requestGraphQL<TreeEntriesResult>({
            request: gql`
                query TreeEntries(
                    $repoName: String!
                    $revision: String!
                    $commitID: String!
                    $filePath: String!
                    $first: Int
                ) {
                    repository(name: $repoName) {
                        id
                        commit(rev: $commitID, inputRevspec: $revision) {
                            tree(path: $filePath) {
                                ...TreeFields
                            }
                        }
                    }
                }
                fragment TreeFields on GitTree {
                    isRoot
                    url
                    entries(first: $first) {
                        __typename
                        ...TreeEntryFields
                        ... on GitBlob {
                            languages
                        }
                    }
                }
                fragment TreeEntryFields on TreeEntry {
                    name
                    path
                    isDirectory
                    url
                    submodule {
                        url
                        commit
                    }
                }
            `,
            variables: args,
            mightContainPrivateInfo: true,
        }).pipe(
            map(({ data, errors }) => {
                if (errors || !data?.repository?.commit?.tree) {
                    throw createAggregateError(errors)
                }
                return data.repository.commit.tree
            })
        ),
    ({ first, requestGraphQL, ...args }) => `${makeRepoGitURI(args)}:first-${String(first)}`
)
