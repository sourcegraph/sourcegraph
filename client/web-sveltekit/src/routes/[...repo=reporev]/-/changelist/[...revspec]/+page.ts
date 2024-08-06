import { error } from '@sveltejs/kit'

import { parseRepoRevision } from '@sourcegraph/shared/src/util/url'

import { IncrementalRestoreStrategy, getGraphQLClient, infinityQuery } from '$lib/graphql'

import type { PageLoad } from './$types'
import { ChangelistPage_ChangelistQuery, ChangelistPage_DiffQuery } from './page.gql'

const PAGE_SIZE = 20

export const load: PageLoad = async ({ params }) => {
    const client = getGraphQLClient()
    const { repoName, revision } = parseRepoRevision(params.repo + '@' + params.revspec)

    // @PROBLEM: We use the url to generate variables to run this query.
    // In this case, the URL rightly includes the Changelist ID, not the commit hash.
    // However, the GraphQL API expects a commit hash, a changelist ID will break the page
    // because it won't return any data.
    //
    // @SOLUTION: Figure out how to get the commit hash into this loader so we can use it
    // to fetch the data we need.
    //
    // IDEA: is there a way to include the commit hash in the URL as well, then redirect
    // to the correct URL once we've got the data?
    //
    // Redirect Docs: https://kit.svelte.dev/docs/load#redirects
    const result = await client.query(ChangelistPage_ChangelistQuery, { repoName: params.repo, cid: revision ?? '' })

    if (result.error) {
        error(500, `Unable to load commit data: ${result.error}`)
    }

    const changelist = result.data?.repository?.changelist

    if (!changelist) {
        error(404, 'Changelist not found')
    }

    // parents is an empty array for the initial commit
    // We currently don't support diffs for the initial commit on the backend

    const diff =
        changelist.cid && changelist?.commit.parents[0]?.parent?.cid
            ? infinityQuery({
                  client,
                  query: ChangelistPage_DiffQuery,
                  variables: {
                      repoName,
                      base: changelist.commit.parents[0].oid,
                      head: changelist.commit.oid,
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
        changelist,
        diff,
    }
}
