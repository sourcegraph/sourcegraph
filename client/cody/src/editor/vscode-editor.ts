import * as vscode from 'vscode'

import {
    ActiveTextEditor,
    ActiveTextEditorSelection,
    ActiveTextEditorVisibleContent,
    Editor,
} from '@sourcegraph/cody-shared/src/editor'
import { SURROUNDING_LINES } from '@sourcegraph/cody-shared/src/prompt/constants'

import { CommentController } from '../services/CommentController'

export class VSCodeEditor implements Editor {
    constructor(public controller: CommentController) {}

    public getWorkspaceRootPath(): string | null {
        const uri = vscode.window.activeTextEditor?.document?.uri
        if (uri) {
            const wsFolder = vscode.workspace.getWorkspaceFolder(uri)
            if (wsFolder) {
                return wsFolder.uri.fsPath
            }
        }
        return vscode.workspace.workspaceFolders?.[0]?.uri?.fsPath ?? null
    }

    public getActiveTextEditor(): ActiveTextEditor | null {
        const activeEditor = this.getActiveTextEditorInstance()
        if (!activeEditor) {
            return null
        }
        const documentUri = activeEditor.document.uri
        const documentText = activeEditor.document.getText()
        return { content: documentText, filePath: documentUri.fsPath }
    }

    private getActiveTextEditorInstance(): vscode.TextEditor | null {
        const activeEditor = vscode.window.activeTextEditor
        return activeEditor && activeEditor.document.uri.scheme === 'file' ? activeEditor : null
    }

    public getActiveTextEditorSelection(): ActiveTextEditorSelection | null {
        if (this.controller.isInProgress) {
            return null
        }
        const activeEditor = this.getActiveTextEditorInstance()
        if (!activeEditor) {
            return null
        }
        const selection = activeEditor.selection
        if (!selection || selection?.start.isEqual(selection.end)) {
            return null
        }
        return this.createActiveTextEditorSelection(activeEditor, selection)
    }

    public getActiveTextEditorSelectionOrEntireFile(): ActiveTextEditorSelection | null {
        const activeEditor = this.getActiveTextEditorInstance()
        if (!activeEditor) {
            return null
        }
        let selection = activeEditor.selection
        if (!selection || selection.isEmpty) {
            selection = new vscode.Selection(0, 0, activeEditor.document.lineCount, 0)
        }
        return this.createActiveTextEditorSelection(activeEditor, selection)
    }

    private createActiveTextEditorSelection(
        activeEditor: vscode.TextEditor,
        selection: vscode.Selection
    ): ActiveTextEditorSelection {
        const precedingText = activeEditor.document.getText(
            new vscode.Range(
                new vscode.Position(Math.max(0, selection.start.line - SURROUNDING_LINES), 0),
                selection.start
            )
        )
        const followingText = activeEditor.document.getText(
            new vscode.Range(selection.end, new vscode.Position(selection.end.line + SURROUNDING_LINES, 0))
        )

        return {
            fileName: vscode.workspace.asRelativePath(activeEditor.document.uri.fsPath),
            selectedText: activeEditor.document.getText(selection),
            precedingText,
            followingText,
        }
    }

    public getActiveTextEditorVisibleContent(): ActiveTextEditorVisibleContent | null {
        const activeEditor = this.getActiveTextEditorInstance()
        if (!activeEditor) {
            return null
        }

        const visibleRanges = activeEditor.visibleRanges
        if (visibleRanges.length === 0) {
            return null
        }

        const visibleRange = visibleRanges[0]
        const content = activeEditor.document.getText(
            new vscode.Range(
                new vscode.Position(visibleRange.start.line, 0),
                new vscode.Position(visibleRange.end.line + 1, 0)
            )
        )

        return {
            fileName: vscode.workspace.asRelativePath(activeEditor.document.uri.fsPath),
            content,
        }
    }

    public async replaceSelection(fileName: string, selectedText: string, replacement: string): Promise<void> {
        const activeEditor = this.getActiveTextEditorInstance()
        if (this.controller.isInProgress) {
            await this.controller.replaceSelection(replacement)
            return
        }
        if (!activeEditor || vscode.workspace.asRelativePath(activeEditor.document.uri.fsPath) !== fileName) {
            // TODO: should return something indicating success or failure
            console.error('Missing file')
            return
        }
        const selection = activeEditor.selection
        if (!selection) {
            console.error('Missing selection')
            return
        }
        if (activeEditor.document.getText(selection) !== selectedText) {
            // TODO: Be robust to this.
            await vscode.window.showInformationMessage(
                'The selection changed while Cody was working. The text will not be edited.'
            )
            return
        }

        // Editing the document
        await activeEditor.edit(edit => {
            edit.replace(selection, replacement)
        })

        return
    }

    public async showQuickPick(labels: string[]): Promise<string | undefined> {
        const label = await vscode.window.showQuickPick(labels)
        return label
    }

    public async showWarningMessage(message: string): Promise<void> {
        await vscode.window.showWarningMessage(message)
    }

    public async showInputBox(prompt?: string): Promise<string | undefined> {
        return vscode.window.showInputBox({
            placeHolder: prompt || 'Enter here...',
        })
    }
}
