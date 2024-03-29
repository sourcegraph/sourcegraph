
import { resolveRoute } from '$app/paths'

import type { ResolvedRevision } from '../../routes/[...repo=reporev]/+layout'

const TREE_ROUTE_ID = '/[...repo=reporev]/(validrev)/(code)/-/tree/[...path]'

/**
 * Returns a [segment, url] mapping for every segement in `path`.
 * The URL for the last segment is empty.
 *
 * Example:
 *   'foo/bar/baz' converts to
 *   [
 *     ['foo', '/<repo>/-/tree/foo'],
 *     ['bar', '/<repo>/-/tree/foo/bar'],
 *     ['baz', '/<repo>/-/tree/foo/bar/baz'],
 *   ]
 *
 */
export function navFromPath(path: string, repo: string): [string, string][] {
    const parts = path.split('/')
    return parts
        .slice(0, -1)
        .map((part, index, all): [string, string] => [
            part,
            resolveRoute(TREE_ROUTE_ID, { repo, path: all.slice(0, index + 1).join('/') }),
        ])
        .concat([[parts.at(-1) ?? '', '']])
}

export function getRevisionLabel(
    urlRevision: string | undefined,
    resolvedRevision: ResolvedRevision | null
): string | undefined {
    return (
        (urlRevision && urlRevision === resolvedRevision?.commitID
            ? resolvedRevision?.commitID.slice(0, 7)
            : urlRevision?.slice(0, 7)) || resolvedRevision?.defaultBranch
    )
}

export function getFileURL(repoURL: string, file: { canonicalURL: string }): string {
    // TODO: Find out whether there is a safer way to do this
    return repoURL + file.canonicalURL.slice(file.canonicalURL.indexOf('/-/'))
}

/**
 * This function is supposed to be used in repository data loaders.
 *
 * In order to ensure data consistency when navigating between repository pages, we have
 * to ensure that the pages fetch data for the same revision. If a revision specifier is
 * present in the URL and is a commit ID, we can use it directly. If it's a branch name,
 * tag name or is missing, we have to wait for the parent loader to resolve the revision
 * to a commit ID.
 */
export async function resolveRevision(
    parent: () => Promise<{ resolvedRevision: ResolvedRevision }>,
    revisionFromURL: string | undefined
): Promise<string> {
    // There is a chance that a commit ID is used as a branch or tag name,
    // but it's unlikely. Avoiding waterfall requests is worth the risk.
    if (revisionFromURL && /[0-9a-f]{40}/.test(revisionFromURL)) {
        return revisionFromURL
    }
    return (await parent()).resolvedRevision.commitID
}
