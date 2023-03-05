import { applyEdits, JSONPath, modify } from 'jsonc-parser'
import { describe, before, after, test } from 'mocha'

import { overwriteSettings } from '@sourcegraph/shared/src/settings/edit'
import { getConfig } from '@sourcegraph/shared/src/testing/config'
import { Driver } from '@sourcegraph/shared/src/testing/driver'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'

import { ExternalServiceKind } from '../graphql-operations'

import { ensureTestExternalService, getUser, setTosAccepted, setUserSiteAdmin } from './util/api'
import { GraphQLClient } from './util/GraphQlClient'
import { ensureSignedInOrCreateTestUser, getGlobalSettings } from './util/helpers'
import { getTestTools } from './util/init'
import { TestResourceManager } from './util/TestResourceManager'

describe('Code graph regression test suite', () => {
    const testUsername = 'test-sg-codeintel'
    const config = getConfig(
        'gitHubToken',
        'headless',
        'keepBrowser',
        'logBrowserConsole',
        'logStatusMessages',
        'noCleanup',
        'slowMo',
        'sourcegraphBaseUrl',
        'sudoToken',
        'sudoUsername',
        'testUserPassword'
    )
    const testExternalServiceInfo = {
        kind: ExternalServiceKind.GITHUB,
        uniqueDisplayName: '[TEST] GitHub (codeintel.test.ts)',
    }

    const testRepoSlugs = [
        'sourcegraph/run',
        'sourcegraph-testing/prometheus-common',
        'sourcegraph-testing/prometheus-client-golang',
        'sourcegraph-testing/prometheus-redefinitions',
    ]

    // const prometheusCommonHeadCommit = 'b5fe7d854c42dc7842e48d1ca58f60feae09d77b' // HEAD
    // const prometheusRedefinitionsHeadCommit = 'c68f0e063cf8a98e7ce3428cfd50588746010f1f'

    let driver: Driver
    let gqlClient: GraphQLClient
    let outerResourceManager: TestResourceManager
    before(async function () {
        // sourcegraph/sourcegraph takes a while to clone
        this.timeout(6 * 6 * 60 * 1000)
        ;({ driver, gqlClient, resourceManager: outerResourceManager } = await getTestTools(config))
        outerResourceManager.add(
            'User',
            testUsername,
            await ensureSignedInOrCreateTestUser(driver, gqlClient, {
                username: testUsername,
                deleteIfExists: true,
                ...config,
            })
        )
        outerResourceManager.add(
            'External service',
            testExternalServiceInfo.uniqueDisplayName,
            await ensureTestExternalService(
                gqlClient,
                {
                    ...testExternalServiceInfo,
                    config: {
                        url: 'https://github.com',
                        token: config.gitHubToken,
                        repos: testRepoSlugs,
                        repositoryQuery: ['none'],
                    },
                    waitForRepos: testRepoSlugs.map(slug => `github.com/${slug}`),
                },
                { ...config, timeout: 3 * 60 * 1000 }
            )
        )

        const user = await getUser(gqlClient, testUsername)
        if (!user) {
            throw new Error(`test user ${testUsername} does not exist`)
        }
        await setUserSiteAdmin(gqlClient, user.id, true)
        await setTosAccepted(gqlClient, user.id)

        outerResourceManager.add('Global setting', 'codeIntel.includeForks', await setIncludeForks(gqlClient, true))
    })

    afterEachSaveScreenshotIfFailed(() => driver.page)

    after(async () => {
        if (!config.noCleanup) {
            await outerResourceManager.destroyAll()
        }
        if (driver) {
            await driver.close()
        }
    })

    describe('Basic code graph regression test suite', () => {
        const innerResourceManager = new TestResourceManager()
        before(async () => {
            innerResourceManager.add('Global setting', 'codeIntel.lsif', await setGlobalLSIFSetting(gqlClient, false))
        })
        after(async () => {
            if (!config.noCleanup) {
                await innerResourceManager.destroyAll()
            }
        })

        // Re-enabling tracked in https://github.com/sourcegraph/sourcegraph/issues/39000
        test.skip('File sidebar, multiple levels of directories', async () => {
            await driver.page.goto(
                config.sourcegraphBaseUrl +
                    '/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3'
            )
            for (const file of ['cmd', 'frontend', 'auth', 'providers', 'providers.go']) {
                await driver.findElementWithText(file, {
                    action: 'click',
                    selector: '.test-repo-revision-sidebar a',
                    wait: { timeout: 2 * 1000 },
                })
            }
            await driver.waitUntilURL(
                `${config.sourcegraphBaseUrl}/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3/-/blob/cmd/frontend/auth/providers/providers.go`,
                { timeout: 2 * 1000 }
            )
        })

        // TODO: Disabled because it's flaky. https://github.com/sourcegraph/sourcegraph/issues/23098
        // test('Symbols sidebar', async () => {
        //     await driver.page.goto(
        //         config.sourcegraphBaseUrl +
        //             '/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3'
        //     )
        //     await driver.findElementWithText('Symbols', {
        //         action: 'click',
        //         selector: '.test-repo-revision-sidebar button',
        //         wait: { timeout: 10 * 1000 },
        //     })
        //     await driver.findElementWithText('backgroundEntry', {
        //         action: 'click',
        //         selector: '.test-repo-revision-sidebar a span',
        //         wait: { timeout: 2 * 1000 },
        //     })
        //     await driver.replaceText({
        //         selector: 'input[placeholder="Search symbols..."]',
        //         newText: 'buildentry',
        //     })
        //     await driver.page.waitForFunction(
        //         () => {
        //             const sidebar = document.querySelector<HTMLElement>('.test-repo-revision-sidebar')
        //             return sidebar && !sidebar.textContent?.includes('backgroundEntry')
        //         },
        //         { timeout: 2 * 1000 }
        //     )
        //     await driver.findElementWithText('buildEntry', {
        //         action: 'click',
        //         selector: '.test-repo-revision-sidebar a span',
        //         wait: { timeout: 2 * 1000 },
        //     })
        //     await driver.waitUntilURL(
        //         `${config.sourcegraphBaseUrl}/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3/-/blob/browser/config/webpack/base.config.ts?L6:7-6:17`,
        //         { timeout: 2 * 1000 }
        //     )
        // })

        // TODO: Disabled because it's flaky. https://github.com/sourcegraph/sourcegraph/issues/32684
        // test('Definitions, references, and hovers', () =>
        //     testCodeNavigation(driver, config, {
        //         page: `/github.com/sourcegraph-testing/prometheus-common@${prometheusCommonHeadCommit}/-/blob/model/value.go`,
        //         line: 225,
        //         token: 'SamplePair',
        //         expectedHoverContains: 'SamplePair pairs a SampleValue with a Timestamp.',
        //         // TODO(efritz) - determine why reference panel shows up during this test,
        //         // but only when automated - doing the same flow manually works correctly.
        //         // expectedDefinition: `/github.com/sourcegraph-testing/prometheus-common@${prometheusCommonHeadCommit}/-/blob/model/value.go#L78:1`,
        //         expectedReferences: [
        //             `/github.com/sourcegraph-testing/prometheus-common@${prometheusCommonHeadCommit}/-/blob/model/value.go?L97:10&subtree=true`,
        //             `/github.com/sourcegraph-testing/prometheus-common@${prometheusCommonHeadCommit}/-/blob/model/value.go?L225:11&subtree=true`,
        //             `/github.com/sourcegraph-testing/prometheus-redefinitions@${prometheusRedefinitionsHeadCommit}/-/blob/sample.go?L7:6&subtree=true`,
        //         ],
        //     }))
    })
})

//
// Code navigation utilities

// interface CodeNavigationTestCase {
//     /**
//      * The source page.
//      */
//     page: string
//
//     /**
//      * The source line.
//      */
//     line: number
//
//     /**
//      * The token to click. Should be unambiguous within this line for the test to succeed.
//      */
//     token: string
//
//     /**
//      * A substring of the expected hover text
//      */
//     expectedHoverContains: string
//
//     /**
//      * A locations (if unambiguous), or a subset of locations that must occur within the definitions panel.
//      */
//     expectedDefinition?: string | string[]
//
//     /**
//      * A subset of locations that must occur within the references panel.
//      */
//     expectedReferences?: string[]
// }

// /**
//  * Navigate to the given page and test the definitions, references, and hovers of the token
//  * on the given line. Will ensure both hover and clicking the token produces the hover overlay.
//  * Will check the precision indicator of the hoverlay and each file match in the definition
//  * and reference panels. Will compare hover text. Will compare location of each file match or
//  * the target of the page navigated to on jump-to-definition (in the case of a single definition).
//  */
// async function testCodeNavigation(
//     driver: Driver,
//     config: Pick<Config, 'sourcegraphBaseUrl'>,
//     { page, line, token, expectedHoverContains, expectedDefinition, expectedReferences }: CodeNavigationTestCase
// ): Promise<void> {
//     await driver.page.goto(config.sourcegraphBaseUrl + page)
//     await driver.page.waitForSelector('.test-blob')
//     const tokenElement = await findTokenElement(driver, line, token)
//
//     // Check hover
//     await tokenElement.hover()
//     await waitForHover(driver, expectedHoverContains)
//
//     // Check click
//     await clickOnEmptyPartOfCodeView(driver)
//     await tokenElement.click()
//     await waitForHover(driver, expectedHoverContains)
//
//     // Find-references
//     if (expectedReferences && expectedReferences.length > 0) {
//         await clickOnEmptyPartOfCodeView(driver)
//         await tokenElement.hover()
//         await waitForHover(driver, expectedHoverContains)
//         await (await driver.findElementWithText('Find references')).click()
//
//         await driver.page.waitForSelector('.test-search-result')
//         const referenceLinks = await collectLinks(driver)
//         for (const expectedReference of expectedReferences) {
//             expect(referenceLinks).toContainEqual(expectedReference)
//         }
//         await clickOnEmptyPartOfCodeView(driver)
//     }
//
//     // Go-to-definition
//     await clickOnEmptyPartOfCodeView(driver)
//     await tokenElement.hover()
//     await waitForHover(driver, expectedHoverContains)
//     await (await driver.findElementWithText('Go to definition')).click()
//
//     if (Array.isArray(expectedDefinition)) {
//         throw new Error('test not implemented')
//     } else if (expectedDefinition) {
//         await driver.page.waitForFunction(
//             (defURL: string) => document.location.href.endsWith(defURL),
//             { timeout: 2000 },
//             expectedDefinition
//         )
//
//         await driver.page.goBack()
//     }
//
//     await driver.page.keyboard.press('Escape')
// }

// /**
//  * Return a list of locations (and their precision) that exist in the file list
//  * panel. This will click on each repository and collect the visible links in a
//  * sequence.
//  */
// async function collectLinks(driver: Driver): Promise<Set<string>> {
//     throw new Error('not implemented')
// }

// /**
//  * Close any visible hover overlay.
//  */
// async function clickOnEmptyPartOfCodeView(driver: Driver): Promise<void> {
//     await driver.page.click('.test-blob tr:nth-child(1) .line')
//     await driver.page.waitForFunction(() => document.querySelectorAll('.test-tooltip-go-to-definition').length === 0)
// }

// /**
//  * Find the element with the token text on the given line.
//  *
//  * Will close any toast so that the entire line is visible and will hover over the line
//  * to ensure that the line is tokenized (as this is done on-demand).
//  */
// async function findTokenElement(driver: Driver, line: number, token: string): Promise<ElementHandle<Element>> {
//     try {
//         // If there's an open toast, close it. If the toast remains open and our target
//         // identifier happens to be hidden by it, we won't be able to select the correct
//         // token. This condition was reproducible in the code navigation test that searches
//         // for the identifier `StdioLogger`.
//         await driver.page.click('.test-close-toast')
//     } catch {
//         // No toast open, this is fine
//     }
//
//     const selector = `.test-blob tr:nth-child(${line}) span`
//     await driver.page.hover(selector)
//     return driver.findElementWithText(token, { selector, fuzziness: 'exact' })
// }

// /**
//  * Wait for the hover tooltip to become visible. Compare the visible text with the expected
//  * contents (expected contents must be a substring of the visible contents).
//  */
// async function waitForHover(driver: Driver, expectedHoverContains: string): Promise<void> {
//     await driver.page.waitForSelector('.test-tooltip-go-to-definition')
//     await driver.page.waitForSelector('.test-tooltip-content')
//     expect(normalizeWhitespace(await getTooltip(driver))).toContain(normalizeWhitespace(expectedHoverContains))
// }

// /**
//  * Return the currently visible hover text.
//  */
// async function getTooltip(driver: Driver): Promise<string> {
//     return driver.page.evaluate(
//         () => (document.querySelector('.test-tooltip-content') as HTMLElement).textContent || ''
//     )
// }

// /**
//  * Collapse multiple spaces into one.
//  */
// function normalizeWhitespace(string: string): string {
//     return string.replace(/\s+/g, ' ')
// }

//
// LSIF utilities

/** Replace the codeIntel.includeForks setting with the given value. */
async function setIncludeForks(gqlClient: GraphQLClient, enabled: boolean): Promise<() => Promise<void>> {
    return writeSetting(gqlClient, ['basicCodeIntel.includeForks'], enabled)
}
/** Replace the codeIntel.lsif setting with the given value. */
async function setGlobalLSIFSetting(gqlClient: GraphQLClient, enabled: boolean): Promise<() => Promise<void>> {
    return writeSetting(gqlClient, ['codeIntel.lsif'], enabled)
}

/**
 * Return a promise that updates the global settings to their original value. This return value
 * is suitable for use with the resource manager's destroy queue.
 */
async function writeSetting(gqlClient: GraphQLClient, path: JSONPath, value: unknown): Promise<() => Promise<void>> {
    const { subjectID, settingsID, contents: oldContents } = await getGlobalSettings(gqlClient)
    const newContents = applyEdits(
        oldContents,
        modify(oldContents, path, value, {
            formattingOptions: {
                eol: '\n',
                insertSpaces: true,
                tabSize: 2,
            },
        })
    )

    await overwriteSettings(gqlClient, subjectID, settingsID, newContents)
    return async () => {
        const { subjectID: currentSubjectID, settingsID: currentSettingsID } = await getGlobalSettings(gqlClient)
        await overwriteSettings(gqlClient, currentSubjectID, currentSettingsID, oldContents)
    }
}
