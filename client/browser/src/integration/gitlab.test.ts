import assert from 'assert'

import { Settings } from '@sourcegraph/shared/src/settings/settings'
import { createDriverForTest, Driver } from '@sourcegraph/shared/src/testing/driver'
import { setupExtensionMocking, simpleHoverProvider } from '@sourcegraph/shared/src/testing/integration/mockExtension'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'
import { readEnvironmentString, retry } from '@sourcegraph/shared/src/testing/utils'

import { BrowserIntegrationTestContext, createBrowserIntegrationTestContext } from './context'
import { closeInstallPageTab } from './shared'

describe('GitLab', () => {
    let driver: Driver
    before(async () => {
        driver = await createDriverForTest({ loadExtension: true })
        await closeInstallPageTab(driver.browser)
        if (driver.sourcegraphBaseUrl !== 'https://sourcegraph.com') {
            await driver.setExtensionSourcegraphUrl()
        }
    })
    after(() => driver?.close())

    let testContext: BrowserIntegrationTestContext
    beforeEach(async function () {
        testContext = await createBrowserIntegrationTestContext({
            driver,
            currentTest: this.currentTest!,
            directory: __dirname,
        })

        // Requests to other origins that we need to ignore to prevent breaking tests.
        testContext.server.any('https://snowplow.trx.gitlab.net/*').intercept((request, response) => {
            response.setHeader('Access-Control-Allow-Origin', 'https://gitlab.com')
            response.setHeader('Access-Control-Allow-Credentials', 'true')
            response.setHeader('Access-Control-Allow-Headers', 'Content-Type')
            response.sendStatus(200)
        })

        testContext.server.any('https://gitlab.com/api/v4/projects/*').intercept((request, response) => {
            response.sendStatus(200).send(JSON.stringify({ visibility: 'public' }))
        })

        testContext.server.any('https://sentry.gitlab.net/*').intercept((request, response) => {
            response.sendStatus(200)
        })

        testContext.overrideGraphQL({
            ViewerConfiguration: () => ({
                viewerConfiguration: {
                    subjects: [],
                    merged: { contents: '', messages: [] },
                },
            }),
            ResolveRepoName: () => ({
                repository: {
                    name: 'gitlab.com/sourcegraph/jsonrpc2',
                },
            }),
            ResolveRev: () => ({
                repository: {
                    mirrorInfo: {
                        cloned: true,
                    },
                    commit: {
                        oid: '1'.repeat(40),
                    },
                },
            }),
            ResolveRepo: ({ rawRepoName }) => ({
                repository: {
                    name: rawRepoName,
                },
            }),
            ResolveRawRepoName: ({ repoName }) => ({
                repository: { uri: `${repoName}`, mirrorInfo: { cloned: true } },
            }),
            BlobContent: () => ({
                repository: {
                    commit: {
                        file: {
                            content:
                                'package jsonrpc2\n\n// CallOption is an option that can be provided to (*Conn).Call to\n// configure custom behavior. See Meta.\ntype CallOption interface {\n\tapply(r *Request) error\n}\n\ntype callOptionFunc func(r *Request) error\n\nfunc (c callOptionFunc) apply(r *Request) error { return c(r) }\n\n// Meta returns a call option which attaches the given meta object to\n// the JSON-RPC 2.0 request (this is a Sourcegraph extension to JSON\n// RPC 2.0 for carrying metadata).\nfunc Meta(meta interface{}) CallOption {\n\treturn callOptionFunc(func(r *Request) error {\n\t\treturn r.SetMeta(meta)\n\t})\n}\n\n// PickID returns a call option which sets the ID on a request. Care must be\n// taken to ensure there are no conflicts with any previously picked ID, nor\n// with the default sequence ID.\nfunc PickID(id ID) CallOption {\n\treturn callOptionFunc(func(r *Request) error {\n\t\tr.ID = id\n\t\treturn nil\n\t})\n}\n',
                        },
                    },
                },
            }),
            SiteProductVersion: () => ({
                site: {
                    productVersion: '129819_2022-02-08_baac612f829f',
                    buildVersion: '129819_2022-02-08_baac612f829f',
                    hasCodeIntelligence: true,
                },
            }),
        })

        // Ensure that the same assets are requested in all environments.
        await driver.page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: 'light' }])
    })
    afterEachSaveScreenshotIfFailed(() => driver.page)
    afterEach(() => testContext?.dispose())

    it('adds "view on Sourcegraph" buttons to files', async () => {
        if (readEnvironmentString({ variable: 'POLLYJS_MODE', defaultValue: 'replay' }) === 'replay') {
            // mock Sourcegraph icon and bootstrap.js loaded by the extension
            // TODO: double-check it after we update tests snapshots
            for (const url of [
                'https://gitlab.com/uploads/-/system/group/avatar/*',
                'https://gitlab.com/assets/webpack/164.0d16728f.chunk.js',
            ]) {
                testContext.server.get(url).intercept((request, response) => {
                    response.sendStatus(200)
                })
            }
        }

        const repoName = 'gitlab.com/sourcegraph/jsonrpc2'

        const url = 'https://gitlab.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go'
        await driver.page.goto(url)

        await driver.page.waitForSelector('[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]', {
            timeout: 10000,
        })
        assert.strictEqual(
            (await driver.page.$$('[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]')).length,
            1
        )

        await retry(async () => {
            assert.strictEqual(
                await driver.page.evaluate(
                    () =>
                        document.querySelector<HTMLAnchorElement>(
                            '[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]'
                        )?.href
                ),
                new URL(
                    `${driver.sourcegraphBaseUrl}/${repoName}@4fb7cd90793ee6ab445f466b900e6bffb9b63d78/-/blob/call_opt.go`
                ).href
            )
        })
    })

    it('shows hover tooltips when hovering a token', async () => {
        const { mockExtension } = setupExtensionMocking()

        const userSettings: Settings = {}
        testContext.overrideGraphQL({
            ViewerConfiguration: () => ({
                viewerConfiguration: {
                    subjects: [
                        {
                            __typename: 'User',
                            displayName: 'Test User',
                            id: 'TestUserSettingsID',
                            latestSettings: {
                                id: 123,
                                contents: JSON.stringify(userSettings),
                            },
                            username: 'test',
                            viewerCanAdminister: true,
                            settingsURL: '/users/test/settings',
                        },
                    ],
                    merged: { contents: JSON.stringify(userSettings), messages: [] },
                },
            }),
        })

        // Serve a mock extension with a simple hover provider
        mockExtension({
            id: 'simple/hover',
            bundle: simpleHoverProvider,
        })

        await driver.page.goto(
            'https://gitlab.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go'
        )
        await driver.page.waitForSelector('[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]')

        // Trigger tokenization of the line.
        const lineNumber = 16
        const line = await driver.page.waitForSelector(`#LC${lineNumber}`, {
            timeout: 10000,
        })

        if (!line) {
            throw new Error(`Found no line with number ${lineNumber}`)
        }

        // Hover line to give codeintellify time to register listeners for
        // tokenization (only necessary in CI, not sure why).
        await line.hover()

        const [token] = await line.$x('.//span[text()="CallOption"]')
        await token.hover()
        await driver.findElementWithText('User is hovering over CallOption', {
            selector: '[data-testid="hover-overlay-content"] > p',
            fuzziness: 'contains',
            wait: {
                timeout: 6000,
            },
        })

        // disable flaky snapshot
        // await percySnapshot(driver.page, 'Browser extension: GitLab - blob view with code intel popup')
    })
})
