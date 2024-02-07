import { getGraphQLClient } from '$lib/graphql'
import { parseRepoRevision } from '$lib/shared'

import type { PageLoad } from './$types'
import { BlobDiffQuery, BlobPageQuery, BlobSyntaxHighlightQuery } from './page.gql'

export const load: PageLoad = async ({ params, url }) => {
    const revisionToCompare = url.searchParams.get('rev')
    const graphqlClient = await getGraphQLClient()
    const { repoName, revision = '' } = parseRepoRevision(params.repo)

    return {
        filePath: params.path,
        blob: graphqlClient
            .query({
                query: BlobPageQuery,
                variables: {
                    repoName,
                    revspec: revision,
                    path: params.path,
                },
            })
            .then(result => {
                if (!result.data.repository?.commit) {
                    throw new Error('Repository not found')
                }
                return result.data.repository.commit.blob
            }),
        highlights: graphqlClient
            .query({
                query: BlobSyntaxHighlightQuery,
                variables: {
                    repoName,
                    revspec: revision,
                    path: params.path,
                    disableTimeout: false,
                },
            })
            .then(result => {
                return result.data.repository?.commit?.blob?.highlight.lsif
            }),
        compare: revisionToCompare
            ? {
                  revisionToCompare,
                  diff: graphqlClient
                      .query({
                          query: BlobDiffQuery,
                          variables: {
                              repoName,
                              revspec: revisionToCompare,
                              paths: [params.path],
                          },
                      })
                      .then(result => {
                          return result.data.repository?.commit?.diff.fileDiffs.nodes[0]
                      }),
              }
            : null,
    }
}
