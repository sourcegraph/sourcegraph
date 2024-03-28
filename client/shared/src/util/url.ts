import { parseURL } from 'whatwg-url'

import { encodeURIPathComponent, escapeRevspecForURL, SourcegraphURL } from '@sourcegraph/common'
import type { Position } from '@sourcegraph/extension-api-types'

import type { WorkspaceRootWithMetadata } from '../api/extension/extensionHostApi'
import type { SearchPatternType } from '../graphql-operations'
import { discreteValueAliases } from '../search/query/filters'
import { findFilter, FilterKind } from '../search/query/query'
import { appendContextFilter, omitFilter } from '../search/query/transformer'
import { SearchMode } from '../search/searchQueryState'

export interface RepoSpec {
    /**
     * The name of this repository on a Sourcegraph instance,
     * as affected by `repositoryPathPattern`.
     *
     * Example: `sourcegraph/sourcegraph`
     */
    repoName: string
}

export interface RawRepoSpec {
    /**
     * The name of this repository, unaffected by `repositoryPathPattern`.
     *
     * Example: `github.com/sourcegraph/sourcegraph`
     */
    rawRepoName: string
}

export interface RevisionSpec {
    /**
     * a revision string (like 'master' or 'my-branch' or '24fca303ac6da784b9e8269f724ddeb0b2eea5e7')
     */
    revision: string
}

export interface ResolvedRevisionSpec {
    /**
     * a 40 character commit SHA
     */
    commitID: string
}

export interface FileSpec {
    /**
     * a path to a directory or file
     */
    filePath: string
}

interface ComparisonSpec {
    /**
     * a diff specifier with optional base and comparison. Examples:
     * - "master..." (implicitly: "master...HEAD")
     * - "...my-branch" (implicitly: "HEAD...my-branch")
     * - "master...my-branch"
     */
    commitRange: string
}

/**
 * 1-indexed position in a blob.
 * Positions in URLs are 1-indexed.
 */
export interface UIPosition {
    /** 1-indexed line number */
    line: number

    /** 1-indexed character number */
    character: number
}

/**
 * 1-indexed range in a blob.
 * Ranges in URLs are 1-indexed.
 */
export interface UIRange {
    start: UIPosition
    end: UIPosition
}

export interface UIPositionSpec {
    /**
     * A 1-indexed point in the blob
     */
    position: UIPosition
}

export interface UIRangeSpec {
    /**
     * A 1-indexed range in the blob
     */
    range: UIRange
}

/**
 * Specifies an LSP mode.
 */
export interface ModeSpec {
    /** The LSP mode, which identifies the language server to use. */
    mode: string
}

// `panelID` is intended for substitution (e.g. `sub(panel.url, 'panelID', 'implementations')`)
export type BlobViewState = 'def' | 'references' | 'panelID'

export interface ViewStateSpec {
    /**
     * The view state (for the blob panel).
     */
    viewState: BlobViewState
}

/**
 * 'code' for Markdown/rich-HTML files rendered as code, 'rendered' for rendering them as
 * Markdown/rich-HTML, undefined for the default for the file type ('rendered' for Markdown, etc.,
 * 'code' otherwise).
 */
export type RenderMode = 'code' | 'rendered' | undefined

export interface RenderModeSpec {
    /**
     * How the file should be rendered.
     */
    renderMode: RenderMode
}

/**
 * Properties of a RepoURI (like git://github.com/gorilla/mux#mux.go) or a URL (like https://sourcegraph.com/github.com/gorilla/mux/-/blob/mux.go)
 */
export interface ParsedRepoURI
    extends RepoSpec,
        Partial<RevisionSpec>,
        Partial<ResolvedRevisionSpec>,
        Partial<FileSpec>,
        Partial<ComparisonSpec>,
        Partial<UIPositionSpec>,
        Partial<UIRangeSpec> {}

/**
 * RepoURI is a URI identifing a repository resource, like
 * - the repository itself: `git://github.com/gorilla/mux`
 * - the repository at a particular revision: `git://github.com/gorilla/mux?revision`
 * - a file in a repository at an immutable revision: `git://github.com/gorilla/mux?SHA#path/to/file.go
 * - a line in a file in a repository at an immutable revision: `git://github.com/gorilla/mux?SHA#path/to/file.go:3
 * - a character position in a file in a repository at an immutable revision: `git://github.com/gorilla/mux?SHA#path/to/file.go:3,5
 * - a rangein a file in a repository at an immutable revision: `git://github.com/gorilla/mux?SHA#path/to/file.go:3,5-4,9
 */
type RepoURI = string

const parsePosition = (string: string): Position => {
    const split = string.split(',')
    if (split.length === 1) {
        return { line: parseInt(string, 10), character: 0 }
    }
    if (split.length === 2) {
        return { line: parseInt(split[0], 10), character: parseInt(split[1], 10) }
    }
    throw new Error('unexpected position: ' + string)
}

/**
 * Parses the properties of a legacy Git URI like git://github.com/gorilla/mux#mux.go.
 *
 * These URIs were used when communicating with language servers over LSP and with extensions. They are being
 * phased out in favor of URLs to resources in the Sourcegraph raw API, which do not require out-of-band
 * information to fetch the contents of.
 * @deprecated Migrate to using URLs to the Sourcegraph raw API (or other concrete URLs) instead.
 */
export function parseRepoGitURI(uri: RepoURI): ParsedRepoURI {
    // We are not using the environments URL constructor because Chrome and Firefox do
    // not correctly parse out the hostname for URLs . We have a polyfill for the main web app
    // (see client/shared/src/polyfills/configure-core-js.ts) but that might not be used in all apps.
    const parsed = parseURL(uri)
    if (!parsed?.host) {
        throw new Error('Unable to parse repo URI: ' + uri)
    }
    const pathname =
        typeof parsed.path === 'string' ? parsed.path : parsed.path.length === 0 ? '' : '/' + parsed.path.join('/')
    const repoName = String(parsed.host) + decodeURIComponent(pathname)
    const revision = parsed.query ? decodeURIComponent(parsed.query) : undefined
    let commitID: string | undefined
    if (revision?.match(/[\dA-f]{40}/)) {
        commitID = revision
    }
    const fragmentSplit = parsed.fragment ? parsed.fragment.split(':').map(decodeURIComponent) : []
    let filePath: string | undefined
    let position: UIPosition | undefined
    let range: UIRange | undefined
    if (fragmentSplit.length === 1) {
        filePath = fragmentSplit[0]
    }
    if (fragmentSplit.length === 2) {
        filePath = fragmentSplit[0]
        const rangeOrPosition = fragmentSplit[1]
        const rangeOrPositionSplit = rangeOrPosition.split('-')

        if (rangeOrPositionSplit.length === 1) {
            position = parsePosition(rangeOrPositionSplit[0])
        }
        if (rangeOrPositionSplit.length === 2) {
            range = { start: parsePosition(rangeOrPositionSplit[0]), end: parsePosition(rangeOrPositionSplit[1]) }
        }
        if (rangeOrPositionSplit.length > 2) {
            throw new Error('unexpected range or position: ' + rangeOrPosition)
        }
    }
    if (fragmentSplit.length > 2) {
        throw new Error('unexpected fragment: ' + parsed.fragment)
    }

    return { repoName, revision, commitID, filePath: filePath || undefined, position, range }
}

/**
 * The inverse of parseRepoGitURI, this generates a string from parsed values.
 * Example output: `git://github.com/gorilla/mux?SHA#mux.go:3,5-4,9`
 */
export function makeRepoGitURI(parsed: ParsedRepoURI): RepoURI {
    const revision = parsed.commitID || parsed.revision
    let uri = `git://${encodeURIPathComponent(parsed.repoName)}`
    uri += revision ? '?' + encodeURIPathComponent(revision) : ''
    uri += parsed.filePath ? '#' + encodeURIPathComponent(parsed.filePath) : ''
    uri += parsed.position || parsed.range ? ':' : ''
    uri += parsed.position ? positionString(parsed.position) : ''
    uri += parsed.range ? positionString(parsed.range.start) + '-' + positionString(parsed.range.end) : ''
    return uri
}

/**
 * A repo
 */
export interface Repo extends RepoSpec {}

/**
 * A repo with a (possibly unresolved) revspec.
 */
export interface RepoRevision extends RepoSpec, RevisionSpec {}

/**
 * A repo resolved to an exact commit
 */
export interface AbsoluteRepo extends RepoSpec, RevisionSpec, ResolvedRevisionSpec {}

/**
 * A file in a repo
 */
export interface RepoFile extends RepoSpec, RevisionSpec, Partial<ResolvedRevisionSpec>, FileSpec {}

/**
 * A file at an exact commit
 */
export interface AbsoluteRepoFile extends RepoSpec, RevisionSpec, ResolvedRevisionSpec, FileSpec {}

/**
 * A position in file at an exact commit
 */
export interface AbsoluteRepoFilePosition
    extends RepoSpec,
        RevisionSpec,
        ResolvedRevisionSpec,
        FileSpec,
        UIPositionSpec,
        Partial<ViewStateSpec>,
        Partial<RenderModeSpec> {}

/** Encodes a repository at a revspec for use in a URL. */
export function encodeRepoRevision({ repoName, revision }: RepoSpec & Partial<RevisionSpec>): string {
    return revision ? `${encodeURIPathComponent(repoName)}@${escapeRevspecForURL(revision)}` : repoName
}

export function toPrettyBlobURL(
    target: RepoSpec &
        Partial<RevisionSpec> &
        Partial<ResolvedRevisionSpec> &
        FileSpec &
        Partial<UIPositionSpec> &
        Partial<ViewStateSpec> &
        Partial<UIRangeSpec> &
        Partial<RenderModeSpec>
): string {
    const url = SourcegraphURL.from({
        pathname: `${toRepoURL(target)}/-/blob/${encodeURIPathComponent(target.filePath)}`,
    })
        .setLineRange(
            target.range
                ? {
                      line: target.range.start.line,
                      character: target.range.start.character,
                      endLine: target.range.end.line,
                      endCharacter: target.range.end.character,
                  }
                : target.position
                ? { line: target.position.line, character: target.position.character }
                : null
        )
        .setViewState(target.viewState)

    if (target.renderMode === 'code') {
        url.setSearchParameter('view', 'code')
    }
    return url.toString()
}

/**
 * Returns an absolute URL to the blob (file) on the Sourcegraph instance.
 */
export function toAbsoluteBlobURL(
    sourcegraphURL: string,
    context: RepoSpec &
        RevisionSpec &
        FileSpec &
        Partial<UIPositionSpec> &
        Partial<ViewStateSpec> &
        Partial<UIRangeSpec>
): string {
    // toPrettyBlobURL() always returns an URL starting with a forward slash,
    // no need to add one here
    return `${sourcegraphURL.replace(/\/$/, '')}${toPrettyBlobURL(context)}`
}

/**
 * Returns the URL path for the given repository name and revision.
 */
export function toRepoURL(target: RepoSpec & Partial<RevisionSpec>): string {
    return '/' + encodeRepoRevision(target)
}

const positionString = (position: Position): string =>
    position.line.toString() + (position.character ? `,${position.character}` : '')

/**
 * Translate a URI to use the input revision (e.g., branch names) instead of the Git commit SHA if the URI is
 * inside of a workspace root. This helper is used to translate URLs (from actions such as go-to-definition) to
 * avoid navigating the user from (e.g.) a URL with a nice Git branch name to a URL with a full Git commit SHA.
 *
 * For example, suppose there is a workspace root `git://r?a9cb9d` with input revision `mybranch`. If {@link uri}
 * is `git://r?a9cb9d#f`, it would be translated to `git://r?mybranch#f`.
 */
export function withWorkspaceRootInputRevision(
    workspaceRoots: readonly WorkspaceRootWithMetadata[],
    uri: ParsedRepoURI
): ParsedRepoURI {
    const inWorkspaceRoot = workspaceRoots.find(root => {
        const rootURI = parseRepoGitURI(root.uri)
        return rootURI.repoName === uri.repoName && rootURI.revision === uri.revision
    })
    if (inWorkspaceRoot?.inputRevision !== undefined) {
        return { ...uri, commitID: undefined, revision: inWorkspaceRoot.inputRevision }
    }
    return uri // unchanged
}

/**
 * Builds a URL query for the given query (without leading `?`).
 * @param query the search query
 * @param patternType the pattern type this query should be interpreted in.
 * Having a `patternType:` filter in the query overrides this argument.
 */
export function buildSearchURLQuery(
    query: string,
    patternType: SearchPatternType,
    caseSensitive: boolean,

    searchContextSpec?: string,
    searchMode?: SearchMode
): string {
    const searchParameters = new URLSearchParams()
    let queryParameter = query
    let patternTypeParameter: string = patternType
    let caseParameter: string = caseSensitive ? 'yes' : 'no'

    const globalPatternType = findFilter(queryParameter, 'patterntype', FilterKind.Global)
    if (globalPatternType?.value) {
        patternTypeParameter = globalPatternType.value.value
        queryParameter = omitFilter(queryParameter, globalPatternType)
    }

    const globalCase = findFilter(queryParameter, 'case', FilterKind.Global)
    if (globalCase?.value) {
        // When case:value is explicit in the query, override any previous value of caseParameter.
        const globalCaseParameterValue = globalCase.value.value
        caseParameter = discreteValueAliases.yes.includes(globalCaseParameterValue) ? 'yes' : 'no'
        queryParameter = omitFilter(queryParameter, globalCase)
    }

    if (searchContextSpec) {
        queryParameter = appendContextFilter(queryParameter, searchContextSpec)
    }

    searchParameters.set('q', queryParameter)
    searchParameters.set('patternType', patternTypeParameter)

    if (caseParameter === 'yes') {
        searchParameters.set('case', caseParameter)
    }

    searchParameters.set('sm', (searchMode || SearchMode.Precise).toString())

    return searchParameters.toString().replaceAll('%2F', '/').replaceAll('%3A', ':')
}

/** The results of parsing a repo-revision string like "my/repo@my/revision". */
export interface ParsedRepoRevision {
    repoName: string

    /** The URI-decoded revision (e.g., "my#branch" in "my/repo@my%23branch"). */
    revision?: string

    /** The raw revision (e.g., "my%23branch" in "my/repo@my%23branch"). */
    rawRevision?: string
}

/**
 * Parses a repo-revision string like "my/repo@my/revision" to the repo and revision components.
 */
export function parseRepoRevision(repoRevision: string): ParsedRepoRevision {
    const firstAtSign = repoRevision.indexOf('@')
    if (firstAtSign === -1) {
        return { repoName: decodeURIComponent(repoRevision) }
    }

    const repository = repoRevision.slice(0, firstAtSign)
    const revision = repoRevision.slice(firstAtSign + 1)
    return {
        repoName: decodeURIComponent(repository),
        revision: revision && decodeURIComponent(revision),
        rawRevision: revision,
    }
}
