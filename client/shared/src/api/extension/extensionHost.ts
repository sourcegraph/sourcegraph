import * as comlink from 'comlink'
import { ReplaySubject, Subscription, Unsubscribable } from 'rxjs'
import * as sourcegraph from 'sourcegraph'

import { EndpointPair } from '../../platform/context'
import { SettingsCascade } from '../../settings/settings'
import { ClientAPI } from '../client/api/api'
import { registerComlinkTransferHandlers } from '../util'

import { activateExtensions, replaceAPIRequire } from './activation'
import { ExtensionHostAPI, ExtensionHostAPIFactory } from './api/api'
import { setActiveLoggers } from './api/logging'
import { createExtensionAPIFactory } from './extensionApi'
import { createExtensionHostAPI } from './extensionHostApi'
import { createExtensionHostState } from './extensionHostState'

/**
 * Required information when initializing an extension host.
 */
export interface InitData {
    /** @see {@link module:sourcegraph.internal.sourcegraphURL} */
    sourcegraphURL: string

    /** @see {@link module:sourcegraph.internal.clientApplication} */
    clientApplication: 'sourcegraph' | 'other'

    /** fetched initial settings object */
    initialSettings: Readonly<SettingsCascade<object>>
}
/**
 * Starts the extension host, which runs extensions. It is a Web Worker or other similar isolated
 * JavaScript execution context. There is exactly 1 extension host, and it has zero or more
 * extensions activated (and running).
 *
 * It expects to receive a message containing {@link InitData} from the client application as the
 * first message.
 *
 * @param endpoints The endpoints to the client.
 * @returns An unsubscribable to terminate the extension host.
 */
export function startExtensionHost(
    endpoints: EndpointPair
): Unsubscribable & { extensionAPI: Promise<typeof sourcegraph> } {
    const subscription = new Subscription()

    // Wait for "initialize" message from client application before proceeding to create the
    // extension host.
    let initialized = false
    const extensionAPI = new Promise<typeof sourcegraph>(resolve => {
        const factory: ExtensionHostAPIFactory = initData => {
            if (initialized) {
                throw new Error('extension host is already initialized')
            }
            initialized = true
            const {
                subscription: extensionHostSubscription,
                extensionAPI,
                extensionHostAPI,
            } = initializeExtensionHost(endpoints, initData)
            subscription.add(extensionHostSubscription)
            resolve(extensionAPI)
            return extensionHostAPI
        }
        comlink.expose(factory, endpoints.expose)
    })

    return { unsubscribe: () => subscription.unsubscribe(), extensionAPI }
}

/**
 * Initializes the extension host using the {@link InitData} from the client application. It is
 * called by {@link startExtensionHost} after the {@link InitData} is received.
 *
 * The extension API is made globally available to all requires/imports of the "sourcegraph" module
 * by other scripts running in the same JavaScript context.
 *
 * @param endpoints The endpoints to the client.
 * @param initData The information to initialize this extension host.
 * @returns An unsubscribable to terminate the extension host.
 */
function initializeExtensionHost(
    endpoints: EndpointPair,
    initData: InitData
): { extensionHostAPI: ExtensionHostAPI; extensionAPI: typeof sourcegraph; subscription: Subscription } {
    const subscription = new Subscription()

    const {
        extensionAPI,
        extensionHostAPI,
        subscription: apiSubscription,
    } = createExtensionAndExtensionHostAPIs(initData, endpoints)
    subscription.add(apiSubscription)

    // Make `import 'sourcegraph'` or `require('sourcegraph')` return the default extension API (for testing).
    replaceAPIRequire(extensionAPI)
    subscription.add(() => {
        globalThis.require = (() => {
            // Prevent callers from attempting to access the extension API after it was
            // unsubscribed.
            throw new Error('require: Sourcegraph extension API was unsubscribed')
        }) as any
    })

    return { subscription, extensionAPI, extensionHostAPI }
}

function createExtensionAndExtensionHostAPIs(
    initData: InitData,
    endpoints: Pick<EndpointPair, 'proxy'>
): { extensionHostAPI: ExtensionHostAPI; extensionAPI: typeof sourcegraph; subscription: Subscription } {
    const subscription = new Subscription()

    // EXTENSION HOST WORKER

    registerComlinkTransferHandlers()

    /**
     * Used to wait until the main thread API has been initialized. Ensures
     * that message of main thread API calls (e.g. getActiveExtensions)
     * during extension host initialization are not dropped.
     *
     * Debt: ensure this works holds true for all clients.
     * If not, add `waitForMainThread` parameter to make this opt-in.
     */
    const mainThreadAPIInitializations = new ReplaySubject<boolean>(1)

    /** Proxy to main thread */
    const proxy = comlink.wrap<ClientAPI>(endpoints.proxy)

    // Create extension host state
    const extensionHostState = createExtensionHostState(initData, proxy, mainThreadAPIInitializations)
    // Create extension host API
    const extensionHostAPINew = createExtensionHostAPI(extensionHostState)
    // Create extension API factory
    const createExtensionAPI = createExtensionAPIFactory(extensionHostState, proxy, initData)

    // Activate extensions. Create extension APIs on extension activation.
    subscription.add(activateExtensions(extensionHostState, proxy, createExtensionAPI, mainThreadAPIInitializations))

    // Observe settings and update active loggers state
    subscription.add(setActiveLoggers(extensionHostState))

    // Expose the extension host API to the client (main thread)
    const extensionHostAPI: ExtensionHostAPI = {
        [comlink.proxyMarker]: true,

        ping: () => 'pong',
        mainThreadAPIInitialized: () => {
            mainThreadAPIInitializations.next(true)
        },
        ...extensionHostAPINew,
    }

    // Create a default extension API (for testing)
    const extensionAPI = createExtensionAPI('DEFAULT')

    return { extensionHostAPI, extensionAPI, subscription }
}
