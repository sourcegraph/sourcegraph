import * as vscode from 'vscode'

import { readConfiguration } from './readConfiguration'

export function endpointSetting(): string {
    const url = vscode.workspace.getConfiguration().get<string>('sourcegraph.url') || 'https://sourcegraph.com'
    return removeEndingSlash(url)
}

export async function setEndpoint(newEndpoint: string): Promise<void> {
    const newEndpointURL = newEndpoint ? removeEndingSlash(newEndpoint) : 'https://sourcegraph.com'
    const currentEndpointHostname = new URL(endpointSetting()).hostname
    const newEndpointHostname = new URL(newEndpointURL).hostname
    if (currentEndpointHostname !== newEndpointHostname) {
        await readConfiguration().update('url', newEndpointURL)
    }
    return
}

export function endpointHostnameSetting(): string {
    return new URL(endpointSetting()).hostname
}

export function endpointPortSetting(): number {
    const port = new URL(endpointSetting()).port
    return port ? parseInt(port, 10) : 443
}

export function endpointProtocolSetting(): string {
    return new URL(endpointSetting()).protocol
}

export function endpointRequestHeadersSetting(): object {
    return vscode.workspace.getConfiguration().get<object>('sourcegraph.requestHeaders') || {}
}

function removeEndingSlash(uri: string): string {
    if (uri.endsWith('/')) {
        return uri.slice(0, -1)
    }
    return uri
}

export function isSourcegraphDotCom(): boolean {
    const hostname = new URL(endpointSetting()).hostname
    return hostname === 'sourcegraph.com' || hostname === 'www.sourcegraph.com'
}
