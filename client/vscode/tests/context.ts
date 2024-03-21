import type puppeteer from 'puppeteer'

import type { SharedGraphQlOperations } from '@sourcegraph/shared/src/graphql-operations'
import type { SearchEvent } from '@sourcegraph/shared/src/search/stream'
import {
    createSharedIntegrationTestContext,
    type IntegrationTestContext,
    type IntegrationTestOptions,
} from '@sourcegraph/shared/src/testing/integration/context'

import type { VSCodeGraphQlOperations } from '../src/graphql-operations'

import { commonVSCodeGraphQlResults } from './graphql'

export interface VSCodeIntegrationTestContext
    extends IntegrationTestContext<
        VSCodeGraphQlOperations & SharedGraphQlOperations,
        string & keyof (VSCodeGraphQlOperations & SharedGraphQlOperations)
    > {
    /**
     * Configures fake responses for streaming search
     *
     * @param overrides The array of events to return.
     */
    overrideSearchStreamEvents: (overrides: SearchEvent[]) => void
}

export async function createVSCodeIntegrationTestContext(
    { currentTest, directory }: Omit<IntegrationTestOptions, 'driver'>,
    vsCodeFrontendPage: puppeteer.Page,
    sourcegraphBaseUrl = 'https://sourcegraph.com'
): Promise<VSCodeIntegrationTestContext> {
    const sharedTestContext = await createSharedIntegrationTestContext({
        driver: {
            newPage: () => Promise.resolve(),
            sourcegraphBaseUrl,
            browser: vsCodeFrontendPage.browser(),
            page: vsCodeFrontendPage,
        },
        currentTest,
        directory,
    })

    sharedTestContext.overrideGraphQL(commonVSCodeGraphQlResults)

    let searchStreamEventOverrides: SearchEvent[] = []

    const streamApiPath = '/.api/search/stream?*params'
    sharedTestContext.server.options(new URL(streamApiPath, sourcegraphBaseUrl).href).intercept((request, response) => {
        response
            .setHeader('Access-Control-Allow-Origin', '*')
            .setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization')
            .send(200)
    })

    sharedTestContext.server.get(new URL(streamApiPath, sourcegraphBaseUrl).href).intercept((request, response) => {
        if (!searchStreamEventOverrides || searchStreamEventOverrides.length === 0) {
            throw new Error(
                'Search stream event overrides missing. Call overrideSearchStreamEvents() to set the events.'
            )
        }

        const responseContent = searchStreamEventOverrides
            .map(event => `event: ${event.type}\ndata: ${JSON.stringify(event.data)}\n\n`)
            .join('')
        response
            .status(200)
            .setHeader('Access-Control-Allow-Origin', '*')
            .type('text/event-stream')
            .send(responseContent)
    })

    return {
        ...sharedTestContext,
        overrideSearchStreamEvents: overrides => {
            searchStreamEventOverrides = overrides
        },
        // Try closing existing search panels when we have multiple test cases.
        dispose: () => sharedTestContext.dispose(),
    }
}
