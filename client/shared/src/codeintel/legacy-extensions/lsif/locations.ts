import * as sourcegraph from '../api'
import { queryGraphQL as sgQueryGraphQL, QueryGraphQLFn } from '../util/graphql'
import { parseGitURI } from '../util/uri'

import { GenericLSIFResponse } from './api'

export interface LocationConnectionNode {
    resource: {
        path: string
        repository?: { name: string }
        commit?: { oid: string }
    }
    range: sourcegraph.Range
}

/**
 * Convert a GraphQL location connection node into a Sourcegraph location.
 *
 * @param textDocument The current document.
 * @param node A location connection node.
 */
export function nodeToLocation(
    textDocument: sourcegraph.TextDocument,
    { resource: { repository, commit, path }, range }: LocationConnectionNode
): sourcegraph.Location {
    const { repo: currentRepo, commit: currentCommit } = parseGitURI(new URL(textDocument.uri))

    return {
        uri: `git://${repository?.name || currentRepo}?${commit?.oid || currentCommit}#${path}`,
        range,
    }
}

export interface LocationCursor {
    locations: sourcegraph.Location[] | null
    endCursor?: string
}

export type getCursorLocation<T> = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position,
    after: string | undefined,
    queryGraphQL: QueryGraphQLFn<GenericLSIFResponse<T | null>>
) => Promise<LocationCursor>

type QueryPageFn = (
    requestsRemaining: number,
    after?: string | undefined
) => AsyncGenerator<sourcegraph.Location[] | null, void, undefined>

export function getQueryPage<T>(
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position,
    queryGraphQL: QueryGraphQLFn<GenericLSIFResponse<T | null>> = sgQueryGraphQL,
    get: getCursorLocation<T>
): QueryPageFn {
    const queryPage: QueryPageFn = async function* (requestsRemaining, after) {
        if (requestsRemaining === 0) {
            return
        }

        // Make the request for the page starting at the after cursor
        const { locations, endCursor } = await get(textDocument, position, after, queryGraphQL)

        // Yield this page's set of results
        yield locations

        if (endCursor) {
            // Recursively yield the remaining pages
            yield* queryPage(requestsRemaining - 1, endCursor)
        }
    }

    return queryPage
}
