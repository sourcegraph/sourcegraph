import { SourcegraphURL } from '@sourcegraph/common'
import type { Position, Range } from '@sourcegraph/extension-api-types'
import {
    encodeRepoRevision,
    type ParsedRepoRevision,
    type ParsedRepoURI,
    parseRepoRevision,
    type RepoFile,
} from '@sourcegraph/shared/src/util/url'

export function toTreeURL(target: RepoFile): string {
    return `/${encodeRepoRevision(target)}/-/tree/${target.filePath}`
}

/**
 * Replaces the revision in the given URL, or adds one if there is not already
 * one.
 *
 * @param href The URL whose revision should be replaced.
 */
export function replaceRevisionInURL(href: string, newRevision: string): string {
    const parsed = parseBrowserRepoURL(href)
    const repoRevision = `/${encodeRepoRevision(parsed)}`

    const url = new URL(href, window.location.href)
    url.pathname = `/${encodeRepoRevision({ ...parsed, revision: newRevision })}${url.pathname.slice(
        repoRevision.length
    )}`
    return `${url.pathname}${url.search}${url.hash}`
}

/**
 * Parses the properties of a blob URL.
 */
export function parseBrowserRepoURL(href: string): ParsedRepoURI & Pick<ParsedRepoRevision, 'rawRevision'> {
    const url = SourcegraphURL.from(href)
    let pathname = url.pathname.slice(1) // trim leading '/'
    if (pathname.endsWith('/')) {
        pathname = pathname.slice(0, -1) // trim trailing '/'
    }

    const indexOfSeparator = pathname.indexOf('/-/')

    // examples:
    // - 'github.com/gorilla/mux'
    // - 'github.com/gorilla/mux@revision'
    // - 'foo/bar' (from 'sourcegraph.mycompany.com/foo/bar')
    // - 'foo/bar@revision' (from 'sourcegraph.mycompany.com/foo/bar@revision')
    // - 'foobar' (from 'sourcegraph.mycompany.com/foobar')
    // - 'foobar@revision' (from 'sourcegraph.mycompany.com/foobar@revision')
    let repoRevision: string
    if (indexOfSeparator === -1) {
        repoRevision = pathname // the whole string
    } else {
        repoRevision = pathname.slice(0, indexOfSeparator) // the whole string leading up to the separator (allows revision to be multiple path parts)
    }
    const { repoName, revision, rawRevision } = parseRepoRevision(repoRevision)
    if (!repoName) {
        throw new Error('unexpected repo url: ' + href)
    }
    const commitID = revision && /^[\da-f]{40}$/i.test(revision) ? revision : undefined

    let filePath: string | undefined
    let commitRange: string | undefined
    const treeSeparator = pathname.indexOf('/-/tree/')
    const blobSeparator = pathname.indexOf('/-/blob/')
    const comparisonSeparator = pathname.indexOf('/-/compare/')
    const commitsSeparator = pathname.indexOf('/-/commits/')
    const changelistsSeparator = pathname.indexOf('/-/changelists/')
    if (treeSeparator !== -1) {
        filePath = decodeURIComponent(pathname.slice(treeSeparator + '/-/tree/'.length))
    }
    if (blobSeparator !== -1) {
        filePath = decodeURIComponent(pathname.slice(blobSeparator + '/-/blob/'.length))
    }
    if (comparisonSeparator !== -1) {
        commitRange = pathname.slice(comparisonSeparator + '/-/compare/'.length)
    }
    if (commitsSeparator !== -1) {
        filePath = decodeURIComponent(pathname.slice(commitsSeparator + '/-/commits/'.length))
    }
    if (changelistsSeparator !== -1) {
        filePath = decodeURIComponent(pathname.slice(changelistsSeparator + '/-/changelists/'.length))
    }
    let position: Position | undefined
    let range: Range | undefined

    const lineRange = url.lineRange
    if (lineRange.line) {
        position = {
            line: lineRange.line,
            character: lineRange.character || 0,
        }
        if (lineRange.endLine) {
            range = {
                start: position,
                end: {
                    line: lineRange.endLine,
                    character: lineRange.endCharacter || 0,
                },
            }
        }
    }
    return { repoName, revision, rawRevision, commitID, filePath, commitRange, position, range }
}
