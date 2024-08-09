import { test, expect } from '$testing/integration'

import { communityPageConfigs } from './config'

test.describe('community', async () => {
    test.beforeEach(async ({ sg }) => {
        await sg.dotcomMode()
        sg.mockOperations({
            CommunitySearchPage_SearchContext() {
                return {
                    searchContextBySpec: {
                        repositories: [
                            { repository: { name: 'repo1' } },
                            { repository: { name: 'repo2' } },
                            { repository: { name: 'repo3' } },
                        ],
                    },
                }
            },
        })
    })

    for (const [name, config] of Object.entries(communityPageConfigs)) {
        test(name, async ({ page }) => {
            await page.goto(`/${name}`)

            await test.step('correct heading is shown', async () => {
                await expect(page.getByRole('heading', { name: config.title, level: 2 })).toBeVisible()
            })

            await test.step('search input is prefilled with search context', async () => {
                await expect(page.getByRole('textbox')).toHaveText(new RegExp(`^context:${config.spec} `))
            })

            // We are not testing description because it may contain markdown which
            // is more difficult to test for

            if (config.lowProfile) {
                await test.step('low profile mode', async () => {
                    await expect(page.getByTestId('page.community.examples')).not.toBeVisible()
                    await expect(page.getByTestId('page.community.repositories')).not.toBeVisible()
                })
            } else {
                if (config.examples.length > 0) {
                    await test.step('search examples are listed', async () => {
                        await expect(page.getByTestId('page.community.examples').getByRole('listitem')).toHaveCount(
                            config.examples.length
                        )
                    })
                } else {
                    await expect(page.getByRole('heading', { name: 'Search examples' })).not.toBeVisible()
                }

                await test.step('repositories are listed', async () => {
                    // Repositories are rendered
                    await expect(page.getByTestId('page.community.repositories').getByRole('listitem')).toHaveCount(3)
                })
            }
        })
    }

    test('show repository resolution error', async ({ page, sg }) => {
        await page.goto('/backstage')

        sg.mockOperations({
            CommunitySearchPage_SearchContext() {
                throw new Error('Test error')
            },
        })

        await expect(page.getByText('Test error')).toBeVisible()
    })
})
