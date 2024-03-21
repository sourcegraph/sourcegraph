import type { ContentMatch } from '$lib/shared'

import { test, expect } from '../../testing/integration'
import {
    createDoneEvent,
    createProgressEvent,
    createCommitMatch,
    createContentMatch,
    createPathMatch,
    createSymbolMatch,
} from '../../testing/search-testdata'

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

test('shows suggestions', async ({ sg, page }) => {
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

test('main navbar menus are visible above search input', async ({ page, sg }) => {
    const stream = sg.mockSearchStream()
    await page.goto('/search?q=test')
    await stream.publish(createProgressEvent(), createDoneEvent())
    await stream.close()
    await page.getByRole('button', { name: 'Code Search' }).click()
    await page.getByRole('link', { name: 'Search Home' }).click()
    await expect(page).toHaveURL(/\/search$/)
})

test.use({
    permissions: ['clipboard-write', 'clipboard-read'],
})
test('copy path button appears and copies path', async ({ page, sg }) => {
    const stream = sg.mockSearchStream()
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

    const copyPathButton = page.getByRole('button', { name: 'Copy path to clipboard' })

    for (const match of [contentMatch, pathMatch, symbolMatch]) {
        await page.getByRole('link', { name: match.path }).hover()
        expect(copyPathButton).toBeVisible()
        await copyPathButton.click()
        let clipboardText = await page.evaluate('navigator.clipboard.readText()')
        expect(clipboardText).toBe(match.path)
    }
})

test.describe('preview panel', async () => {
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
    test('can be opened and closed', async ({ page, sg }) => {
        const stream = sg.mockSearchStream()
        await page.goto('/search?q=test')
        await page.getByRole('heading', { name: 'Filter results' }).waitFor()
        sg.mockOperations({
            BlobPageQuery: () => ({
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
        const stream = sg.mockSearchStream()
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
            BlobPageQuery: () => ({
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
