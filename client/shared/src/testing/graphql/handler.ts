import { fakerEN as faker } from '@faker-js/faker'
import { addMocksToSchema, createMockStore } from '@graphql-tools/mock'
import { type DocumentNode, type GraphQLError, buildSchema, graphqlSync, isObjectType } from 'graphql'
import { merge } from 'lodash'
import { graphql as mswgraphql, HttpResponse, type RequestHandler } from 'msw'

import { getDocumentNode } from '@sourcegraph/http-client'

import { getDefaultResolvers } from './resolvers'

type Mocks = Record<string, () => unknown>

export interface GraphQLMockOptions<T extends Mocks> {
    /**
     * The graphql query to mock. Altnernatively, you can specify the query name.
     * If neither `query` nor `name` is specified, the handler will use this
     * configuration for all queries.
     */
    query?: DocumentNode | string

    /**
     * The name of the graphql operation to mock. Alternatively, you can specify the query.
     * If neither `name` nor `query` is specified, the handler will use this
     * configuration for all queries.
     */
    name?: string

    /**
     * When doing a `node` query the handler tries to determine the typename of
     * the node to mock from the query. This only works if the node query contains an inline fragment.
     * If it doesn't you can specify the typename here.
     */
    nodeTypename?: string

    /**
     * Additional mock generators to use.
     */
    mocks?: T

    /**
     * When set to true, the mock result will be logged to the console.
     */
    inspect?: boolean

    /**
     * Seed to use to initialze the random data generator. The default seed is
     * 1. If you want to generate different data for each test run, you can
     * set this to a random number or to undefined.
     */
    seed?: number
}

export interface MockSetupOptions<T extends Mocks> {
    /**
     * The GraphQL schema to mock.
     */
    typeDefs: string

    /**
     * Default mocks to use for all queries.
     */
    defaultMocks?: T
}

interface RequestMock<T extends Mocks> {
    query: string
    variables: Record<string, any>
    operationName: string
    options: GraphQLMockOptions<T>
}

export interface GraphQLMock<T extends Mocks> {
    /**
     * Mock store that can be used to access the mocked data and setup additional mocks.
     */
    store: ReturnType<typeof createMockStore>
    /**
     * Mocks a GraphQL request.
     */
    mockRequest(request: RequestMock<T>): { data: unknown; errors: readonly GraphQLError[] | undefined }
}

/**
 * createGraphQLMock returns an object that can be used to mock GraphQL requests.
 */
export function createGraphQLMock<T extends Mocks>(options: MockSetupOptions<T>): GraphQLMock<T> {
    const schema = buildSchema(options.typeDefs)
    // Extract GraphQL type names from the schema.
    const typeNames = Object.values(schema.getTypeMap())
        .filter(type => !type.name.startsWith('__') && isObjectType(type))
        .map(type => type.name)
    const defaultMocks = options.defaultMocks
    let requestMocks: T | undefined
    const mocks: T = { ...defaultMocks } as T
    for (const typeName of typeNames) {
        ;(mocks as Mocks)[typeName] = () => {
            if (requestMocks?.[typeName]) {
                return requestMocks[typeName]()
            }
            if (defaultMocks?.[typeName]) {
                return defaultMocks[typeName]()
            }
            return {}
        }
    }
    // NOTE: We are using a patched version of @graphql-tools/mock that allows us to
    // pass a random number generator to the mock store. This allows us to generate
    // deterministic random data for booleans and enums without having to manually
    // mock them.
    const store = createMockStore({
        schema,
        mocks,
        random: () => faker.number.float(),
    })

    const mockedSchema = addMocksToSchema({
        schema,
        store,
        resolvers: store => merge(getDefaultResolvers(store)),
    })

    return {
        store,
        mockRequest(request) {
            const { query, variables, operationName, options } = request

            let data: unknown
            let errors: readonly GraphQLError[] | undefined

            const context = {
                nodeTypename: options.nodeTypename,
                operationName,
            }

            try {
                requestMocks = options.mocks
                // Only use the seed if it is explicitly set. Otherwise, we want to use the default seed.
                faker.seed('seed' in options ? options.seed : 1)
                ;({ data, errors } = graphqlSync(mockedSchema, query, undefined, context, variables))
            } catch (error) {
                errors = [error]
            } finally {
                requestMocks = undefined
            }
            if (errors) {
                // eslint-disable-next-line no-console
                console.error(
                    `Operation '${operationName}' with ${JSON.stringify(variables)} errored:\n${errors
                        .map(error => error.message)
                        .join('\n')}`
                )
            }
            if (options.inspect) {
                // eslint-disable-next-line no-console
                console.log(
                    `Mocked operation '${operationName}' with ${JSON.stringify(variables)}: ${JSON.stringify(
                        { data, errors },
                        null,
                        2
                    )}`
                )
            }
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            return { data, errors: (errors as any) ?? undefined }
        },
    }
}

/**
 * Helper function for creating a MSW request handler that mocks a specific operation/query.
 */
export function createGraphQLMockRequestHandler<T extends Mocks>(
    mock: GraphQLMock<T>,
    options: GraphQLMockOptions<T>
): RequestHandler {
    const name: string | undefined = getOperationName(options)
    return mswgraphql.operation(({ query, variables, operationName }) => {
        if (!name || operationName === name) {
            let data: unknown
            let errors: readonly GraphQLError[] | undefined
            try {
                ;({ data, errors } = mock.mockRequest({ query, variables, operationName, options }))
            } catch (error) {
                errors = [error]
            }
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            return HttpResponse.json({ data: (data as any) ?? undefined, errors: (errors as any) ?? undefined })
        }
        return undefined
    })
}

/**
 * Returns the operation name specified in the options or tries to determine it from the query.
 */
export function getOperationName(options: GraphQLMockOptions<Mocks>): string | undefined {
    if (options.name) {
        return options.name
    }
    if (options.query) {
        return getOperationNameFromQuery(options.query)
    }
    return undefined
}

function getOperationNameFromQuery(query: DocumentNode | string): string | undefined {
    const document = getDocumentNode(query)
    for (const definition of document.definitions) {
        if (definition.kind === 'OperationDefinition' && definition.operation === 'query') {
            return definition.name?.value
        }
    }
    return undefined
}
