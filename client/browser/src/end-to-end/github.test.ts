import assert from 'assert'

import { startCase } from 'lodash'
import { describe, it } from 'mocha'
import type { Target, Page } from 'puppeteer'
import { firstValueFrom, fromEvent } from 'rxjs'
import { filter, timeout, mergeMap } from 'rxjs/operators'

import { isDefined } from '@sourcegraph/common'
import { getConfig } from '@sourcegraph/shared/src/testing/config'
import { type Driver, createDriverForTest } from '@sourcegraph/shared/src/testing/driver'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'
import { retry } from '@sourcegraph/shared/src/testing/utils'

import { closeInstallPageTab, testSingleFilePage } from './shared'

describe('Sourcegraph browser extension on github.com', function () {
    this.slow(8000)

    const { browser, sourcegraphBaseUrl, ...restConfig } = getConfig('browser', 'sourcegraphBaseUrl')

    let driver: Driver

    before('Open browser', async function () {
        this.timeout(90 * 1000)
        driver = await createDriverForTest({ loadExtension: true, browser, sourcegraphBaseUrl, ...restConfig })
        if (sourcegraphBaseUrl !== 'https://sourcegraph.com') {
            if (restConfig.testUserPassword) {
                await driver.ensureSignedIn({ username: 'test', password: restConfig.testUserPassword })
            }
            await driver.setExtensionSourcegraphUrl()
        }
    })

    // Take a screenshot when a test fails
    afterEachSaveScreenshotIfFailed(() => driver.page)

    after('Close browser', () => driver?.close())

    testSingleFilePage({
        getDriver: () => driver,
        url: 'https://github.com/sourcegraph/jsonrpc2/blob/6864d8cc6d35a79f50745f8990cb4d594a8036f4/call_opt.go',
        repoName: 'github.com/sourcegraph/jsonrpc2',
        commitID: '6864d8cc6d35a79f50745f8990cb4d594a8036f4',
        sourcegraphBaseUrl,
        // Hovercards are broken on the new GitHub file page
        // getLineSelector: lineNumber => `#LC${lineNumber}`,
        goToDefinitionURL:
            'https://github.com/sourcegraph/jsonrpc2/blob/6864d8cc6d35a79f50745f8990cb4d594a8036f4/call_opt.go#L5:6',
    })

    const tokens = {
        // https://github.com/gorilla/mux/pull/117/files#diff-9ef8a22c4ce5141c30a501c542fb1adeL244
        base: {
            token: 'varsN',
            lineId: 'diff-a609417fa264c6aed88fb8cfe2d9b4fb24226ffdf7db1f685e344d5239783d46L244',
            goToDefinitionURLs: [
                'https://github.com/gorilla/mux/blob/f15e0c49460fd49eebe2bcc8486b05d1bef68d3a/regexp.go#L139:2',
            ],
        },
        // https://github.com/gorilla/mux/pull/117/files#diff-9ef8a22c4ce5141c30a501c542fb1adeR247
        head: {
            token: 'host',
            lineId: 'diff-a609417fa264c6aed88fb8cfe2d9b4fb24226ffdf7db1f685e344d5239783d46R247',
            // There are multiple passing go to definition URLs:
            // https://github.com/sourcegraph/sourcegraph/pull/20520
            goToDefinitionURLs: [
                'https://github.com/gorilla/mux/blob/e73f183699f8ab7d54609771e1fa0ab7ffddc21b/regexp.go#L233:2',
                'https://sourcegraph.com/github.com/gorilla/mux@e73f183699f8ab7d54609771e1fa0ab7ffddc21b/-/blob/regexp.go#L247:24&tab=def',
            ],
        },
    }

    // Replace these unstable tests with integration tests with stubs:
    // https://github.com/sourcegraph/sourcegraph/pull/20520#issuecomment-829726482
    describe.skip('Pull request pages', () => {
        for (const diffType of ['unified', 'split']) {
            describe(`${startCase(diffType)} view`, () => {
                for (const side of ['base', 'head'] as const) {
                    const { token, lineId, goToDefinitionURLs } = tokens[side]
                    it(`provides hover tooltips on token "${token}" in the ${side} part`, async () => {
                        await driver.page.goto(`https://github.com/gorilla/mux/pull/117/files?diff=${diffType}`)
                        await closeInstallPageTab(driver.browser)
                        // The browser extension takes a bit to initialize and register all event listeners.
                        // Waiting here saves one retry cycle below in the common case.
                        // If it's not enough, the retry will catch it.
                        await driver.page.waitFor(1500)
                        const tokenElement = await retry(async () => {
                            const lineNumberElement = await driver.page.waitForSelector(`#${lineId}`, {
                                timeout: 10000,
                            })
                            const row = (
                                await driver.page.evaluateHandle(
                                    (element: Element) => element.closest('tr'),
                                    lineNumberElement
                                )
                            ).asElement()!
                            assert(row, 'Expected row to exist')
                            const tokenElement = (
                                await driver.page.evaluateHandle(
                                    (row: Element, token: string) =>
                                        [...row.querySelectorAll('span')].find(
                                            element => element.textContent === token
                                        ),
                                    row,
                                    token
                                )
                            ).asElement()
                            assert(tokenElement, 'Expected token element to exist')
                            return tokenElement
                        })
                        // Retry is here to wait for listeners to be registered
                        await retry(async () => {
                            await tokenElement.hover()
                            await driver.page.waitForSelector('.test-tooltip-go-to-definition', { timeout: 5000 })
                        })

                        // Check go-to-definition jumps to the right place
                        let goToDefinitionURL = ''
                        await retry(async () => {
                            const href =
                                (await driver.page.evaluate(
                                    () =>
                                        document.querySelector<HTMLAnchorElement>('.test-tooltip-go-to-definition')
                                            ?.href
                                )) ?? ''

                            assert.strictEqual(
                                goToDefinitionURLs.includes(href),
                                true,
                                `Expected goToDefinitionURL (${href}) to be one of:\n\t${goToDefinitionURLs.join(
                                    '\n\t'
                                )}`
                            )
                            goToDefinitionURL = href
                        })
                        if (!goToDefinitionURL) {
                            throw new Error('Expected goToDefinitionURL to be truthy')
                        }

                        let page: Page = driver.page
                        if (new URL(goToDefinitionURL).hostname !== 'github.com') {
                            ;[page] = await Promise.all([
                                firstValueFrom(
                                    fromEvent<Target>(driver.browser, 'targetcreated').pipe(
                                        timeout(5000),
                                        mergeMap(target => target.page()),
                                        filter(isDefined)
                                    )
                                ),
                                driver.page.click('.test-tooltip-go-to-definition'),
                            ])
                        } else {
                            await Promise.all([
                                driver.page.waitForNavigation(),
                                driver.page.click('.test-tooltip-go-to-definition'),
                            ])
                        }
                        await retry(async () => {
                            assert.strictEqual(await page.evaluate(() => location.href), goToDefinitionURL)
                        })
                        // If an additional page was opened, close it so we return to the original `driver.page`.
                        if (page !== driver.page) {
                            await page.close()
                        }
                    })
                }
            })
        }
    })
})
