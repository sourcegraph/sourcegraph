import { BehaviorSubject, concatMap, from, map } from 'rxjs'

import {
    Occurrence,
    Range,
    Position,
    SymbolRole,
    nonOverlappingOccurrences,
} from '@sourcegraph/shared/src/codeintel/scip'
import { type BlameHunkData, fetchBlameHunksMemoized } from '@sourcegraph/web/src/repo/blame/shared'
import type { CodeGraphData } from '@sourcegraph/web/src/repo/blob/codemirror/codeintel/occurrences'

import { getGraphQLClient, mapOrThrow, type GraphQLClient } from '$lib/graphql'
import { SymbolRole as GraphQLSymbolRole } from '$lib/graphql-types'
import { resolveRevision } from '$lib/repo/utils'
import { parseRepoRevision } from '$lib/shared'
import { assertNonNullable } from '$lib/utils'

import type { PageLoad, PageLoadEvent } from './$types'
import type { FileViewCodeGraphData } from './FileView.gql'
import {
    BlobDiffViewCommitQuery,
    BlobFileViewBlobQuery,
    BlobFileViewCodeGraphDataQuery,
    BlobFileViewCommitQuery_revisionOverride,
    BlobFileViewHighlightedFileQuery,
    BlobViewCodeGraphDataNextPage,
} from './page.gql'
import { redirect } from '@sveltejs/kit'
import { DepotChangelist } from '../../../../../layout.gql'

async function loadDiffView({ params, url }: PageLoadEvent) {
    const client = getGraphQLClient()
    const revisionOverride = url.searchParams.get('rev')
    const { repoName } = parseRepoRevision(params.repo)
    const filePath = decodeURIComponent(params.path)

    assertNonNullable(revisionOverride, 'revisionOverride is set')

    return {
        type: 'DiffView' as const,
        enableInlineDiff: true,
        enableViewAtCommit: true,
        filePath,
        commit: client
            .query(BlobDiffViewCommitQuery, {
                repoName,
                revspec: revisionOverride,
                path: filePath,
            })
            .then(mapOrThrow(result => result.data?.repository?.commit ?? null)),
    }
}

async function fetchCodeGraphData(
    client: GraphQLClient,
    repoName: string,
    resolvedRevision: string,
    path: string
): Promise<CodeGraphData[]> {
    async function fetchAllOccurrences(codeGraphDatum: FileViewCodeGraphData): Promise<FileViewCodeGraphData> {
        while (codeGraphDatum.occurrences?.pageInfo?.hasNextPage) {
            const response = await client.query(BlobViewCodeGraphDataNextPage, {
                codeGraphDataID: codeGraphDatum.id,
                after: codeGraphDatum.occurrences?.pageInfo?.endCursor ?? '',
            })
            if (response.error) {
                throw new Error('failed to hydrate paginated occurrences', { cause: response.error })
            }
            if (response.data?.node?.__typename !== 'CodeGraphData') {
                throw new Error('unexpected node')
            }
            codeGraphDatum.occurrences = {
                nodes: [...codeGraphDatum.occurrences.nodes, ...(response.data.node.occurrences?.nodes ?? [])],
                pageInfo: response.data.node.occurrences?.pageInfo ?? {
                    __typename: 'PageInfo',
                    hasNextPage: false,
                    endCursor: null,
                },
            }
        }
        return codeGraphDatum
    }

    function translateRole(graphQLRole: GraphQLSymbolRole): SymbolRole {
        switch (graphQLRole) {
            case GraphQLSymbolRole.DEFINITION:
                return SymbolRole.Definition
            case GraphQLSymbolRole.REFERENCE:
                // The REFERENCE role from the API is just the negation of the
                // DEFINITION role, so simply do not set the definition bit.
                return SymbolRole.Unspecified
            case GraphQLSymbolRole.FORWARD_DEFINITION:
                return SymbolRole.ForwardDefinition
            default:
                return SymbolRole.Unspecified
        }
    }

    const response = await client.query(BlobFileViewCodeGraphDataQuery, {
        repoName,
        revspec: resolvedRevision,
        path,
    })
    if (response.error) {
        throw new Error('failed fetching code graph data', { cause: response.error })
    }

    const rawCodeGraphData = response.data?.repository?.commit?.blob?.codeGraphData
    if (!rawCodeGraphData) {
        return []
    }
    // Fetch any additional pages of occurrences
    const hydratedCodeGraphData = await Promise.all([...rawCodeGraphData.map(fetchAllOccurrences)])

    return hydratedCodeGraphData.map(({ provenance, toolInfo, commit, occurrences }) => {
        const overlapping =
            occurrences?.nodes?.map(
                occ =>
                    new Occurrence(
                        new Range(
                            new Position(occ.range.start.line, occ.range.start.character),
                            new Position(occ.range.end.line, occ.range.end.character)
                        ),
                        undefined,
                        occ.symbol ?? undefined,
                        occ.roles?.map(translateRole).reduce((acc, role) => acc | role, 0),
                        provenance
                    )
            ) ?? []
        const nonOverlapping = nonOverlappingOccurrences([...overlapping])
        return {
            provenance,
            toolInfo,
            commit,
            occurrences: overlapping,
            nonOverlappingOccurrences: nonOverlapping,
        }
    })
}

async function loadFileView({ parent, params, url }: PageLoadEvent) {
    const client = getGraphQLClient()
    const revisionOverride = url.searchParams.get('rev')
    const isBlame = url.searchParams.get('view') === 'blame'
    const { repoName, revision = '' } = parseRepoRevision(params.repo)
    const resolvedRevision = revisionOverride ? Promise.resolve(revisionOverride) : resolveRevision(parent, revision)
    const filePath = decodeURIComponent(params.path)
    const isPerforceDepot = await parent().then(parent => parent.isPerforceDepot)

    if (isPerforceDepot) {
        const changelistInfo = await client
            .query(DepotChangelist, {
                depotName: repoName,
                revision: revision
            })
            .then(result => result.data?.repository?.commit)

        const redirectURL = new URL(url)
        redirectURL.pathname = `${repoName}@changelist/${changelistInfo?.perforceChangelist?.cid}`
        redirect(301, redirectURL)
    }

    // Create a BehaviorSubject so preloading does not create a subscriberless observable
    const blameData = new BehaviorSubject<BlameHunkData>({ current: undefined, externalURLs: undefined })
    if (isBlame) {
        const blameHunks = from(resolvedRevision).pipe(
            concatMap(resolvedRevision => fetchBlameHunksMemoized({ repoName, revision: resolvedRevision, filePath }))
        )

        from(parent())
            .pipe(
                concatMap(({ resolvedRevision }) =>
                    blameHunks.pipe(
                        map(blameHunks => ({
                            externalURLs: resolvedRevision.repo.externalURLs,
                            current: blameHunks,
                        }))
                    )
                )
            )
            .subscribe(v => blameData.next(v))
    }

    return {
        type: 'FileView' as const,
        enableInlineDiff: true,
        enableViewAtCommit: true,
        graphQLClient: client,
        filePath,
        blob: resolvedRevision
            .then(resolvedRevision =>
                client.query(BlobFileViewBlobQuery, {
                    repoName,
                    revspec: resolvedRevision,
                    path: filePath,
                })
            )
            .then(mapOrThrow(result => result.data?.repository?.commit?.blob ?? null)),
        highlights: resolvedRevision
            .then(resolvedRevision =>
                client.query(BlobFileViewHighlightedFileQuery, {
                    repoName,
                    revspec: resolvedRevision,
                    path: filePath,
                    disableTimeout: false,
                })
            )
            .then(mapOrThrow(result => result.data?.repository?.commit?.blob?.highlight ?? null)),
        codeGraphData: resolvedRevision.then(async resolvedRevision => {
            return fetchCodeGraphData(client, repoName, resolvedRevision, filePath)
        }),
        // We can ignore the error because if the revision doesn't exist, other queries will fail as well
        revisionOverride: revisionOverride
            ? await client
                .query(BlobFileViewCommitQuery_revisionOverride, {
                    repoName,
                    revspec: revisionOverride,
                })
                .then(result => result.data?.repository?.commit)
            : null,
        externalServiceType: parent()
            .then(({ resolvedRevision }) => resolvedRevision.repo?.externalRepository?.serviceType)
            .catch(error => {
                console.error('Failed to fetch repository data:', error)
                return null
            }),
        blameData,
    }
}

export const load: PageLoad = (event) => {
    const showDiff = event.url.searchParams.has('diff')
    const revisionOverride = event.url.searchParams.get('rev')

    if (showDiff && revisionOverride) {
        return loadDiffView(event)
    }

    return loadFileView(event)
}
