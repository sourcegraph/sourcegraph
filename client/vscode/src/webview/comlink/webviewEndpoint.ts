import * as Comlink from 'comlink'
import { isObject } from 'lodash'

import type { EndpointPair } from '@sourcegraph/shared/src/platform/context'

import type { VsCodeApi } from '../../vsCodeApi'

import { generateUUID, isNestedConnection, type NestedConnectionData, type RelationshipType } from '.'

const panelId = self.document ? self.document.documentElement.dataset.panelId! : 'web-worker'

const endpointFactories: {
    webToWeb?: (connectionId: string) => Comlink.Endpoint
    webToNode?: (connectionId: string) => Comlink.Endpoint
} = {}

const vscodeWebviewProxyTransferHandler: Comlink.TransferHandler<
    object,
    // Send panelId (in serialize), no need to receive it.
    // Return proxyMarkedValue in serialize so we can expose it in postMessage, but
    // only end up sending the nestedConnectionId (and relationshipType for `webToWeb`)
    { nestedConnectionId: string; proxyMarkedValue?: object; relationshipType?: RelationshipType; panelId?: string }
> = {
    canHandle: (value): value is Comlink.ProxyMarked =>
        isObject(value) && (value as Comlink.ProxyMarked)[Comlink.proxyMarker],
    serialize: proxyMarkedValue => {
        const nestedConnectionId = generateUUID()

        // Add relationshipType in `postMessage`
        return [{ nestedConnectionId, proxyMarkedValue, panelId }, []]
    },
    deserialize: serialized => {
        // Get endpoint factory based on relationship type
        const endpointFactory =
            endpointFactories[serialized.relationshipType === 'webToWeb' ? 'webToWeb' : 'webToNode']!

        // Create endpoint, return wrapped proxy.
        const endpoint = endpointFactory(serialized.nestedConnectionId)
        const proxy = Comlink.wrap(endpoint)

        return proxy
    },
}

Comlink.transferHandlers.set('proxy', vscodeWebviewProxyTransferHandler)

/**
 *
 * @param target Typically a WebWorker or `self` from a WebWorker (`Endpoint` for main thread).
 */
export function createEndpointsForWebToWeb(target: Comlink.Endpoint): {
    webview: Comlink.Endpoint
    worker: Comlink.Endpoint
} {
    const onMessages = new WeakMap<EventListenerOrEventListenerObject, EventListener>()

    /**
     * Handles values sent to webviews that are marked to be proxied.
     */
    function toWireValue(value: NestedConnectionData): void {
        const proxyMarkedValue = value.proxyMarkedValue!
        // The proxyMarkedValue is probably not cloneable, so don't
        // send it "over the wire"
        delete value.proxyMarkedValue

        value.relationshipType = 'webToWeb'

        const endpoint = createEndpoint(value.nestedConnectionId)
        Comlink.expose(proxyMarkedValue, endpoint)
    }

    function createEndpoint(connectionId: string): Comlink.Endpoint {
        return {
            postMessage: message => {
                // Add relationship type to all nested connection values (i.e values to be proxied).
                // TODO is the above necessary for this endpoint type?
                const value = message.value
                const argumentList = message.argumentList

                if (isNestedConnection(value)) {
                    toWireValue(value)
                }
                if (Array.isArray(argumentList)) {
                    for (const argument of argumentList) {
                        const value = argument.value
                        if (isNestedConnection(value)) {
                            toWireValue(value)
                        }
                    }
                }

                target.postMessage({ ...message, connectionId, panelId })
            },

            addEventListener: (type, listener) => {
                // This event listener will be called for all proxy method calls.
                // Comlink will send the message to the appropriate caller
                // based on UUID (generated internally).

                function onMessage(event: MessageEvent): void {
                    if (event.data?.connectionId === connectionId) {
                        return typeof listener === 'function' ? listener(event) : listener.handleEvent(event)
                    }
                }

                onMessages.set(listener, onMessage as EventListener)

                target.addEventListener('message', onMessage as EventListener)
            },
            removeEventListener: (type, listener) => {
                const onMessage = onMessages.get(listener)
                target.removeEventListener('message', onMessage ?? listener)
            },
        }
    }

    endpointFactories.webToWeb = createEndpoint

    const webview = createEndpoint('webview')
    const worker = createEndpoint('worker')

    return {
        webview,
        worker,
    }
}

export function createEndpointsForWebToNode(vscodeApi: VsCodeApi): EndpointPair {
    const onMessages = new WeakMap<EventListenerOrEventListenerObject, EventListener>()

    /**
     * Handles values sent to the VS Code extension that are marked to be proxied.
     */
    function toWireValue(value: NestedConnectionData): void {
        const proxyMarkedValue = value.proxyMarkedValue!
        // The proxyMarkedValue is probably not cloneable, so don't
        // send it "over the wire"
        delete value.proxyMarkedValue

        value.relationshipType = 'webToNode'

        const endpoint = createEndpoint(value.nestedConnectionId)
        Comlink.expose(proxyMarkedValue, endpoint)
    }

    function createEndpoint(connectionId: string): Comlink.Endpoint {
        return {
            postMessage: message => {
                // Add relationship type to all nested connection values (i.e values to be proxied).
                // TODO is the above necessary for this endpoint type?
                const value = message.value
                const argumentList = message.argumentList

                if (isNestedConnection(value)) {
                    toWireValue(value)
                }
                if (Array.isArray(argumentList)) {
                    for (const argument of argumentList) {
                        const value = argument.value
                        if (isNestedConnection(value)) {
                            toWireValue(value)
                        }
                    }
                }

                vscodeApi.postMessage({ ...message, connectionId, panelId })
            },

            addEventListener: (type, listener) => {
                // This event listener will be called for all proxy method calls.
                // Comlink will send the message to the appropriate caller
                // based on UUID (generated internally).
                function onMessage(event: MessageEvent): void {
                    if (event.data?.connectionId === connectionId) {
                        return typeof listener === 'function' ? listener(event) : listener.handleEvent(event)
                    }
                }

                onMessages.set(listener, onMessage as EventListener)

                window.addEventListener('message', onMessage)
            },
            removeEventListener: (type, listener) => {
                const onMessage = onMessages.get(listener)
                window.removeEventListener('message', onMessage ?? listener)
            },
        }
    }

    endpointFactories.webToNode = createEndpoint

    const extensionEndpoint = createEndpoint('extension')
    const webviewEndpoint = createEndpoint('webview')

    return {
        proxy: extensionEndpoint,
        expose: webviewEndpoint,
    }
}
