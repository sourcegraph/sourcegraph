import * as comlink from 'comlink'
import { firstValueFrom, Subscription, type Unsubscribable } from 'rxjs'

import { logger } from '@sourcegraph/common'

import type { PlatformContext, ClosableEndpointPair } from '../../platform/context'
import { isSettingsValid } from '../../settings/settings'
import type { FlatExtensionHostAPI, MainThreadAPI } from '../contract'
import type { ExtensionHostAPIFactory } from '../extension/api/api'
import type { InitData } from '../extension/extensionHost'
import { registerComlinkTransferHandlers } from '../util'

import type { ClientAPI } from './api/api'
import { type ExposedToClient, initMainThreadAPI } from './mainthread-api'

/**
 * @param endpoints The Worker object to communicate with
 */
export async function createExtensionHostClientConnection(
    endpointsPromise: Promise<ClosableEndpointPair>,
    initData: Omit<InitData, 'initialSettings'>,
    platformContext: Pick<
        PlatformContext,
        'settings' | 'updateSettings' | 'getGraphQLClient' | 'requestGraphQL' | 'telemetryService' | 'clientApplication'
    >
): Promise<{
    subscription: Unsubscribable
    api: comlink.Remote<FlatExtensionHostAPI>
    mainThreadAPI: MainThreadAPI
    exposedToClient: ExposedToClient
}> {
    const subscription = new Subscription()

    // MAIN THREAD

    registerComlinkTransferHandlers()

    const { endpoints, subscription: endpointsSubscription } = await endpointsPromise
    subscription.add(endpointsSubscription)

    /** Proxy to the exposed extension host API */
    const initializeExtensionHost = comlink.wrap<ExtensionHostAPIFactory>(endpoints.proxy)

    const initialSettings = await firstValueFrom(platformContext.settings)
    const proxy = await initializeExtensionHost({
        ...initData,
        // TODO what to do in error case?
        initialSettings: isSettingsValid(initialSettings) ? initialSettings : { final: {}, subjects: [] },
    })

    const { api: newAPI, exposedToClient, subscription: apiSubscriptions } = initMainThreadAPI(proxy, platformContext)

    subscription.add(apiSubscriptions)

    const clientAPI: ClientAPI = {
        ping: () => 'pong',
        ...newAPI,
    }

    comlink.expose(clientAPI, endpoints.expose)
    proxy.mainThreadAPIInitialized().catch(() => {
        logger.error('Error notifying extension host of main thread API init.')
    })

    // TODO(tj): return MainThreadAPI and add to Controller interface
    // to allow app to interact with APIs whose state lives in the main thread
    return { subscription, api: proxy, mainThreadAPI: newAPI, exposedToClient }
}
