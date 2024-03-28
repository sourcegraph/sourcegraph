import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError, memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import {
    CloneInProgressError,
    RepoNotFoundError,
    RepoSeeOtherError,
    RepoDeniedError,
    RevisionNotFoundError,
} from '@sourcegraph/shared/src/backend/errors'
import {
    makeRepoGitURI,
    type RepoRevision,
    type RepoSpec,
    type ResolvedRevisionSpec,
    type RevisionSpec,
} from '@sourcegraph/shared/src/util/url'

import { queryGraphQL, requestGraphQL } from '../backend/graphql'
import type {
    ExternalLinkFields,
    FileExternalLinksResult,
    RepositoryFields,
    ResolveRepoRevResult,
} from '../graphql-operations'

export const externalLinkFieldsFragment = gql`
    fragment ExternalLinkFields on ExternalLink {
        url
        serviceKind
    }
`

export const repositoryFragment = gql`
    fragment RepositoryFields on Repository {
        id
        name
        url
        sourceType
        externalURLs {
            url
            serviceKind
        }
        externalRepository {
            serviceType
            serviceID
        }
        description
        viewerCanAdminister
        defaultBranch {
            displayName
            abbrevName
        }
        isFork
        metadata {
            key
            value
        }
        topics
    }
`

export interface ResolvedRevision extends ResolvedRevisionSpec {
    defaultBranch: string

    /** The URL to the repository root tree at the revision. */
    rootTreeURL: string
}

export interface Repo {
    repo: RepositoryFields
}

/**
 * When `revision` is undefined, the default branch is resolved.
 *
 * @returns Observable that emits the commit ID. Errors with a `CloneInProgressError` if the repo is still being cloned.
 */
export const resolveRepoRevision = memoizeObservable(
    ({ repoName, revision }: RepoSpec & Partial<RevisionSpec>): Observable<ResolvedRevision & Repo> =>
        queryGraphQL<ResolveRepoRevResult>(
            gql`
                query ResolveRepoRev($repoName: String!, $revision: String!) {
                    repositoryRedirect(name: $repoName) {
                        __typename
                        ... on Repository {
                            ...RepositoryFields
                            mirrorInfo {
                                cloneInProgress
                                cloneProgress
                                cloned
                            }
                            commit(rev: $revision) {
                                __typename
                                ...GitCommitFieldsWithTree
                            }
                            changelist(cid: $revision) {
                                __typename
                                cid
                                canonicalURL
                                commit {
                                    __typename
                                    ...GitCommitFieldsWithTree
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

                fragment GitCommitFieldsWithTree on GitCommit {
                    oid
                    tree(path: "") {
                        url
                    }
                }
                ${repositoryFragment}
            `,
            { repoName, revision: revision || '' }
        ).pipe(
            map(({ data, errors }) => {
                if (errors?.length === 1 && errors[0].extensions?.code === 'ErrRepoDenied') {
                    throw new RepoDeniedError(errors[0].message)
                }
                if (!data) {
                    throw createAggregateError(errors)
                }
                if (!data.repositoryRedirect) {
                    throw new RepoNotFoundError(repoName)
                }
                if (data.repositoryRedirect.__typename === 'Redirect') {
                    throw new RepoSeeOtherError(data.repositoryRedirect.url)
                }
                if (data.repositoryRedirect.mirrorInfo.cloneInProgress) {
                    throw new CloneInProgressError(
                        repoName,
                        data.repositoryRedirect.mirrorInfo.cloneProgress || undefined
                    )
                }
                if (!data.repositoryRedirect.mirrorInfo.cloned) {
                    throw new CloneInProgressError(repoName, 'queued for cloning')
                }

                // The "revision" we queried for could be a commit or a changelist.
                const commit = data.repositoryRedirect.commit || data.repositoryRedirect.changelist?.commit
                if (!commit) {
                    throw new RevisionNotFoundError(revision)
                }

                const defaultBranch = data.repositoryRedirect.defaultBranch?.abbrevName || 'HEAD'

                if (!commit.tree) {
                    throw new RevisionNotFoundError(defaultBranch)
                }

                return {
                    repo: data.repositoryRedirect,
                    commitID: commit.oid,
                    defaultBranch,
                    rootTreeURL: commit.tree.url,
                }
            })
        ),
    makeRepoGitURI
)

export const fetchFileExternalLinks = memoizeObservable(
    (context: RepoRevision & { filePath: string }): Observable<ExternalLinkFields[]> =>
        queryGraphQL<FileExternalLinksResult>(
            gql`
                query FileExternalLinks($repoName: String!, $revision: String!, $filePath: String!) {
                    repository(name: $repoName) {
                        commit(rev: $revision) {
                            file(path: $filePath) {
                                externalURLs {
                                    ...ExternalLinkFields
                                }
                            }
                        }
                    }
                }

                ${externalLinkFieldsFragment}
            `,
            context
        ).pipe(
            map(({ data, errors }) => {
                if (!data?.repository?.commit?.file?.externalURLs) {
                    throw createAggregateError(errors)
                }
                return data.repository.commit.file.externalURLs
            })
        ),
    makeRepoGitURI
)

interface FetchCommitMessageResult {
    __typename?: 'Query'
    repository: {
        commit: {
            message: string
        }
    }
}

export const fetchCommitMessage = memoizeObservable(
    (context: RepoRevision): Observable<string> =>
        requestGraphQL<FetchCommitMessageResult, RepoRevision>(
            gql`
                query CommitMessage($repoName: String!, $revision: String!) {
                    repository(name: $repoName) {
                        commit(rev: $revision) {
                            message
                        }
                    }
                }
            `,
            context
        ).pipe(
            map(dataOrThrowErrors),
            map(data => data.repository.commit.message)
        ),
    makeRepoGitURI
)
