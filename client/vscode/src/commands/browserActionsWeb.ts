import vscode, { env } from 'vscode'

import { generateSourcegraphBlobLink } from './initialize'

/**
 * browser Actions for Web does not run node modules to get git info
 * Open active file in the browser on the configured Sourcegraph instance.
 */
export async function browserActions(action: string, logRedirectEvent: (uri: string) => void): Promise<void> {
    const editor = vscode.window.activeTextEditor
    if (!editor) {
        throw new Error('No active editor')
    }
    const uri = editor.document.uri
    const instanceUrl = vscode.workspace.getConfiguration('sourcegraph').get('url')
    let sourcegraphUrl = String()
    // check if the current file is a remote file or not
    if (uri.scheme === 'sourcegraph') {
        sourcegraphUrl = generateSourcegraphBlobLink(
            uri,
            editor.selection.start.line,
            editor.selection.start.character,
            editor.selection.end.line,
            editor.selection.end.character
        )
    } else if (uri.authority === 'github' && typeof instanceUrl === 'string') {
        // For remote github files
        const repoInfo = uri.fsPath.split('/')
        const repositoryName = `${repoInfo[1]}/${repoInfo[2]}`
        const filePath = repoInfo.length === 4 ? repoInfo[3] : repoInfo.slice(3).join('/')
        sourcegraphUrl = `${instanceUrl}github.com/${repositoryName}/-/blob/${filePath || ''}`
    } else {
        await vscode.window.showInformationMessage('Non-Remote files are not supported on VS Code Web currently')
    }
    // Log redirect events
    logRedirectEvent(sourcegraphUrl)

    // Open in browser or Copy file link
    if (action === 'open' && sourcegraphUrl) {
        await vscode.env.openExternal(vscode.Uri.parse(sourcegraphUrl))
    } else if (action === 'copy' && sourcegraphUrl) {
        const decodedUri = decodeURIComponent(sourcegraphUrl)
        await env.clipboard.writeText(decodedUri).then(() => vscode.window.showInformationMessage('Copied!'))
    } else {
        throw new Error(`Failed to ${action} file link: invalid URL`)
    }
}
