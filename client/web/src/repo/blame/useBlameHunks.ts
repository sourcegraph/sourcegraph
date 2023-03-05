import { useMemo } from 'react'

import { fetchEventSource } from '@microsoft/fetch-event-source'
import { formatDistanceStrict } from 'date-fns'
import { truncate } from 'lodash'
import { Observable, of } from 'rxjs'
import { map, throttleTime } from 'rxjs/operators'

import { memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { makeRepoURI } from '@sourcegraph/shared/src/util/url'
import { useObservable } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../backend/graphql'
import { useFeatureFlag } from '../../featureFlags/useFeatureFlag'
import {
    ExternalServiceKind,
    FirstCommitDateResult,
    FirstCommitDateVariables,
    GitBlameResult,
    GitBlameVariables,
} from '../../graphql-operations'

import { useBlameVisibility } from './useBlameVisibility'

interface BlameHunkDisplayInfo {
    displayName: string
    username: string
    dateString: string
    timestampString: string
    linkURL: string
    message: string
    commitDate: Date
}

export interface BlameHunk {
    startLine: number
    endLine: number
    message: string
    rev: string
    author: {
        date: string
        person: {
            email: string
            displayName: string
            avatarURL: string | null
            user:
                | undefined
                | null
                | {
                      username: string | null
                      displayName: string | null
                      avatarURL: string | null
                  }
        }
    }
    commit: {
        url: string
        parents: {
            oid: string
        }[]
    }
    displayInfo: BlameHunkDisplayInfo
}

export interface BlameHunkData {
    current: BlameHunk[] | undefined
    externalURLs: { url: string; serviceKind: ExternalServiceKind | null }[] | undefined
    firstCommitDate: Date | undefined
}

const fetchBlameViaGraphQL = memoizeObservable(
    ({
        repoName,
        revision,
        filePath,
        sourcegraphURL,
    }: {
        repoName: string
        revision: string
        filePath: string
        sourcegraphURL: string
    }): Observable<BlameHunkData> =>
        requestGraphQL<GitBlameResult, GitBlameVariables>(
            gql`
                query GitBlame($repo: String!, $rev: String!, $path: String!) {
                    repository(name: $repo) {
                        externalURLs {
                            url
                            serviceKind
                        }
                        firstEverCommit {
                            author {
                                date
                            }
                        }
                        commit(rev: $rev) {
                            blob(path: $path) {
                                blame(startLine: 0, endLine: 0) {
                                    startLine
                                    endLine
                                    author {
                                        person {
                                            email
                                            displayName
                                            avatarURL
                                            user {
                                                username
                                                displayName
                                                avatarURL
                                            }
                                        }
                                        date
                                    }
                                    message
                                    rev
                                    commit {
                                        url
                                        parents {
                                            oid
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            `,
            { repo: repoName, rev: revision, path: filePath }
        ).pipe(
            map(dataOrThrowErrors),
            map(({ repository }) => {
                const hunks = repository?.commit?.blob?.blame
                const firstCommitDate = repository?.firstEverCommit?.author?.date
                const externalURLs = repository?.externalURLs
                if (hunks) {
                    return {
                        current: hunks.map(blame => addDisplayInfoForHunk(blame, sourcegraphURL)),
                        externalURLs,
                        firstCommitDate: firstCommitDate ? new Date(firstCommitDate) : undefined,
                    }
                }

                return { current: undefined, externalURLs: undefined, firstCommitDate: undefined }
            })
        ),
    makeRepoURI
)

interface RawStreamHunk {
    author: {
        Name: string
        Email: string
        Date: string
    }
    commit: {
        parents: string[]
        url: string
    }
    commitID: string
    endLine: number
    startLine: number
    filename: string
    message: string
    user?: {
        username: string
        displayName: string | null
        avatarURL: string | null
    }
}

/**
 * Calculating blame hunks on the backend is an expensive operation that gets
 * slower the larger the file and the longer the commit history.
 *
 * To reduce the backend pressure and improve the experience, this fetch
 * implementation uses a SSE stream to load the blame hunks in chunks.
 *
 * It is controlled via the `enable-streaming-git-blame` feature flag and is
 * currently not enabled by default.
 *
 * Since we also need the first commit date for the blame recency calculations,
 * this implementation uses Promise.all() to load both data sources in parallel.
 */
const fetchBlameViaStreaming = memoizeObservable(
    ({
        repoName,
        revision,
        filePath,
        sourcegraphURL,
    }: {
        repoName: string
        revision: string
        filePath: string
        sourcegraphURL: string
    }): Observable<BlameHunkData> =>
        new Observable<BlameHunkData>(subscriber => {
            let didEmitFirstCommitDate = false
            let firstCommitDate: Date | undefined
            let externalURLs: BlameHunkData['externalURLs']

            const assembledHunks: BlameHunk[] = []
            const repoAndRevisionPath = `/${repoName}${revision ? `@${revision}` : ''}`

            Promise.all([
                fetchRepositoryData(repoName).then(res => {
                    firstCommitDate = res.firstCommitDate
                    externalURLs = res.externalURLs
                }),
                fetchEventSource(`/.api/blame${repoAndRevisionPath}/stream/${filePath}`, {
                    method: 'GET',
                    headers: {
                        'X-Requested-With': 'Sourcegraph',
                        'X-Sourcegraph-Should-Trace':
                            new URLSearchParams(window.location.search).get('trace') || 'false',
                    },
                    onmessage(event) {
                        if (event.event === 'hunk') {
                            const rawHunks: RawStreamHunk[] = JSON.parse(event.data)
                            for (const rawHunk of rawHunks) {
                                const hunk: Omit<BlameHunk, 'displayInfo'> = {
                                    startLine: rawHunk.startLine,
                                    endLine: rawHunk.endLine,
                                    message: rawHunk.message,
                                    rev: rawHunk.commitID,
                                    author: {
                                        date: rawHunk.author.Date,
                                        person: {
                                            email: rawHunk.author?.Email,
                                            displayName: rawHunk.author.Name,
                                            avatarURL: rawHunk.user?.avatarURL ?? null,
                                            user: rawHunk.user ?? null,
                                        },
                                    },
                                    commit: {
                                        url: rawHunk.commit.url,
                                        parents: rawHunk.commit.parents
                                            ? rawHunk.commit.parents.map(oid => ({ oid }))
                                            : [],
                                    },
                                }
                                assembledHunks.push(addDisplayInfoForHunk(hunk, sourcegraphURL))
                            }
                            if (firstCommitDate !== undefined) {
                                didEmitFirstCommitDate = true
                            }
                            subscriber.next({ current: assembledHunks, externalURLs, firstCommitDate })
                        }
                    },
                    onerror(event) {
                        // eslint-disable-next-line no-console
                        console.error(event)
                    },
                }),
            ]).then(
                () => {
                    // This case can happen when the event source yields before the commit date is resolved
                    if (!didEmitFirstCommitDate) {
                        subscriber.next({ current: assembledHunks, externalURLs, firstCommitDate })
                    }

                    subscriber.complete()
                },
                error => subscriber.error(error)
            )
        })
            // Throttle the results to avoid re-rendering the blame sidebar for every hunk
            .pipe(throttleTime(1000, undefined, { leading: true, trailing: true })),
    makeRepoURI
)

async function fetchRepositoryData(repoName: string): Promise<Omit<BlameHunkData, 'current'>> {
    return requestGraphQL<FirstCommitDateResult, FirstCommitDateVariables>(
        gql`
            query FirstCommitDate($repo: String!) {
                repository(name: $repo) {
                    firstEverCommit {
                        author {
                            date
                        }
                    }
                    externalURLs {
                        url
                        serviceKind
                    }
                }
            }
        `,
        { repo: repoName }
    )
        .pipe(
            map(dataOrThrowErrors),
            map(({ repository }) => {
                const firstCommitDate = repository?.firstEverCommit?.author?.date
                return {
                    externalURLs: repository?.externalURLs,
                    firstCommitDate: firstCommitDate ? new Date(firstCommitDate) : undefined,
                }
            })
        )
        .toPromise()
}

/**
 * Get display info.
 */
const addDisplayInfoForHunk = (hunk: Omit<BlameHunk, 'displayInfo'>, sourcegraphURL: string): BlameHunk => {
    const now = Date.now()
    const { author, commit, message } = hunk

    const displayName = truncate(author.person.displayName, { length: 25 })
    const username = author.person.user ? `(${author.person.user.username}) ` : ''
    const commitDate = new Date(author.date)
    const dateString = formatDateForBlame(commitDate, now)
    const timestampString = commitDate.toLocaleString()
    const linkURL = new URL(commit.url, sourcegraphURL).href
    const content = truncate(message, { length: 45 })

    ;(hunk as BlameHunk).displayInfo = {
        displayName,
        username,
        commitDate,
        dateString,
        timestampString,
        linkURL,
        message: content,
    }
    return hunk as BlameHunk
}

/**
 * For performance reasons, the hunks array can be mutated in place. To still be
 * able to propagate updates accordingly, this is wrapped in a ref object that
 * can be recreated whenever we emit new values.
 */
export const useBlameHunks = (
    {
        isPackage,
        repoName,
        revision,
        filePath,
        enableCodeMirror,
    }: {
        isPackage: boolean
        repoName: string
        revision: string
        filePath: string
        enableCodeMirror: boolean
    },
    sourcegraphURL: string
): BlameHunkData => {
    const [enableStreamingGitBlame, status] = useFeatureFlag('enable-streaming-git-blame')

    const [isBlameVisible] = useBlameVisibility(isPackage)
    const shouldFetchBlame = isBlameVisible && status !== 'initial'

    const hunks = useObservable(
        useMemo(
            () =>
                shouldFetchBlame
                    ? enableCodeMirror && enableStreamingGitBlame
                        ? fetchBlameViaStreaming({ revision, repoName, filePath, sourcegraphURL })
                        : fetchBlameViaGraphQL({ revision, repoName, filePath, sourcegraphURL })
                    : of({ current: undefined, externalURLs: undefined, firstCommitDate: undefined }),
            [shouldFetchBlame, enableCodeMirror, enableStreamingGitBlame, revision, repoName, filePath, sourcegraphURL]
        )
    )

    return hunks || { current: undefined, externalURLs: undefined, firstCommitDate: undefined }
}

const ONE_MONTH = 30 * 24 * 60 * 60 * 1000
function formatDateForBlame(commitDate: Date, now: number): string {
    if (now - commitDate.getTime() < ONE_MONTH) {
        return formatDistanceStrict(commitDate, now, { addSuffix: true })
    }
    if (commitDate.getFullYear() === new Date(now).getFullYear()) {
        return commitDate.toLocaleString('default', { month: 'short', day: 'numeric' })
    }
    return commitDate.toLocaleString('default', { year: 'numeric', month: 'short', day: 'numeric' })
}
