import type { ContentMatch } from '$lib/shared'
import { test, expect, type Page, Sourcegraph, type Locator } from '$testing/integration'
import {
    createDoneEvent,
    createProgressEvent,
    createCommitMatch,
    createContentMatch,
    createPathMatch,
    createSymbolMatch,
    createRepositoryMatch,
} from '$testing/search-testdata'

const chunkMatch: ContentMatch = {
    type: 'content',
    path: 'README.md',
    pathMatches: [],
    repository: 'github.com/sourcegraph/conc',
    repoStars: 9001,
    commit: 'abcde12345',
    chunkMatches: [
        {
            content: 'lorem ipsum\ndolor sit\namet',
            contentStart: { offset: 0, line: 1, column: 1 },
            ranges: [
                {
                    // "lorem"
                    start: { offset: 0, line: 0, column: 0 },
                    end: { offset: 5, line: 0, column: 5 },
                },
                {
                    // "sit"
                    start: { offset: 18, line: 1, column: 6 },
                    end: { offset: 21, line: 1, column: 9 },
                },
            ],
        },
    ],
    language: 'text',
}

test('search input is autofocused', async ({ page }) => {
    await page.goto('/search')
    const searchInput = page.getByRole('textbox')
    const suggestions = page.getByLabel('Narrow your search')
    await expect(searchInput).toBeFocused()

    // Doesn't show suggestions without user interaction
    await expect(suggestions).not.toBeVisible()

    await searchInput.click()
    await expect(suggestions).toBeVisible()
})

test.describe('page.spec.ts', () => {
    test.beforeEach(async ({ sg, page }) => {
        sg.mockOperations({
            Init: () => ({
                currentUser: null,
                viewerSettings: {
                    final: '{"experimentalFeatures":{"enableLazyBlobSyntaxHighlighting":true,"newSearchResultFiltersPanel":true,"newSearchResultsUI":true,"proactiveSearchResultsAggregations":true,"searchResultsAggregations":true,"showMultilineSearchConsole":true}}',
                },
            }),
        })
    })

    test.skip('shows suggestions', async ({ page, sg }) => {
        await page.goto('/search')
        const searchInput = page.getByRole('textbox')
        await searchInput.click()

        // Default suggestions
        await expect(page.getByLabel('Narrow your search')).toBeVisible()

        sg.mockTypes({
            SearchResults: () => ({
                repositories: [{ name: 'github.com/sourcegraph/sourcegraph' }],
                results: [
                    {
                        __typename: 'FileMatch',
                        file: {
                            path: 'sourcegraph.md',
                            url: '',
                        },
                    },
                ],
            }),
        })

        // Repo suggestions
        await searchInput.fill('source')
        await expect(page.getByLabel('Repositories')).toBeVisible()
        await expect(page.getByLabel('Files')).toBeVisible()

        // Fills suggestion
        await page.getByText('github.com/sourcegraph/sourcegraph').click()
        await expect(searchInput).toHaveText('repo:^github\\.com/sourcegraph/sourcegraph$ ')
    })

    test('submits search on enter', async ({ page }) => {
        await page.goto('/search')
        const searchInput = page.getByRole('textbox')
        await searchInput.fill('source')

        // Submit search
        await searchInput.press('Enter')
        await expect(page).toHaveURL(/\/search\?q=.+$/)
    })

    test('fills search query from URL', async ({ page }) => {
        await page.goto('/search?q=test')
        await expect(page.getByRole('textbox')).toHaveText('test')
    })
})

test.use({
    permissions: ['clipboard-write', 'clipboard-read'],
})
test('copy path button appears and copies path', async ({ page, sg }) => {
    const stream = await sg.mockSearchStream()
    await page.goto('/search?q=test')
    await page.getByRole('heading', { name: 'Filter results' }).waitFor()

    const contentMatch = createContentMatch()
    const pathMatch = createPathMatch()
    const symbolMatch = createSymbolMatch()

    await stream.publish(
        { type: 'matches', data: [contentMatch, pathMatch, symbolMatch] },
        createProgressEvent(),
        createDoneEvent()
    )
    await stream.close()

    for (const match of [contentMatch, pathMatch, symbolMatch]) {
        await page.getByText(match.path).hover()
        const copyPathButton = page
            .locator('article')
            .filter({ hasText: match.path })
            .getByLabel('Copy path to clipboard')
        await expect(copyPathButton).toBeVisible()
        await copyPathButton.click()
        const clipboardText = await page.evaluate('navigator.clipboard.readText()')
        expect(clipboardText).toBe(match.path)
    }
})

test.describe('preview panel', async () => {
    test('can be opened and closed', async ({ page, sg }) => {
        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        sg.mockOperations({
            BlobFileViewBlobQuery: () => ({
                repository: { commit: { blob: { content: chunkMatch.chunkMatches![0].content } } },
            }),
        })

        await stream.publish(
            {
                type: 'matches',
                data: [chunkMatch, createCommitMatch(), createPathMatch()],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        // 2 preview buttons: one for content match and one for path match
        const previewButtons = await page.getByRole('button', { name: 'Preview' }).all()
        expect(previewButtons).toHaveLength(2)

        // Open preview panel
        await previewButtons[0].click()
        await expect(page.getByRole('heading', { name: 'File Preview' })).toBeVisible()

        // Close preview panel
        await page.getByTestId('preview-close').click()
        await expect(page.getByRole('heading', { name: 'File Preview' })).toBeHidden()
    })

    test('can iterate over matches', async ({ page, sg }) => {
        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        await stream.publish(
            {
                type: 'matches',
                data: [chunkMatch],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        sg.mockOperations({
            BlobFileViewBlobQuery: () => ({
                repository: { commit: { blob: { content: chunkMatch.chunkMatches![0].content } } },
            }),
        })

        // Open preview panel
        await page.getByRole('button', { name: 'Preview' }).click()

        const currentSelection = page.locator('span.cm-sg-static-highlight-selected')
        const nextButton = page.locator('button[aria-label="next result"]')
        const previousButton = page.locator('button[aria-label="previous result"]')

        await expect(currentSelection, 'the first match is selected by default').toHaveText('lorem')

        await nextButton.click()
        await expect(currentSelection, 'clicking next should select the next match').toHaveText('sit')

        await previousButton.click()
        await expect(currentSelection, 'clicking previous should select the previous match').toHaveText('lorem')

        await previousButton.click()
        await expect(currentSelection, 'clicking previous from the first result wraps backwards').toHaveText('sit')

        await nextButton.click()
        await expect(currentSelection, 'clicking next on the last result should wrap forwards').toHaveText('lorem')
    })
})

test.describe('search results', async () => {
    test('first result visible', async ({ page, sg }) => {
        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        await stream.publish(
            {
                type: 'matches',
                data: [chunkMatch],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        const chunkPath = page.getByRole('link', { name: 'README.md' })
        await expect(chunkPath).toBeVisible()
    })

    test('alert is shown', async ({ page, sg }) => {
        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        await stream.publish(
            {
                type: 'alert',
                data: {
                    title: 'Test alert',
                    description: 'Test description',
                    proposedQueries: null,
                },
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        const alert = page.getByRole('heading', { name: 'Test alert' })
        await expect(alert).toBeVisible()
    })

    test('toggle regexp', async ({ page, sg }) => {
        sg.mockOperations({
            Init: () => ({
                currentUser: null,
                viewerSettings: {
                    final: '{"search.defaultPatternType": "standard"}',
                },
            }),
        })

        await page.goto('/search?q=test')

        const searchInput = page.getByRole('textbox')
        const regexpToggle = page.getByTitle('regexp toggle')

        // Toggle regexp on and submit search
        await regexpToggle.click()
        await searchInput.press('Enter')
        await expect(page).toHaveURL(/\/search\?q=test&patternType=regexp&sm=0/)

        // Toggle regexp off and submit: should use default pattern type
        await regexpToggle.click()
        await searchInput.press('Enter')
        await expect(page).toHaveURL(/\/search\?q=test&patternType=standard&sm=0/)
    })

    test('focus shortcuts', async ({ page, sg }) => {
        sg.mockOperations({
            HighlightedFile: () => ({
                repository: {
                    commit: {
                        blob: {
                            highlight: {
                                aborted: true,
                                lineRanges: [],
                            },
                        },
                    },
                },
            }),
        })
        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        const contentMatch = createContentMatch()
        contentMatch.chunkMatches = contentMatch.chunkMatches?.slice(0, 2)
        const pathMatch = createPathMatch()
        const repoMatch = createRepositoryMatch()
        const symbolMatch = createSymbolMatch()
        const commitMatch = createCommitMatch('commit')

        await stream.publish(
            {
                type: 'matches',
                data: [contentMatch, pathMatch, repoMatch, symbolMatch, commitMatch],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        function firstLine(s: string): string {
            return s.split('\n')[0].trim()
        }

        // Iterate downwards through each result type
        {
            for (const chunkMatch of contentMatch.chunkMatches?.slice(0, 2) ?? []) {
                await expect(page.locator('*:focus')).toContainText(firstLine(chunkMatch.content))
                await page.keyboard.press('j')
            }

            await expect(page.locator('*:focus')).toContainText(pathMatch.path.split('/').at(-1) ?? '')
            await page.keyboard.press('ArrowDown') // Check a down arrow too

            await expect(page.locator('*:focus')).toContainText(repoMatch.repository.split('/').slice(1).join('/'))
            await page.keyboard.press('j')

            for (const symbol of symbolMatch.symbols) {
                await expect(page.locator(`*:focus [data-line="${symbol.line + 1}"]`)).toBeVisible()
                await page.keyboard.press('j')
            }

            await expect(page.locator('*:focus')).toContainText(firstLine(commitMatch.message))
            await page.keyboard.press('j')

            // Pressing down on the last result keeps us on the last result
            await expect(page.locator('*:focus')).toContainText(firstLine(commitMatch.message))
        }

        // Go in reverse, iterating up through the results
        {
            for (const symbol of symbolMatch.symbols.reverse()) {
                await page.keyboard.press('k')
                await expect(page.locator(`*:focus [data-line="${symbol.line + 1}"]`)).toBeVisible()
            }

            await page.keyboard.press('k')
            await expect(page.locator('*:focus')).toContainText(repoMatch.repository.split('/').slice(1).join('/'), {
                useInnerText: true,
            })

            await page.keyboard.press('ArrowUp') // Check an up arrow too
            await expect(page.locator('*:focus')).toContainText(pathMatch.path.split('/').at(-1) ?? '')

            for (const chunkMatch of contentMatch.chunkMatches?.slice(0, 2)?.reverse() ?? []) {
                await page.keyboard.press('k')
                await expect(page.locator('*:focus')).toContainText(firstLine(chunkMatch.content))
            }

            // Pressing up on the top result stays on the top result
            await page.keyboard.press('k')
            await expect(page.locator('*:focus')).toContainText(firstLine(contentMatch.chunkMatches?.[0].content ?? ''))
        }
    })
})

test.describe('search filters', async () => {
    test('type filters are always visible', async ({ page, sg }) => {
        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        await stream.publish(
            {
                type: 'matches',
                data: [chunkMatch],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        for (const typeFilter of ['Code', 'Repositories', 'Paths', 'Symbols', 'Commits', 'Diffs']) {
            await expect(page.getByRole('link', { name: typeFilter })).toBeVisible()
        }
    })

    test('snippets are shown', async ({ page, sg }) => {
        sg.mockOperations({
            Init: () => ({
                currentUser: null,
                viewerSettings: {
                    final: '{"search.scopes":[{"name":"Test snippet", "value": "repo:testsnippet"}]}',
                },
            }),
        })

        const stream = await sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        await stream.publish(
            {
                type: 'matches',
                data: [chunkMatch],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()

        await page.getByRole('link', { name: 'Test snippet' }).click()
        await page.waitForURL(/Test\+snippet/)
    })
})

test.describe('search jobs', async () => {
    let button: Locator
    let popover: Locator
    let createSearchJobHeading: Locator
    let createSearchJobButton: Locator

    async function setup(page: Page, sg: Sourcegraph): Promise<void> {
        const stream = await sg.mockSearchStream()
        await sg.signIn()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        await stream.publish(
            {
                type: 'matches',
                data: [chunkMatch],
            },
            createProgressEvent(),
            createDoneEvent()
        )
        await stream.close()
        await button.click()
        await expect(popover).toBeVisible()
    }

    test.beforeEach(async ({ page }) => {
        button = page.getByTestId('page.search-results.progress-button')
        popover = page.getByTestId('page.search-results.progress-popover')
        createSearchJobHeading = popover.getByRole('heading', { name: 'Create a search job' })
        createSearchJobButton = popover.getByRole('button', { name: 'Create a search job' })
    })

    test('not visible when search jobs is disabled', async ({ page, sg }) => {
        await sg.setWindowContext({ searchJobsEnabled: false })
        await setup(page, sg)
        await expect(createSearchJobHeading).toBeHidden()
    })

    test('visible when search jobs are enabled', async ({ page, sg }) => {
        await sg.setWindowContext({ searchJobsEnabled: true })
        await setup(page, sg)
        await expect(createSearchJobHeading).toBeVisible()
    })

    test('shows error if query validation fails', async ({ page, sg }) => {
        await sg.setWindowContext({ searchJobsEnabled: true })
        sg.mockOperations({
            ValidateSearchJob() {
                throw new Error('Test error')
            },
        })
        await setup(page, sg)
        await expect(popover.getByText('Test error')).toBeVisible()
        await expect(createSearchJobButton).toBeDisabled()
    })

    test('shows error if job creation fails', async ({ page, sg }) => {
        await sg.setWindowContext({ searchJobsEnabled: true })
        sg.mockOperations({
            CreateSearchJob() {
                throw new Error('Test error')
            },
        })
        await setup(page, sg)
        await createSearchJobButton.click()
        await expect(popover.getByText('Test error')).toBeVisible()
        await expect(createSearchJobButton).toBeEnabled()
    })

    test('redirects on job creation', async ({ page, sg }) => {
        await sg.setWindowContext({ searchJobsEnabled: true })
        await setup(page, sg)
        await createSearchJobButton.click()
        await expect(page).toHaveURL(/\/search-jobs/)
    })
})
