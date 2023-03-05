import { fetchCache } from '@sourcegraph/common'
import { GraphQLResult } from '@sourcegraph/http-client'

import { OptionFlagValues } from '../../shared/util/optionFlags'

export interface PhabricatorMapping {
    callsign: string
    path: string
}

/**
 * The feature flags available.
 */
export interface FeatureFlags {
    /**
     * Allow error reporting.
     *
     * @todo Since this is not really a feature flag, just unnest it into settings (and potentially get rid of the feature flags abstraction completely)
     */
    allowErrorReporting: boolean

    /**
     * Send telemetry
     */
    sendTelemetry: boolean
}

export const featureFlagDefaults: FeatureFlags = {
    allowErrorReporting: false,
    sendTelemetry: true,
}

interface SourcegraphURL {
    /**
     * Current connected/active sourcegraph URL
     */
    sourcegraphURL: string
    /**
     * All previously successfully used sourcegraph URLs
     */
    previouslyUsedURLs?: string[]
}

export interface SyncStorageItems extends SourcegraphURL {
    sourcegraphAnonymousUid: string
    /**
     * Temporarily disable the browser extension features.
     */
    disableExtension: boolean
    /**
     * Storage for feature flags.
     */
    featureFlags: Partial<OptionFlagValues>
    /**
     * Overrides settings from Sourcegraph.
     */
    clientSettings: string
}

export interface LocalStorageItems {}

export interface ManagedStorageItems extends SourcegraphURL {
    phabricatorMappings: PhabricatorMapping[]
}

/**
 * Functions in the background page that can be invoked from content scripts.
 */
export interface BackgroundPageApi {
    openOptionsPage(): Promise<void>
    createBlobURL(bundleUrl: string): Promise<string>
    requestGraphQL<T, V = object>(options: {
        request: string
        variables: V
        sourcegraphURL?: string
    }): Promise<GraphQLResult<T>>
    notifyRepoSyncError(payload: { sourcegraphURL: string; hasRepoSyncError: boolean }): Promise<void>
    checkRepoSyncError(payload: { tabId: number; sourcegraphURL: string }): Promise<boolean>
    fetchCache: typeof fetchCache
}

/**
 * Shape of the handler object in the background page.
 * The handlers get access to the sender tab of the message as a parameter.
 */
export type BackgroundPageApiHandlers = {
    [M in keyof BackgroundPageApi]: (
        payload: Parameters<BackgroundPageApi[M]>[0],
        sender: browser.runtime.MessageSender
    ) => ReturnType<BackgroundPageApi[M]>
}
