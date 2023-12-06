import { isInlineFragment } from '@apollo/client/utilities'
import type { IMockStore } from '@graphql-tools/mock'
import type { GraphQLResolveInfo } from 'graphql'
import type { GraphQLVariables } from 'msw'

interface GenericGraphQLContext {
    nodeTypename?: string
    operationName?: string
}

/**
 * Default resolvers for GraphQL mocking.
 */
export function getDefaultResolvers(store: IMockStore): Record<string, any> {
    return {
        Query: {
            node: (
                _value: any,
                { id }: GraphQLVariables,
                context: GenericGraphQLContext | undefined,
                info: GraphQLResolveInfo
            ) => {
                // Node is an interface. We need to resolve it to requested type otherwise @graphql-tools/mock will
                // return a random type.
                let typename = context?.nodeTypename
                if (!typename && (!context?.operationName || info.operation.name?.value === context.operationName)) {
                    // Try to determine type name from inline fragment
                    typename =
                        info.fieldNodes[0].selectionSet?.selections.find(isInlineFragment)?.typeCondition?.name.value
                }

                if (typename) {
                    return store.get(typename, id)
                }
                throw new Error(
                    'Mock error: Unable to determine typename for node query. Please use an inline fragment in the query or explicitly specify the type to use.'
                )
            },
        },
    }
}
