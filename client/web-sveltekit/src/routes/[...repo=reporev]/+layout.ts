import { error, redirect } from '@sveltejs/kit'

import { loadMarkdownSyntaxHighlighting } from '$lib/common'
import { getGraphQLClient, mapOrThrow, type GraphQLClient } from '$lib/graphql'
import { GitRefType } from '$lib/graphql-types'
import { CloneInProgressError, RepoNotFoundError, displayRepoName, parseRepoRevision } from '$lib/shared'

import type { LayoutLoad } from './$types'
import {
    DepotChangelists,
    RepositoryGitCommits,
    RepositoryGitRefs,
    ResolveRepoRevision,
    ResolvedRepository,
    type ResolveRepoRevisionResult,
} from './layout.gql'

export const load: LayoutLoad = async ({ params, url, depends }) => {
    const client = getGraphQLClient()

    // This allows other places to reload all repo related data by calling
    // invalidate('repo:root')
    depends('repo:root')

    // Repo pages render markdown, so we ensure that syntax highlighting for code blocks
    // inside markdown is loaded.
    loadMarkdownSyntaxHighlighting()

    // An empty revision means we are at the default branch
    const { repoName, revision = '' } = parseRepoRevision(params.repo)
    const resolvedRepository = await resolveRepoRevision({
        client,
        repoName,
        revspec: revision,
        url,
    })

    return {
        repoURL: '/' + params.repo,
        repoURLWithoutRevision: '/' + repoName,
        repoName,
        displayRepoName: displayRepoName(repoName),
        /**
         * Revision from URL
         */
        revision,
        /**
         * A friendly display form of the revision. This can be:
         * - an empty string which signifies the default branch
         * - an abbreviated commit SHA
         * - a symbolic revision (e.g. a branch or tag name)
         */
        displayRevision: displayRevision(revision, resolvedRepository),
        defaultBranch: resolvedRepository.defaultBranch?.abbrevName || 'HEAD',
        resolvedRepository: resolvedRepository,
        isPerforceDepot: resolvedRepository.externalRepository.serviceType === 'perforce',

        // Repository pickers queries (branch, tags and commits)
        getRepoBranches: (searchTerm: string) =>
            client
                .query(RepositoryGitRefs, {
                    repoName,
                    query: searchTerm,
                    type: GitRefType.GIT_BRANCH,
                })
                .then(
                    mapOrThrow(({ data, error }) => {
                        if (!data?.repository?.gitRefs) {
                            throw new Error(error?.message)
                        }

                        return data.repository.gitRefs
                    })
                ),
        getRepoTags: (searchTerm: string) =>
            client
                .query(RepositoryGitRefs, {
                    repoName,
                    query: searchTerm,
                    type: GitRefType.GIT_TAG,
                })
                .then(
                    mapOrThrow(({ data, error }) => {
                        if (!data?.repository?.gitRefs) {
                            throw new Error(error?.message)
                        }

                        return data.repository.gitRefs
                    })
                ),
        getRepoCommits: (searchTerm: string) =>
            client
                .query(RepositoryGitCommits, {
                    repoName,
                    query: searchTerm,
                    revision: resolvedRepository.commit?.oid || '',
                })
                .then(
                    mapOrThrow(({ data }) => {
                        let nodes = data?.repository?.ancestorCommits?.ancestors.nodes ?? []

                        // If we got a match for the OID, add it to the list if it doesn't already exist.
                        // We double check that the OID contains the search term because we cannot search
                        // specifically by OID, and an empty string resolves to HEAD.
                        const commitByHash = data?.repository?.commitByHash
                        if (
                            commitByHash &&
                            commitByHash.oid.includes(searchTerm) &&
                            !nodes.some(node => node.oid === commitByHash.oid)
                        ) {
                            nodes = [commitByHash, ...nodes]
                        }
                        return { nodes }
                    })
                ),

        // Depot pickers queries (changelists, @TODO: labels)
        getDepotChangelists: (searchTerm: string) =>
            client
                .query(DepotChangelists, {
                    depotName: repoName,
                    query: searchTerm,
                    revision: resolvedRepository.commit?.oid || ''
                })
                .then(
                    mapOrThrow(({ data, error }) => {
                        if (error) {
                            throw new Error('Could not load depot changelists:', error)
                        }
                        return data?.repository?.ancestorChangelists
                    })
                )
    }
}

/**
 * Returns a friendly display form of the revision. If the resolved revision's commit ID starts with the revision,
 * the first 7 characters of the commit ID are returned. Otherwise, the revision is returned as is.
 *
 * @param revision The revision from the URL
 * @param resolvedRevision The resolved revision
 * @returns A human readable revision string
 */
function displayRevision(revision: string, resolvedRevision: ResolvedRepository | undefined): string {
    if (!resolvedRevision) {
        return revision
    }

    if (revision && resolvedRevision.commit?.oid.startsWith(revision)) {
        return resolvedRevision.commit.oid?.slice(0, 7)
    }

    return revision
}

// This is a cache for resolved repository information to help in the following case:
// - The user navigates to a repository page with a symbolic revspec (e.g. a branch or tag name)
// - The user navigates to a permalink (i.e. URL with commit ID) for that very same revision
//
// Without additional steps this would result in a second query to resolve the repository information,
// because we now have a different revspec in the URL (the commit ID instead of the symbolic rev).
// But that request would return exactly the same data as the first request. To avoid this, we cache
// the resolved repository information here and reuse if we make a request for a commit ID that we
// have previously seen in a response.
const resolvedRepoRevision = new Map<string, ResolveRepoRevisionResult>()

/**
 * This function takes the repository name and revision from the URL and fetches the corresponding
 * repository information.
 * One of three things can happen:
 * - If the repository has a server side redirect configured, the user is redirected to the new URL
 * - If the repository was not found, is currently being cloned or is scheduled for cloning, an error is thrown
 * - Otherwise the resolved repository information is returned
 *
 * Note that it's possible that the provided revision does not exist in the repository. In that case
 * the repository information is still returned, but the commit information will be missing.
 */
async function resolveRepoRevision({
    client,
    repoName,
    revspec = '',
    url,
}: {
    client: GraphQLClient
    repoName: string
    revspec?: string
    url: URL
}): Promise<ResolvedRepository> {
    const cacheKey = `${repoName}@${revspec}`

    let data: ResolveRepoRevisionResult | undefined

    if (resolvedRepoRevision.has(cacheKey)) {
        // See if we have resolved this revision with another revspec before. This can happen
        // when the user navigates from a symbolic revspec to its respective permalink (commit ID).
        data = resolvedRepoRevision.get(cacheKey)
    } else {
        // See if we have a cached response for the same revision
        data = client.readQuery(ResolveRepoRevision, { repoName, revision: revspec })?.data

        if (shouldResolveRepositoryInformation(data)) {
            data = (
                await client.query(
                    ResolveRepoRevision,
                    { repoName, revision: revspec },
                    { requestPolicy: 'network-only' }
                )
            ).data
        }
    }

    if (!data?.repositoryRedirect) {
        error(404, new RepoNotFoundError(repoName))
    }

    if (data.repositoryRedirect.__typename === 'Redirect') {
        const redirectURL = new URL(url)
        const externalURL = new URL(data.repositoryRedirect.url)
        // Preserve the path of the current URL and redirect to the repo on the external host.
        redirectURL.host = externalURL.host
        redirectURL.protocol = externalURL.protocol
        redirect(303, redirectURL)
    }
    if (data.repositoryRedirect.mirrorInfo.cloneInProgress) {
        error(503, new CloneInProgressError(repoName, data.repositoryRedirect.mirrorInfo.cloneProgress || undefined))
    }
    if (!data.repositoryRedirect.mirrorInfo.cloned) {
        error(503, new CloneInProgressError(repoName, 'queued for cloning'))
    }

    // The "revision" we queried for could be a commit or a changelist.
    const commit = data.repositoryRedirect.commit || data.repositoryRedirect.changelist?.commit

    // Cache the resolved repository information
    if (commit) {
        resolvedRepoRevision.set(`${repoName}@${commit.oid}`, data)
    }

    return data.repositoryRedirect
}

/**
 * We want to resolve the repository and revision information in two cases:
 * - The data is not available yet
 * - The repository is being cloned or the clone is in progress
 *
 * In all other cases, we can use the cached data. That means if the URL specifies a
 * "symbolic" revspec (e.g. a branch or tag name), we will resolve that revspec to the
 * corresponding commit ID only once.
 * This ensures consistentcy as the user navigates to and away from the repository page.
 */
function shouldResolveRepositoryInformation(data: ResolveRepoRevisionResult | undefined): boolean {
    if (!data) {
        return true
    }
    if (data.repositoryRedirect?.__typename === 'Repository') {
        return data.repositoryRedirect.mirrorInfo.cloneInProgress || !data.repositoryRedirect.mirrorInfo.cloned
    }
    return false
}
