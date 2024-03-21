import vscode, { env } from 'vscode'

import { getSourcegraphFileUrl, repoInfo } from './git-helpers'
import { generateSourcegraphBlobLink } from './initialize'

/**
 * Open active file in the browser on the configured Sourcegraph instance.
 */
export async function browserActions(action: string, logRedirectEvent: (uri: string) => void): Promise<void> {
    const editor = vscode.window.activeTextEditor
    if (!editor) {
        throw new Error('No active editor')
    }
    const uri = editor.document.uri
    const instanceUrl =
        vscode.workspace.getConfiguration('sourcegraph').get<string>('url') || 'https://sourcegraph.com/'
    let sourcegraphUrl = ''
    // check if the current file is a remote file or not
    if (uri.scheme === 'sourcegraph') {
        sourcegraphUrl = generateSourcegraphBlobLink(
            uri,
            editor.selection.start.line,
            editor.selection.start.character,
            editor.selection.end.line,
            editor.selection.end.character
        )
    } else {
        const repositoryInfo = await repoInfo(editor.document.uri.fsPath)
        if (!repositoryInfo) {
            await vscode.window.showErrorMessage('Cannot get git info for this repository.')
            return
        }
        let { remoteURL, branch, fileRelative } = repositoryInfo
        // construct sourcegraph url for current file
        // Ask if user want to open file in HEAD instead if set current branch or default branch
        // do not exist on Sourcegraph
        if (!branch) {
            const userChoice = await vscode.window.showInformationMessage(
                'Current (or default) branch does not exist on Sourcegraph. Publish your branch or continue to main branch.',
                'Continue to main',
                'Cancel'
            )
            branch = userChoice === 'Continue to main' ? 'HEAD' : ''
            if (!branch) {
                return
            }
        }
        sourcegraphUrl = getSourcegraphFileUrl(instanceUrl, remoteURL, branch, fileRelative, editor)
    }
    // Decode URI
    const decodedUri = decodeURIComponent(sourcegraphUrl)
    // Log redirect events
    logRedirectEvent(sourcegraphUrl)
    // Open in browser or Copy file link
    switch (action) {
        case 'open': {
            await vscode.env.openExternal(vscode.Uri.parse(decodedUri))
            break
        }
        case 'copy': {
            await env.clipboard.writeText(decodedUri).then(() => vscode.window.showInformationMessage('Copied!'))
            break
        }
        default: {
            throw new Error(`Failed to ${action} file link: invalid URL`)
        }
    }
}
