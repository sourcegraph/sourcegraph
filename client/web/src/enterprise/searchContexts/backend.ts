import { lastValueFrom } from 'rxjs'

import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'

import { requestGraphQL } from '../../backend/graphql'
import type { InputMaybe, RepositoriesByNamesResult, RepositoriesByNamesVariables } from '../../graphql-operations'

const query = gql`
    query RepositoriesByNames($names: [String!]!, $first: Int!, $after: String) {
        repositories(names: $names, first: $first, after: $after) {
            nodes {
                id
                name
            }
            pageInfo {
                endCursor
                hasNextPage
            }
        }
    }
`

export async function fetchRepositoriesByNames(
    names: string[]
): Promise<RepositoriesByNamesResult['repositories']['nodes']> {
    let repos: RepositoriesByNamesResult['repositories']['nodes'] = []
    const first = names.length
    let after: InputMaybe<string> = null

    while (true) {
        const result = await lastValueFrom(
            requestGraphQL<RepositoriesByNamesResult, RepositoriesByNamesVariables>(query, {
                names,
                first,
                after,
            })
        )

        const data: RepositoriesByNamesResult = dataOrThrowErrors(result)

        repos = repos.concat(data.repositories.nodes)
        if (!data.repositories.pageInfo.hasNextPage) {
            break
        }
        after = data.repositories.pageInfo.endCursor
    }
    return repos
}
