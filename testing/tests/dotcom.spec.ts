import { test, expect } from '@playwright/test'

const baseURL = 'https://sourcegraph.com'

test.describe('search smoke tests', () => {
    test.describe('react', () => {
        test('has title', async ({ page }) => {
            await page.goto(`${baseURL}/search`)
            await expect(page).toHaveTitle('Sourcegraph')
        })
    })

    test.describe('svelte', () => {
        const useSvelte = 'feat=web-next'
        test('has title', async ({ page }) => {
            await page.goto(`${baseURL}/search?${useSvelte}`)
            await expect(page).toHaveTitle('Sourcegraph')
        })

        test('navigate to repo via search', async ({ page }) => {
            await page.goto(
                `${baseURL}/search?${useSvelte}&q=context:global+repo:sgtest/weird&patternType=keyword&sm=0`
            )
            await page.getByRole('link', { name: 'sgtest/weird' }).click()
            await page.waitForURL(`${baseURL}/github.com/sgtest/weird`)
        })

        test('navigate to repo at a specific branch via search', async ({ page }) => {
            await page.goto(
                `${baseURL}/search?${useSvelte}&q=context:global+repo:sgtest/weird%40main&patternType=keyword&sm=0`
            )
            await page.getByRole('link', { name: 'sgtest/weird' }).click()
            await page.waitForURL(`${baseURL}/github.com/sgtest/weird@main`)
        })

        test('navigate to a file via search', async ({ page }) => {
            await page.goto(
                `${baseURL}/search?${useSvelte}&q=context:global+repo:%5Egithub%5C.com/sgtest/weird%24%40af4f7a3be2ba4be3a1804302f0ff97f56ac8130d+file:ALLCAPS&patternType=keyword&sm=0`
            )
            await page.getByRole('link', { name: 'filenames/ALLCAPS' }).click()
            await page.waitForURL(
                `${baseURL}/github.com/sgtest/weird@af4f7a3be2ba4be3a1804302f0ff97f56ac8130d/-/blob/filenames/ALLCAPS`
            )
        })
    })
})
