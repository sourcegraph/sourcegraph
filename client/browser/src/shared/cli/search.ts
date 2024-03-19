import { firstValueFrom } from 'rxjs'

import { type ErrorLike, isErrorLike, isDefined, isNot } from '@sourcegraph/common'
import type { Settings } from '@sourcegraph/shared/src/settings/settings'
import { createURLWithUTM } from '@sourcegraph/shared/src/tracking/utm'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'

import { SearchPatternType } from '../../graphql-operations'
import { createSuggestionFetcher } from '../backend/search'
import { createPlatformContext } from '../platform/context'
import { observeSourcegraphURL, getAssetsURL, DEFAULT_SOURCEGRAPH_URL, getPlatformName } from '../util/context'

const isURL = /^https?:\/\//
const IS_EXTENSION = true // This feature is only supported in browser extension

export class SearchCommand {
    public description = 'Enter a search query'

    private suggestionFetcher = createSuggestionFetcher()

    private prev: { query: string; suggestions: browser.omnibox.SuggestResult[] } = { query: '', suggestions: [] }

    public getSuggestions = async (query: string): Promise<browser.omnibox.SuggestResult[]> => {
        const sourcegraphURL = await firstValueFrom(observeSourcegraphURL(IS_EXTENSION))
        return new Promise(resolve => {
            if (this.prev.query === query) {
                resolve(this.prev.suggestions)
                return
            }

            this.suggestionFetcher({
                sourcegraphURL,
                queries: [`${query} type:repo count:5`, `${query} type:path count:5`, `${query} type:symbol count:5`],
                handler: suggestions => {
                    const built = suggestions.map(({ title, url, urlLabel }) => ({
                        content: `${sourcegraphURL}${url}`,
                        description: `${title} - ${urlLabel}`,
                    }))

                    this.prev = {
                        query,
                        suggestions: built,
                    }

                    resolve(built)
                },
            })
        })
    }

    public action = async (
        query: string,
        disposition?: 'newForegroundTab' | 'newBackgroundTab' | 'currentTab',
        currentTabId?: number
    ): Promise<void> => {
        const sourcegraphURL = await firstValueFrom(observeSourcegraphURL(IS_EXTENSION))

        const [patternType, caseSensitive] = await this.getDefaultSearchSettings(sourcegraphURL)

        const props = {
            url: isURL.test(query)
                ? query
                : createURLWithUTM(
                      new URL(`${sourcegraphURL}/search?${buildSearchURLQuery(query, patternType, caseSensitive)}`),
                      {
                          utm_source: getPlatformName(),
                          utm_campaign: 'omnibox',
                      }
                  ).toString(),
        }

        switch (disposition) {
            case 'currentTab': {
                if (currentTabId) {
                    // Note: this is done in order to blur browser omnibox and set focus on page
                    await browser.tabs
                        .get(currentTabId)
                        .then(currentTab => currentTab.index)
                        .then(currentTabIndex => browser.tabs.create({ ...props, index: currentTabIndex }))
                        .then(() => browser.tabs.remove(currentTabId))
                    break
                }

                await browser.tabs.update(props)
                break
            }
            case 'newForegroundTab': {
                await browser.tabs.create(props)
                break
            }
            case 'newBackgroundTab': {
                await browser.tabs.create({ ...props, active: false })
                break
            }
        }
    }

    private lastSourcegraphUrl = ''
    private settingsTimeoutHandler = 0
    private defaultPatternType = SearchPatternType.standard
    private defaultCaseSensitive = false
    private readonly settingsTimeoutDuration = 60 * 60 * 1000 // one hour

    // Returns the pattern type to use, and whether the query should be treated case sensitively.
    private async getDefaultSearchSettings(sourcegraphURL: string): Promise<[SearchPatternType, boolean]> {
        try {
            // Refresh settings when either:
            // - First search
            // - Sourcegraph URL changes
            // - Over an hour has passed since last refresh

            if (this.lastSourcegraphUrl !== sourcegraphURL || this.settingsTimeoutHandler === 0) {
                clearTimeout(this.settingsTimeoutHandler)
                this.settingsTimeoutHandler = 0

                const platformContext = createPlatformContext(
                    { urlToFile: undefined },
                    { sourcegraphURL, assetsURL: getAssetsURL(DEFAULT_SOURCEGRAPH_URL) },
                    IS_EXTENSION
                )

                await platformContext.refreshSettings()
                const settings = (await firstValueFrom(platformContext.settings)).final

                if (isDefined(settings) && isNot<ErrorLike | Settings, ErrorLike>(isErrorLike)(settings)) {
                    this.defaultPatternType =
                        (settings['search.defaultPatternType'] as SearchPatternType) || this.defaultPatternType
                    this.defaultCaseSensitive =
                        (settings['search.defaultCaseSensitive'] as boolean) || this.defaultCaseSensitive
                }

                this.lastSourcegraphUrl = sourcegraphURL
                this.settingsTimeoutHandler = window.setTimeout(() => {
                    this.settingsTimeoutHandler = 0
                }, this.settingsTimeoutDuration)
            }
        } catch {
            // Ignore errors trying to get settings, fall to return default below
        }

        return [this.defaultPatternType, this.defaultCaseSensitive]
    }
}
