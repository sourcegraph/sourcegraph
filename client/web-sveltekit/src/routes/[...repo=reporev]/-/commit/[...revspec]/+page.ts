import { error, redirect } from '@sveltejs/kit'

import { IncrementalRestoreStrategy, getGraphQLClient, infinityQuery } from '$lib/graphql'
import { parseRepoRevision } from '$lib/shared'

import type { PageLoad } from './$types'
import { CommitPage_CommitQuery, CommitPage_DiffQuery } from './page.gql'

const PAGE_SIZE = 20

export const load: PageLoad = async ({ url, params }) => {
    const client = getGraphQLClient()
    const { repoName } = parseRepoRevision(params.repo)

    const result = await client.query(CommitPage_CommitQuery, { repoName, revspec: params.revspec })

    if (result.error) {
        error(500, `Unable to load commit data: ${result.error}`)
    }

    const commit = result.data?.repository?.commit
    const isPerforceDepot = commit?.perforceChangelist !== null

    if (!commit) {
        error(404, 'Commit not found')
    }

    if (isPerforceDepot) {
        const redirectURL = new URL(url)
        redirectURL.pathname = `${params.repo}/-/changelist/${commit.perforceChangelist?.cid}`
        redirect(301, redirectURL)
    }

    // parents is an empty array for the initial commit
    // We currently don't support diffs for the initial commit on the backend
    const diff =
        commit?.oid && commit?.parents[0]?.oid
            ? infinityQuery({
                  client,
                  query: CommitPage_DiffQuery,
                  variables: {
                      repoName,
                      base: commit.parents[0].oid,
                      head: commit.oid,
                      first: PAGE_SIZE,
                      after: null as string | null,
                  },
                  map: result => {
                      const diffs = result.data?.repository?.comparison.fileDiffs
                      return {
                          nextVariables: diffs?.pageInfo.hasNextPage ? { after: diffs?.pageInfo.endCursor } : undefined,
                          data: diffs?.nodes,
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
            : null

    return {
        commit,
        diff,
    }
}
