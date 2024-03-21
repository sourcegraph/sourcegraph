import * as vscode from 'vscode'

import { isOlderThan, observeInstanceVersionNumber } from '../backend/instanceVersion'
import { scretTokenKey } from '../webview/platform/AuthProvider'

import { endpointHostnameSetting, endpointProtocolSetting } from './endpointSetting'
import { readConfiguration } from './readConfiguration'

// IMPORTANT: Call this function only once when extention is first activated
export async function processOldToken(secretStorage: vscode.SecretStorage): Promise<void> {
    // Process the token that lives in user configuration
    // Move them to secrets and then remove them by setting it as undefined
    const storageToken = await secretStorage.get(scretTokenKey)
    const oldToken = vscode.workspace.getConfiguration().get<string>('sourcegraph.accessToken') || ''
    if (!storageToken && oldToken.length > 8) {
        await secretStorage.store(scretTokenKey, oldToken)
        await removeOldAccessTokenSetting()
    }
    return
}

export async function accessTokenSetting(secretStorage: vscode.SecretStorage): Promise<string> {
    const currentToken = await secretStorage.get(scretTokenKey)
    return currentToken || ''
}

export async function removeOldAccessTokenSetting(): Promise<void> {
    await readConfiguration().update('accessToken', undefined, vscode.ConfigurationTarget.Global)
    await readConfiguration().update('accessToken', undefined, vscode.ConfigurationTarget.Workspace)
    return
}

// Ensure that only one access token error message is shown at a time.
let showingAccessTokenErrorMessage = false

export async function handleAccessTokenError(badToken: string, endpointURL: string): Promise<void> {
    if (badToken !== undefined && !showingAccessTokenErrorMessage) {
        showingAccessTokenErrorMessage = true

        const message = !badToken
            ? `A valid access token is required to connect to ${endpointURL}`
            : `Connection to ${endpointURL} failed. Please try reloading VS Code if your Sourcegraph instance URL has been updated.`

        const version = await observeInstanceVersionNumber(badToken, endpointURL).toPromise()
        const supportsTokenCallback = version && isOlderThan(version, { major: 3, minor: 41 })
        const action = await vscode.window.showErrorMessage(message, 'Get Token', 'Reload Window')

        if (action === 'Reload Window') {
            await vscode.commands.executeCommand('workbench.action.reloadWindow')
        } else if (action === 'Get Token') {
            const path = supportsTokenCallback ? '/user/settings/tokens/new/callback' : '/user/settings/'
            const query = supportsTokenCallback ? 'requestFrom=VSCEAUTH' : ''

            await vscode.env.openExternal(
                vscode.Uri.from({
                    scheme: endpointProtocolSetting().slice(0, -1),
                    authority: endpointHostnameSetting(),
                    path,
                    query,
                })
            )
        }
        showingAccessTokenErrorMessage = false
    }
}
