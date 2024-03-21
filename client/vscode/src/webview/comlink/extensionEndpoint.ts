import * as Comlink from 'comlink'
import type vscode from 'vscode'

import type { EndpointPair } from '@sourcegraph/shared/src/platform/context'

import {
    generateUUID,
    isNestedConnection,
    isProxyMarked,
    isUnsubscribable,
    type NestedConnectionData,
    type RelationshipType,
} from '.'

// Used to scope message to panel (and `connectionId` further scopes to function call).
let nextPanelId = 1

const endpointFactories = new Map<string, ((connectionId: string) => Comlink.Endpoint) | undefined>()

const vscodeExtensionProxyTransferHandler: Comlink.TransferHandler<
    object,
    // Receive panelId (in deserialize), no need to send it.
    // Return proxyMarkedValue in serialize so we can expose it in postMessage, but
    // only end up sending the nestedConnectionId
    { nestedConnectionId: string; proxyMarkedValue?: object; panelId?: string; relationshipType: RelationshipType }
> = {
    canHandle: isProxyMarked,
    serialize: proxyMarkedValue => {
        const nestedConnectionId = generateUUID()
        // Defer endpoint creation/object exposition to `postMessage` (to scope it to panel)

        return [{ nestedConnectionId, proxyMarkedValue, relationshipType: 'webToNode' }, []]
    },
    deserialize: serialized => {
        // Create endpoint, return wrapped proxy.
        const endpointFactory = endpointFactories.get(serialized.panelId!)!
        const endpoint = endpointFactory(serialized.nestedConnectionId)
        const proxy = Comlink.wrap(endpoint)

        return proxy
    },
}

Comlink.transferHandlers.set('proxy', vscodeExtensionProxyTransferHandler)

export function createEndpointsForWebview(
    panel: Pick<vscode.WebviewPanel, 'onDidDispose' | 'webview'>
): EndpointPair & { panelId: string } {
    const listenerDisposables = new WeakMap<EventListenerOrEventListenerObject, vscode.Disposable>()
    const panelId = nextPanelId.toString()
    nextPanelId++
    let disposed = false

    // Keep track of proxied unsubscribables to clean up when a webview is closed. In that case,
    // the webview will likely be unable to send an unsubscribe message.
    const proxiedUnsubscribables = new Set<{ unsubscribe: () => unknown }>()

    /**
     * Handles values sent to webviews that are marked to be proxied.
     */
    function toWireValue(value: NestedConnectionData): void {
        const proxyMarkedValue = value.proxyMarkedValue!
        // The proxyMarkedValue is probably not cloneable, so don't
        // send it "over the wire"
        delete value.proxyMarkedValue

        if (isUnsubscribable(proxyMarkedValue)) {
            proxiedUnsubscribables.add(proxyMarkedValue)
            // Debt: ideally remove unsubscribable from set when we receive a unsubscribe message.
        }

        const endpoint = createEndpoint(value.nestedConnectionId)
        Comlink.expose(proxyMarkedValue, endpoint)
    }

    function createEndpoint(connectionId: string): Comlink.Endpoint {
        return {
            postMessage: (message: any) => {
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

                if (!disposed) {
                    panel.webview.postMessage({ ...message, connectionId, panelId }).then(
                        () => {},
                        error => console.error('postMessage error', error)
                    )
                }
            },
            addEventListener: (type, listener) => {
                // This event listener will be called for all proxy method calls.
                // Comlink will send the message to the appropriate caller
                // based on UUID (generated internally).
                function onMessage(message: any): void {
                    if (message?.connectionId === connectionId) {
                        // Comlink is listening for a message event, only uses the `data` property.
                        const messageEvent = {
                            data: message,
                        } as MessageEvent

                        return typeof listener === 'function'
                            ? listener(messageEvent)
                            : listener.handleEvent(messageEvent)
                    }
                }

                const disposable = panel.webview.onDidReceiveMessage(onMessage)
                listenerDisposables.set(listener, disposable)
            },
            removeEventListener: (type, listener) => {
                const disposable = listenerDisposables.get(listener)
                disposable?.dispose()
                listenerDisposables.delete(listener)
            },
        }
    }

    endpointFactories.set(panelId, createEndpoint)
    panel.onDidDispose(() => {
        disposed = true
        endpointFactories.delete(panelId)

        for (const unsubscribable of proxiedUnsubscribables) {
            unsubscribable.unsubscribe()
        }
    })

    const webviewEndpoint = createEndpoint('webview')
    const extensionEndpoint = createEndpoint('extension')

    return {
        proxy: webviewEndpoint,
        expose: extensionEndpoint,
        panelId,
    }
}
