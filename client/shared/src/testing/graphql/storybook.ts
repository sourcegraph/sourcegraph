import type { RequestHandler } from 'msw'

import type { TypeMocks } from '../../graphql-types'

import { defaultMocks } from './defaultMocks'
import {
    type GraphQLMockOptions,
    createGraphQLMockRequestHandler,
    createGraphQLMock as defaultCreateGraphQLMock,
} from './handler'

const typeDefs: string = Object.values(
    import.meta.glob('../../../../../cmd/frontend/graphqlbackend/*.graphql', {
        as: 'raw',
        eager: true,
    })
).join('\n')

/**
 * Sets up GraphQL mocking for Storybook.
 */
export function createGraphQLMock(): { mockGraphQL: (options: GraphQLMockOptions<TypeMocks>) => RequestHandler } {
    const mock = defaultCreateGraphQLMock<TypeMocks>({ typeDefs, defaultMocks })
    return {
        mockGraphQL(options) {
            return createGraphQLMockRequestHandler(mock, options)
        },
    }
}
