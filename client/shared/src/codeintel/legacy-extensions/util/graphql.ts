import { fromBase64 } from 'js-base64'

import { createAggregateError } from '@sourcegraph/common'

import * as sourcegraph from '../api'

/** The generic type of the queryGraphQL function. */
export type QueryGraphQLFn<T> = (query: string, vars?: { [name: string]: unknown }) => Promise<T>

/**
 * Perform a GraphQL query.
 *
 * @param query The GraphQL query string.
 * @param vars The query variables.
 */
export async function queryGraphQL<T>(query: string, vars: { [name: string]: unknown } = {}): Promise<T> {
    const response = await sourcegraph.requestGraphQL<T>(query, vars)

    if (response.errors !== undefined) {
        throw response.errors.length === 1 ? response.errors[0] : createAggregateError(response.errors)
    }

    return response.data
}

export function graphqlIdToRepoId(id: string): number {
    const decodedId = fromBase64(id)
    return parseInt(decodedId.split(':')[1], 10)
}
