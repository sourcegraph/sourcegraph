import * as vscode from 'vscode'

export function readConfiguration(): vscode.WorkspaceConfiguration {
    return vscode.workspace.getConfiguration('sourcegraph')
}
