/** @jest-environment setup-polly-jest/jest-environment-node */

import { Driver, createDriverForTest } from '@sourcegraph/shared/src/testing/driver'
import { setupPollyServer } from '@sourcegraph/shared/src/testing/integration/context'
import { afterEachSaveScreenshotIfFailedWithJest } from '@sourcegraph/shared/src/testing/screenshotReporter'

import { WebIntegrationTestContext, createWebIntegrationTestContext } from './context'
import { commonWebGraphQlResults } from './graphQlResults'
import { siteID, siteGQLID } from './jscontext'
import { percySnapshotWithVariants } from './utils'

describe('Code monitoring', () => {
    let driver: Driver
    const pollyServer = setupPollyServer(__dirname)

    beforeAll(async () => {
        driver = await createDriverForTest()
    })
    afterAll(() => driver?.close())
    let testContext: WebIntegrationTestContext
    beforeEach(async () => {
        testContext = await createWebIntegrationTestContext({
            driver,
            directory: __dirname,
            pollyServer: pollyServer.polly,
        })
        testContext.overrideGraphQL({
            ...commonWebGraphQlResults,
            AutoDefinedSearchContexts: () => ({
                autoDefinedSearchContexts: [],
            }),
            ViewerSettings: () => ({
                viewerSettings: {
                    __typename: 'SettingsCascade',
                    subjects: [
                        {
                            __typename: 'DefaultSettings',
                            settingsURL: null,
                            viewerCanAdminister: false,
                            latestSettings: {
                                id: 0,
                                contents: JSON.stringify({
                                    experimentalFeatures: { showOnboardingTour: true, codeMonitoring: true },
                                }),
                            },
                        },
                        {
                            __typename: 'Site',
                            id: siteGQLID,
                            siteID,
                            latestSettings: {
                                id: 470,
                                contents: JSON.stringify({
                                    experimentalFeatures: { showOnboardingTour: true, codeMonitoring: true },
                                }),
                            },
                            settingsURL: '/site-admin/global-settings',
                            viewerCanAdminister: true,
                            allowSiteSettingsEdits: true,
                        },
                    ],
                    final: JSON.stringify({}),
                },
            }),
            ListUserCodeMonitors: () => ({
                node: {
                    __typename: 'User',
                    monitors: {
                        nodes: [
                            {
                                id: 'Q29kZU1vbml0b3I6Mg==',
                                description: 'Test123',
                                enabled: true,
                                actions: {
                                    nodes: [
                                        {
                                            __typename: 'MonitorEmail',
                                            enabled: true,
                                            includeResults: false,
                                            id: 'Q29kZU1vbml0b3JBY3Rpb25FbWFpbDoy',
                                            recipients: {
                                                nodes: [{ id: 'VXNlcjoyNDc=' }],
                                            },
                                        },
                                    ],
                                },
                                trigger: {
                                    id: 'Q29kZU1vbml0b3JUcmlnZ2VyUXVlcnk6Mg==',
                                    query:
                                        'type:diff repo:sourcegraph/sourcegraph after:\\"1 week ago\\" filtered  patternType:literal',
                                },
                            },
                        ],
                        __typename: 'MonitorConnection',
                        pageInfo: { endCursor: null, hasNextPage: false },
                        totalCount: 1,
                    },
                },
            }),
        })
    })
    afterEachSaveScreenshotIfFailedWithJest(() => driver.page)
    afterEach(() => testContext?.dispose())

    describe('Code monitoring', () => {
        it('is styled correctly', async () => {
            await driver.page.goto(driver.sourcegraphBaseUrl + '/code-monitoring')
            await driver.page.waitForSelector('[data-testid="code-monitoring-page"]')
            await percySnapshotWithVariants(driver.page, 'Code monitor list')
        })
    })

    describe('Code monitoring form advances sequentially', () => {
        it('validates trigger query input', async () => {
            await driver.page.goto(driver.sourcegraphBaseUrl + '/code-monitoring/new')
            await driver.page.waitForSelector('.test-name-input')

            await percySnapshotWithVariants(driver.page, 'Code monitoring - Form')

            await driver.page.type('.test-name-input', 'test monitor')

            await driver.page.waitForSelector('.test-action-button-email')
            expect(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLButtonElement>('.test-action-button-email')!.disabled
                )
            ).toBe(true)

            await driver.page.waitForSelector('.test-trigger-button')
            await driver.page.click('.test-trigger-button')

            await driver.page.waitForSelector('.test-trigger-input')
            await driver.page.type('.test-trigger-input', 'foobar')
            await driver.page.waitForSelector('.test-is-invalid')

            await driver.page.type('.test-trigger-input', ' type:diff')
            await driver.page.waitForSelector('.test-is-invalid')

            await driver.page.type('.test-trigger-input', ' repo:test')
            await driver.page.waitForSelector('.test-is-valid')
            await driver.page.waitForSelector('.test-preview-link')
            expect(
                await driver.page.evaluate(() => document.querySelectorAll('.test-preview-link').length)
            ).toBeGreaterThan(0)
        })

        it('disables the actions area until trigger is complete', async () => {
            await driver.page.goto(driver.sourcegraphBaseUrl + '/code-monitoring/new')
            await driver.page.waitForSelector('.test-name-input')
            await driver.page.type('.test-name-input', 'test monitor')

            await driver.page.waitForSelector('.test-action-button-email')
            expect(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLButtonElement>('.test-action-button-email')!.disabled
                )
            ).toBe(true)

            await driver.page.waitForSelector('.test-trigger-button')
            await driver.page.click('.test-trigger-button')

            await driver.page.waitForSelector('.test-trigger-input')
            await driver.page.type('.test-trigger-input', 'foobar type:diff repo:test')
            await driver.page.waitForSelector('.test-is-valid')
            await driver.page.waitForSelector('.test-preview-link')
            await driver.page.waitForSelector('.test-submit-trigger')
            await driver.page.click('.test-submit-trigger')

            await driver.page.waitForSelector('.test-action-button-email')
            expect(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLButtonElement>('.test-action-button-email')!.disabled
                )
            ).toBe(false)

            await driver.page.click('.test-action-button-email')
            await driver.page.waitForSelector('.test-action-form-email')
        })

        it('disables submitting the code monitor area until trigger and action are complete', async () => {
            await driver.page.goto(driver.sourcegraphBaseUrl + '/code-monitoring/new')
            await driver.page.waitForSelector('.test-name-input')
            await driver.page.type('.test-name-input', 'test monitor')

            await driver.page.waitForSelector('.test-submit-monitor')
            expect(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLButtonElement>('.test-submit-monitor')!.disabled
                )
            ).toBe(true)

            await driver.page.waitForSelector('.test-trigger-button')
            await driver.page.click('.test-trigger-button')

            await driver.page.waitForSelector('.test-trigger-input')
            await driver.page.type('.test-trigger-input', 'foobar type:diff repo:test')
            await driver.page.waitForSelector('.test-is-valid')
            await driver.page.waitForSelector('.test-preview-link')
            await driver.page.waitForSelector('.test-submit-trigger')
            await driver.page.click('.test-submit-trigger')

            await driver.page.waitForSelector('.test-action-button-email')
            await driver.page.click('.test-action-button-email')
            await driver.page.waitForSelector('.test-action-form-email')
            await driver.page.waitForSelector('.test-submit-action-email')
            await driver.page.click('.test-submit-action-email')

            expect(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLButtonElement>('.test-submit-monitor')!.disabled
                )
            ).toBe(false)
        })
    })
})
