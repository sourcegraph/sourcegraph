import * as Comlink from 'comlink'
import { isObject } from 'lodash'

import { hasProperty } from '@sourcegraph/common'

// Sourcegraph VS Code extension Comlink "communication layer" documentation
// -----
// NOTE: This is a working work-in-progress. We should be able to swap out this
// "communication layer" with either our own VS Code webview RPC solution
// or by applying this adapter to a fork of Comlink.

// MOTIVATION:
// Comlink is a great library that makes it easy to work with Web Workers: https://github.com/GoogleChromeLabs/comlink.
// We use it to implement a bi-directional communication channel between our web application/browser extension and
// our Sourcegraph extension host. Given the need to sync state between {search panel <- extension "Core" -> search webview},
// Comlink was a natural fit. However, we needed to implement some hacky adapters to get it to work for our needs:

// web <-> web: Default Comlink use case. Depends on ability to transfer `MessageChannel`s, which
// is built into browsers.
// web <-> node: Cannot transfer `MessageChannel`s between VS Code webviews and Node.js extension "Core",
//  so we have to hijack Comlink's Proxy transfer handler to manage nested Proxied objects ourselves.
//  Since the transfer handler registry is global, a web context that needs to communicate with Node.js will
//  have pushed out the default Proxy transfer handler out and therefore needs to manage web to web messages as well.

// Consequently, there are three endpoint generators (all return endpoints for both directions):
// - extension <-> webview (extension context)
// - web <-> extension (webview context)
// - web <-> web (webview context, used for Sourcegraph extension host Web Worker)

export function generateUUID(): string {
    return new Array(4)
        .fill(0)
        .map(() => Math.floor(Math.random() * Number.MAX_SAFE_INTEGER).toString(16))
        .join('-')
}

export type RelationshipType = 'webToWeb' | 'webToNode'

export interface NestedConnectionData {
    nestedConnectionId: string
    proxyMarkedValue?: object
    panelId: string
    relationshipType?: RelationshipType
}

export function isNestedConnection(value: unknown): value is NestedConnectionData {
    return isObject(value) && hasProperty('nestedConnectionId')(value) && hasProperty('proxyMarkedValue')(value)
}

export function isProxyMarked(value: unknown): value is Comlink.ProxyMarked {
    return isObject(value) && (value as Comlink.ProxyMarked)[Comlink.proxyMarker]
}

export function isUnsubscribable(value: object): value is { unsubscribe: () => unknown } {
    return hasProperty('unsubscribe')(value) && typeof value.unsubscribe === 'function'
}
