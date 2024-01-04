import { fetchBlobPlaintext } from '$lib/repo/api/blob'
import { fetchTreeEntries } from '$lib/repo/api/tree'
import { findReadme } from '$lib/repo/tree'

import type { PageLoad } from './$types'
import { TreeEntriesCommitInfo } from './page.gql'

export const load: PageLoad = async ({ params, parent, url }) => {
    const { resolvedRevision, graphqlClient } = await parent()

    const treeEntries = fetchTreeEntries({
        repoID: resolvedRevision.repo.id,
        commitID: resolvedRevision.commitID,
        filePath: params.path,
        first: null,
    }).then(
        commit => commit.tree,
        () => null
    )

    return {
        filePath: params.path,
        deferred: {
            treeEntries,
            commitInfo: graphqlClient
                .query({
                    query: TreeEntriesCommitInfo,
                    variables: {
                        repoID: resolvedRevision.repo.id,
                        commitID: resolvedRevision.commitID,
                        filePath: params.path,
                        first: null,
                    },
                })
                .then(result => {
                    if (result.data.node?.__typename !== 'Repository') {
                        throw new Error('Unable to load repository')
                    }
                    return result.data.node.commit?.tree ?? null
                }),
            readme: treeEntries.then(result => {
                if (!result) {
                    return null
                }
                const readme = findReadme(result.entries)
                if (!readme) {
                    return null
                }
                return fetchBlobPlaintext({
                    repoID: resolvedRevision.repo.id,
                    commitID: resolvedRevision.commitID,
                    filePath: readme.path,
                }).then(result => ({
                    name: readme.name,
                    ...result,
                }))
            }),
        },
    }
}
