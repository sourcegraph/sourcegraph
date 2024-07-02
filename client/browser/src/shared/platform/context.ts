import { combineLatest, lastValueFrom, ReplaySubject } from 'rxjs'
import { map } from 'rxjs/operators'

import { asError } from '@sourcegraph/common'
import { isHTTPAuthError } from '@sourcegraph/http-client'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import { mutateSettings, updateSettings } from '@sourcegraph/shared/src/settings/edit'
import { EMPTY_SETTINGS_CASCADE, gqlToCascade, type SettingsSubject } from '@sourcegraph/shared/src/settings/settings'
import { toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'

import { createGraphQLHelpers } from '../backend/requestGraphQl'
import type { CodeHost } from '../code-hosts/shared/codeHost'
import { noOpTelemetryRecorder } from '../telemetry'

import { createExtensionHost } from './extensionHost'
import { getInlineExtensions } from './inlineExtensionsService'
import { editClientSettings, fetchViewerSettings, mergeCascades, storageSettingsCascade } from './settings'

export interface SourcegraphIntegrationURLs {
    /**
     * The URL of the configured Sourcegraph instance. Used for extensions, find-references, ...
     */
    sourcegraphURL: string

    /**
     * The base URL where assets will be fetched from (CSS, extension host
     * worker bundle, ...)
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
    { sourcegraphURL, assetsURL }: SourcegraphIntegrationURLs,
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
                const settings = await lastValueFrom(fetchViewerSettings(requestGraphQL))
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
        updateSettings: async (subject, edit) => {
            if (subject === 'Client') {
                // Support storing settings on the client (in the browser extension) so that unauthenticated
                // Sourcegraph viewers can update settings.
                await updateSettings(context, subject, edit, () => editClientSettings(edit))
                return
            }

            try {
                await updateSettings(context, subject, edit, mutateSettings)
            } catch (error) {
                if (asError(error).message.includes('version mismatch')) {
                    // The user probably edited the settings in another tab, so
                    // try once more.
                    await context.refreshSettings()
                    await updateSettings(context, subject, edit, mutateSettings)
                } else {
                    throw error
                }
            }
            // TODO: We shouldn't need to make another HTTP request to get the latest state
            await context.refreshSettings()
        },
        requestGraphQL,
        getGraphQLClient: getBrowserGraphQLClient,
        createExtensionHost: () => createExtensionHost({ assetsURL }),
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
        clientApplication: 'other',
        getStaticExtensions: () => getInlineExtensions(assetsURL),
        /**
         * This will be replaced by a real telemetry recorder in codeHost.tsx.
         */
        telemetryRecorder: noOpTelemetryRecorder,
    }
    return context
}
