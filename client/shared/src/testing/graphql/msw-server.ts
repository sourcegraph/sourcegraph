import type { IMockStore } from '@graphql-tools/mock'
import type { RequestHandler } from 'msw'
import { setupServer } from 'msw/node'

import { defaultMocks } from './defaultMocks'
import { type GraphQLMockOptions, createGraphQLMock, createGraphQLMockRequestHandler } from './handler'

type Mocks = Record<string, () => unknown>

/**
 * Options for setting up the mock server.
 */
export interface MockServerOptions {
    /**
     * The graphql schema to use for mocking.
     */
    typeDefs: string
    /**
     * If true, log a message when a request is intercepted.
     */
    inspect?: boolean
    /**
     * A function that will be called after each test. The server
     * will be reset all handlers between tests.
     */
    afterEach: (hook: () => void) => void
    /**
     * A function that will be called before all tests. The server
     * will be started before all tests.
     */
    beforeAll: (hook: () => void) => void
    /**
     * A function that will be called after all tests. The server
     * will be closed after all tests.
     */
    afterAll: (hook: () => void) => void
}

export interface MockServer<T extends Mocks> {
    /**
     * The msw server instance.
     */
    server: ReturnType<typeof setupServer>

    /**
     * Convenience method for adding request handlers to the server.
     */
    use(...handlers: RequestHandler[]): void

    /**
     * The mock store used by the graphql handlers to generate mock data.
     * You can use this to add additional mocks to the store.
     */
    store: IMockStore

    /**
     * Helper function for creating a graphql handler that mocks a specific operation/query.
     */
    mockGraphql(options: GraphQLMockOptions<T>): RequestHandler
}

/**
 * Sets up MSW for mocking GraphQL requests in tests.
 */
export function setupMockServer<T extends Mocks>(
    options: MockServerOptions,
    ...handlers: RequestHandler[]
): MockServer<T> {
    const mock = createGraphQLMock({ typeDefs: options.typeDefs, defaultMocks })
    const server = setupServer(...handlers, createGraphQLMockRequestHandler(mock, {}))

    options.beforeAll(() => server.listen())
    options.afterEach(() => server.resetHandlers())
    options.afterAll(() => server.close())

    if (options?.inspect) {
        server.events.on('request:match', ({ request }) => {
            // eslint-disable-next-line no-console
            console.info(`[MSW] Intercepting ${request.method} ${request.url}`)
        })
    }

    server.events.on('request:unhandled', ({ request }) => {
        // eslint-disable-next-line no-console
        console.warn(`[MSW] Unhandled!!!!! ${request.method} ${request.url}`)
    })

    return {
        server,
        use(...handlers: RequestHandler[]) {
            server.use(...handlers)
        },
        store: mock.store,
        mockGraphql(options) {
            return createGraphQLMockRequestHandler(mock, options)
        },
    }
}
