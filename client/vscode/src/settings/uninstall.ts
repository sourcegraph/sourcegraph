import vscode from 'vscode'

import type { EventSource } from '@sourcegraph/shared/src/graphql-operations'

import { version } from '../../package.json'
import { logEvent } from '../backend/eventLogger'

import { ANONYMOUS_USER_ID_KEY, type LocalStorageService } from './LocalStorageService'

// This function allows us to watch for uninstall event while still having access to the VS Code API
export function watchUninstall(eventSourceType: EventSource, localStorageService: LocalStorageService): void {
    const extensionName = 'sourcegraph.sourcegraph'
    try {
        const extensionPath = vscode.extensions.getExtension(extensionName)?.extensionPath
        const pathComponents = extensionPath?.split('/').slice(0, -1)
        const extensionsDirectoryPath = pathComponents?.join('/')
        // All upgrades, downgrades, and uninstalls will be logged in the .obsolete file
        pathComponents?.push('.obsolete')
        const uninstalledPath = pathComponents?.join('/')
        if (extensionsDirectoryPath && uninstalledPath) {
            // Watch the .obsolete file - it does not exist when VS Code is started
            // Check if uninstall has happened when the file was created or when changes are made
            const watchPattern = new vscode.RelativePattern(extensionsDirectoryPath, '.obsolete')
            const watchFileListener = vscode.workspace.createFileSystemWatcher(watchPattern)
            watchFileListener.onDidCreate(() => checkUninstall(uninstalledPath, extensionsDirectoryPath))
            watchFileListener.onDidChange(() => checkUninstall(uninstalledPath, extensionsDirectoryPath))
        }
    } catch (error) {
        console.error('failed to invoke uninstall:', error)
    }

    /**
     * Assume the extension has been uninstalled if the count of all the versions listed in
     * the .obsolete file is equal to the count of all the version-divided directories.
     * For example, if there are 5 versions of the extension were installed while there
     * are 4 versions of the extension listed in the .obsolete file pending to be deleted,
     * it means 1 version of the extension is still installed, therefore no uninstallation
     * has happened
     **/
    function checkUninstall(uninstalledPath: string, extensionsDirectoryPath: string): void {
        Promise.all([
            // .obsolete file includes all extensions versions that need to be remove at restart
            vscode.workspace.fs.readFile(vscode.Uri.file(uninstalledPath)),
            // Each versions of the extension has its own directory
            vscode.workspace.fs.readDirectory(vscode.Uri.parse(extensionsDirectoryPath)),
        ])
            .then(([obsoleteExtensionsRaw, extensionsDirectory]) => {
                const obsoleteExtensionsCount = Object.keys(JSON.parse(obsoleteExtensionsRaw.toString())).filter(id =>
                    id.includes(extensionName)
                ).length
                const downloadedExtensionsCount = extensionsDirectory
                    .map(([name]) => name)
                    .filter(id => id.includes(extensionName)).length
                // Compare count of extension name in .obsolete file vs count of directories with the same extension name
                if (downloadedExtensionsCount === obsoleteExtensionsCount) {
                    logEvent({
                        event: 'IDEUninstalled',
                        userCookieID: localStorageService.getValue(ANONYMOUS_USER_ID_KEY),
                        referrer: 'VSCE',
                        url: '',
                        source: eventSourceType,
                        argument: JSON.stringify({ editor: 'vscode', version }),
                        publicArgument: JSON.stringify({ editor: 'vscode', version }),
                    })
                }
            })
            .catch(error => console.error(error))
    }
}
