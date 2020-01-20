import { describe, before, after, test } from 'mocha'
import { Driver } from '../../../shared/src/e2e/driver'
import { getConfig } from '../../../shared/src/e2e/config'
import { getTestTools } from './util/init'
import { TestResourceManager } from './util/TestResourceManager'
import { GraphQLClient } from './util/GraphQLClient'
import { ensureLoggedInOrCreateTestUser } from './util/helpers'
import { ScreenshotVerifier } from './util/ScreenshotVerifier'
import * as GQL from '../../../shared/src/graphql/schema'
import { ensureTestExternalService, setUserSiteAdmin, getUser } from './util/api'
import { testCodeNavigation } from './util/codenav'
import { disableLSIF } from './util/codeintel'
import { saveScreenshotsUponFailures } from '../../../shared/src/e2e/screenshotReporter'

describe('Code navigation regression test suite', () => {
    const testUsername = 'test-codenav'
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
        'keepBrowser',
        'logStatusMessages'
    )
    const testExternalServiceInfo = {
        kind: GQL.ExternalServiceKind.GITHUB,
        uniqueDisplayName: '[TEST] GitHub (codenav.test.ts)',
    }
    const testRepoSlugs = [
        'sourcegraph/sourcegraph', // Go and TypeScript
        'sourcegraph/javascript-typescript-langserver', // TypeScript
        'sourcegraph/appdash', // Python
    ]

    let driver: Driver
    let gqlClient: GraphQLClient
    let resourceManager: TestResourceManager
    let screenshots: ScreenshotVerifier
    before(async function() {
        this.timeout(2 * 60 * 1000 + 10 * 1000)
        ;({ driver, gqlClient, resourceManager } = await getTestTools(config))
        resourceManager.add(
            'User',
            testUsername,
            await ensureLoggedInOrCreateTestUser(driver, gqlClient, {
                username: testUsername,
                deleteIfExists: true,
                ...config,
            })
        )
        resourceManager.add(
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
                    waitForRepos: testRepoSlugs.map(slug => 'github.com/' + slug),
                },
                { ...config, timeout: 2 * 60 * 1000 }
            )
        )
        const user = await getUser(gqlClient, testUsername)
        if (!user) {
            throw new Error(`test user ${testUsername} does not exist`)
        }
        await setUserSiteAdmin(gqlClient, user.id, true)
        screenshots = new ScreenshotVerifier(driver)

        // Ensure precise code intel is disabled for navigation assertions
        resourceManager.add('Global setting', 'codeIntel.lsif', await disableLSIF(driver, gqlClient))
    })

    saveScreenshotsUponFailures(() => driver.page)

    after(async () => {
        if (!config.noCleanup) {
            await resourceManager.destroyAll()
        }
        if (driver) {
            await driver.close()
        }
        if (screenshots.screenshots.length > 0) {
            console.log(screenshots.verificationInstructions())
        }
    })

    test('Basic code intel', async function() {
        this.timeout(30 * 1000)

        await testCodeNavigation(driver, config, [
            {
                repoRev: 'github.com/sourcegraph/sourcegraph@7d557b9cbcaa5d4f612016bddd2f4ef0a7efed25',
                files: [
                    {
                        path: '/cmd/frontend/backend/repos.go',
                        locations: [
                            {
                                line: 46,
                                token: 'Get',
                                precise: false,
                                expectedHoverContains:
                                    'func (s *repos) Get(ctx context.Context, repo api.RepoID) (_ *types.Repo, err error)',
                                expectedDefinition: [
                                    {
                                        url:
                                            '/github.com/sourcegraph/sourcegraph@7d557b9cbcaa5d4f612016bddd2f4ef0a7efed25/-/blob/cmd/frontend/backend/repos.go#L46:17',
                                        precise: false,
                                    },
                                ],
                                expectedReferences: [],
                            },
                            {
                                line: 33,
                                token: 'ErrRepoSeeOther',
                                precise: false,
                                expectedHoverContains:
                                    'ErrRepoSeeOther indicates that the repo does not exist on this server but might exist on an external Sourcegraph server.',
                                expectedDefinition: {
                                    url:
                                        '/github.com/sourcegraph/sourcegraph@7d557b9cbcaa5d4f612016bddd2f4ef0a7efed25/-/blob/cmd/frontend/backend/repos.go#L33:6',
                                    precise: false,
                                },
                                expectedReferences: [
                                    '/cmd/frontend/backend/repos.go#L38:9',
                                    '/cmd/frontend/graphqlbackend/graphqlbackend.go#L290:30',
                                ].map(path => ({
                                    url: `/github.com/sourcegraph/sourcegraph@7d557b9cbcaa5d4f612016bddd2f4ef0a7efed25/-/blob${path}`,
                                    precise: false,
                                })),
                            },
                        ],
                    },
                    {
                        path: '/cmd/frontend/graphqlbackend/git_commit_test.go',
                        locations: [
                            {
                                line: 15,
                                token: 'gitCommitBody',
                                precise: false,
                                expectedHoverContains:
                                    'gitCommitBody returns the contents of the Git commit message after the subject.',
                                expectedDefinition: {
                                    url: '/cmd/frontend/graphqlbackend/git_commit.go#L263:6',
                                    precise: false,
                                },
                                expectedReferences: [
                                    '/cmd/frontend/graphqlbackend/git_commit_test.go#L15:10',
                                    '/cmd/frontend/graphqlbackend/git_commit.go#L93:10',
                                    '/cmd/frontend/graphqlbackend/git_commit.go#L253:4',
                                    '/cmd/frontend/graphqlbackend/git_commit.go#L262:4',
                                ].map(path => ({
                                    url: `/github.com/sourcegraph/sourcegraph@7d557b9cbcaa5d4f612016bddd2f4ef0a7efed25/-/blob${path}`,
                                    precise: false,
                                })),
                            },
                        ],
                    },
                ],
            },
            {
                repoRev:
                    'github.com/sourcegraph/javascript-typescript-langserver@221d798d749fbfc822e1c5bc94bde5a2364f47ea',
                files: [
                    {
                        path: '/src/language-server.ts',
                        locations: [
                            {
                                line: 33,
                                token: 'StdioLogger',
                                precise: false,
                                expectedHoverContains:
                                    'Logger implementation that logs to STDOUT and STDERR depending on level',
                                expectedDefinition: {
                                    url:
                                        '/github.com/sourcegraph/javascript-typescript-langserver@221d798d749fbfc822e1c5bc94bde5a2364f47ea/-/blob/src/logging.ts#L104:14',
                                    precise: false,
                                },
                                expectedReferences: [
                                    '/src/language-server.ts#L4:22',
                                    '/src/language-server.ts#L33:69',
                                    '/src/logging.ts#L104:14',
                                    '/src/server.ts#L7:34',
                                    '/src/server.ts#L27:50',
                                ].map(path => ({
                                    url: `/github.com/sourcegraph/javascript-typescript-langserver@221d798d749fbfc822e1c5bc94bde5a2364f47ea/-/blob${path}`,
                                    precise: false,
                                })),
                            },
                        ],
                    },
                ],
            },
            {
                repoRev: 'github.com/sourcegraph/appdash@ebfcffb1b5c00031ce797183546746715a3cfe87',
                files: [
                    {
                        path: '/python/appdash/recorder.py',
                        locations: [
                            {
                                line: 20,
                                token: 'SpanID',
                                precise: false,
                                expectedHoverContains:
                                    'trace (a 64-bit integer) is the root ID of the tree that contains all of the spans related to this one.',
                                expectedDefinition: {
                                    url:
                                        '/github.com/sourcegraph/appdash@ebfcffb1b5c00031ce797183546746715a3cfe87/-/blob/python/appdash/spanid.py#L34:7',
                                    precise: false,
                                },
                                expectedReferences: [
                                    {
                                        url:
                                            '/github.com/sourcegraph/appdash@ebfcffb1b5c00031ce797183546746715a3cfe87/-/blob/python/appdash/recorder.py#L3:20',
                                        precise: false,
                                    },
                                ],
                            },
                        ],
                    },
                ],
            },
        ])
    })

    test('File sidebar, multiple levels of directories', async function() {
        this.timeout(20 * 1000)
        await driver.page.goto(
            config.sourcegraphBaseUrl + '/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3'
        )
        for (const file of ['cmd', 'frontend', 'auth', 'providers', 'providers.go']) {
            await driver.findElementWithText(file, {
                action: 'click',
                selector: '.e2e-repo-rev-sidebar a',
                wait: { timeout: 2 * 1000 },
            })
        }
        await driver.waitUntilURL(
            `${config.sourcegraphBaseUrl}/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3/-/blob/cmd/frontend/auth/providers/providers.go`,
            { timeout: 2 * 1000 }
        )
    })

    test('Symbols sidebar', async () => {
        await driver.page.goto(
            config.sourcegraphBaseUrl + '/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3'
        )
        await driver.findElementWithText('SYMBOLS', {
            action: 'click',
            selector: '.e2e-repo-rev-sidebar button',
            wait: { timeout: 10 * 1000 },
        })
        await driver.findElementWithText('backgroundEntry', {
            action: 'click',
            selector: '.e2e-repo-rev-sidebar a span',
            wait: { timeout: 2 * 1000 },
        })
        await driver.replaceText({
            selector: 'input[placeholder="Search symbols..."]',
            newText: 'buildentry',
        })
        await driver.page.waitForFunction(
            () => {
                const sidebar = document.querySelector<HTMLElement>('.e2e-repo-rev-sidebar')
                return sidebar && !sidebar.innerText.includes('backgroundEntry')
            },
            {
                timeout: 2 * 1000,
            }
        )
        await driver.findElementWithText('buildEntry', {
            action: 'click',
            selector: '.e2e-repo-rev-sidebar a span',
            wait: { timeout: 2 * 1000 },
        })
        await driver.waitUntilURL(
            `${config.sourcegraphBaseUrl}/github.com/sourcegraph/sourcegraph@c543dfd3936019befe94b881ade89e637d1a3dc3/-/blob/browser/config/webpack/base.config.ts#L6:7-6:17`,
            { timeout: 2 * 1000 }
        )
    })
})
