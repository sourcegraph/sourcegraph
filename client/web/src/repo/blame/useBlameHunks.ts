import { useMemo } from 'react'

import { EventStreamContentType, fetchEventSource } from '@microsoft/fetch-event-source'
import { formatDistanceStrict } from 'date-fns'
import { truncate } from 'lodash'
import { Observable, lastValueFrom, of } from 'rxjs'
import { catchError, map, throttleTime } from 'rxjs/operators'

import { type ErrorLike, memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { makeRepoURI } from '@sourcegraph/shared/src/util/url'
import { useObservable } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../backend/graphql'
import type { ExternalServiceKind, FirstCommitDateResult, FirstCommitDateVariables } from '../../graphql-operations'

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
    }): Observable<BlameHunkData | ErrorLike> =>
        new Observable<BlameHunkData | ErrorLike>(subscriber => {
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
                    async onopen(response) {
                        if (response.ok && response.headers.get('content-type') === EventStreamContentType) {
                            return
                        }
                        throw new Error('request for blame data failed: ' + (await response.text()))
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
                        throw new Error(event)
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
            .pipe(
                throttleTime(1000, undefined, { leading: true, trailing: true }),
                catchError(error => of(error))
            ),
    makeRepoURI
)

async function fetchRepositoryData(repoName: string): Promise<Omit<BlameHunkData, 'current'>> {
    return lastValueFrom(
        requestGraphQL<FirstCommitDateResult, FirstCommitDateVariables>(
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
        ).pipe(
            map(dataOrThrowErrors),
            map(({ repository }) => {
                const firstCommitDate = repository?.firstEverCommit?.author?.date
                return {
                    externalURLs: repository?.externalURLs,
                    firstCommitDate: firstCommitDate ? new Date(firstCommitDate) : undefined,
                }
            })
        )
    )
}

/**
 * Get display info shared between status bar items and text document decorations.
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
    }: {
        isPackage: boolean
        repoName: string
        revision: string
        filePath: string
    },
    sourcegraphURL: string
): BlameHunkData | ErrorLike => {
    const [isBlameVisible] = useBlameVisibility(isPackage)
    const shouldFetchBlame = isBlameVisible

    const hunks = useObservable(
        useMemo(
            () =>
                shouldFetchBlame
                    ? fetchBlameViaStreaming({ revision, repoName, filePath, sourcegraphURL })
                    : of({ current: undefined, externalURLs: undefined, firstCommitDate: undefined }),
            [shouldFetchBlame, revision, repoName, filePath, sourcegraphURL]
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
