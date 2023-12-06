import { readFileSync } from 'node:fs'
import path from 'node:path'

import glob from 'glob'
import type { RequestHandler } from 'msw'
import { afterAll, beforeAll, afterEach } from 'vitest'

import type { TypeMocks } from '../../graphql-types'

import { setupMockServer as baseSetupMockServer, type MockServer } from './msw-server'

const SCHEMA_DIR = path.resolve(path.join(__dirname, '../../../../../cmd/frontend/graphqlbackend'))

interface VitestMockServerOptions {
    inspect?: boolean
}

/**
 * Sets up GraphQL mocking for vitest.
 */
export function setupMockServer(
    options?: VitestMockServerOptions,
    ...handlers: RequestHandler[]
): MockServer<TypeMocks> {
    const typeDefs = glob
        .sync('**/*.graphql', { cwd: SCHEMA_DIR })
        .map(file => readFileSync(path.join(SCHEMA_DIR, file), 'utf8'))
        .join('\n')

    return baseSetupMockServer<TypeMocks>(
        {
            typeDefs,
            inspect: options?.inspect,
            afterEach,
            beforeAll,
            afterAll,
        },
        ...handlers
    )
}
