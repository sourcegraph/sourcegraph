import delay from 'delay'
import expect from 'expect'
import { applyEdits, parse, modify } from 'jsonc-parser'
import { describe, before, beforeEach, after, afterEach, test } from 'mocha'
import { lastValueFrom } from 'rxjs'
import { map } from 'rxjs/operators'

import { logger } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import { overwriteSettings } from '@sourcegraph/shared/src/settings/edit'
import { getConfig } from '@sourcegraph/shared/src/testing/config'
import type { Driver } from '@sourcegraph/shared/src/testing/driver'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'

import { getUser, setTosAccepted } from './util/api'
import { type GraphQLClient, createGraphQLClient } from './util/GraphQlClient'
import { ensureSignedInOrCreateTestUser, getGlobalSettings } from './util/helpers'
import { getTestTools } from './util/init'
import { ScreenshotVerifier } from './util/ScreenshotVerifier'
import { TestResourceManager } from './util/TestResourceManager'

// TODO: Disabled because all of the tests in this suite are flaky:
// - https://github.com/sourcegraph/sourcegraph/issues/29098
// - https://github.com/sourcegraph/sourcegraph/issues/49161
// - https://github.com/sourcegraph/sourcegraph/issues/23049
describe.skip('Core functionality regression test suite', () => {
    const testUsername = 'test-core'
    const config = getConfig(
        'sudoToken',
        'sudoUsername',
        'gitHubToken',
        'sourcegraphBaseUrl',
        'noCleanup',
        'testUserPassword',
        'logBrowserConsole',
        'slowMo',
        'headless',
        'keepBrowser'
    )

    let driver: Driver
    let gqlClient: GraphQLClient
    let resourceManager: TestResourceManager
    let screenshots: ScreenshotVerifier
    before(async () => {
        ;({ driver, gqlClient, resourceManager } = await getTestTools(config))
        resourceManager.add(
            'User',
            testUsername,
            await ensureSignedInOrCreateTestUser(driver, gqlClient, {
                username: testUsername,
                deleteIfExists: true,
                ...config,
            })
        )
        const user = await getUser(gqlClient, testUsername)
        if (!user) {
            throw new Error(`test user ${testUsername} does not exist`)
        }
        await setTosAccepted(gqlClient, user.id)
        screenshots = new ScreenshotVerifier(driver)
    })

    afterEachSaveScreenshotIfFailed(() => driver.page)

    after(async () => {
        if (!config.noCleanup) {
            await resourceManager.destroyAll()
        }
        if (driver) {
            await driver.close()
        }
        if (screenshots.screenshots.length > 0) {
            logger.log(screenshots.verificationInstructions())
        }
    })

    let alwaysCleanupManager: TestResourceManager
    beforeEach(() => {
        alwaysCleanupManager = new TestResourceManager()
    })
    afterEach(async () => {
        await alwaysCleanupManager.destroyAll()
    })

    test('2.2.1 User settings are saved and applied', async () => {
        const getSettings = async () => {
            await driver.page.waitForSelector('.test-settings-file .monaco-editor .view-lines')
            return driver.page.evaluate(() => {
                const editor = document.querySelector('.test-settings-file .monaco-editor .view-lines') as HTMLElement

                return editor ? editor.innerText : null
            })
        }

        await driver.page.goto(config.sourcegraphBaseUrl + `/users/${testUsername}/settings`)
        const previousSettings = await getSettings()
        if (!previousSettings) {
            throw new Error('Previous settings were null')
        }
        const newSettings = '{\u00A0/*\u00A0These\u00A0are\u00A0new\u00A0settings\u00A0*/}'
        await driver.replaceText({
            selector: '.test-settings-file .monaco-editor',
            newText: newSettings,
            selectMethod: 'keyboard',
            enterTextMethod: 'paste',
        })
        await driver.page.reload()

        const currentSettings = await getSettings()
        if (currentSettings !== previousSettings) {
            throw new Error(
                `Settings ${JSON.stringify(currentSettings)} did not match (old) saved settings ${JSON.stringify(
                    previousSettings
                )}`
            )
        }

        await driver.replaceText({
            selector: '.test-settings-file .monaco-editor',
            newText: newSettings,
            selectMethod: 'keyboard',
            enterTextMethod: 'type',
        })
        await driver.findElementWithText('Save', { action: 'click' })
        await driver.page.waitForFunction(
            () => document.evaluate("//*[text() = ' Saving...']", document).iterateNext() === null
        )
        await driver.page.reload()

        const currentSettings2 = await getSettings()
        if (JSON.stringify(currentSettings2) !== JSON.stringify(newSettings)) {
            throw new Error(
                `Settings ${JSON.stringify(currentSettings2)} did not match (new) saved settings ${JSON.stringify(
                    newSettings
                )}`
            )
        }

        await driver.replaceText({
            selector: '.test-settings-file .monaco-editor',
            newText: previousSettings,
            selectMethod: 'keyboard',
            enterTextMethod: 'paste',
        })
        await driver.findElementWithText('Save changes', { action: 'click' })
        await driver.page.waitForFunction(
            () => document.evaluate("//*[text() = ' Saving...']", document).iterateNext() === null
        )
        const previousSettings2 = await getSettings()
        await driver.page.reload()

        const currentSettings3 = await getSettings()
        if (currentSettings3 !== previousSettings2) {
            throw new Error(
                `Settings ${JSON.stringify(currentSettings3)} did not match (old) saved settings ${JSON.stringify(
                    previousSettings2
                )}`
            )
        }
    })

    test('2.2.4 Access tokens work and invalid access tokens return "401 Unauthorized"', async () => {
        await driver.page.goto(config.sourcegraphBaseUrl + `/users/${testUsername}/settings/tokens`)
        await driver.findElementWithText('Generate new token', { action: 'click', wait: { timeout: 5000 } })
        await driver.findElementWithText('New access token', { wait: { timeout: 1000 } })
        await driver.replaceText({
            selector: '[data-testid=test-create-access-token-description]',
            newText: 'test-regression',
        })
        await driver.findElementWithText('Generate token', { action: 'click', wait: { timeout: 1000 } })
        await driver.findElementWithText("Copy the new access token now. You won't be able to see it again.", {
            wait: { timeout: 1000 },
        })
        await driver.findElementWithText('Copy', { action: 'click' })
        const token = await driver.page.evaluate(() => {
            const tokenElement = document.querySelector('.test-access-token')
            if (!tokenElement) {
                return null
            }
            const inputElement = tokenElement.querySelector('input')
            if (!inputElement) {
                return null
            }
            return inputElement.value
        })
        if (!token) {
            throw new Error('Could not obtain access token')
        }
        const gqlClientWithToken = createGraphQLClient({
            baseUrl: config.sourcegraphBaseUrl,
            token,
        })
        await delay(2000)
        const currentUsernameQuery = gql`
            query CurrentUsername {
                currentUser {
                    username
                }
            }
        `
        const response = await lastValueFrom(
            gqlClientWithToken.queryGraphQL(currentUsernameQuery).pipe(map(dataOrThrowErrors))
        )
        expect(response).toEqual({ currentUser: { username: testUsername } })

        const gqlClientWithInvalidToken = createGraphQLClient({
            baseUrl: config.sourcegraphBaseUrl,
            token: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        })

        await expect(
            lastValueFrom(gqlClientWithInvalidToken.queryGraphQL(currentUsernameQuery).pipe(map(dataOrThrowErrors)))
        ).rejects.toThrowError('401 Unauthorized')
    })

    test('2.5 Quicklinks: add a quicklink, test that it appears on the front page and works.', async () => {
        const quicklinkInfo = {
            name: 'Quicklink',
            url: config.sourcegraphBaseUrl + '/api/console',
            description: 'This is a quicklink',
        }

        const { subjectID, settingsID, contents: oldContents } = await getGlobalSettings(gqlClient)
        const parsedOldContents = parse(oldContents)
        if (parsedOldContents?.quicklinks) {
            throw new Error('Global setting quicklinks already exists, aborting test')
        }
        const newContents = applyEdits(
            oldContents,
            modify(oldContents, ['quicklinks'], [quicklinkInfo], {
                formattingOptions: {
                    eol: '\n',
                    insertSpaces: true,
                    tabSize: 2,
                },
            })
        )
        await overwriteSettings(gqlClient, subjectID, settingsID, newContents)
        alwaysCleanupManager.add('Global setting', 'quicklinks', async () => {
            const { subjectID: currentSubjectID, settingsID: currentSettingsID } = await getGlobalSettings(gqlClient)
            await overwriteSettings(gqlClient, currentSubjectID, currentSettingsID, oldContents)
        })

        await driver.page.goto(config.sourcegraphBaseUrl + '/search')
        await (
            await driver.findElementWithText(quicklinkInfo.name, {
                selector: 'a',
                wait: { timeout: 1000 },
            })
        ).hover()
        await driver.findElementWithText(quicklinkInfo.description, {
            wait: { timeout: 1000 },
        })
        await driver.findElementWithText(quicklinkInfo.name, {
            action: 'click',
            selector: 'a',
            wait: { timeout: 1000 },
        })
        await driver.page.waitForNavigation()
        expect(driver.page.url()).toEqual(quicklinkInfo.url)
    })
})
