import assert from 'assert'
import { createDriverForTest, Driver, percySnapshot } from '../../../shared/src/testing/driver'
import { commonWebGraphQlResults } from './graphQlResults'
import { createWebIntegrationTestContext, WebIntegrationTestContext } from './context'
import {
    createRepositoryRedirectResult,
    createResolveRevisionResult,
    createFileExternalLinksResult,
    createTreeEntriesResult,
    createBlobContentResult,
} from './graphQlResponseHelpers'
import { afterEachSaveScreenshotIfFailed } from '../../../shared/src/testing/screenshotReporter'
import * as path from 'path'
import { DiffHunkLineType } from '../graphql-operations'
import { encodeURIPathComponent } from '../../../shared/src/util/url'
import { ExtensionManifest } from '../../../shared/src/extensions/extensionManifest'
import { Settings } from '../../../shared/src/settings/settings'
import { ExternalServiceKind } from '../../../shared/src/graphql/schema'
import type * as sourcegraph from 'sourcegraph'

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
            const clickedFileName = 'async.go'
            const clickedCommit = ''
            const fileEntries = ['jsonrpc2.go', clickedFileName]

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                RepositoryRedirect: ({ repoName }) => createRepositoryRedirectResult(repoName),
                ResolveRev: ({ repoName }) => createResolveRevisionResult(repoName),
                FileExternalLinks: ({ filePath }) => createFileExternalLinksResult(filePath),
                TreeEntries: () => createTreeEntriesResult(repositorySourcegraphUrl, fileEntries),
                Blob: () => createBlobContentResult('mock file blob'),
                TreeCommits: () => ({
                    node: {
                        __typename: 'Repository',
                        commit: {
                            ancestors: {
                                nodes: [
                                    {
                                        id: 'CommitID1',
                                        oid: '15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                        abbreviatedOID: '15c2290',
                                        message: 'update LSIF indexing CI workflow\n',
                                        subject: 'update LSIF indexing CI workflow',
                                        body: null,
                                        author: {
                                            person: {
                                                avatarURL: '',
                                                name: 'garo (they/them)',
                                                email: 'gbrik@users.noreply.github.com',
                                                displayName: 'garo (they/them)',
                                                user: null,
                                            },
                                            date: '2020-04-29T18:40:54Z',
                                        },
                                        committer: {
                                            person: {
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
                                                oid: '96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                                abbreviatedOID: '96c4efa',
                                                url:
                                                    '/github.com/sourcegraph/jsonrpc2/-/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                            },
                                            {
                                                oid: '9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                                abbreviatedOID: '9e615b1',
                                                url:
                                                    '/github.com/sourcegraph/jsonrpc2/-/commit/9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                            },
                                        ],
                                        url:
                                            '/github.com/sourcegraph/jsonrpc2/-/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                        canonicalURL:
                                            '/github.com/sourcegraph/jsonrpc2/-/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                        externalURLs: [
                                            {
                                                url:
                                                    'https://github.com/sourcegraph/jsonrpc2/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                                serviceKind: ExternalServiceKind.GITHUB,
                                            },
                                        ],
                                        tree: {
                                            canonicalURL:
                                                '/github.com/sourcegraph/jsonrpc2@15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                        },
                                    },
                                    {
                                        id: 'CommitID2',
                                        oid: '9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                        abbreviatedOID: '9e615b1',
                                        message: 'LSIF Indexing Campaign',
                                        subject: 'LSIF Indexing Campaign',
                                        body: null,
                                        author: {
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
                                            person: {
                                                avatarURL: '',
                                                name: 'Sourcegraph Bot',
                                                email: 'batch-changes@sourcegraph.com',
                                                displayName: 'Sourcegraph Bot',
                                                user: null,
                                            },
                                            date: '2020-04-29T16:57:20Z',
                                        },
                                        parents: [
                                            {
                                                oid: '96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                                abbreviatedOID: '96c4efa',
                                                url:
                                                    '/github.com/sourcegraph/jsonrpc2/-/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                            },
                                        ],
                                        url:
                                            '/github.com/sourcegraph/jsonrpc2/-/commit/9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                        canonicalURL:
                                            '/github.com/sourcegraph/jsonrpc2/-/commit/9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                        externalURLs: [
                                            {
                                                url:
                                                    'https://github.com/sourcegraph/jsonrpc2/commit/9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                                serviceKind: ExternalServiceKind.GITHUB,
                                            },
                                        ],
                                        tree: {
                                            canonicalURL:
                                                '/github.com/sourcegraph/jsonrpc2@9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                        },
                                    },
                                    {
                                        id: 'CommitID3',
                                        oid: '96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                        abbreviatedOID: '96c4efa',
                                        message:
                                            'Produce LSIF data for each commit for fast/precise code nav (#35)\n\n* Produce LSIF data for each commit for fast/precise code nav\r\n\r\n* Update lsif.yml\r',
                                        subject: 'Produce LSIF data for each commit for fast/precise code nav (#35)',
                                        body:
                                            '* Produce LSIF data for each commit for fast/precise code nav\r\n\r\n* Update lsif.yml',
                                        author: {
                                            person: {
                                                avatarURL: '',
                                                name: 'Quinn Slack',
                                                email: 'qslack@qslack.com',
                                                displayName: 'Quinn Slack',
                                                user: {
                                                    id: 'VXNlcjo2',
                                                    username: 'sqs',
                                                    url: '/users/sqs',
                                                    displayName: 'sqs',
                                                },
                                            },
                                            date: '2019-12-22T04:34:38Z',
                                        },
                                        committer: {
                                            person: {
                                                avatarURL: '',
                                                name: 'GitHub',
                                                email: 'noreply@github.com',
                                                displayName: 'GitHub',
                                                user: null,
                                            },
                                            date: '2019-12-22T04:34:38Z',
                                        },
                                        parents: [
                                            {
                                                oid: 'cee7209801bf50cee868f8e0696ba0b76ae21792',
                                                abbreviatedOID: 'cee7209',
                                                url:
                                                    '/github.com/sourcegraph/jsonrpc2/-/commit/cee7209801bf50cee868f8e0696ba0b76ae21792',
                                            },
                                        ],
                                        url:
                                            '/github.com/sourcegraph/jsonrpc2/-/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                        canonicalURL:
                                            '/github.com/sourcegraph/jsonrpc2/-/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                        externalURLs: [
                                            {
                                                url:
                                                    'https://github.com/sourcegraph/jsonrpc2/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                                serviceKind: ExternalServiceKind.GITHUB,
                                            },
                                        ],
                                        tree: {
                                            canonicalURL:
                                                '/github.com/sourcegraph/jsonrpc2@96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                        },
                                    },
                                ],
                                pageInfo: { hasNextPage: false },
                            },
                        },
                    },
                }),
                RepositoryCommit: () => ({
                    node: {
                        __typename: 'Repository',
                        commit: {
                            __typename: 'GitCommit',
                            id: 'CommitID1',
                            oid: '15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                            abbreviatedOID: '15c2290',
                            message: 'update LSIF indexing CI workflow\n',
                            subject: 'update LSIF indexing CI workflow',
                            body: null,
                            author: {
                                person: {
                                    avatarURL: '',
                                    name: 'garo (they/them)',
                                    email: 'gbrik@users.noreply.github.com',
                                    displayName: 'garo (they/them)',
                                    user: null,
                                },
                                date: '2020-04-29T18:40:54Z',
                            },
                            committer: {
                                person: {
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
                                    oid: '96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                    abbreviatedOID: '96c4efa',
                                    url:
                                        '/github.com/sourcegraph/jsonrpc2/-/commit/96c4efab7ee28f3d1cf1d248a0139cea37368b18',
                                },
                                {
                                    oid: '9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                    abbreviatedOID: '9e615b1',
                                    url:
                                        '/github.com/sourcegraph/jsonrpc2/-/commit/9e615b1c32cc519130575e8d10d0d0fee8a5eb6c',
                                },
                            ],
                            url: '/github.com/sourcegraph/jsonrpc2/-/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                            canonicalURL:
                                '/github.com/sourcegraph/jsonrpc2/-/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                            externalURLs: [
                                {
                                    url:
                                        'https://github.com/sourcegraph/jsonrpc2/commit/15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                                    serviceKind: ExternalServiceKind.GITHUB,
                                },
                            ],
                            tree: {
                                canonicalURL:
                                    '/github.com/sourcegraph/jsonrpc2@15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81',
                            },
                        },
                    },
                }),
                RepositoryComparisonDiff: () => ({
                    node: {
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
                                            url:
                                                '/github.com/sourcegraph/jsonrpc2@15c2290dcb37731cc4ee5a2a1c1e5a25b4c28f81/-/blob/.github/workflows/lsif.yml',
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
                                                            html:
                                                                '<div><span style="color:#657b83;">  </span><span style="color:#268bd2;">build</span><span style="color:#657b83;">:\n</span></div>',
                                                        },
                                                        {
                                                            kind: DiffHunkLineType.ADDED,
                                                            html:
                                                                '<div><span style="color:#657b83;">  </span><span style="color:#268bd2;">lsif-go</span><span style="color:#657b83;">:\n</span></div>',
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
                                diffStat: { added: 1, changed: 3, deleted: 4 },
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

            await driver.page.waitForSelector('h2.tree-page__title')

            // Assert that the directory listing displays properly
            await driver.page.waitForSelector('.test-tree-entries')
            await percySnapshot(driver.page, 'Repository index page')

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

            await driver.page.waitForSelector('.test-repo-blob')
            await driver.assertWindowLocation(`/${repositoryName}/-/blob/${clickedFileName}`)

            // Assert breadcrumb order
            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', clickedFileName])

            // Return to repo page
            await driver.page.waitForSelector('.test-repo-header-repo-link')
            await driver.page.click('.test-repo-header-repo-link')

            await driver.page.waitForSelector('h2.tree-page__title')
            await assertSelectorHasText('h2.tree-page__title', ' ' + shortRepositoryName)
            await driver.assertWindowLocation(repositorySourcegraphUrl)

            await driver.findElementWithText(clickedCommit, { selector: '.git-commit-node__oid', action: 'click' })
            await driver.page.waitForSelector('.git-commit-node__message-subject')
            await assertSelectorHasText('.git-commit-node__message-subject', 'update LSIF indexing CI workflow')
        })

        it('works with files with spaces in the name', async () => {
            const shortRepositoryName = 'ggilmore/q-test'
            const fileName = '% token.4288249258.sql'
            const directoryName = "Geoffrey's random queries.32r242442bf"
            const filePath = path.posix.join(directoryName, fileName)

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                RepositoryRedirect: ({ repoName }) => createRepositoryRedirectResult(repoName),
                ResolveRev: ({ repoName }) => createResolveRevisionResult(repoName),
                FileExternalLinks: ({ filePath, repoName, revision }) =>
                    createFileExternalLinksResult(
                        `https://${encodeURIPathComponent(repoName)}/blob/${encodeURIPathComponent(
                            revision
                        )}/${encodeURIPathComponent(filePath)}`
                    ),
                TreeEntries: () => ({
                    repository: {
                        commit: {
                            tree: {
                                isRoot: false,
                                url: '/github.com/ggilmore/q-test/-/tree/Geoffrey%27s%20random%20queries.32r242442bf',
                                entries: [
                                    {
                                        name: fileName,
                                        path: filePath,
                                        isDirectory: false,
                                        url:
                                            '/github.com/ggilmore/q-test/-/blob/Geoffrey%27s%20random%20queries.32r242442bf/%25%20token.4288249258.sql',
                                        submodule: null,
                                        isSingleChild: false,
                                    },
                                ],
                            },
                        },
                    },
                }),
                TreeCommits: () => ({
                    node: {
                        __typename: 'Repository',
                        commit: { ancestors: { nodes: [], pageInfo: { hasNextPage: false } } },
                    },
                }),
                Blob: ({ filePath }) => createBlobContentResult(`content for: ${filePath}`),
            })

            await driver.page.goto(
                `${driver.sourcegraphBaseUrl}/github.com/ggilmore/q-test/-/tree/Geoffrey's%20random%20queries.32r242442bf`
            )
            await driver.page.waitForSelector('.test-tree-file-link')
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector('.test-tree-file-link')?.textContent),
                fileName
            )

            // page.click() fails for some reason with Error: Node is either not visible or not an HTMLElement
            await driver.page.$eval('.test-tree-file-link', linkElement => (linkElement as HTMLElement).click())
            await driver.page.waitForSelector('.test-repo-blob')

            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', filePath])

            await driver.page.waitForSelector('#monaco-query-input .view-lines')
            // TODO: find a more reliable way to get the current search query,
            // to account for the fact that it may _actually_ contain non-breaking spaces
            // (and not just have spaces rendered as non-breaking in the DOM by Monaco)
            // https://github.com/sourcegraph/sourcegraph/issues/14756
            const searchQuery = (
                await driver.page.evaluate(() => document.querySelector('#monaco-query-input .view-lines')?.textContent)
            )?.replace(/\u00A0/g, ' ')
            assert.strictEqual(
                searchQuery,
                "repo:^github\\.com/ggilmore/q-test$ file:^Geoffrey's\\ random\\ queries\\.32r242442bf/%\\ token\\.4288249258\\.sql"
            )

            await driver.page.waitForSelector('.test-go-to-code-host')
            assert.strictEqual(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLAnchorElement>('.test-go-to-code-host')?.href
                ),
                "https://github.com/ggilmore/q-test/blob/master/Geoffrey's%20random%20queries.32r242442bf/%25%20token.4288249258.sql"
            )

            const blobContent = await driver.page.evaluate(() => document.querySelector('.test-repo-blob')?.textContent)
            assert.strictEqual(blobContent, `content for: ${filePath}`)
        })

        it('works with a plus sign in the repository name', async () => {
            const shortRepositoryName = 'ubuntu/+source/quemu'
            const repositorySourcegraphUrl = '/ubuntu/+source/quemu'

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                RepositoryRedirect: ({ repoName }) => createRepositoryRedirectResult(repoName),
                ResolveRev: ({ repoName }) => createResolveRevisionResult(repoName),
                FileExternalLinks: ({ filePath }) => createFileExternalLinksResult(filePath),
                TreeEntries: () => createTreeEntriesResult(repositorySourcegraphUrl, ['readme.md']),

                TreeCommits: () => ({
                    node: {
                        __typename: 'Repository',
                        commit: { ancestors: { nodes: [], pageInfo: { hasNextPage: false } } },
                    },
                }),
                Blob: ({ filePath }) => createBlobContentResult(`content for: ${filePath}`),
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)

            await driver.page.waitForSelector('h2.tree-page__title')
            await assertSelectorHasText('h2.tree-page__title', ` ${shortRepositoryName}`)
            await assertSelectorHasText('.test-tree-entry-file', 'readme.md')

            await driver.page.waitForSelector('#monaco-query-input .view-lines')
            // TODO: find a more reliable way to get the current search query,
            // to account for the fact that it may _actually_ contain non-breaking spaces
            // (and not just have spaces rendered as non-breaking in the DOM by Monaco)
            // https://github.com/sourcegraph/sourcegraph/issues/14756
            const searchQuery = (
                await driver.page.evaluate(() => document.querySelector('#monaco-query-input .view-lines')?.textContent)
            )?.replace(/\u00A0/g, ' ')
            assert.strictEqual(searchQuery, 'repo:^ubuntu/\\+source/quemu$ ') // + should be escaped in regular expression

            // page.click() fails for some reason with Error: Node is either not visible or not an HTMLElement
            await driver.page.$eval('.test-tree-file-link', linkElement => (linkElement as HTMLElement).click())
            await driver.page.waitForSelector('.test-repo-blob')

            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', 'readme.md'])
        })

        it('works with spaces in the repository name', async () => {
            const shortRepositoryName = 'my org/repo with spaces'
            const repositorySourcegraphUrl = '/github.com/my%20org/repo%20with%20spaces'

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                RepositoryRedirect: ({ repoName }) => createRepositoryRedirectResult(repoName),
                ResolveRev: ({ repoName }) => createResolveRevisionResult(repoName),
                FileExternalLinks: ({ filePath }) => createFileExternalLinksResult(filePath),
                TreeEntries: () => createTreeEntriesResult(repositorySourcegraphUrl, ['readme.md']),

                TreeCommits: () => ({
                    node: {
                        __typename: 'Repository',
                        commit: { ancestors: { nodes: [], pageInfo: { hasNextPage: false } } },
                    },
                }),
                Blob: ({ filePath }) => createBlobContentResult(`content for: ${filePath}`),
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + repositorySourcegraphUrl)

            await driver.page.waitForSelector('h2.tree-page__title')
            await assertSelectorHasText('h2.tree-page__title', ' my org/repo with spaces')
            await assertSelectorHasText('.test-tree-entry-file', 'readme.md')

            // page.click() fails for some reason with Error: Node is either not visible or not an HTMLElement
            await driver.page.$eval('.test-tree-file-link', linkElement => (linkElement as HTMLElement).click())
            await driver.page.waitForSelector('.test-repo-blob')

            await driver.page.waitForSelector('.test-breadcrumb')
            const breadcrumbTexts = await driver.page.evaluate(() =>
                [...document.querySelectorAll('.test-breadcrumb')].map(breadcrumb => breadcrumb.textContent?.trim())
            )
            assert.deepStrictEqual(breadcrumbTexts, [shortRepositoryName, '@master', 'readme.md'])
        })
    })

    // Describes the ways the directory viewer and tree sidebar can be extended through Sourcegraph extensions.
    describe('extensibility', () => {
        const repoName = 'github.com/sourcegraph/file-decs'

        beforeEach(() => {
            const userSettings: Settings = {
                extensions: {
                    'test/test': true,
                },
            }
            const extensionManifest: ExtensionManifest = {
                url: new URL('/-/static/extension/0001-test-test.js?hash--test-test', driver.sourcegraphBaseUrl).href,
                activationEvents: ['*'],
            }

            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                RepositoryRedirect: ({ repoName }) => createRepositoryRedirectResult(repoName),
                ResolveRev: ({ repoName }) => createResolveRevisionResult(repoName),
                FileExternalLinks: ({ filePath, repoName, revision }) =>
                    createFileExternalLinksResult(
                        `https://${encodeURIPathComponent(repoName)}/blob/${encodeURIPathComponent(
                            revision
                        )}/${encodeURIPathComponent(filePath)}`
                    ),
                ViewerSettings: () => ({
                    viewerSettings: {
                        final: JSON.stringify(userSettings),
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
                    },
                }),
                TreeEntries: ({ filePath, repoName }) => {
                    if (filePath === '') {
                        return {
                            repository: {
                                commit: {
                                    tree: {
                                        isRoot: true,
                                        url: `/${repoName}`,
                                        entries: [
                                            {
                                                isDirectory: true,
                                                isSingleChild: true,
                                                name: 'nested',
                                                path: 'nested',
                                                url: `/${repoName}/-/tree/nested`,
                                                submodule: null,
                                            },
                                            // recursiveSingleChild is always true in the web app
                                            {
                                                name: 'test.ts',
                                                path: 'nested/test.ts',
                                                isDirectory: false,
                                                url: `/${repoName}/-/blob/nested/test.ts`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                            {
                                                name: 'ReactComponent.tsx',
                                                path: 'nested/ReactComponent.tsx',
                                                isDirectory: false,
                                                url: `/${repoName}/-/blob/nested/ReactComponent.tsx`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                            {
                                                name: 'doubly-nested',
                                                path: 'nested/doubly-nested',
                                                isDirectory: true,
                                                url: `/${repoName}/-/tree/nested/doubly-nested`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                        ],
                                    },
                                },
                            },
                        }
                    }

                    if (filePath === 'nested') {
                        return {
                            repository: {
                                commit: {
                                    tree: {
                                        isRoot: false,
                                        url: `/${repoName}/-/tree/nested`,
                                        entries: [
                                            {
                                                name: 'test.ts',
                                                path: 'nested/test.ts',
                                                isDirectory: false,
                                                url: `/${repoName}/-/blob/nested/test.ts`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                            {
                                                name: 'ReactComponent.tsx',
                                                path: 'nested/ReactComponent.tsx',
                                                isDirectory: false,
                                                url: `/${repoName}/-/blob/nested/ReactComponent.tsx`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                            {
                                                name: 'doubly-nested',
                                                path: 'nested/doubly-nested',
                                                isDirectory: true,
                                                url: `/${repoName}/-/tree/nested/doubly-nested`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                        ],
                                    },
                                },
                            },
                        }
                    }

                    if (filePath === 'nested/doubly-nested') {
                        return {
                            repository: {
                                commit: {
                                    tree: {
                                        isRoot: false,
                                        url: `/${repoName}/-/tree/nested/doubly-nested`,
                                        entries: [
                                            {
                                                name: 'triply-nested.ts',
                                                path: 'nested/doubly-nested/triply-nested.ts',
                                                isDirectory: false,
                                                url: `/${repoName}/-/blob/nested/doubly-nested/triply-nested.ts`,
                                                submodule: null,
                                                isSingleChild: false,
                                            },
                                        ],
                                    },
                                },
                            },
                        }
                    }

                    // unknown
                    return {
                        repository: {
                            commit: {
                                tree: {
                                    isRoot: false,
                                    url: `/${repoName}/${filePath}`,
                                    entries: [],
                                },
                            },
                        },
                    }
                },
                TreeCommits: () => ({
                    node: {
                        __typename: 'Repository',
                        commit: { ancestors: { nodes: [], pageInfo: { hasNextPage: false } } },
                    },
                }),
                Blob: ({ filePath }) => createBlobContentResult(`content for: ${filePath}`),
                Extensions: () => ({
                    extensionRegistry: {
                        extensions: {
                            nodes: [
                                {
                                    id: 'TestExtensionID',
                                    extensionID: 'test/test',
                                    manifest: {
                                        raw: JSON.stringify(extensionManifest),
                                    },
                                    url: '/extensions/test/test',
                                    viewerCanAdminister: false,
                                },
                            ],
                        },
                    },
                }),
            })

            // Serve a mock extension bundle with a simple file decoration provider
            testContext.server
                .get(new URL(extensionManifest.url, driver.sourcegraphBaseUrl).href)
                .intercept((request, response) => {
                    function extensionBundle(): void {
                        // eslint-disable-next-line @typescript-eslint/no-require-imports, @typescript-eslint/no-var-requires
                        const sourcegraph = require('sourcegraph') as typeof import('sourcegraph')

                        const vowels = 'aeiouAEIOU'

                        function activate(context: sourcegraph.ExtensionContext): void {
                            context.subscriptions.add(
                                sourcegraph.app.registerFileDecorationProvider({
                                    provideFileDecorations: ({ files }) =>
                                        files.map(file => {
                                            const fragments = file.path.split('/')
                                            const name = fragments[fragments.length - 1]
                                            return {
                                                uri: file.uri,
                                                after: {
                                                    contentText: `${
                                                        name.split('').filter(char => vowels.includes(char)).length
                                                    } vowels`,
                                                    color: file.isDirectory ? 'red' : 'blue',
                                                },
                                                meter: {
                                                    value: file.isDirectory ? 50 : 100,
                                                },
                                            }
                                        }),
                                })
                            )
                        }

                        exports.activate = activate
                    }
                    // Create an immediately-invoked function expression for the extensionBundle function
                    const extensionBundleString = `(${extensionBundle.toString()})()`
                    response.type('application/javascript; charset=utf-8').send(extensionBundleString)
                })
        })
        async function getDecorationsByFilename(
            pageOrSidebar: 'page' | 'sidebar',
            filename: string
        ): Promise<{ textContent?: string | null; percentage?: string | null } | null> {
            return driver.page.evaluate(
                ({ pageOrSidebar, filename }) => {
                    const decorable = [
                        ...document.querySelectorAll('.test-' + String(pageOrSidebar) + '-file-decorable'),
                    ].find(decorable =>
                        decorable?.querySelector('.test-file-decorable-name')?.textContent?.includes(filename)
                    )

                    if (!decorable) {
                        return null
                    }

                    return {
                        textContent: decorable.querySelector('.test-file-decoration-text')?.textContent,
                        percentage: decorable.querySelector('.test-file-decoration-meter')?.getAttribute('value'),
                    }
                },
                { pageOrSidebar, filename }
            )
        }

        it('file decorations work on tree page and sidebar', async () => {
            await driver.page.goto(`${driver.sourcegraphBaseUrl}/${repoName}`)

            try {
                await driver.page.waitForSelector('.test-file-decoration-container', { timeout: 5000 })
            } catch {
                throw new Error('Expected to see file decorations')
            }

            // TREE SIDEBAR ASSERTIONS

            const nestedDecorations = await getDecorationsByFilename('sidebar', 'nested')

            assert.deepStrictEqual(
                nestedDecorations,
                {
                    textContent: '2 vowels',
                    percentage: '50', // dirs are 50% in mock extension
                },
                'Incorrect decorations for nested on tree sidebar'
            )

            // Since nested is a single child, its children should be visible and decorated as well

            const testDecorations = await getDecorationsByFilename('sidebar', 'test.ts')

            assert.deepStrictEqual(
                testDecorations,
                {
                    textContent: '1 vowels',
                    percentage: '100', // files are 100% in mock extension
                },
                'Incorrect decorations for test.ts on tree sidebar'
            )

            const doublyNestedDecorations = await getDecorationsByFilename('sidebar', 'doubly-nested')

            assert.deepStrictEqual(
                doublyNestedDecorations,
                {
                    textContent: '4 vowels',
                    percentage: '50',
                },
                'Incorrect decorations for doubly-nested on tree sidebar'
            )

            // Expand directory. we want to trigger "noopRowClick" handler in order to not navigate to new tree page
            await driver.page.evaluate(() =>
                ([...document.querySelectorAll('.test-sidebar-file-decorable')]
                    .find(directory => directory.textContent?.includes('doubly-nested'))
                    ?.querySelector('.test-tree-noop-link') as HTMLAnchorElement | undefined)?.click()
            )

            // Wait for file decorations to be sent from extension host
            try {
                await driver.page.waitForFunction(
                    () =>
                        !![...document.querySelectorAll('.test-sidebar-file-decorable')]
                            .find(file =>
                                file.querySelector('.test-file-decorable-name')?.textContent?.includes('triply-nested')
                            )
                            ?.querySelector('.test-file-decoration-container'),
                    { timeout: 5000 }
                )
            } catch {
                throw new Error('Timed out waiting for "triply-nested" decorations in tree sidebar')
            }
            const triplyNestedDecorations = await getDecorationsByFilename('sidebar', 'triply-nested')

            assert.deepStrictEqual(
                triplyNestedDecorations,
                {
                    textContent: '3 vowels',
                    percentage: '100',
                },
                'Incorrect decorations for triply-nested.ts on tree sidebar'
            )

            // TREE PAGE ASSERTIONS

            try {
                await driver.findElementWithText('nested', {
                    selector: '.test-page-file-decorable .test-file-decorable-name',
                    fuzziness: 'contains',
                    wait: {
                        timeout: 3000,
                    },
                })
            } catch {
                throw new Error('timed out waiting for "nested" in tree page')
            }

            // Wait for decorations
            try {
                await driver.page.waitForSelector('.test-page-file-decorable .test-file-decoration-container')
            } catch {
                throw new Error('Timed out waiting for "nested" decorations in tree page')
            }

            await driver.page.evaluate(() =>
                ([...document.querySelectorAll('.test-page-file-decorable .test-file-decorable-name')].find(name =>
                    name?.textContent?.includes('nested')
                ) as HTMLAnchorElement | undefined)?.click()
            )

            // Wait for decorations
            try {
                await driver.page.waitForSelector('.test-page-file-decorable .test-file-decoration-container')
            } catch {
                throw new Error('Timed out waiting for "ReactComponent.tsx" decorations in tree page')
            }

            const reactDecorations = await getDecorationsByFilename('page', 'ReactComponent.tsx')
            assert.deepStrictEqual(
                reactDecorations,
                {
                    textContent: '5 vowels',
                    percentage: '100',
                },
                'Incorrect decorations for ReactComponent.tsx on tree page'
            )

            const doublyNestedPageDecorations = await getDecorationsByFilename('page', 'doubly-nested')
            // This should be equal to its sidebar decorations
            assert.deepStrictEqual(
                doublyNestedPageDecorations,
                {
                    textContent: '4 vowels',
                    percentage: '50',
                },
                'Incorrect decorations for doubly-nested on tree page'
            )

            await driver.page.evaluate(() =>
                ([...document.querySelectorAll('.test-page-file-decorable .test-file-decorable-name')].find(name =>
                    name?.textContent?.includes('doubly-nested')
                ) as HTMLAnchorElement | undefined)?.click()
            )

            // Wait for new tree page
            await driver.findElementWithText('triply-nested', {
                selector: '.test-page-file-decorable .test-file-decorable-name',
                fuzziness: 'contains',
                wait: {
                    timeout: 3000,
                },
            })

            // Wait for decorations
            try {
                await driver.page.waitForSelector('.test-page-file-decorable .test-file-decoration-container')
            } catch {
                throw new Error('Timed out waiting for "triply-nested" decorations in tree page')
            }

            const triplyNestedPageDecorations = await getDecorationsByFilename('page', 'triply-nested.ts')
            // This should be equal to its sidebar decorations
            assert.deepStrictEqual(
                triplyNestedPageDecorations,
                {
                    textContent: '3 vowels',
                    percentage: '100',
                },
                'Incorrect decorations for triply-nested.ts on tree page'
            )
        })
    })
})
