/**
 * Disabled due to violation of the VS Code's UX guidelines for notifications
 * To be revaluated in the future: https://code.visualstudio.com/api/ux-guidelines/notifications
 * This functions add Sourcegraph to workspace recommendations if haven't already
 * eg: recommendSourcegraph(localStorageService).catch(() => {})
 */

import * as vscode from 'vscode'

import { DISMISS_WORKSPACERECS_CTA_KEY, type LocalStorageService } from './LocalStorageService'

/**
 * Ask if user wants to add Sourcegraph to their Workspace Recommendations list by displaying built-in popup
 * It will not show popup if the user already has Sourcegraph added to their recommendations list
 * or has dismissed the message previously
 */
export async function recommendSourcegraph(localStorageService: LocalStorageService): Promise<void> {
    // Check if user has clicked on the 'Don't show again' button previously
    // Reset stored value for testing: await localStorageService.setValue(DISMISS_WORKSPACERECS_CTA_KEY, '')
    const isDismissed = localStorageService.getValue(DISMISS_WORKSPACERECS_CTA_KEY)
    if (isDismissed === 'true') {
        return
    }
    const rootPath = vscode.workspace.workspaceFolders ? vscode.workspace.workspaceFolders[0].uri.fsPath : null
    if (!rootPath) {
        return
    }
    // File path of the workspace recommendations file
    const filePath = vscode.Uri.file(`${rootPath}/.vscode/extensions.json`)
    // Check if sourcegraph is already added to their Workspace Recommendations
    const bytes = await vscode.workspace.fs.readFile(filePath)
    const decoded = new TextDecoder('utf-8').decode(bytes)
    // Assume sourcegraph is in the recommendation list if 'sourcegraph.sourcegraph' exists in the file
    if (decoded.includes('sourcegraph.sourcegraph')) {
        return
    }
    // Display Cta
    await vscode.window
        .showInformationMessage('Add Sourcegraph to your workspace recommendations', '👍 Yes', "Don't show again")
        .then(async answer => {
            if (answer === '👍 Yes') {
                await vscode.commands.executeCommand(
                    'workbench.extensions.action.addExtensionToWorkspaceRecommendations',
                    'sourcegraph.sourcegraph'
                )
            }
            // Store as dismissed so it won't show user again when answer No or after being added
            await localStorageService.setValue(DISMISS_WORKSPACERECS_CTA_KEY, 'true')
        })
}
