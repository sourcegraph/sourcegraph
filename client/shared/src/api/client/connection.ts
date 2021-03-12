import * as comlink from 'comlink'
import { from, Subscription } from 'rxjs'
import { first } from 'rxjs/operators'
import { Unsubscribable } from 'sourcegraph'
import { PlatformContext, ClosableEndpointPair } from '../../platform/context'
import { ExtensionHostAPIFactory } from '../extension/api/api'
import { InitData } from '../extension/extensionHost'
import { ClientAPI } from './api/api'
import { registerComlinkTransferHandlers } from '../util'
import { ExposedToClient, initMainThreadAPI } from './mainthread-api'
import { isSettingsValid } from '../../settings/settings'
import { FlatExtensionHostAPI, MainThreadAPI } from '../contract'

export interface ExtensionHostClientConnection {
    /**
     * Closes the connection to and terminates the extension host.
     */
    unsubscribe(): void
}

/**
 * An activated extension.
 */
export interface ActivatedExtension {
    /**
     * The extension's extension ID (which uniquely identifies it among all activated extensions).
     */
    id: string

    /**
     * Deactivate the extension (by calling its "deactivate" function, if any).
     */
    deactivate(): void | Promise<void>
}

/**
 * @param endpoints The Worker object to communicate with
 */
export async function createExtensionHostClientConnection(
    endpointsPromise: Promise<ClosableEndpointPair>,
    initData: Omit<InitData, 'initialSettings'>,
    platformContext: Pick<
        PlatformContext,
        | 'settings'
        | 'updateSettings'
        | 'requestGraphQL'
        | 'telemetryService'
        | 'sideloadedExtensionURL'
        | 'getScriptURLForExtension'
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

    const initialSettings = await from(platformContext.settings).pipe(first()).toPromise()
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

    // TODO(tj): return MainThreadAPI and add to Controller interface
    // to allow app to interact with APIs whose state lives in the main thread
    return { subscription, api: proxy, mainThreadAPI: newAPI, exposedToClient }
}
