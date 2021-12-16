import * as vscode from 'vscode'

import { readConfiguration } from './readConfiguration'

export function activateEndpointSetting(): vscode.Disposable {
    return vscode.workspace.onDidChangeConfiguration(config => {
        if (config.affectsConfiguration('sourcegraph.url')) {
            // TODO reload extension (or invalidate gql if we have to)
        }
    })
}

export function endpointSetting(): string {
    // has default value
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    const url = readConfiguration().get<string>('url')!
    if (url.endsWith('/')) {
        return url.slice(0, -1)
    }
    return url
}

export function endpointHostnameSetting(): string {
    return new URL(endpointSetting()).hostname
}

export function endpointPortSetting(): number {
    const port = new URL(endpointSetting()).port
    return port ? parseInt(port, 10) : 443
}

export function endpointCorsSetting(): string {
    // has default value = null
    const corsUrl = readConfiguration().get<string>('corsUrl')!
    return corsUrl !== '' ? new URL('', corsUrl).origin : ''
}

// Check if Access Token is configured in setting
export function endpointAccessTokenSetting(): boolean {
    if (readConfiguration().get<string>('accessToken')) {
        return true
    }
    return false
}
