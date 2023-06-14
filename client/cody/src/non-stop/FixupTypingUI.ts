import * as vscode from 'vscode'

import { FixupTaskFactory } from './roles'

/**
 * The UI for creating non-stop fixup tasks by typing instructions.
 */
export class FixupTypingUI {
    constructor(private readonly taskFactory: FixupTaskFactory) {}

    public async show(): Promise<void> {
        const editor = vscode.window.activeTextEditor
        if (!editor) {
            return
        }
        const range = editor.selection
        const text = editor.document.getText(editor.selection)

        // TODO: Do not require any text to be selected
        if (range.isEmpty) {
            await vscode.window.showWarningMessage('Select some text to fix up')
            return
        }

        const CHAT_COMMAND = '/chat'
        const CHAT_RE = /\/chat(|\s.*)^/
        const instruction = (
            await vscode.window.showInputBox({
                title: `Ask Cody to edit your code, or use ${CHAT_COMMAND} to ask a question`,
            })
        )?.trim()
        if (!instruction) {
            return
        }
        const match = instruction.match(CHAT_RE)
        if (match) {
            void vscode.commands.executeCommand('cody.focus', match[1].trim())
            return
        }

        this.taskFactory.createTask(editor.document.uri, instruction, text, range)
    }
}
