import { combineLatest, ReplaySubject } from 'rxjs'
import { map } from 'rxjs/operators'

import { isHTTPAuthError } from '@sourcegraph/http-client'
import { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import { EMPTY_SETTINGS_CASCADE, gqlToCascade, SettingsSubject } from '@sourcegraph/shared/src/settings/settings'
import { toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'

import { createGraphQLHelpers } from '../backend/requestGraphQl'
import { CodeHost } from '../code-hosts/shared/codeHost'

import { fetchViewerSettings, mergeCascades, storageSettingsCascade } from './settings'

export interface SourcegraphIntegrationURLs {
    /**
     * The URL of the configured Sourcegraph instance. Used for extensions, find-references, ...
     */
    sourcegraphURL: string

    /**
     * The base URL where assets will be fetched from (CSS, bundle, ...)
     *
     * This is the sourcegraph URL in most cases, but may be different for
     * native code hosts that self-host the integration bundle.
     */
    assetsURL: string
}

/**
 * The PlatformContext provided in the browser (for browser extensions and native integrations).
 */
export interface BrowserPlatformContext extends PlatformContext {
    /**
     * Re-fetches the settings cascade from the Sourcegraph instance.
     */
    refreshSettings(): Promise<void>
}

/**
 * Creates the {@link PlatformContext} for the browser (for browser extensions and native integrations)
 */
export function createPlatformContext(
    { urlToFile }: Pick<CodeHost, 'urlToFile'>,
    { sourcegraphURL }: SourcegraphIntegrationURLs,
    isExtension: boolean
): BrowserPlatformContext {
    const updatedViewerSettings = new ReplaySubject<{
        final: string
        subjects: SettingsSubject[]
    }>(1)
    const { requestGraphQL, getBrowserGraphQLClient } = createGraphQLHelpers(sourcegraphURL, isExtension)

    const context: BrowserPlatformContext = {
        /**
         * The active settings cascade.
         *
         * - For unauthenticated users, this is the GraphQL settings plus client settings (which are stored locally in the browser extension).
         * - For authenticated users, this is just the GraphQL settings (client settings are ignored to simplify the UX).
         */
        settings: combineLatest([updatedViewerSettings, storageSettingsCascade]).pipe(
            map(([gqlCascade, storageCascade]) =>
                gqlCascade
                    ? mergeCascades(
                          gqlToCascade(gqlCascade),
                          gqlCascade.subjects.some(subject => subject.__typename === 'User')
                              ? EMPTY_SETTINGS_CASCADE
                              : storageCascade
                      )
                    : EMPTY_SETTINGS_CASCADE
            )
        ),
        refreshSettings: async () => {
            try {
                const settings = await fetchViewerSettings(requestGraphQL).toPromise()
                updatedViewerSettings.next(settings)
            } catch (error) {
                if (isHTTPAuthError(error)) {
                    // User is not signed in
                    console.warn(
                        `Could not fetch Sourcegraph settings from ${sourcegraphURL} because user is not signed into Sourcegraph`
                    )
                    updatedViewerSettings.next({ final: '{}', subjects: [] })
                } else {
                    throw error
                }
            }
        },
        updateSettings: () => {
            throw new Error('not implemented')
        },
        requestGraphQL,
        getGraphQLClient: getBrowserGraphQLClient,
        urlToFile: ({ rawRepoName, ...target }, context) => {
            // We don't always resolve the rawRepoName, e.g. if there are multiple definitions.
            // Construct URL to file on code host, if possible.
            if (rawRepoName && urlToFile) {
                return urlToFile(sourcegraphURL, { rawRepoName, ...target }, context)
            }
            // Otherwise fall back to linking to Sourcegraph (with an absolute URL).
            return `${sourcegraphURL}${toPrettyBlobURL(target)}`
        },
        sourcegraphURL,
    }
    return context
}
