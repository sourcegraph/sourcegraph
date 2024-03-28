import { of, lastValueFrom } from 'rxjs'

import {
    type ContentMatch,
    getFileMatchUrl,
    getRepositoryUrl,
    type SearchMatch,
    type PathMatch,
    type RepositoryMatch,
    type CommitMatch,
    getCommitMatchUrl,
    type SymbolMatch,
    type PersonMatch,
    type TeamMatch,
    getOwnerMatchUrl,
    type StreamSearchOptions,
    aggregateStreamingSearch,
    type AggregateStreamingSearchResults,
} from '@sourcegraph/shared/src/search/stream'

import { eventLogger } from '../../../tracking/eventLogger'

export const searchResultsToFileContent = (
    searchResults: SearchMatch[],
    sourcegraphURL: string,
    enableRepositoryMetadata?: boolean
): string => {
    let content = []
    const resultType = searchResults[0].type
    const headers = ['Match type', 'Repository', 'Repository external URL']

    switch (resultType) {
        case 'content':
        case 'path': {
            content = [
                [
                    ...headers,
                    'File path',
                    'File URL',
                    'Path matches [path [start end]]',
                    'Chunk matches [line [start end]]',
                ],
                ...searchResults
                    .filter(
                        (result: SearchMatch): result is ContentMatch | PathMatch =>
                            result.type === 'content' || result.type === 'path'
                    )
                    .map(result => {
                        const repoURL = new URL(
                            getRepositoryUrl(result.repository, result.branches),
                            sourcegraphURL
                        ).toString()
                        const fileURL = new URL(getFileMatchUrl(result), sourcegraphURL).toString()

                        // e.g. for query "codehost" the path match record can be
                        // "[pkg/microservice/systemconfig/core/codehost/repository/models/codehost.go, [[35, 43], [62,70]]]"
                        const pathMatches = result.pathMatches
                            ? `[${result.path}, [${result.pathMatches
                                  .map(match => `[${match.start.column}, ${match.end.column}]`)
                                  .join(' ')}]]`
                            : ''

                        // e.g. for query "codehost" the chunk match record can be
                        // "[24, [[1, 9] [18, 26]]]; [39, [[2, 10] [22, 30]]];" representing:
                        // - line 24 with matches starting from 1 to 9 and from 18 to 26
                        // - line 39 with matches starting from 2 to 10 and from 22 to 30
                        const chunkMatches =
                            'chunkMatches' in result
                                ? result.chunkMatches
                                      ?.map(
                                          match =>
                                              `[${match.contentStart.line}, [${match.ranges
                                                  .map(range => `[${range.start.column}, ${range.end.column}]`)
                                                  .join(' ')}]]`
                                      )
                                      .join('; ')
                                : ''

                        return [
                            result.type,
                            result.repository,
                            repoURL,
                            result.path,
                            fileURL,
                            pathMatches,
                            chunkMatches,
                        ]
                    }),
            ]
            break
        }

        case 'symbol': {
            content = [
                [...headers, 'File path', 'File URL', 'Symbols [kind name url]'],
                ...searchResults
                    .filter((result: SearchMatch): result is SymbolMatch => result.type === 'symbol')
                    .map(result => {
                        const repoURL = new URL(getRepositoryUrl(result.repository), sourcegraphURL).toString()
                        const fileURL = new URL(getFileMatchUrl(result), sourcegraphURL).toString()

                        // e.g. "[FIELD, codeHost, http://localhost:3443/repo/file1.java?L27:20-27:28]; [METHOD, getCodeHost, http://localhost:3443/repo/file2.java?L74:22-74:33]"
                        const symbols = result.symbols
                            .map(
                                symbol =>
                                    `[${symbol.kind}, ${symbol.name}, ${new URL(
                                        symbol.url,
                                        sourcegraphURL
                                    ).toString()}]`
                            )
                            .join('; ')
                        return [result.type, result.repository, repoURL, result.path, fileURL, symbols]
                    }),
            ]
            break
        }

        case 'repo': {
            content = [
                enableRepositoryMetadata ? [...headers, 'Repository metadata', 'Repository metadata JSON'] : headers,
                ...searchResults
                    .filter((result: SearchMatch): result is RepositoryMatch => result.type === 'repo')
                    .map(result => [
                        result.type,
                        result.repository,
                        new URL(getRepositoryUrl(result.repository, result.branches), sourcegraphURL).toString(),
                        ...(enableRepositoryMetadata
                            ? [
                                  Object.entries(result.metadata ?? {})
                                      .map(([key, value]) => (value ? `${key}:${value}` : key))
                                      .join('\n'),
                              ]
                            : []),
                        ...(enableRepositoryMetadata
                            ? [
                                  JSON.stringify(
                                      Object.entries(result.metadata ?? {}).reduce(
                                          (obj: { [key: string]: string | null }, [key, value]) => {
                                              obj[key] = value ?? null
                                              return obj
                                          },
                                          {}
                                      )
                                  ),
                              ]
                            : []),
                    ]),
            ]
            break
        }

        case 'commit': {
            content = [
                [...headers, 'Date', 'Author', 'Message', 'oid', 'Commit URL'],
                ...searchResults
                    .filter((result: SearchMatch): result is CommitMatch => result.type === 'commit')
                    .map(result => {
                        const repoURL = new URL(getRepositoryUrl(result.repository), sourcegraphURL).toString()
                        const commitURL = new URL(getCommitMatchUrl(result), sourcegraphURL).toString()
                        return [
                            result.type,
                            result.repository,
                            repoURL,
                            result.authorDate,
                            result.authorName,
                            result.message,
                            result.oid,
                            commitURL,
                        ]
                    }),
            ]
            break
        }

        case 'person':
        case 'team': {
            content = [
                ['Match type', 'Handle', 'Email', 'User or team name', 'Display name', 'Profile URL'],
                ...searchResults
                    .filter(
                        (result: SearchMatch): result is PersonMatch | TeamMatch =>
                            result.type === 'person' || result.type === 'team'
                    )
                    .map(result => {
                        let profileUrl = getOwnerMatchUrl(result, true)
                        if (profileUrl) {
                            profileUrl = new URL(profileUrl, sourcegraphURL).toString()
                        }

                        return [
                            result.type,
                            result.handle,
                            result.email,
                            result.type === 'person' ? result.user?.username : result.name,
                            result.type === 'person' ? result.user?.displayName : result.displayName,
                            profileUrl,
                        ]
                    }),
            ]
            break
        }

        default: {
            return ''
        }
    }

    return content
        .filter(cells => cells.length > 0)
        .map(cells => cells.map(escapeCell).join(','))
        .join('\n')
}

// Escape a cell value based on IETF RFC 4180
const escapeCell = (cell: string | undefined): string | undefined => {
    if (cell === undefined) {
        return cell
    }
    if (/[\n\r",]/.test(cell)) {
        return `"${cell.replaceAll('"', '""')}"`
    }
    return cell
}

export const buildFileName = (query?: string): string => {
    const formattedQuery = query?.trim().replaceAll(/\W/g, '-')
    // truncate query to account for Windows OS failing to build a file with a name > 255 characters in length
    const truncatedQuery = formattedQuery?.slice(0, 225)
    return `sourcegraph-search-export${truncatedQuery ? `-${truncatedQuery}` : ''}.csv`
}

// If this number is too big, the search will take a very long time and likely fail
// due to the browser terminating the connection on the streaming search request.
// 100k felt like the right balance for this; 1 million was too big for both dotcom and S2.
export const EXPORT_RESULT_DISPLAY_LIMIT = 100000

export const downloadSearchResults = (
    sourcegraphURL: string,
    query: string,
    options: StreamSearchOptions,
    results: AggregateStreamingSearchResults | undefined,
    shouldRerunSearch: boolean
): Promise<void> => {
    const resultsObservable = shouldRerunSearch
        ? aggregateStreamingSearch(of(query), {
              ...options,
              displayLimit: EXPORT_RESULT_DISPLAY_LIMIT,
              maxLineLen: -1, // disable content truncation
          })
        : of(results)

    return lastValueFrom(resultsObservable, { defaultValue: undefined }).then(results => {
        if (results?.state === 'error') {
            const error = results.progress.skipped.find(skipped => skipped.reason === 'error')
            if (error) {
                throw new Error(`${error.title}: ${error.message}`)
            } else {
                throw new Error('Unknown error occured loading search results.')
            }
        }

        if (!results?.results || results?.results?.length === 0) {
            throw new Error('No search results found.')
        }

        const content = searchResultsToFileContent(results.results, sourcegraphURL, options.enableRepositoryMetadata)
        const blob = new Blob([content], { type: 'text/csv' })
        const url = URL.createObjectURL(blob)

        const a = document.createElement('a')
        a.href = url
        a.style.display = 'none'
        a.download = buildFileName(query)
        a.click()
        eventLogger.log('SearchExportPerformed', { count: results.results.length }, { count: results.results.length })

        // cleanup
        a.remove()
        URL.revokeObjectURL(url)
    })
}
