import { test, expect } from '../../testing/integration'
import {
    createDoneEvent,
    createProgressEvent,
    createCommitMatch,
    createContentMatch,
    createPathMatch,
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

test('preview can be opened and closed', async ({ page, sg }) => {
    const stream = sg.mockSearchStream()
    await page.goto('/search?q=test')
    await page.getByRole('heading', { name: 'Filter results' }).waitFor()
    await stream.publish(
        {
            type: 'matches',
            data: [createContentMatch(), createCommitMatch(), createPathMatch()],
        },
        createProgressEvent(),
        createDoneEvent()
    )
    await stream.close()

    // 2 preview buttons: one for content match and one for path match
    const previewButtons = await page.getByRole('button', { name: 'Preview' }).all()
    expect(previewButtons).toHaveLength(2)

    sg.mockOperations({
        BlobPageQuery: () => ({
            repository: {
                commit: {
                    blob: {
                        content: 'lorem\nipsum\ndolor\n',
                    },
                },
            },
        }),
    })

    // Open preview panel
    await previewButtons[0].click()
    await expect(page.getByRole('heading', { name: 'File Preview' })).toBeVisible()

    // Close preview panel
    await page.getByTestId('preview-close').click()
    await expect(page.getByRole('heading', { name: 'File Preview' })).toBeHidden()
})
