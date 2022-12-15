import {
    AggregateStreamingSearchResults,
    ContentMatch,
    getFileMatchUrl,
    getRepositoryUrl,
    SearchMatch,
    PathMatch,
    RepositoryMatch,
    CommitMatch,
    getCommitMatchUrl,
    SymbolMatch,
} from '@sourcegraph/shared/src/search/stream'

import { eventLogger } from '../../tracking/eventLogger'

const sanitizeString = (str: string): string =>
    `"${str
        .replaceAll(/"/g, '""') // escape quotes
        .replaceAll(/ +(?= )/g, '') // remove extra spaces
        .replaceAll(/\n/g, '')}"` // remove extra newlines

const searchResultsToFileContent = (searchResults: SearchMatch[], sourcegraphURL: string): string => {
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
                        const pathMatches = result.pathMatches
                            ? JSON.stringify(
                                  `[${result.path}, [${result.pathMatches
                                      .map(match => `[${match.start.column}, ${match.end.column}]`)
                                      .join(' ')}]]`
                              )
                            : ''
                        const chunkMatches =
                            'chunkMatches' in result
                                ? JSON.stringify(
                                      result.chunkMatches
                                          ?.map(
                                              match =>
                                                  `[${match.contentStart.line}, [${match.ranges
                                                      .map(range => `[${range.start.column}, ${range.end.column}]`)
                                                      .join(' ')}]]`
                                          )
                                          .join('; ')
                                  )
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
                        const symbols = JSON.stringify(
                            result.symbols
                                .map(
                                    symbol =>
                                        `[${symbol.kind}, ${symbol.name}, ${new URL(
                                            symbol.url,
                                            sourcegraphURL
                                        ).toString()}]`
                                )
                                .join('; ')
                        )
                        return [result.type, result.repository, repoURL, result.path, fileURL, symbols]
                    }),
            ]
            break
        }

        case 'repo': {
            content = [
                headers,
                ...searchResults
                    .filter((result: SearchMatch): result is RepositoryMatch => result.type === 'repo')
                    .map(result => [
                        result.type,
                        result.repository,
                        new URL(getRepositoryUrl(result.repository, result.branches), sourcegraphURL).toString(),
                    ]),
            ]
            break
        }

        case 'commit': {
            content = [
                [...headers, 'Date', 'Author', 'Subject', 'oid', 'Commit URL'],
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
                            sanitizeString(result.message),
                            result.oid,
                            commitURL,
                        ]
                    }),
            ]
            break
        }

        default:
            return ''
    }

    return content
        .filter(cells => cells.length > 0)
        .map(cells => cells.join(','))
        .join('\n')
}

export const requestSearchResultsExport = (
    results: AggregateStreamingSearchResults,
    sourcegraphURL: string,
    query?: string
) => {
    const content = searchResultsToFileContent(results.results, sourcegraphURL)
    const formattedQuery = query?.trim().replace(/\W/g, '-')
    const downloadFilename = `sourcegraph-search-export${formattedQuery ? `-${formattedQuery}` : ''}.csv`
    const blob = new Blob([content], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)

    const a = document.createElement('a')
    a.href = url
    a.style.display = 'none'
    a.download = downloadFilename
    a.click()
    eventLogger.log('SearchExportPerformed', { count: results.results.length }, { count: results.results.length })

    // cleanup
    a.remove()
    URL.revokeObjectURL(url)
}
