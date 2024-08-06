import { parseRepoRevision } from '@sourcegraph/shared/src/util/url'

import { IncrementalRestoreStrategy, getGraphQLClient, infinityQuery } from '$lib/graphql'
import { resolveRevision } from '$lib/repo/utils'

import type { PageLoad } from './$types'
import { ChangelistsPage_ChangelistsQuery } from './page.gql'

const PAGE_SIZE = 20

export const load: PageLoad = ({ parent, params }) => {
    const client = getGraphQLClient()
    const { repoName, revision = '' } = parseRepoRevision(params.repo)
    const path = params.path ? decodeURIComponent(params.path) : ''
    const resolvedRevision = resolveRevision(parent, revision)

    const changelistsQuery = infinityQuery({
        client,
        query: ChangelistsPage_ChangelistsQuery,
        variables: resolvedRevision.then(revision => ({
            depotName: repoName,
            revision,
            first: PAGE_SIZE,
            path,
            afterCursor: null as string | null,
        })),
        map: result => {
            const ancestors = result.data?.repository?.commit?.ancestors
            return {
                nextVariables:
                    ancestors?.pageInfo?.endCursor && ancestors?.pageInfo?.hasNextPage
                        ? { afterCursor: ancestors.pageInfo.endCursor }
                        : undefined,
                data: ancestors?.nodes,
                error: result.error,
            }
        },
        merge: (previous, next) => (previous ?? []).concat(next ?? []),
        createRestoreStrategy: api =>
            new IncrementalRestoreStrategy(
                api,
                n => n.length,
                n => ({ first: n.length })
            ),
    })

    return {
        changelistsQuery,
        path,
    }
}
