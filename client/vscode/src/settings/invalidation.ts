import * as vscode from 'vscode'

import { invalidateClient } from '../backend/requestGraphQl'
import type { VSCEStateMachine } from '../state'

/**
 * Listens for Sourcegraph URL and invalidates the GraphQL client
 * to prevent data "contamination" (e.g. sending private repo names to Cloud instance).
 */
export function invalidateContextOnSettingsChange({
    context,
    stateMachine,
}: {
    context: vscode.ExtensionContext
    stateMachine: VSCEStateMachine
}): void {
    function disposeAllResources(): void {
        for (const subscription of context.subscriptions) {
            subscription.dispose()
        }
    }

    context.subscriptions.push(
        vscode.workspace.onDidChangeConfiguration(async config => {
            if (config.affectsConfiguration('sourcegraph.url')) {
                invalidateClient()
                disposeAllResources()
                stateMachine.emit({ type: 'sourcegraph_url_change' })
                // Swallow errors since if `showInformationMessage` fails, we assume that something is wrong
                // with the VS Code extension host and don't retry.
                await vscode.window.showInformationMessage(
                    'Restart VS Code to use the Sourcegraph extension after URL change.'
                )
            }
        })
    )
}
