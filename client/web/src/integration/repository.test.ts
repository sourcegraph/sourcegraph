import assert from 'assert'
import * as path from 'path'

import { subDays } from 'date-fns'
import { afterEach, beforeEach, describe, it } from 'mocha'

import { encodeURIPathComponent } from '@sourcegraph/common'
import {
    RepositoryType,
    type SharedGraphQlOperations,
    type TreeEntriesResult,
} from '@sourcegraph/shared/src/graphql-operations'
import { accessibilityAudit } from '@sourcegraph/shared/src/testing/accessibility'
import { createDriverForTest, type Driver } from '@sourcegraph/shared/src/testing/driver'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'

import {
    DiffHunkLineType,
    type RepositoryContributorsResult,
    type WebGraphQlOperations,
    ExternalServiceKind,
} from '../graphql-operations'

import { createWebIntegrationTestContext, type WebIntegrationTestContext } from './context'
import {
    createResolveRepoRevisionResult,
    createFileExternalLinksResult,
    createTreeEntriesResult,
    createBlobContentResult,
    createRepoChangesetsStatsResult,
    createFileNamesResult,
    createResolveCloningRepoRevisionResult,
    createFileTreeEntriesResult,
} from './graphQlResponseHelpers'
import { commonWebGraphQlResults } from './graphQlResults'
import { createEditorAPI, percySnapshotWithVariants, removeContextFromQuery } from './utils'

export const getCommonRepositoryGraphQlResults = (
    repositoryName: string,
    repositoryUrl: string,
    fileEntries: string[] = []
): Partial<WebGraphQlOperations & SharedGraphQlOperations> => ({
    ...commonWebGraphQlResults,
    RepoChangesetsStats: () => createRepoChangesetsStatsResult(),
    ResolveRepoRev: () => createResolveRepoRevisionResult(repositoryName),
    FileNames: () => createFileNamesResult(),
    FileExternalLinks: ({ filePath }) => createFileExternalLinksResult(filePath),
    TreeEntries: () => createTreeEntriesResult(repositoryUrl, fileEntries),
    FileTreeEntries: () => createFileTreeEntriesResult(repositoryUrl, fileEntries),
    Blob: ({ filePath }) => createBlobContentResult(`content for: ${filePath}\nsecond line\nthird line`),
})

const now = new Date()
describe('Repository', () => {
    let driver: Driver
    before(async () => {
        driver = await createDriverForTest()
    })
    after(() => driver?.close())
    let testContext: WebIntegrationTestContext
    beforeEach(async function () {
        testContext = await createWebIntegrationTestContext({
            driver,
            currentTest: this.currentTest!,
            directory: __dirname,
        })
    })
    afterEachSaveScreenshotIfFailed(() => driver.page)
    afterEach(() => testContext?.dispose())

    async function assertSelectorHasText(selector: string, text: string) {
        assert.strictEqual(
            await driver.page.evaluate(
                selector => document.querySelector<HTMLButtonElement>(selector)?.textContent,
                selector
            ),
            text
        )
    }

    describe('index page', () => {
        it('loads when accessed with a repo url', async () => {
            const shortRepositoryName = 'sourcegraph/jsonrpc2'
            const repositoryName = `github.com/${shortRepositoryName}`
            const repositorySourcegraphUrl = `/${repositoryName}`
            const commitUrl = `${repositorySourcegraphUrl}/-/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81?visible=1`
            const clickedFileName = 'async.go'
            const clickedCommit = ''
            const fileEntries = ['jsonrpc2.go', clickedFileName]

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, fileEntries),
                FileCommits: () => ({
                    repository: {
                        __typename: 'Repository',
                        id: repositoryName,
                        commit: {
                            __typename: 'GitCommit',
                            id: '1',
                            tree: {
                                __typename: 'GitTree',
                                entries: fileEntries.map((fileName, index) => ({
                                    __typename: 'GitBlob',
                                    path: fileName,
                                    history: {
                                        __typename: 'TreeEntryConnection',
                                        nodes: [
                                            {
                                                __typename: 'GitBlob',
                                                commit: {
                                                    __typename: 'GitCommit',
                                                    id: '1',
                                                    author: {
                                                        __typename: 'Signature',
                                                        person: {
                                                            avatarURL: '',
                                                            name: 'Sourcegraph Bot',
                                                            email: 'batch-changes@sourcegraph.com',
                                                            displayName: 'Sourcegraph Bot',
                                                            user: null,
                                                        },
                                                        date: '2020-04-29T16:57:20Z',
                                                    },
                                                    committer: {
                                                        __typename: 'Signature',
                                                        person: {
                                                            avatarURL: '',
                                                            name: 'Sourcegraph Bot',
                                                            email: 'batch-changes@sourcegraph.com',
                                                            displayName: 'Sourcegraph Bot',
                                                            user: null,
                                                        },
                                                        date: '2020-04-29T16:57:20Z',
                                                    },
                                                    subject: 'update LSIF indexing CI workflow',
                                                    canonicalURL: commitUrl,
                                                    externalURLs: [
                                                        {
                                                            url: 'https://github.com/sourcegraph/jsonrpc2/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                                            serviceKind: ExternalServiceKind.GITHUB,
                                                        },
                                                    ],
                                                },
                                            },
                                        ],
                                    },
                                })),
                            },
                        },
                    },
                }),

                RepositoryCommit: () => ({
                    node: {
                        __typename: 'Repository',
                        sourceType: RepositoryType.GIT_REPOSITORY,
                        commit: {
                            __typename: 'GitCommit',
                            id: 'CommitID1',
                            oid: '15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                            abbreviatedOID: '15c2290',
                            perforceChangelist: null,
                            message: 'update LSIF indexing CI workflow\n',
                            subject: 'update LSIF indexing CI workflow',
                            body: null,
                            author: {
                                __typename: 'Signature',
                                person: {
                                    __typename: 'Person',
                                    avatarURL: '',
                                    name: 'garo (they/them)',
                                    email: 'gbrik@users.noreply.github.com',
                                    displayName: 'garo (they/them)',
                                    user: null,
                                },
                                date: '2020-04-29T18:40:54Z',
                            },
                            committer: {
                                __typename: 'Signature',
                                person: {
                                    __typename: 'Person',
                                    avatarURL: '',
                                    name: 'GitHub',
                                    email: 'noreply@github.com',
                                    displayName: 'GitHub',
                                    user: null,
                                },
                                date: '2020-04-29T18:40:54Z',
                            },
                            parents: [
                                {
                                    __typename: 'GitCommit',
                                    oid: '96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                    abbreviatedOID: '96c4efa',
                                    perforceChangelist: null,
                                    url: '/github.com/sourcegraph/jsonrpc2/-/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                },
                                {
                                    __typename: 'GitCommit',
                                    oid: '9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                    abbreviatedOID: '9e615b1',
                                    perforceChangelist: null,
                                    url: '/github.com/sourcegraph/jsonrpc2/-/commit/9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                },
                            ],
                            url: commitUrl,
                            canonicalURL: commitUrl,
                            externalURLs: [
                                {
                                    __typename: 'ExternalLink',
                                    url: 'https://github.com/sourcegraph/jsonrpc2/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                    serviceKind: ExternalServiceKind.GITHUB,
                                },
                            ],
                            tree: {
                                __typename: 'GitTree',
                                canonicalURL:
                                    '/github.com/sourcegraph/jsonrpc2@15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                            },
                        },
                    },
                }),
                RepositoryComparisonDiff: () => ({
                    node: {
                        __typename: 'Repository',
                        comparison: {
                            fileDiffs: {
                                nodes: [
                                    {
                                        __typename: 'FileDiff',
                                        oldPath: '.github/workflows/lsif.yml',
                                        oldFile: { __typename: 'GitBlob', binary: false, byteSize: 381 },
                                        newFile: { __typename: 'GitBlob', binary: false, byteSize: 304 },
                                        newPath: '.github/workflows/lsif.yml',
                                        mostRelevantFile: {
                                            __typename: 'GitBlob',
                                            url: '/github.com/sourcegraph/jsonrpc2@15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81/-/blob/.github/workflows/lsif.yml',
                                            changelistURL: '',
                                        },
                                        hunks: [
                                            {
                                                oldRange: { startLine: 2, lines: 15 },
                                                oldNoNewlineAt: false,
                                                newRange: { startLine: 2, lines: 12 },
                                                section: 'name: LSIF',
                                                highlight: {
                                                    aborted: false,
                                                    lines: [
                                                        {
                                                            kind: DiffHunkLineType.DELETED,
                                                            html: '<div><span style="color:#657b83;">  </span><span style="color:#268bd2;">build</span><span style="color:#657b83;">:\n</span></div>',
                                                        },
                                                        {
                                                            kind: DiffHunkLineType.ADDED,
                                                            html: '<div><span style="color:#657b83;">  </span><span style="color:#268bd2;">lsif-go</span><span style="color:#657b83;">:\n</span></div>',
                                                        },
                                                    ],
                                                },
                                            },
                                        ],
                                        stat: { added: 1, changed: 3, deleted: 4 },
                                        internalID: '084bcb27838a8adbbbe10f664420f2d2',
                                    },
                                ],
                                totalCount: 1,
                                pageInfo: { endCursor: null, hasNextPage: false },
                                diffStat: { added: 1, changed: 3, deleted: 4, __typename: 'DiffStat' },
                            },
                        },
                    },
                }),
            })

            // Mock `Date.now` to stabilize timestamps
            await driver.page.evaluateOnNewDocument(() => {
                // Number of ms between Unix epoch and July 1, 2020 (arbitrary)
                const mockMs = new Date('July 1, 2020 00:00:00 UTC').getTime()
                Date.now = () => mockMs
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + '/' + repositoryName)

            await driver.page.waitForSelector('div.test-tree-page-title')

            // Assert that the directory listing displays properly
            await driver.page.waitForSelector('.test-tree-entries')

            // TODO: Reenable later, percy is erroring out on remote images not loading.
            // await percySnapshotWithVariants(driver.page, 'Repository index page')
            await accessibilityAudit(driver.page)

            const numberOfFileEntries = await driver.page.evaluate(
                () => document.querySelectorAll<HTMLButtonElement>('.test-tree-entry-file')?.length
            )

            assert.strictEqual(numberOfFileEntries, fileEntries.length, 'Number of files in directory listing')

            await testContext.waitForGraphQLRequest(async () => {
                await driver.findElementWithText(clickedFileName, {
                    selector: '.test-tree-entry-file',
                    action: 'click',
                })
            }, 'Blob')

            await driver.page.waitForSelector('[data-testid="repo-blob"]')
            await driver.assertWindowLocation(`/${repositoryName}/-/blob/${clickedFileName}`)

            // Assert breadcrumb order
            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', `${clickedFileName}`])

            // Return to repo page
            await driver.page.waitForSelector('.test-repo-header-repo-link')
            await driver.page.click('.test-repo-header-repo-link')

            await driver.page.waitForSelector('div.test-tree-page-title')
            await assertSelectorHasText('div.test-tree-page-title', shortRepositoryName)
            await driver.assertWindowLocation(repositorySourcegraphUrl)

            await driver.findElementWithText(clickedCommit, {
                selector: '[data-testid="git-commit-message-with-links"] a',
                action: 'click',
            })
            await driver.page.waitForSelector('[data-testid="repository-commit-page"]')
            await driver.page.waitForSelector('[data-testid="git-commit-node-message-subject"]')
            await driver.assertWindowLocation(commitUrl)

            await assertSelectorHasText(
                '[data-testid="git-commit-node-message-subject"]',
                'update LSIF indexing CI workflow'
            )
        })

        it('works with files with spaces in the name', async () => {
            const shortRepositoryName = 'ggilmore/q-test'
            const repositoryName = `github.com/${shortRepositoryName}`
            const repositorySourcegraphUrl = `/${repositoryName}`
            const fileName = '% token.4288249258.sql'
            const directoryName = "Geoffrey's random queries.32r242442bf"
            const filePath = path.posix.join(directoryName, fileName)

            const TreeEntries: TreeEntriesResult = {
                repository: {
                    id: 'test-repo-id',
                    commit: {
                        tree: {
                            isRoot: false,
                            url: '/github.com/ggilmore/q-test/-/tree/Geoffrey%27s%20random%20queries.32r242442bf',
                            entries: [
                                {
                                    __typename: 'GitBlob',
                                    languages: ['SQL'],
                                    name: fileName,
                                    path: filePath,
                                    isDirectory: false,
                                    url: '/github.com/ggilmore/q-test/-/blob/Geoffrey%27s%20random%20queries.32r242442bf/%25%20token.4288249258.sql',
                                    submodule: null,
                                },
                            ],
                        },
                    },
                },
            }

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl),
                FileExternalLinks: ({ filePath, repoName, revision }) =>
                    createFileExternalLinksResult(
                        `https://${encodeURIPathComponent(repoName)}/blob/${encodeURIPathComponent(
                            revision
                        )}/${encodeURIPathComponent(filePath)}`
                    ),
                TreeEntries: () => TreeEntries,
                FileTreeEntries: () => TreeEntries,
            })

            await driver.page.goto(
                `${driver.sourcegraphBaseUrl}/github.com/ggilmore/q-test/-/tree/Geoffrey's%20random%20queries.32r242442bf`
            )
            await driver.page.waitForSelector('.test-tree-file-link')
            assert.strictEqual(
                await driver.page.evaluate(
                    () =>
                        document.querySelector(
                            '.test-tree-file-link[href="/github.com/ggilmore/q-test/-/blob/Geoffrey%27s%20random%20queries.32r242442bf/%25%20token.4288249258.sql"]'
                        )?.textContent
                ),
                fileName
            )

            // page.click() fails for some reason with Error: Node is either not visible or not an HTMLElement
            await driver.page.$eval(
                '.test-tree-file-link[href="/github.com/ggilmore/q-test/-/blob/Geoffrey%27s%20random%20queries.32r242442bf/%25%20token.4288249258.sql"]',
                linkElement => (linkElement as HTMLElement).click()
            )
            await driver.page.waitForSelector('[data-testid="repo-blob"]')

            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )

            assert.deepStrictEqual(breadcrumbTexts, [
                shortRepositoryName,
                '@master',
                "Geoffrey's random queries.32r242442bf/% token.4288249258.sql",
            ])

            {
                const queryInput = await createEditorAPI(driver, '.test-query-input')
                assert.strictEqual(
                    removeContextFromQuery((await queryInput.getValue()) ?? ''),
                    "repo:^github\\.com/ggilmore/q-test$ file:^Geoffrey's\\ random\\ queries\\.32r242442bf/%\\ token\\.4288249258\\.sql"
                )
            }

            await driver.page.waitForSelector('.test-go-to-code-host')
            assert.strictEqual(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLAnchorElement>('.test-go-to-code-host')?.href
                ),
                "https://github.com/ggilmore/q-test/blob/master/Geoffrey's%20random%20queries.32r242442bf/%25%20token.4288249258.sql"
            )

            const blobContent = await driver.page.evaluate(() =>
                [...document.querySelectorAll('[data-testid="repo-blob"] .cm-line')]
                    .map(line => line.textContent)
                    .join('\n')
            )
            // CodeMirror blob content has no newline characters
            const expectedBlobContent = `content for: ${filePath}\nsecond line\nthird line`
            assert.strictEqual(blobContent, expectedBlobContent)
        })

        it('works with a plus sign in the repository name', async () => {
            const shortRepositoryName = 'ubuntu/+source/quemu'
            const repositoryName = `github.com/${shortRepositoryName}`
            const repositorySourcegraphUrl = `/${shortRepositoryName}`

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, ['readme.md']),
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)

            await driver.page.waitForSelector('div.test-tree-page-title')
            await assertSelectorHasText('div.test-tree-page-title', shortRepositoryName)
            await assertSelectorHasText('.test-tree-entry-file', 'readme.md')

            {
                const queryInput = await createEditorAPI(driver, '.test-query-input')
                assert.strictEqual(
                    removeContextFromQuery((await queryInput.getValue()) ?? ''),
                    'repo:^ubuntu/\\+source/quemu$ '
                ) // + should be escaped in regular expression
            }

            // page.click() fails for some reason with Error: Node is either not visible or not an HTMLElement
            await driver.page.$eval('.test-tree-file-link', linkElement => (linkElement as HTMLElement).click())
            await driver.page.waitForSelector('[data-testid="repo-blob"]')

            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', 'readme.md'])
        })

        it('shows repo cloning in progress page', async () => {
            const shortRepositoryName = 'sourcegraph/jsonrpc2'
            const repositoryName = `github.com/${shortRepositoryName}`
            const repositorySourcegraphUrl = `/${repositoryName}`

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, ['readme.md']),
                // Return cloning error instead of the successful GraphQL result here.
                ResolveRepoRev: () => createResolveCloningRepoRevisionResult(repositoryName),
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)

            // Wait for clone in progress message before Percy snapshot.
            await driver.page.waitForSelector('[data-testid="hero-page-subtitle"]')
            // Verify that we show the respective message in the UI.
            await assertSelectorHasText('[data-testid="hero-page-subtitle"]', 'Cloning in progress')

            await percySnapshotWithVariants(driver.page, 'Repository cloning in progress page')
        })

        it('works with spaces in the repository name', async () => {
            const shortRepositoryName = 'my org/repo with spaces'
            const repositoryName = `github.com/${shortRepositoryName}`
            const repositorySourcegraphUrl = '/github.com/my%20org/repo%20with%20spaces'

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, ['readme.md']),
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)

            await driver.page.waitForSelector('div.test-tree-page-title')
            await assertSelectorHasText('div.test-tree-page-title', 'my org/repo with spaces')
            await assertSelectorHasText('.test-tree-entry-file', 'readme.md')

            // page.click() fails for some reason with Error: Node is either not visible or not an HTMLElement
            await driver.page.$eval('.test-tree-file-link', linkElement => (linkElement as HTMLElement).click())
            await driver.page.waitForSelector('[data-testid="repo-blob"]')

            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', 'readme.md'])
        })
    })

    describe('commits page', () => {
        it('loads commits of a repository', async () => {
            const shortRepositoryName = 'sourcegraph/sourcegraph'
            const repositoryName = `github.com/${shortRepositoryName}`
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ResolveRepoRev: () => createResolveRepoRevisionResult(repositoryName),
                RepositoryGitCommits: () => ({
                    __typename: 'Query',
                    node: {
                        __typename: 'Repository',
                        sourceType: RepositoryType.GIT_REPOSITORY,
                        externalURLs: [
                            {
                                __typename: 'ExternalLink',
                                serviceKind: ExternalServiceKind.GITHUB,
                                url: 'https://' + repositoryName,
                            },
                        ],
                        commit: {
                            __typename: 'GitCommit',
                            ancestors: {
                                __typename: 'GitCommitConnection',
                                nodes: [
                                    {
                                        __typename: 'GitCommit',
                                        id: 'R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3hORGs9IiwiYyI6IjI4NGFiYTAyNGIxYjU1ODU5MGU4ZTJmOTdkYmMzNTUzYTVlMGM3NmIifQ==',
                                        oid: '284aba024b1b558590e8e2f97dbc3553a5e0c76b',
                                        abbreviatedOID: '284aba0',
                                        perforceChangelist: null,
                                        message: 'sg: create a test command to run e2e tests locally (#34627)\n',
                                        subject: 'sg: create a test command to run e2e tests locally (#34627)',
                                        body: null,
                                        author: {
                                            __typename: 'Signature',
                                            person: {
                                                __typename: 'Person',
                                                avatarURL: null,
                                                name: 'Jean-Hadrien Chabran',
                                                email: 'jr9@gmail.com',
                                                displayName: 'Jean-Hadrien Chabran',
                                                user: null,
                                            },
                                            date: subDays(now, 5).toISOString(),
                                        },
                                        committer: {
                                            __typename: 'Signature',
                                            person: {
                                                __typename: 'Person',
                                                avatarURL: null,
                                                name: 'GitHub',
                                                email: 'noreply@yahoo.com',
                                                displayName: 'GitHub',
                                                user: null,
                                            },
                                            date: subDays(now, 5).toISOString(),
                                        },
                                        parents: [
                                            {
                                                oid: 'a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                                abbreviatedOID: 'a2d1fd4',
                                                perforceChangelist: null,
                                                url: '/github.com/sourcegraph/sourcegraph/-/commit/a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                            },
                                        ],
                                        url: '/github.com/sourcegraph/sourcegraph/-/commit/284aba024b1b558590e8e2f97dbc3553a5e0c76b',
                                        canonicalURL:
                                            '/github.com/sourcegraph/sourcegraph/-/commit/284aba024b1b558590e8e2f97dbc3553a5e0c76b',
                                        externalURLs: [
                                            {
                                                url: 'https://github.com/sourcegraph/sourcegraph/commit/284aba024b1b558590e8e2f97dbc3553a5e0c76b',
                                                serviceKind: ExternalServiceKind.GITHUB,
                                            },
                                        ],
                                        tree: {
                                            canonicalURL:
                                                '/github.com/sourcegraph/sourcegraph@284aba024b1b558590e8e2f97dbc3553a5e0c76b',
                                        },
                                    },
                                    {
                                        __typename: 'GitCommit',
                                        id: 'R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3hORGs9IiwiYyI6ImEyZDFmZDQ3NGQ3OWRjMjlhZjZjN2I0YzMzZjAyZmUyMjI4N2JkMTEifQ==',
                                        oid: 'a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                        abbreviatedOID: 'a2d1fd4',
                                        perforceChangelist: null,
                                        message:
                                            'Wildcard V2: <Checkbox /> migration (#34324)\n\nCo-authored-by: gitstart-sourcegraph <gitstart@users.noreply.github.com>',
                                        subject: 'Wildcard V2: <Checkbox /> migration (#34324)',
                                        body: 'Co-authored-by: gitstart-sourcegraph <gitstart@users.noreply.github.com>',
                                        author: {
                                            __typename: 'Signature',
                                            person: {
                                                __typename: 'Person',
                                                avatarURL: null,
                                                name: 'GitStart-SourceGraph',
                                                email: '89894075h@facebook.net',
                                                displayName: 'GitStart-SourceGraph',
                                                user: null,
                                            },
                                            date: subDays(now, 5).toISOString(),
                                        },
                                        committer: {
                                            __typename: 'Signature',
                                            person: {
                                                __typename: 'Person',
                                                avatarURL: null,
                                                name: 'GitHub',
                                                email: 'google@yahoo.com',
                                                displayName: 'GitHub',
                                                user: null,
                                            },
                                            date: subDays(now, 5).toISOString(),
                                        },
                                        parents: [
                                            {
                                                oid: '3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                                abbreviatedOID: '3a163b9',
                                                perforceChangelist: null,
                                                url: '/github.com/sourcegraph/sourcegraph/-/commit/3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                            },
                                        ],
                                        url: '/github.com/sourcegraph/sourcegraph/-/commit/a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                        canonicalURL:
                                            '/github.com/sourcegraph/sourcegraph/-/commit/a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                        externalURLs: [
                                            {
                                                url: 'https://github.com/sourcegraph/sourcegraph/commit/a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                                serviceKind: ExternalServiceKind.GITHUB,
                                            },
                                        ],
                                        tree: {
                                            canonicalURL:
                                                '/github.com/sourcegraph/sourcegraph@a2d1fd474d79dc29af6c7b4c33f02fe22287bd11',
                                        },
                                    },
                                    {
                                        __typename: 'GitCommit',
                                        id: 'R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3hORGs9IiwiYyI6IjNhMTYzYjkyYjVjNDU5MjFmYmM3MzBmZjIwNDdmZjdlNjBkODY4OWIifQ==',
                                        oid: '3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                        abbreviatedOID: '3a163b9',
                                        perforceChangelist: null,
                                        message: 'web: ban `reactstrap` imports (#34881)\n',
                                        subject: 'web: ban `reactstrap` imports (#34881)',
                                        body: null,
                                        author: {
                                            __typename: 'Signature',
                                            person: {
                                                __typename: 'Person',
                                                avatarURL: null,
                                                name: 'Valery Bugakov',
                                                email: 'user23@gmail.com',
                                                displayName: 'Valery Bugakov',
                                                user: null,
                                            },
                                            date: subDays(now, 5).toISOString(),
                                        },
                                        committer: {
                                            __typename: 'Signature',
                                            person: {
                                                __typename: 'Person',
                                                avatarURL: null,
                                                name: 'GitHub',
                                                email: 'user43@gmail.com',
                                                displayName: 'GitHub',
                                                user: null,
                                            },
                                            date: subDays(now, 5).toISOString(),
                                        },
                                        parents: [
                                            {
                                                oid: 'e2e91f0dcdc90811c4f2f4df638bc459b2358e7d',
                                                abbreviatedOID: 'e2e91f0',
                                                perforceChangelist: null,
                                                url: '/github.com/sourcegraph/sourcegraph/-/commit/e2e91f0dcdc90811c4f2f4df638bc459b2358e7d',
                                            },
                                        ],
                                        url: '/github.com/sourcegraph/sourcegraph/-/commit/3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                        canonicalURL:
                                            '/github.com/sourcegraph/sourcegraph/-/commit/3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                        externalURLs: [
                                            {
                                                url: 'https://github.com/sourcegraph/sourcegraph/commit/3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                                serviceKind: ExternalServiceKind.GITHUB,
                                            },
                                        ],
                                        tree: {
                                            canonicalURL:
                                                '/github.com/sourcegraph/sourcegraph@3a163b92b5c45921fbc730ff2047ff7e60d8689b',
                                        },
                                    },
                                ],
                                pageInfo: {
                                    __typename: 'PageInfo',
                                    hasNextPage: true,
                                    endCursor: 'abc',
                                },
                            },
                        },
                    },
                }),
                FileNames: () => ({
                    repository: {
                        id: 'repo-123',
                        __typename: 'Repository',
                        commit: {
                            id: 'c0ff33',
                            __typename: 'GitCommit',
                            fileNames: ['README.md'],
                        },
                    },
                }),
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/github.com/sourcegraph/sourcegraph/-/commits')
            await driver.page.waitForSelector('[data-testid="commits-page"]', { visible: true })
            await driver.page.waitForSelector('.list-group-item', { visible: true })
            await percySnapshotWithVariants(driver.page, 'Repository commits page')
            await accessibilityAudit(driver.page)
        })
    })

    describe('Accessibility', () => {
        const shortRepositoryName = 'sourcegraph/sourcegraph'
        const repositoryName = `github.com/${shortRepositoryName}`

        describe('Contributors page', () => {
            const repositorySourcegraphUrl = `/${repositoryName}/-/stats/contributors`

            it('Should render correctly all contributors', async () => {
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, []),
                    RepositoryContributors: (): RepositoryContributorsResult => ({
                        node: {
                            contributors: {
                                nodes: [
                                    {
                                        person: {
                                            name: 'alice',
                                            displayName: 'alice',
                                            email: 'alice@sourcegraph.test',
                                            avatarURL: null,
                                            user: null,
                                            __typename: 'Person',
                                        },
                                        count: 1,
                                        commits: {
                                            nodes: [
                                                {
                                                    oid: '1'.repeat(40),
                                                    abbreviatedOID: '1'.repeat(7),
                                                    url: `/${repositoryName}/-/commit/${'1'.repeat(40)}`,
                                                    subject: 'Commit message 1',
                                                    author: { date: subDays(new Date(), 1).toISOString() },
                                                    __typename: 'GitCommit',
                                                },
                                            ],
                                            __typename: 'GitCommitConnection',
                                        },
                                        __typename: 'RepositoryContributor',
                                    },
                                    {
                                        person: {
                                            name: 'jack',
                                            displayName: 'jack',
                                            email: 'jack@sourcegraph.test',
                                            avatarURL: null,
                                            user: null,
                                            __typename: 'Person',
                                        },
                                        count: 1,
                                        commits: {
                                            nodes: [
                                                {
                                                    oid: '2'.repeat(40),
                                                    abbreviatedOID: '2'.repeat(7),
                                                    url: `/${repositoryName}/-/commit/${'2'.repeat(40)}`,
                                                    subject: 'Commit message 2',
                                                    author: { date: subDays(new Date(), 2).toISOString() },
                                                    __typename: 'GitCommit',
                                                },
                                            ],
                                            __typename: 'GitCommitConnection',
                                        },
                                        __typename: 'RepositoryContributor',
                                    },
                                    {
                                        person: {
                                            name: 'jill',
                                            displayName: 'jill',
                                            email: 'jill@sourcegraph.test',
                                            avatarURL: null,
                                            user: null,
                                            __typename: 'Person',
                                        },
                                        count: 1,
                                        commits: {
                                            nodes: [
                                                {
                                                    oid: '3'.repeat(40),
                                                    abbreviatedOID: '3'.repeat(7),
                                                    url: `/${repositoryName}/-/commit/${'3'.repeat(40)}`,
                                                    subject: 'Commit message 3',
                                                    author: { date: subDays(new Date(), 3).toISOString() },
                                                    __typename: 'GitCommit',
                                                },
                                            ],
                                            __typename: 'GitCommitConnection',
                                        },
                                        __typename: 'RepositoryContributor',
                                    },
                                ],
                                totalCount: 3,
                                pageInfo: {
                                    hasNextPage: false,
                                    hasPreviousPage: false,
                                    startCursor: 'abc',
                                    endCursor: 'def',
                                    __typename: 'BidirectionalPageInfo',
                                },
                                __typename: 'RepositoryContributorConnection',
                            },
                            __typename: 'Repository',
                        },
                    }),
                })
                await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)
                await driver.page.waitForSelector('.test-filtered-contributors-connection')
                await percySnapshotWithVariants(driver.page, 'Contributor list')
                await accessibilityAudit(driver.page)
            })
        })

        describe('Branches page', () => {
            it('should render correctly branches', async () => {
                const repositorySourcegraphUrl = `/${repositoryName}/-/branches`
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, []),
                    RepositoryGitBranchesOverview: () => ({
                        node: {
                            defaultBranch: {
                                id: 'QmV3b2Q=',
                                displayName: 'main',
                                name: 'refs/heads/main',
                                abbrevName: 'main',
                                url: `/${repositoryName}/-/branches/${'1'.repeat(40)}`,
                                target: {
                                    commit: {
                                        author: {
                                            __typename: 'Signature',
                                            person: {
                                                displayName: 'John Doe',
                                                user: {
                                                    username: 'johndoe',
                                                },
                                            },
                                            date: subDays(new Date(), 3).toISOString(),
                                        },
                                        committer: {
                                            __typename: 'Signature',
                                            person: {
                                                displayName: 'John Doe',
                                                user: null,
                                            },
                                            date: subDays(new Date(), 1).toISOString(),
                                        },
                                        behindAhead: {
                                            behind: 0,
                                            ahead: 0,
                                        },
                                    },
                                },
                            },
                            gitRefs: {
                                pageInfo: { hasNextPage: false },
                                nodes: [
                                    {
                                        id: 'BranchId1',
                                        displayName: 'integration-tests-trigramming',
                                        name: 'refs/heads/integration-tests-trigramming',
                                        abbrevName: 'integration-tests-trigramming',
                                        url: `/${repositoryName}/-/branches/${'1'.repeat(40)}`,
                                        target: {
                                            commit: {
                                                author: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'John Doe',
                                                        user: {
                                                            username: 'johndoe',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                committer: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'John Doe',
                                                        user: {
                                                            username: 'johndoe',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                behindAhead: {
                                                    behind: 12633,
                                                    ahead: 1,
                                                },
                                            },
                                        },
                                    },
                                    {
                                        id: 'BranchId2',
                                        displayName: 'integration-tests-quadgramming',
                                        name: 'refs/heads/integration-tests-quadgramming',
                                        abbrevName: 'integration-tests-quadgramming',
                                        url: `/${repositoryName}/-/branches/${'1'.repeat(40)}`,
                                        target: {
                                            commit: {
                                                author: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'Alice',
                                                        user: {
                                                            username: 'alice',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                committer: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'Alice',
                                                        user: {
                                                            username: 'alice',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                behindAhead: {
                                                    behind: 12633,
                                                    ahead: 1,
                                                },
                                            },
                                        },
                                    },
                                ],
                            },
                        },
                    }),
                })

                await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)
                await driver.page.waitForSelector('[data-testid="active-branches-list"]')
                await percySnapshotWithVariants(driver.page, 'Repository branches page')
                await accessibilityAudit(driver.page)
            })
        })

        describe('Tags page', () => {
            const repositorySourcegraphUrl = `/${repositoryName}/-/tags`
            it('should render correctly tags list page', async () => {
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, []),
                    RepositoryGitRefs: () => ({
                        node: {
                            __typename: 'Repository',
                            gitRefs: {
                                __typename: 'GitRefConnection',
                                nodes: [
                                    {
                                        __typename: 'GitRef',
                                        name: 'refs/heads/main',
                                        abbrevName: 'v3.39.1',
                                        displayName: 'v3.39.1',
                                        id: 'GitRef:refs/heads/main',
                                        url: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                        target: {
                                            commit: {
                                                author: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'John Doe',
                                                        user: {
                                                            username: 'johndoe',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                committer: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'John Doe',
                                                        user: {
                                                            username: 'johndoe',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                behindAhead: {
                                                    __typename: 'BehindAheadCounts',
                                                    ahead: 0,
                                                    behind: 0,
                                                },
                                            },
                                        },
                                    },
                                    {
                                        __typename: 'GitRef',
                                        name: 'refs/heads/mai2n',
                                        abbrevName: 'v3.39.2',
                                        displayName: 'v3.39.2',
                                        id: 'GitRef:refs/heads/main2',
                                        url: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                        target: {
                                            commit: {
                                                author: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'Alice',
                                                        user: {
                                                            username: 'alice',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                committer: {
                                                    __typename: 'Signature',
                                                    person: {
                                                        displayName: 'Alice',
                                                        user: {
                                                            username: 'alice',
                                                        },
                                                    },
                                                    date: subDays(new Date(), 1).toISOString(),
                                                },
                                                behindAhead: {
                                                    __typename: 'BehindAheadCounts',
                                                    ahead: 0,
                                                    behind: 0,
                                                },
                                            },
                                        },
                                    },
                                ],
                                totalCount: 2,
                                pageInfo: {
                                    hasNextPage: false,
                                },
                            },
                        },
                    }),
                })
                await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)
                await driver.page.waitForSelector('.test-filtered-tags-connection')
                await driver.page.click('input[name="query"]')
                await driver.page.waitForSelector('input[name="query"].focus-visible')
                await percySnapshotWithVariants(driver.page, 'Repository tags page')
                await accessibilityAudit(driver.page)
            })
        })

        describe('Compare page', () => {
            const repositorySourcegraphUrl = `/${repositoryName}/-/compare/main...bl/readme?visible=1`
            it('should render correctly compare page, including diff view', async () => {
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...getCommonRepositoryGraphQlResults(repositoryName, repositorySourcegraphUrl, []),
                    RepositoryComparison: () => ({
                        node: {
                            comparison: {
                                range: {
                                    expr: 'main...bl/readme',
                                    baseRevSpec: { object: { oid: '1'.repeat(40) } },
                                    headRevSpec: { object: { oid: '2'.repeat(40) } },
                                },
                            },
                        },
                    }),
                    RepositoryComparisonCommits: () => ({
                        node: {
                            sourceType: RepositoryType.GIT_REPOSITORY,
                            comparison: {
                                commits: {
                                    nodes: [
                                        {
                                            id: '1'.repeat(70),
                                            oid: '1'.repeat(40),
                                            abbreviatedOID: '1'.repeat(7),
                                            perforceChangelist: null,
                                            message: 'update README',
                                            subject: 'update README',
                                            body: null,
                                            author: {
                                                person: {
                                                    avatarURL: null,
                                                    name: 'alice',
                                                    email: 'alice@sourcegraph.com',
                                                    displayName: 'alice',
                                                    user: {
                                                        id: '1'.repeat(70),
                                                        username: 'alice',
                                                        url: '/users/alice',
                                                        displayName: 'alice',
                                                    },
                                                },
                                                date: subDays(new Date(), 1).toISOString(),
                                            },
                                            committer: {
                                                person: {
                                                    avatarURL: null,
                                                    name: 'alice',
                                                    email: 'alice@sourcegraph.com',
                                                    displayName: 'alice',
                                                    user: {
                                                        id: '1'.repeat(70),
                                                        username: 'alice',
                                                        url: '/users/alice',
                                                        displayName: 'alice',
                                                    },
                                                },
                                                date: subDays(new Date(), 1).toISOString(),
                                            },
                                            parents: [
                                                {
                                                    oid: '2'.repeat(40),
                                                    abbreviatedOID: '2'.repeat(7),
                                                    perforceChangelist: null,
                                                    url: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                                },
                                            ],
                                            url: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                            canonicalURL: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                            externalURLs: [
                                                {
                                                    url: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                                    serviceKind: ExternalServiceKind.GITHUB,
                                                },
                                            ],
                                            tree: {
                                                canonicalURL: '/github.com/sourcegraph/sourcegraph@2'.repeat(70),
                                            },
                                        },
                                    ],
                                    pageInfo: { hasNextPage: false },
                                },
                            },
                        },
                    }),
                    RepositoryComparisonDiff: () => ({
                        node: {
                            __typename: 'Repository',
                            comparison: {
                                fileDiffs: {
                                    nodes: [
                                        {
                                            oldPath: 'README.md',
                                            oldFile: { __typename: 'GitBlob', binary: false, byteSize: 2262 },
                                            newFile: { __typename: 'GitBlob', binary: false, byteSize: 3083 },
                                            newPath: 'README.md',
                                            mostRelevantFile: {
                                                __typename: 'GitBlob',
                                                url: `/${repositoryName}/-/commit/${'1'.repeat(40)}`,
                                                changelistURL: '',
                                            },
                                            hunks: [
                                                {
                                                    oldRange: { startLine: 3, lines: 47 },
                                                    oldNoNewlineAt: false,
                                                    newRange: { startLine: 3, lines: 64 },
                                                    section: null,
                                                    highlight: {
                                                        aborted: false,
                                                        lines: [
                                                            {
                                                                kind: DiffHunkLineType.UNCHANGED,
                                                                html: '\u003Cdiv\u003E\u003Cspan class="hl-text hl-html hl-markdown"\u003E[![build](https://badge.buildkite.com/00bbe6fa9986c78b8e8591cffeb0b0f2e8c4bb610d7e339ff6.svg?branch=master)](https://buildkite.com/sourcegraph/sourcegraph)\n\u003C/span\u003E\u003C/div\u003E',
                                                            },
                                                            {
                                                                kind: DiffHunkLineType.DELETED,
                                                                html: '\u003Cdiv\u003E\u003Cspan class="hl-text hl-html hl-markdown"\u003E[![build](https://badge.buildkite.com/00bbe6fa9986c78b8e8591cffeb0b0f2e8c4bb610d7e339ff6.svg?branch=master)](https://buildkite.com/sourcegraph/sourcegraph)\n\u003C/span\u003E\u003C/div\u003E',
                                                            },
                                                            {
                                                                kind: DiffHunkLineType.ADDED,
                                                                html: '\u003Cdiv\u003E\u003Cspan class="hl-text hl-html hl-markdown"\u003E[![build](https://badge.buildkite.com/00bbe6fa9986c78b8e8591cffeb0b0f2e8c4bb610d7e339ff6.svg?branch=master)](https://buildkite.com/sourcegraph/sourcegraph)\n\u003C/span\u003E\u003C/div\u003E',
                                                            },
                                                        ],
                                                    },
                                                },
                                            ],
                                            stat: { added: 31, changed: 13, deleted: 14 },
                                            internalID: '1'.repeat(70),
                                        },
                                    ],
                                    totalCount: 1,
                                    pageInfo: { endCursor: null, hasNextPage: false },
                                    diffStat: { __typename: 'DiffStat', added: 1, changed: 1, deleted: 1 },
                                },
                            },
                        },
                    }),
                })
                await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)
                await driver.page.waitForSelector('.test-file-diff-connection')
                await percySnapshotWithVariants(driver.page, 'Repository compare page')
                await accessibilityAudit(driver.page)
            })
        })
    })
})
