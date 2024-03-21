import * as vscode from 'vscode'

import type { SourcegraphFileSystemProvider } from './SourcegraphFileSystemProvider'
import type { SourcegraphUri } from './SourcegraphUri'

/**
 * Try to find local copy of the search result file first
 * Remote copy will be opened instead if basePath is not set or local copy cannot be found
 **/
export async function openSourcegraphUriCommand(fs: SourcegraphFileSystemProvider, uri: SourcegraphUri): Promise<void> {
    if (uri.compareRange) {
        // noop. v2 Debt: implement. Open in browser for v1
        return
    }
    let textDocument
    try {
        textDocument = await getLocalCopy(uri)
    } catch (error) {
        console.error('Failed to get local copy:', error)
        if (!uri.revision) {
            const metadata = await fs.repositoryMetadata(uri.repositoryName)
            uri = uri.withRevision(metadata?.defaultBranch || 'HEAD')
        }
        // Load Remote Copy instead
        textDocument = await vscode.workspace.openTextDocument(vscode.Uri.parse(uri.uri))
    }
    const selection = getSelection(uri, textDocument)
    await vscode.window.showTextDocument(textDocument, {
        selection,
        viewColumn: vscode.ViewColumn.Active,
        preview: false,
    })
}

async function getLocalCopy(remoteUri: SourcegraphUri): Promise<vscode.TextDocument> {
    const repoName = remoteUri.repositoryName.split('/').pop() || '' // ex: github.com/sourcegraph/sourcegraph => sourcegraph
    const filePath = remoteUri.path || '' // ex: "client/vscode/package.json"
    // Get basePath from configuration
    const basePath = vscode.workspace.getConfiguration('sourcegraph').get<string>('basePath') || null
    const workspaceFilePath = await vscode.workspace
        .findFiles(filePath, null, 1)
        .then(result => result[0]?.path || null)
    // If basePath is not configured, we will try to find file in the current workspace
    const absolutePath = basePath
        ? vscode.Uri.file(vscode.Uri.joinPath(vscode.Uri.parse(basePath), repoName, filePath).path)
        : workspaceFilePath
        ? vscode.Uri.file(workspaceFilePath)
        : null
    // if both basePath and workspaceFilePath are null, the operation will fail
    if (!absolutePath) {
        throw new Error('Try to configure your basePath to open this file.')
    }
    // Set current workspace folder path as basePath if it doesn't exist
    if (!basePath && workspaceFilePath) {
        // get current workspace folder uri
        const workspaceFolderUri = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(workspaceFilePath))?.uri
        if (workspaceFolderUri) {
            // go one level up and set that as the new basePath
            const newBasePath = vscode.Uri.file(vscode.Uri.joinPath(workspaceFolderUri, '../').fsPath).path
            await vscode.workspace
                .getConfiguration('sourcegraph')
                .update('basePath', newBasePath, vscode.ConfigurationTarget.Global)
        }
    }
    const textDocument = await vscode.workspace.openTextDocument(absolutePath)
    return textDocument
}

function getSelection(uri: SourcegraphUri, textDocument: vscode.TextDocument): vscode.Range | undefined {
    if (uri?.position?.line !== undefined && uri?.position?.character !== undefined) {
        return offsetRange(uri.position.line, uri.position.character)
    }
    if (uri?.position?.line !== undefined) {
        return offsetRange(uri.position.line, 0)
    }
    // There's no explicitly provided line number. Instead of focusing on the
    // first line (which usually contains lots of imports), we use a heuristic
    // to guess the location where the "main symbol" is defined (a
    // function/class/struct/interface with the same name as the filename).
    if (uri.path && isFilenameThatMayDefineSymbols(uri.path)) {
        const fileNames = uri.path.split('/')
        const fileName = fileNames.at(-1)!
        const symbolName = fileName.split('.')[0]
        const text = textDocument.getText()
        const symbolMatches = new RegExp(` ${symbolName}\\b`).exec(text)
        if (symbolMatches) {
            const position = textDocument.positionAt(symbolMatches.index + 1)
            return new vscode.Range(position, position)
        }
    }
    return undefined
}

function offsetRange(line: number, character: number): vscode.Range {
    const position = new vscode.Position(line, character)
    return new vscode.Range(position, position)
}

/**
 * @returns true if this file may contain code from a programming language that
 * defines symbol.
 */
function isFilenameThatMayDefineSymbols(path: string): boolean {
    return !(path.endsWith('.md') || path.endsWith('.markdown') || path.endsWith('.txt') || path.endsWith('.log'))
}
