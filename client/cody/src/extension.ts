import * as vscode from 'vscode'

import { PromptMixin, languagePromptMixin } from '@sourcegraph/cody-shared/src/prompt/prompt-mixin'

import { start } from './command/CommandsProvider'
import { ExtensionApi } from './extension-api'

export function activate(context: vscode.ExtensionContext): ExtensionApi {
    console.log('Cody extension activated')

    PromptMixin.add(languagePromptMixin(vscode.env.language))

    // Register commands and webview
    start(context)
        .then(disposable => context.subscriptions.push(disposable))
        .catch(error => console.error(error))

    return new ExtensionApi()
}
