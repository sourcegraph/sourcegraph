import type { Observable } from 'rxjs'
import * as vscode from 'vscode'

import polyfillEventSource from '@sourcegraph/shared/src/polyfills/vendor/eventSource'
import { LATEST_VERSION } from '@sourcegraph/shared/src/search/stream'

import { getProxyAgent } from '../backend/fetch'
import type { initializeSourcegraphSettings } from '../backend/sourcegraphSettings'
import { initializeCodeIntel } from '../code-intel/initialize'
import type { ExtensionCoreAPI } from '../contract'
import type { SourcegraphFileSystemProvider } from '../file-system/SourcegraphFileSystemProvider'
import { SearchPatternType } from '../graphql-operations'
import { endpointRequestHeadersSetting, endpointSetting } from '../settings/endpointSetting'

import {
    initializeHelpSidebarWebview,
    initializeSearchPanelWebview,
    initializeSearchSidebarWebview,
} from './initialize'
import { scretTokenKey } from './platform/AuthProvider'

// Track current active webview panel to make sure only one panel exists at a time
let currentSearchPanel: vscode.WebviewPanel | 'initializing' | undefined
let searchSidebarWebviewView: vscode.WebviewView | 'initializing' | undefined

export function registerWebviews({
    context,
    extensionCoreAPI,
    initializedPanelIDs,
    sourcegraphSettings,
    fs,
    instanceURL,
}: {
    context: vscode.ExtensionContext
    extensionCoreAPI: ExtensionCoreAPI
    initializedPanelIDs: Observable<string>
    sourcegraphSettings: ReturnType<typeof initializeSourcegraphSettings>
    fs: SourcegraphFileSystemProvider
    instanceURL: string
}): void {
    // TODO if remote files are open from previous session, we need
    // to focus search sidebar to activate code intel (load extension host)

    // Register URI Handler to resolve data sending back from Browser
    const handleUri = async (uri: vscode.Uri): Promise<void> => {
        const token = new URLSearchParams(uri.query).get('code')
        // TODO: Decrypt token
        // TODO: Match returnedNonce to stored nonce
        if (token && token.length > 8) {
            await context.secrets.store(scretTokenKey, token)
            await vscode.window.showInformationMessage('Token has been retreived and updated successfully')
        }
    }

    context.subscriptions.push(
        vscode.window.registerUriHandler({
            handleUri,
        })
    )

    // Update `EventSource` Authorization header on access token / headers change.
    // It will also be changed when the token has been changed --handled by Auth Provider
    context.subscriptions.push(
        vscode.workspace.onDidChangeConfiguration(async config => {
            const session = await vscode.authentication.getSession(endpointSetting(), [], { forceNewSession: false })
            if (config.affectsConfiguration('sourcegraph.requestHeaders') && session) {
                const newCustomHeaders = endpointRequestHeadersSetting()
                polyfillEventSource(
                    session.accessToken ? { Authorization: `token ${session.accessToken}`, ...newCustomHeaders } : {},
                    getProxyAgent()
                )
            }
        })
    )

    // Open Sourcegraph search tab on `sourcegraph.search` command.
    context.subscriptions.push(
        vscode.commands.registerCommand('sourcegraph.search', async () => {
            // If text selected, submit search for it. Capture selection first.
            const activeEditor = vscode.window.activeTextEditor
            const selection = activeEditor?.selection
            const selectedQuery = activeEditor?.document.getText(selection)

            // Focus search sidebar in case this command was the activation event,
            // as opposed to visibiilty of sidebar.
            if (!searchSidebarWebviewView) {
                focusSearchSidebar()
            }

            if (currentSearchPanel && currentSearchPanel !== 'initializing') {
                currentSearchPanel.reveal()
            } else if (!currentSearchPanel) {
                sourcegraphSettings.refreshSettings()

                currentSearchPanel = 'initializing'

                const { webviewPanel, searchPanelAPI } = await initializeSearchPanelWebview({
                    extensionUri: context.extensionUri,
                    extensionCoreAPI,
                    initializedPanelIDs,
                })

                currentSearchPanel = webviewPanel

                webviewPanel.onDidChangeViewState(() => {
                    if (webviewPanel.active) {
                        extensionCoreAPI.emit({ type: 'search_panel_focused' })
                        focusSearchSidebar()
                        searchPanelAPI.focusSearchBox().catch(() => {})
                    }

                    if (webviewPanel.visible) {
                        searchPanelAPI.focusSearchBox().catch(() => {})
                    }

                    if (!webviewPanel.visible) {
                        // TODO emit event (should go to idle state if not remote browsing)
                        extensionCoreAPI.emit({ type: 'search_panel_unfocused' })
                    }
                })

                webviewPanel.onDidDispose(() => {
                    currentSearchPanel = undefined
                    // Ideally focus last used sidebar tab on search panel close. In lieu of that (for v1),
                    // just focus the file explorer if the search sidebar is currently focused.
                    if (searchSidebarWebviewView !== 'initializing' && searchSidebarWebviewView?.visible) {
                        focusFileExplorer()
                    }
                    // Clear search result
                    extensionCoreAPI.emit({ type: 'search_panel_disposed' })
                })
            }

            if (selectedQuery) {
                extensionCoreAPI.streamSearch(selectedQuery, {
                    patternType: SearchPatternType.standard,
                    caseSensitive: false,
                    version: LATEST_VERSION,
                    trace: undefined,
                    sourcegraphURL: instanceURL,
                })
            }
        })
    )

    context.subscriptions.push(
        vscode.window.registerWebviewViewProvider(
            'sourcegraph.searchSidebar',
            {
                // This typically will be called only once since `retainContextWhenHidden` is set to `true`.
                resolveWebviewView: (webviewView, _context, _token) => {
                    const { searchSidebarAPI } = initializeSearchSidebarWebview({
                        extensionUri: context.extensionUri,
                        extensionCoreAPI,
                        webviewView,
                    })
                    searchSidebarWebviewView = webviewView
                    // Initialize search panel.
                    openSearchPanelCommand()

                    initializeCodeIntel({ context, fs, searchSidebarAPI })

                    // Bring search panel back if it was previously closed on sidebar visibility change
                    webviewView.onDidChangeVisibility(() => {
                        if (webviewView.visible) {
                            openSearchPanelCommand()
                        }
                    })
                },
            },
            { webviewOptions: { retainContextWhenHidden: true } }
        )
    )

    context.subscriptions.push(
        vscode.window.registerWebviewViewProvider(
            'sourcegraph.helpSidebar',
            {
                // This typically will be called only once since `retainContextWhenHidden` is set to `true`.
                resolveWebviewView: (webviewView, _context, _token) => {
                    initializeHelpSidebarWebview({
                        extensionUri: context.extensionUri,
                        extensionCoreAPI,
                        webviewView,
                    })
                },
            },
            { webviewOptions: { retainContextWhenHidden: true } }
        )
    )

    // Clone Remote Git Repos Locally using VS Code Git API
    // https://github.com/microsoft/vscode/issues/48428
    context.subscriptions.push(
        vscode.commands.registerCommand('sourcegraph.gitClone', async () => {
            const editor = vscode.window.activeTextEditor
            if (!editor) {
                throw new Error('No active editor')
            }
            const uri = editor.document.uri.path
            const gitUrl = `https:/${uri.split('@')[0]}.git`
            const vsCodeCloneUrl = `vscode://vscode.git/clone?url=${gitUrl}`
            await vscode.env.openExternal(vscode.Uri.parse(vsCodeCloneUrl))
            // vscode://vscode.git/clone?url=${gitUrl}
        })
    )
}

function openSearchPanelCommand(): void {
    vscode.commands.executeCommand('sourcegraph.search').then(
        () => {},
        error => {
            console.error(error)
        }
    )
}

function focusSearchSidebar(): void {
    vscode.commands.executeCommand('sourcegraph.searchSidebar.focus').then(
        () => {},
        error => {
            console.error(error)
        }
    )
}

export function focusSearchPanel(): void {
    if (currentSearchPanel && currentSearchPanel !== 'initializing') {
        currentSearchPanel.reveal()
    }
}

function focusFileExplorer(): void {
    vscode.commands.executeCommand('workbench.view.explorer').then(
        () => {},
        error => {
            console.error(error)
        }
    )
}

export async function openSourcegraphLinks(uri: string): Promise<void> {
    await vscode.env.openExternal(vscode.Uri.parse(uri))
    return
}

export async function copySourcegraphLinks(uri: string): Promise<void> {
    try {
        await vscode.env.clipboard.writeText(uri)
        await vscode.window.showInformationMessage('Link Copied!')
    } catch (error) {
        console.error('Error copying search link to clipboard:', error)
    }

    return
}
