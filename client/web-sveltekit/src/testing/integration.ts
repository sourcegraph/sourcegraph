import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'path'

import { faker } from '@faker-js/faker'
import { test as base, type Page, type Locator } from '@playwright/test'
import glob from 'glob'
import { buildSchema } from 'graphql'

import { type SearchEvent } from '../lib/shared'

import { GraphQLMockServer } from './graphql-mocking'
import type { TypeMocks, ObjectMock, UserMock, OperationMocks } from './graphql-type-mocks'

// For mocking EventSource for search results
declare global {
    interface Window {
        $$sources: EventSource[]
    }
}

export { expect, defineConfig, type Locator, type Page } from '@playwright/test'

const defaultMocks: TypeMocks = {
    Query: () => ({
        // null means not signed in
        currentUser: null,
    }),
    Person: () => {
        const firstName = faker.person.firstName()
        const lastName = faker.person.lastName()
        return {
            name: `${firstName} ${lastName}`,
            email: faker.internet.email({ firstName, lastName }),
            displayName: faker.internet.userName({ firstName, lastName }),
            avatarURL: null,
        }
    },
    User: () => ({
        avatarURL: null,
    }),
    SettingsCascade: () => ({
        // Ensure this is valid JSON
        final: '{}',
    }),
    TemporarySettings: () => ({
        // Ensure this is valid JSON
        contents: '{}',
    }),
    GitBlob: () => ({
        highlight: {
            // Ensure this is valid JSON
            lsif: '{}',
        },
    }),
    GitRef: () => ({
        url: faker.internet.url(),
    }),
    Signature: () => ({
        date: faker.date.past().toISOString(),
    }),
    GitObjectID: () => faker.git.commitSha(),
    GitCommit: () => ({
        abbreviatedOID: faker.git.commitSha({ length: 7 }),
        subject: faker.git.commitMessage(),
    }),
    JSONCString: () => '{}',
}

interface MockSearchStream {
    publish(...events: SearchEvent[]): Promise<void>
    close(): Promise<void>
}

const SCHEMA_DIR = path.resolve(
    path.join(path.dirname(fileURLToPath(import.meta.url)), '../../../../cmd/frontend/graphqlbackend')
)
const typeDefs = glob
    .sync('**/*.graphql', { cwd: SCHEMA_DIR })
    .map(file => readFileSync(path.join(SCHEMA_DIR, file), 'utf8'))
    .join('\n')

class Sourcegraph {
    private debugMode = false
    constructor(private readonly page: Page, private readonly graphqlMock: GraphQLMockServer) {}

    async setup(): Promise<void> {
        await this.page.route(/\.api\/graphql/, route => {
            const { query, variables, operationName } = JSON.parse(route.request().postData() ?? '')
            const result = this.graphqlMock.query(
                query,
                variables,
                operationName,
                this.debugMode
                    ? {
                          logGraphQLErrors: true,
                          warnOnMissingOperationMocks: true,
                      }
                    : undefined
            )
            route.fulfill({ json: result })
        })
    }

    public debug() {
        this.debugMode = true
    }

    public mockTypes(mocks: TypeMocks): void {
        this.graphqlMock.addTypeMocks(mocks)
    }

    public mockOperations(mocks: OperationMocks): void {
        this.graphqlMock.addOperationMocks(mocks)
    }

    /**
     * Mocks an empty search result stream. Returns a function that can be called to simulate
     * the search results being received. The returned function will wait for the search results
     * page to be "ready" by waiting for the "Filter results" heading to be visible.
     */
    public mockSearchStream(): MockSearchStream {
        this.page.addInitScript(function () {
            window.$$sources = []
            window.EventSource = class MockEventSource {
                static readonly CONNECTING = 0
                static readonly OPEN = 1
                static readonly CLOSED = 2

                public readonly CONNECTING = 0
                public readonly OPEN = 1
                public readonly CLOSED = 2

                private listeners: Record<string, EventListener[]> = {}
                public readonly withCredentials = false
                public readyState = 0
                public onopen: EventListener | null = null
                public onmessage: EventListener | null = null
                public onerror: EventListener | null = null
                public url: string

                constructor(url: string | URL) {
                    this.readyState = 1
                    this.url = typeof url === 'string' ? url : url.href
                    console.log('Mocking event source for', url)
                    window.$$sources.push(this)
                }
                dispatchEvent(event: Event): boolean {
                    for (const listener of this.listeners[event.type] ?? []) {
                        listener(event)
                    }
                    return false
                }
                addEventListener(event: string, listener: any): void {
                    if (!this.listeners[event]) {
                        this.listeners[event] = []
                    }
                    this.listeners[event].push(listener)
                }
                removeEventListener(event: string, listener: any): void {
                    if (this.listeners[event]) {
                        this.listeners[event] = this.listeners[event].filter(l => l !== listener)
                    }
                }
                close(): void {
                    this.readyState = 2
                }
            }
        })

        return {
            publish: async (...events: SearchEvent[]): Promise<void> => {
                return this.page.evaluate(
                    ([events]) => {
                        for (const event of events) {
                            for (const source of window.$$sources) {
                                source.dispatchEvent(new MessageEvent(event.type, { data: JSON.stringify(event.data) }))
                            }
                        }
                    },
                    [events]
                )
            },
            close: async (): Promise<void> => {
                return this.page.evaluate(() => {
                    for (const source of window.$$sources) {
                        source.close()
                    }
                })
            },
        }
    }

    public fixture(fixtures: (ObjectMock & { __typename: NonNullable<ObjectMock['__typename']> })[]): void {
        // @ts-expect-error - Unclear how to type this correctly. ObjectMock is missing string index signature
        // which is required by addFixtures
        this.graphqlMock.addFixtures(fixtures)
    }

    public signIn(userMock: UserMock = {}): void {
        this.mockTypes({
            Query: () => ({
                currentUser: {
                    avatarURL: null,
                    ...userMock,
                },
            }),
        })
    }

    public signOut(): void {
        this.mockTypes({
            Query: () => ({
                currentUser: null,
            }),
        })
    }

    public teardown(): void {
        this.graphqlMock.reset()
    }
}

interface Utils {
    scrollYAt(locator: Locator, distance: number): Promise<void>
}

export const test = base.extend<{ sg: Sourcegraph; utils: Utils }, { graphqlMock: GraphQLMockServer }>({
    utils: async ({ page }, use) => {
        use({
            async scrollYAt(locator: Locator, distance: number): Promise<void> {
                // Position mouse over target that wheel events will scrolls the container
                // that contains the target
                const { x, y } = (await locator.boundingBox()) ?? { x: 0, y: 0 }
                await page.mouse.move(x, y)

                // Scroll list, which should load next page
                await page.mouse.wheel(0, distance)
            },
        })
    },
    sg: [
        async ({ page, graphqlMock }, use) => {
            const sg = new Sourcegraph(page, graphqlMock)
            await sg.setup()
            await use(sg)
            sg.teardown()
        },
        { auto: true },
    ],
    graphqlMock: [
        async ({}, use) => {
            const graphqlMock = new GraphQLMockServer({
                schema: buildSchema(typeDefs),
                mocks: defaultMocks,
                typePolicies: {
                    GitBlob: {
                        keyField: 'canonicalURL',
                    },
                    GitTree: {
                        keyField: 'canonicalURL',
                    },
                },
            })
            await use(graphqlMock)
        },
        { scope: 'worker' },
    ],
})
