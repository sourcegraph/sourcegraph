import type { ChatMessage } from '@sourcegraph/cody-shared'

export const FIXTURE_TRANSCRIPT: Record<string, ChatMessage[]> = {
    simple: [
        { speaker: 'human', displayText: 'Hello, world!' },
        { speaker: 'assistant', displayText: 'Thank you' },
    ],
    codeQuestion: [
        {
            speaker: 'human',
            displayText: 'What does `document.getSelection()?.isCollapsed` mean?',
        },
        {
            speaker: 'assistant',
            displayText:
                '`document.getSelection()?.isCollapsed` means that the current selection in the document is collapsed, meaning it is a caret (no text is selected).\n\nThe `?.` operator is optional chaining - it will return `undefined` if `document.getSelection()` returns `null` or `undefined`.\n\nSo in short, that line is checking if there is currently a text selection in the document, and if not, focusing the textarea.\n\n',
        },
    ],
    explainCode: [
        {
            speaker: 'human',
            displayText:
                "Explain the following code at a high level:\n\n```\nprivate getNonce(): string {\n  let text = ''\n  const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'\n  for (let i = 0; i < 32; i++) {\n    text += possible.charAt(Math.floor(Math.random() * possible.length))\n  }\n  return text\n}\n```",
        },
        {
            speaker: 'assistant',
            displayText: 'This code generates a random 32-character string (nonce) using characters A-Z, a-z, and 0-9.',
            contextFiles: [
                { fileName: 'vscode/src/chat/ChatViewProvider.ts' },
                { fileName: 'lib/shared/src/timestamp.ts' },
            ],
        },
        {
            speaker: 'human',
            displayText: 'Rewrite it to only use hexadecimal encoding.',
        },
        {
            speaker: 'assistant',
            displayText:
                "Here is the rewritten code using only hexadecimal encoding:\n\n```\nprivate getNonce(): string {\n  let text = ''\n  const possible = '0123456789ABCDEF'\n  for (let i = 0; i < 32; i++) {\n    text += possible.charAt(Math.floor(Math.random() * possible.length))\n  }\n  return text\n}\n```",
        },
    ],
}
