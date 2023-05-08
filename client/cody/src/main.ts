import * as vscode from 'vscode'

import { ConfigurationWithAccessToken } from '@sourcegraph/cody-shared/src/configuration'

import { ChatViewProvider } from './chat/ChatViewProvider'
import { DOTCOM_URL } from './chat/protocol'
import { isValidLogin } from './chat/utils'
import { CodyContentProvider } from './command/ContentProvider'
import { FileChatProvider } from './command/FileChatProvider'
import { LocalStorage } from './command/LocalStorageProvider'
import { CodyCompletionItemProvider } from './completions'
import { CompletionsDocumentProvider } from './completions/docprovider'
import { History } from './completions/history'
import { getConfiguration, getFullConfig } from './configuration'
import { VSCodeEditor } from './editor/vscode-editor'
import { logEvent, updateEventLogger } from './event-logger'
import { configureExternalServices } from './external-services'
import { getRgPath } from './rg'
import { CODY_ACCESS_TOKEN_SECRET, InMemorySecretStorage, SecretStorage, VSCodeSecretStorage } from './secret-storage'

/**
 * Start the extension, watching all relevant configuration and secrets for changes.
 */
export async function start(context: vscode.ExtensionContext): Promise<vscode.Disposable> {
    const secretStorage =
        process.env.CODY_TESTING === 'true' ? new InMemorySecretStorage() : new VSCodeSecretStorage(context.secrets)
    const localStorage = new LocalStorage(context.globalState)
    const rgPath = await getRgPath(context.extensionPath)

    const disposables: vscode.Disposable[] = []

    const { disposable, onConfigurationChange } = await register(
        context,
        await getFullConfig(secretStorage),
        secretStorage,
        localStorage,
        rgPath
    )
    disposables.push(disposable)

    // Re-initialize when configuration or secrets change.
    disposables.push(
        secretStorage.onDidChange(async key => {
            if (key === CODY_ACCESS_TOKEN_SECRET) {
                onConfigurationChange(await getFullConfig(secretStorage))
            }
        }),
        vscode.workspace.onDidChangeConfiguration(async event => {
            if (event.affectsConfiguration('cody')) {
                onConfigurationChange(await getFullConfig(secretStorage))
            }
        })
    )

    return vscode.Disposable.from(...disposables)
}

// Registers commands and webview given the config.
const register = async (
    context: vscode.ExtensionContext,
    initialConfig: ConfigurationWithAccessToken,
    secretStorage: SecretStorage,
    localStorage: LocalStorage,
    rgPath: string
): Promise<{
    disposable: vscode.Disposable
    onConfigurationChange: (newConfig: ConfigurationWithAccessToken) => void
}> => {
    const disposables: vscode.Disposable[] = []

    await updateEventLogger(initialConfig, localStorage)

    // File Chat Provider
    const fixupContentProvider = new CodyContentProvider()
    const fileChatProvider = new FileChatProvider(
        context.extensionPath,
        fixupContentProvider,
        initialConfig.experimentalNonStop
    )
    disposables.push(
        vscode.workspace.registerTextDocumentContentProvider('codyDoc', fixupContentProvider),
        fileChatProvider.get()
    )

    const editor = new VSCodeEditor(fileChatProvider)
    const workspaceConfig = vscode.workspace.getConfiguration()
    const config = getConfiguration(workspaceConfig)

    const {
        intentDetector,
        codebaseContext,
        chatClient,
        completionsClient,
        onConfigurationChange: externalServicesOnDidConfigurationChange,
    } = await configureExternalServices(initialConfig, rgPath, editor)

    // Create chat webview
    const chatProvider = new ChatViewProvider(
        context.extensionPath,
        initialConfig,
        chatClient,
        intentDetector,
        codebaseContext,
        editor,
        secretStorage,
        localStorage,
        rgPath
    )
    disposables.push(
        chatProvider,
        vscode.window.registerWebviewViewProvider('cody.chat', chatProvider, {
            webviewOptions: { retainContextWhenHidden: true },
        }),
        { dispose: () => vscode.commands.executeCommand('setContext', 'cody.activated', false) }
    )

    const executeRecipe = async (recipe: string): Promise<void> => {
        await vscode.commands.executeCommand('cody.chat.focus')
        await chatProvider.executeRecipe(recipe)
    }

    // Reguster Commands
    disposables.push(
        // File Chat Provider
        vscode.commands.registerCommand('cody.file.chat', (reply: vscode.CommentReply) =>
            chatProvider.fileChat(reply, false)
        ),
        vscode.commands.registerCommand('cody.file.fix', (reply: vscode.CommentReply) =>
            chatProvider.fileChat(reply, true)
        ),
        vscode.commands.registerCommand('cody.file.delete', (thread: vscode.CommentThread) => {
            chatProvider.fileChatDelete(thread)
        }),
        // Toggle Chat
        vscode.commands.registerCommand('cody.toggle-enabled', async () => {
            await workspaceConfig.update(
                'cody.enabled',
                !workspaceConfig.get('cody.enabled'),
                vscode.ConfigurationTarget.Global
            )
            logEvent('CodyVSCodeExtension:codyToggleEnabled:clicked')
        }),
        // Access token
        vscode.commands.registerCommand('cody.set-access-token', async (args: any[]) => {
            const tokenInput = args?.length ? (args[0] as string) : await vscode.window.showInputBox()
            if (tokenInput === undefined || tokenInput === '') {
                return
            }
            await secretStorage.store(CODY_ACCESS_TOKEN_SECRET, tokenInput)
            logEvent('CodyVSCodeExtension:codySetAccessToken:clicked')
        }),
        vscode.commands.registerCommand('cody.delete-access-token', async () => {
            await secretStorage.delete(CODY_ACCESS_TOKEN_SECRET)
            logEvent('CodyVSCodeExtension:codyDeleteAccessToken:clicked')
        }),
        // Commands
        vscode.commands.registerCommand('cody.focus', () => vscode.commands.executeCommand('cody.chat.focus')),
        vscode.commands.registerCommand('cody.settings', () => chatProvider.setWebviewView('settings')),
        vscode.commands.registerCommand('cody.history', () => chatProvider.setWebviewView('history')),
        vscode.commands.registerCommand('cody.interactive.clear', () => chatProvider.clearAndRestartSession()),
        vscode.commands.registerCommand('cody.recipe.explain-code', () => executeRecipe('explain-code-detailed')),
        vscode.commands.registerCommand('cody.recipe.explain-code-high-level', () =>
            executeRecipe('explain-code-high-level')
        ),
        vscode.commands.registerCommand('cody.recipe.generate-unit-test', () => executeRecipe('generate-unit-test')),
        vscode.commands.registerCommand('cody.recipe.generate-docstring', () => executeRecipe('generate-docstring')),
        vscode.commands.registerCommand('cody.recipe.fixup', () => executeRecipe('fixup')),
        vscode.commands.registerCommand('cody.recipe.non-stop-cody', () => chatProvider.nonStopCody()),
        vscode.commands.registerCommand('cody.recipe.translate-to-language', () =>
            executeRecipe('translate-to-language')
        ),
        vscode.commands.registerCommand('cody.recipe.git-history', () => executeRecipe('git-history')),
        vscode.commands.registerCommand('cody.recipe.improve-variable-names', () =>
            executeRecipe('improve-variable-names')
        ),
        vscode.commands.registerCommand('cody.recipe.find-code-smells', async () => executeRecipe('find-code-smells')),
        // Register URI Handler for resolving token sending back from sourcegraph.com
        vscode.window.registerUriHandler({
            handleUri: async (uri: vscode.Uri) => {
                await workspaceConfig.update('cody.serverEndpoint', DOTCOM_URL.href, vscode.ConfigurationTarget.Global)
                const token = new URLSearchParams(uri.query).get('code')
                if (token && token.length > 8) {
                    await context.secrets.store(CODY_ACCESS_TOKEN_SECRET, token)
                    const isAuthed = await isValidLogin({
                        serverEndpoint: DOTCOM_URL.href,
                        accessToken: token,
                        customHeaders: config.customHeaders,
                    })
                    await chatProvider.sendLogin(isAuthed)
                    logEvent(
                        'CodyVSCodeExtension:codySetAccessToken:clicked',
                        { serverEndpoint: config.serverEndpoint },
                        { serverEndpoint: config.serverEndpoint }
                    )
                    void vscode.window.showInformationMessage('Token has been retreived and updated successfully')
                }
            },
        })
    )

    // experimental flags
    if (initialConfig.experimentalSuggest) {
        // TODO(sqs): make this listen to config and not just use initialConfig
        const docprovider = new CompletionsDocumentProvider()
        disposables.push(vscode.workspace.registerTextDocumentContentProvider('cody', docprovider))

        const history = new History()
        const completionsProvider = new CodyCompletionItemProvider(completionsClient, docprovider, history)
        disposables.push(
            vscode.commands.registerCommand('cody.experimental.suggest', async () => {
                await completionsProvider.fetchAndShowCompletions()
            }),
            vscode.commands.registerCommand('cody.completions.inline.accepted', (...args) => {
                const params = {
                    type: 'inline',
                }
                logEvent('CodyVSCodeExtension:completion:accepted', params, params)
            }),
            vscode.languages.registerInlineCompletionItemProvider({ scheme: 'file' }, completionsProvider)
        )
    }
    if (initialConfig.experimentalNonStop) {
        void vscode.commands.executeCommand('setContext', 'cody.experimental.nonStop.enabled', true)
    }

    return {
        disposable: vscode.Disposable.from(...disposables),
        onConfigurationChange: newConfig => {
            chatProvider.onConfigurationChange(newConfig)
            externalServicesOnDidConfigurationChange(newConfig)
        },
    }
}
