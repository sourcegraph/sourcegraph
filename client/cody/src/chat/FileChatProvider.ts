import * as vscode from 'vscode'

import { ActiveTextEditorSelection } from '@sourcegraph/cody-shared/src/editor'

export class FileChatMessage implements vscode.Comment {
    public id = 0
    public label: string | undefined
    public markdownBody: string | vscode.MarkdownString
    constructor(
        public body: string | vscode.MarkdownString,
        public mode: vscode.CommentMode,
        public author: vscode.CommentAuthorInformation,
        public parent?: vscode.CommentThread,
        public contextValue?: string
    ) {
        this.id = this.id++
        this.markdownBody = this.body
    }
}

export class FileChatProvider {
    private commentController: vscode.CommentController
    private options = {
        prompt: 'Click here to ask Cody anything about this file.',
        placeHolder: 'Enter your question here...',
    }

    private readonly id = 'cody-file-chat'
    private readonly label = 'Cody: In-File Chat'
    private codyIcon: vscode.Uri
    private userIcon: vscode.Uri
    public threads: vscode.CommentReply | null = null
    public thread: vscode.CommentThread | null = null
    public editor: vscode.TextEditor | null = null
    public selection: ActiveTextEditorSelection | null = null
    public selectionRange: vscode.Range | null = null

    constructor(private extensionPath: string) {
        this.commentController = vscode.comments.createCommentController(this.id, this.label)
        // A `CommentingRangeProvider` controls where gutter decorations that allow adding comments are shown
        this.commentController.commentingRangeProvider = {
            provideCommentingRanges: (document: vscode.TextDocument) => {
                const lineCount = document.lineCount
                return [new vscode.Range(0, 0, lineCount - 1, 0)]
            },
        }
        this.commentController.options = this.options
        this.codyIcon = this.getIconPath('cody')
        this.userIcon = this.getIconPath('user')
    }

    public get(): vscode.CommentController {
        return this.commentController
    }

    // Add response from Human
    public async chat(threads: vscode.CommentReply, isFixMode: boolean = false): Promise<void> {
        const humanInput = threads.text
        const thread = threads.thread
        thread.label = 'Ask Cody anything about this file 😉'
        const newComment = new FileChatMessage(
            this.markdown(humanInput),
            vscode.CommentMode.Preview,
            { name: 'Me', iconPath: this.userIcon },
            thread,
            thread.comments.length ? 'canDelete' : undefined
        )

        thread.comments = [...thread.comments, newComment]

        this.threads = threads
        this.thread = thread
        console.log('get selection')
        this.selection = await this.getSelection(isFixMode)

        console.log('got selection', this.selection)
        // Add loading message
        if (isFixMode) {
            this.reply('Got it! Working on it!')
        }
    }

    // Add response from Cody
    public reply(text: string): void {
        if (!this.thread) {
            return
        }

        const codyReply = new FileChatMessage(
            this.markdown(text),
            vscode.CommentMode.Preview,
            { name: 'Cody', iconPath: this.codyIcon },
            this.thread,
            this.thread.comments.length ? 'canDelete' : undefined
        )
        this.thread.comments = [...this.thread.comments, codyReply]
    }

    private markdown(text: string): vscode.MarkdownString {
        const markdownText = new vscode.MarkdownString(text)
        markdownText.isTrusted = true
        markdownText.supportHtml = true
        return markdownText
    }

    public async getEditor(): Promise<vscode.TextEditor | null> {
        if (!this.thread) {
            return null
        }
        await vscode.window.showTextDocument(this.thread.uri)
        this.editor = vscode.window.activeTextEditor || null
        return this.editor
    }

    public async getSelection(isFixMode: boolean): Promise<ActiveTextEditorSelection | null> {
        if (!this.thread) {
            return null
        }
        const SURROUNDING_LINES = 50

        const activeDocument = await vscode.workspace.openTextDocument(this.thread.uri)

        const lineLength = activeDocument.lineAt(this.thread.range.end.line).text.length

        const endPostFix = new vscode.Position(this.thread.range.end.line, lineLength)

        const endPostAsk = new vscode.Position(this.thread.range.end.line + 1, 0)

        const selectionRange = new vscode.Range(this.thread.range.start, isFixMode ? endPostFix : endPostAsk)

        const precedingText = activeDocument.getText(
            new vscode.Range(
                new vscode.Position(Math.max(0, this.thread.range.start.line - SURROUNDING_LINES), 0),
                this.thread.range.start
            )
        )
        const followingText = activeDocument.getText(
            new vscode.Range(
                this.thread.range.end,
                new vscode.Position(this.thread.range.end.line + 1 + SURROUNDING_LINES, 0)
            )
        )

        const selection = {
            fileName: vscode.workspace.asRelativePath(this.thread.uri.fsPath),
            selectedText: activeDocument.getText(selectionRange),
            precedingText,
            followingText,
        }

        this.selectionRange = selectionRange
        this.selection = selection
        return selection
    }

    private getIconPath(speaker: string): vscode.Uri {
        const extensionPath = vscode.Uri.file(this.extensionPath)
        const webviewPath = vscode.Uri.joinPath(extensionPath, 'dist')
        return vscode.Uri.joinPath(webviewPath, speaker === 'cody' ? 'cody.png' : 'sourcegraph.png')
    }
}
