import * as Comlink from 'comlink'
import type { Observable } from 'rxjs'
import { filter, first } from 'rxjs/operators'
import * as vscode from 'vscode'

import type { ExtensionCoreAPI, HelpSidebarAPI, SearchPanelAPI, SearchSidebarAPI } from '../contract'
import { endpointSetting } from '../settings/endpointSetting'

import { createEndpointsForWebview } from './comlink/extensionEndpoint'

interface SourcegraphWebviewConfig {
    extensionUri: vscode.Uri
    extensionCoreAPI: ExtensionCoreAPI
}

export async function initializeSearchPanelWebview({
    extensionUri,
    extensionCoreAPI,
    initializedPanelIDs,
}: SourcegraphWebviewConfig & {
    initializedPanelIDs: Observable<string>
}): Promise<{
    searchPanelAPI: Comlink.Remote<SearchPanelAPI>
    webviewPanel: vscode.WebviewPanel
}> {
    const webviewPath = vscode.Uri.joinPath(extensionUri, 'dist', 'webview')
    const extensionsDistributionPath = vscode.Uri.joinPath(extensionUri, 'dist', 'extensions')

    const panel = vscode.window.createWebviewPanel('sourcegraphSearch', 'Sourcegraph', vscode.ViewColumn.One, {
        enableScripts: true,
        retainContextWhenHidden: true,
        enableFindWidget: true,
        localResourceRoots: [webviewPath, extensionsDistributionPath],
    })

    const extensionsDistributionWebviewPath = panel.webview.asWebviewUri(extensionsDistributionPath)
    const scriptSource = panel.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'searchPanel.js'))
    const cssModuleSource = panel.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'searchPanel.css'))
    const styleSource = panel.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'style.css'))

    const { proxy, expose, panelId } = createEndpointsForWebview(panel)

    // Wait for the webview to initialize or else messages will be dropped
    const hasInitialized = initializedPanelIDs
        .pipe(
            filter(initializedPanelId => initializedPanelId === panelId),
            first()
        )
        .toPromise()

    // Get a proxy for the search panel API to communicate with the Webview.
    const searchPanelAPI = Comlink.wrap<SearchPanelAPI>(proxy)

    // Expose the "Core" extension API to the Webview.
    Comlink.expose(extensionCoreAPI, expose)

    panel.iconPath = vscode.Uri.joinPath(extensionUri, 'images', 'logo.svg')

    // Apply Content-Security-Policy
    // panel.webview.cspSource comes from the webview object
    panel.webview.html = `<!DOCTYPE html>
    <html lang="en" data-panel-id="${panelId}" data-extensions-dist-path=${extensionsDistributionWebviewPath.toString()}>
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <meta http-equiv="Content-Security-Policy" content="default-src 'none'; child-src data: ${
            panel.webview.cspSource
        }; img-src data: vscode-resource: https:; script-src ${panel.webview.cspSource}; style-src data: ${
        panel.webview.cspSource
    } vscode-resource: 'unsafe-inline' http: https: data:; connect-src 'self' http: https:; frame-src https:; font-src ${
        panel.webview.cspSource
    };">
        <title>Sourcegraph Search</title>
        <link rel="stylesheet" href="${styleSource.toString()}" />
        <link rel="stylesheet" href="${cssModuleSource.toString()}" />
    </head>
    <body class="search-panel">
        <div id="root" />
        <script type="module" src="${scriptSource.toString()}"></script>
    </body>
    </html>`

    await hasInitialized

    return {
        searchPanelAPI,
        webviewPanel: panel,
    }
}

// TODO expand CSP for Sourcegraph extension loading
export function initializeSearchSidebarWebview({
    extensionUri,
    extensionCoreAPI,
    webviewView,
}: SourcegraphWebviewConfig & {
    webviewView: vscode.WebviewView
}): {
    searchSidebarAPI: Comlink.Remote<SearchSidebarAPI>
} {
    const webviewPath = vscode.Uri.joinPath(extensionUri, 'dist', 'webview')
    const extensionsDistributionPath = vscode.Uri.joinPath(extensionUri, 'dist', 'extensions')
    const extensionsDistributionWebviewPath = webviewView.webview.asWebviewUri(extensionsDistributionPath)

    webviewView.webview.options = {
        enableScripts: true,
        localResourceRoots: [webviewPath, extensionsDistributionPath],
    }

    const scriptSource = webviewView.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'searchSidebar.js'))
    const cssModuleSource = webviewView.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'searchSidebar.css'))
    const styleSource = webviewView.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'style.css'))

    const { proxy, expose, panelId } = createEndpointsForWebview(webviewView)

    // Get a proxy for the Sourcegraph Webview API to communicate with the Webview.
    const searchSidebarAPI = Comlink.wrap<SearchSidebarAPI>(proxy)

    // Expose the Sourcegraph VS Code Extension API to the Webview.
    Comlink.expose(extensionCoreAPI, expose)

    // Apply Content-Security-Policy
    webviewView.webview.html = `<!DOCTYPE html>
    <html lang="en" data-panel-id="${panelId}" data-instance-url=${endpointSetting()} data-extensions-dist-path=${extensionsDistributionWebviewPath.toString()}>
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <meta http-equiv="Content-Security-Policy" content="default-src 'none'; child-src data: ${
            webviewView.webview.cspSource
        }; worker-src blob: data:; img-src data: https:; script-src blob: https:; style-src 'unsafe-inline' ${
        webviewView.webview.cspSource
    } http: https: data:; connect-src 'self' http: https:; font-src vscode-resource: blob: https:;">
        <title>Sourcegraph Search</title>
        <link rel="stylesheet" href="${styleSource.toString()}" />
        <link rel="stylesheet" href="${cssModuleSource.toString()}" />
    </head>
    <body class="search-sidebar">
        <div id="root" />
        <script type="module" src="${scriptSource.toString()}"></script>
    </body>
    </html>`

    return {
        searchSidebarAPI,
    }
}

export function initializeHelpSidebarWebview({
    extensionUri,
    extensionCoreAPI,
    webviewView,
}: SourcegraphWebviewConfig & {
    webviewView: vscode.WebviewView
}): {
    helpSidebarAPI: Comlink.Remote<HelpSidebarAPI>
} {
    const webviewPath = vscode.Uri.joinPath(extensionUri, 'dist', 'webview')

    webviewView.webview.options = {
        enableScripts: true,
        localResourceRoots: [webviewPath],
    }

    const scriptSource = webviewView.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'helpSidebar.js'))
    const cssModuleSource = webviewView.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'helpSidebar.css'))
    const styleSource = webviewView.webview.asWebviewUri(vscode.Uri.joinPath(webviewPath, 'style.css'))

    const { proxy, expose, panelId } = createEndpointsForWebview(webviewView)

    // Get a proxy for the Sourcegraph Webview API to communicate with the Webview.
    const helpSidebarAPI = Comlink.wrap<HelpSidebarAPI>(proxy)

    // Expose the Sourcegraph VS Code Extension API to the Webview.
    Comlink.expose(extensionCoreAPI, expose)

    // Apply Content-Security-Policy
    webviewView.webview.html = `<!DOCTYPE html>
    <html lang="en" data-panel-id="${panelId}" >
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: https:; font-src ${
            webviewView.webview.cspSource
        }; style-src ${webviewView.webview.cspSource}; script-src ${webviewView.webview.cspSource};">
        <title>Help and Feedback</title>
        <link rel="stylesheet" href="${styleSource.toString()}" />
        <link rel="stylesheet" href="${cssModuleSource.toString()}" />
    </head>
        <div id="root" />
        <script type="module" src="${scriptSource.toString()}"></script>
    </body>
    </html>`

    return {
        helpSidebarAPI,
    }
}
