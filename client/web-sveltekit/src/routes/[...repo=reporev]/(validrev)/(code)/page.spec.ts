import { test, expect, type Page } from '../../../../testing/integration'

const repoName = 'github.com/sourcegraph/sourcegraph'

test.beforeEach(({ sg }) => {
    sg.fixture([
        {
            __typename: 'Repository',
            id: '1',
            name: repoName,
            description: 'Example description',
            mirrorInfo: {
                cloned: true,
                cloneInProgress: false,
            },
            commit: {
                id: '2',
            },
        },
        {
            __typename: 'GitCommit',
            id: '2',
            tree: {
                isRoot: true,
                canonicalURL: `/${repoName}/-/tree/`,
                isDirectory: true,
                entries: [
                    {
                        canonicalURL: `/${repoName}/-/tree/src`,
                    },
                    {
                        canonicalURL: `/${repoName}/-/blob/index.js`,
                    },
                    {
                        canonicalURL: `/${repoName}/-/blob/README.md`,
                    },
                ],
            },
        },
        {
            __typename: 'GitTree',
            path: 'src',
            name: 'src',
            canonicalURL: `/${repoName}/-/tree/src`,
            isDirectory: true,
            isRoot: false,
            entries: [
                {
                    canonicalURL: `/${repoName}/-/blob/src/notes.txt`,
                },
            ],
        },
        {
            __typename: 'GitBlob',
            path: 'index.js',
            name: 'index.js',
            canonicalURL: `/${repoName}/-/blob/index.js`,
            isDirectory: false,
            languages: ['JavaScript'],
            richHTML: '',
            content: 'var hello = "world"',
        },
        {
            __typename: 'GitBlob',
            canonicalURL: `/${repoName}/-/blob/README.md`,
            name: 'README.md',
            path: 'README.md',
            isDirectory: false,
            richHTML: 'Example readme content',
        },
        {
            __typename: 'GitBlob',
            canonicalURL: `/${repoName}/-/blob/src/notes.txt`,
            name: 'notes.txt',
            path: 'src/notes.txt',
            isDirectory: false,
            content: 'Some notes',
            richHTML: '',
        },
    ])

    sg.mockOperations({
        ResolveRepoRevision: () => ({
            repositoryRedirect: {
                id: '1',
            },
        }),
        TreeEntries: () => ({
            repository: {
                id: '1',
            },
        }),
        RepoPageReadmeQuery: ({ path }) => ({
            repository: {
                id: '1',
                commit: {
                    blob: {
                        canonicalURL: `/${repoName}/-/blob/${path}`,
                    },
                },
            },
        }),
    })
})

test.describe('file sidebar', () => {
    test.skip('basic functionality', async ({ page }) => {
        const readmeEntry = page.getByRole('treeitem', { name: 'README.md' })

        await page.goto(`/${repoName}`)

        // Open sidebar
        await page.getByLabel('Open sidebar').click()
        await expect(readmeEntry).toBeVisible()

        // Go to a file
        await readmeEntry.click()
        await expect(page).toHaveURL(`/${repoName}/-/blob/README.md`)
        // Verify that entry is selected
        await expect(page.getByRole('treeitem', { name: 'README.md', selected: true })).toBeVisible()

        // Go to other file
        await page.getByRole('treeitem', { name: 'index.js' }).click()
        await expect(page).toHaveURL(`/${repoName}/-/blob/index.js`)
        // Verify that entry is selected
        await expect(page.getByRole('treeitem', { name: 'index.js', selected: true })).toBeVisible()

        // Close file sidebar
        await page.getByLabel('Close sidebar').click()
        await expect(readmeEntry).toBeHidden()
    })

    test('error handling root', async ({ page, sg }) => {
        sg.mockOperations({
            TreeEntries: () => {
                throw new Error('Sidebar error')
            },
        })

        await page.goto(`/${repoName}`)
        await page.getByLabel('Open sidebar').click()
        await expect(page.getByText(/Sidebar error/)).toBeVisible()
    })

    test('error handling children', async ({ page, sg }) => {
        await page.goto(`/${repoName}`)
        await page.getByLabel('Open sidebar').click()

        const treeItem = page.getByRole('treeitem', { name: 'src' })
        // For some reason we need to wait for the tree to be rendered
        // before we mock the GraphQL response to throw an error
        await expect(treeItem).toBeVisible()

        sg.mockOperations({
            TreeEntries: () => {
                throw new Error('Child error')
            },
        })
        // Clicks the toggle button next to the tree entry, to expand the tree
        // and _not_ follow the link
        await treeItem.getByRole('button').click()
        await expect(page.getByText(/Child error/)).toBeVisible()
    })

    test('error handling non-existing directory -> root', async ({ page, sg, context }) => {
        // Here we expect the sidebar to show an error message, and after navigigating
        // to an existing directory, the directory contents
        sg.mockOperations({
            TreeEntries: () => {
                throw new Error('Sidebar error')
            },
        })

        await page.goto(`/${repoName}/-/tree/non-existing-directory`)
        await page.getByLabel('Open sidebar FALSE').click()
        await expect(page.getByText(/Sidebar error/).first()).toBeVisible()

        sg.mockOperations({
            TreeEntries: () => ({
                repository: {
                    id: '1',
                },
            }),
        })

        await page.goto(`/${repoName}`)
        await page.getByLabel('Open sidebar').click()
        await expect(page.getByRole('treeitem', { name: 'README.md' })).toBeVisible()
    })
})

test('repo readme', async ({ page }) => {
    // Shows the readme file if there is one in the repo root
    await page.goto(`/${repoName}`)
    await expect(page.getByRole('heading', { name: 'README.md' })).toBeVisible()
    await expect(page.getByText('Example readme content')).toBeVisible()
})

test('repo description', async ({ page, sg }) => {
    // Shows the repo description if there is no readme file in the repo root
    sg.mockOperations({
        TreeEntries: () => ({
            repository: {
                commit: {
                    tree: {
                        isRoot: true,
                        entries: [],
                    },
                },
            },
        }),
    })

    await page.goto(`/${repoName}`)
    await expect(page.getByRole('heading', { name: 'Description' })).toBeVisible()
    await expect(page.getByText('Example description')).toBeVisible()
})

test('history panel', async ({ page, sg }) => {
    sg.mockOperations({
        GitHistoryQuery: () => ({
            repository: {
                id: '1',
                commit: {
                    ancestors: {
                        nodes: [{ subject: 'Test commit' }, { subject: 'Test commit 2' }],
                        pageInfo: {
                            hasNextPage: false,
                        },
                    },
                },
            },
        }),
    })

    await page.goto(`/${repoName}`)

    // Open history panel
    await page.getByRole('tab', { name: 'History' }).click()
    await expect(page.getByText('Test commit', { exact: true })).toBeVisible()

    // Close history panel
    await page.getByRole('tab', { name: 'History' }).click()
    await expect(page.getByText('Test commit')).toBeHidden()
})

test('file popover', async ({ page, sg }) => {
    await page.goto(`/${repoName}`)

    // Open the sidebar
    await page.getByLabel('Open sidebar').click()

    // Hover a tree entry, expect the popover to be visible
    await page.getByRole('link', { name: 'index.js' }).hover()
    await expect(page.getByText('Last Changed')).toBeVisible()

    // Hover outside the popover (the Sourcegraph logo), expect the popover to be hidden
    await page.getByRole('banner').getByRole('link').first().hover()
    await expect(page.getByText('Last Changed')).toBeHidden()

    sg.mockOperations({
        TreeEntries: () => ({
            repository: {
                id: '1',
                commit: {
                    tree: {
                        entries: [
                            {
                                path: 'src/notes.txt',
                                name: 'notes.txt',
                                isDirectory: false,
                                canonicalURL: `/${repoName}/-/blob/src/notes.txt`,
                            },
                        ],
                    },
                },
            },
        }),
        FileOrDirPopoverQuery: () => ({
            repository: {
                commit: {
                    path: {
                        path: 'src/notes.txt',
                        languages: ['Text'],
                        byteSize: 32,
                        totalLines: 42,
                        name: 'notes.txt',
                    },
                },
            },
        }),
    })

    // Open a subdirectory so we get an entry with a clickable parent dir
    await page.getByRole('link', { name: 'src' }).click()

    // Hover the file to get a popover
    await page.getByRole('treeitem', { name: 'notes.txt' }).getByRole('link').click()
    await page.getByRole('treeitem', { name: 'notes.txt' }).getByRole('link').hover()

    // Expect the popover to show up
    await expect(page.getByText('Last Changed')).toBeVisible()

    // Click the parent dir in the popover and expect to navigate to that page
    await page.locator('div').filter({ hasText: /^src$/ }).getByRole('link').click()
    await page.waitForURL(/src$/)
})

test.describe('cody sidebar', () => {
    const path = `/${repoName}/-/blob/index.js`

    async function hasCody(page: Page): Promise<void> {
        const codyButton = page.getByLabel('Open Cody chat')
        await expect(codyButton).toBeVisible()
        await codyButton.click()
        await expect(page.getByRole('complementary', { name: 'Cody' })).toBeVisible()
    }

    async function doesNotHaveCody(page: Page): Promise<void> {
        const codyButton = page.getByLabel('Open Cody chat')
        await expect(page.getByRole('link', { name: 'index.js' })).toBeVisible()
        await expect(codyButton).not.toBeAttached()
    }

    test.describe('dotcom', () => {
        test.beforeEach(({ sg }) => {
            sg.dotcomMode()
        })

        test('enabled when signed out', async ({ page, sg }) => {
            await page.goto(path)
            await hasCody(page)
            await expect(
                page.getByText('Cody is only available to signed-in users. Sign in to use Cody.')
            ).toBeVisible()
        })

        test('enabled when signed in', async ({ page, sg }) => {
            sg.signIn()

            await page.goto(path)
            await hasCody(page)
        })

        test('ignores context filters', async ({ page, sg }) => {
            sg.mockTypes({
                Site: () => ({
                    codyContextFilters: {
                        raw: {
                            include: [String.raw`source.*`],
                        },
                    },
                }),
            })

            await page.goto(path)
            await hasCody(page)
        })
    })

    test.describe('enterprise', () => {
        test.beforeEach(({ sg }) => {
            sg.signIn()

            sg.mockTypes({
                Site: () => ({
                    codyContextFilters: {
                        raw: null,
                    },
                }),
            })
        })

        test.fixme('disabled when disabled on instance', async ({ page, sg }) => {
            // These tests seem to take longer than the default timeout
            test.setTimeout(10000)

            sg.setWindowContext({
                codyEnabledOnInstance: false,
            })

            await page.goto(path)
            await doesNotHaveCody(page)
        })

        test.fixme('disabled when disabled for user', async ({ page, sg }) => {
            // These tests seem to take longer than the default timeout
            test.setTimeout(10000)

            sg.setWindowContext({
                codyEnabledOnInstance: true,
                codyEnabledForCurrentUser: false,
            })

            await page.goto(path)
            await doesNotHaveCody(page)
        })

        test('enabled for user', async ({ page, sg }) => {
            // teardown takes longer than default timeout
            test.setTimeout(10000)

            sg.setWindowContext({
                codyEnabledOnInstance: true,
                codyEnabledForCurrentUser: true,
            })

            await page.goto(path)
            await hasCody(page)
        })

        test('disabled for excluded repo', async ({ page, sg }) => {
            sg.setWindowContext({
                codyEnabledOnInstance: true,
                codyEnabledForCurrentUser: true,
            })
            sg.mockTypes({
                Site: () => ({
                    codyContextFilters: {
                        raw: {
                            include: [String.raw`source.*`],
                        },
                    },
                }),
            })

            await page.goto(path)
            await doesNotHaveCody(page)
        })

        test('disabled with invalid context filter', async ({ page, sg }) => {
            sg.setWindowContext({
                codyEnabledOnInstance: true,
                codyEnabledForCurrentUser: true,
            })
            sg.mockTypes({
                Site: () => ({
                    codyContextFilters: {
                        raw: {
                            include: [String.raw`*`],
                        },
                    },
                }),
            })

            await page.goto(path)
            await doesNotHaveCody(page)
        })
    })
})
