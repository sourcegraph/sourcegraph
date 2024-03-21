import * as vscode from 'vscode'

import { log } from '../log'

import type { SourcegraphFileSystemProvider } from './SourcegraphFileSystemProvider'
import { SourcegraphUri } from './SourcegraphUri'

export class FilesTreeDataProvider implements vscode.TreeDataProvider<string> {
    constructor(public readonly fs: SourcegraphFileSystemProvider) {
        fs.onDidDownloadRepositoryFilenames(() => this.didChangeTreeData.fire(undefined))
    }

    private _isViewVisible = false
    private isExpandedNode = new Set<string>()
    private treeView: vscode.TreeView<string> | undefined
    private activeUri: vscode.Uri | undefined
    private selectedRepository: string | undefined
    private didFocusToken = new vscode.CancellationTokenSource()
    private treeItemCache = new Map<string, vscode.TreeItem>()
    private readonly didChangeTreeData = new vscode.EventEmitter<string | undefined>()
    public readonly onDidChangeTreeData: vscode.Event<string | undefined> = this.didChangeTreeData.event

    public activeTextDocument(): SourcegraphUri | undefined {
        return this.activeUri && this.activeUri.scheme === 'sourcegraph'
            ? this.fs.sourcegraphUri(this.activeUri)
            : undefined
    }

    public isViewVisible(): boolean {
        return this._isViewVisible
    }

    public setTreeView(treeView: vscode.TreeView<string>): void {
        this.treeView = treeView
        treeView.onDidChangeSelection(async event => {
            // Check if a repository is selected for removing purpose
            await this.isRepository(event.selection[0])
        })
        treeView.onDidChangeVisibility(async event => {
            const didBecomeVisible = !this._isViewVisible && event.visible
            this._isViewVisible = event.visible
            if (didBecomeVisible) {
                // NOTE: do not remove the line below even if you think it
                // doesn't have an effect. Before you remove this line, make
                // sure that the following steps don't cause the "Collapse All"
                // button to become disabled:
                //   1. Close "Files" view.
                //   2. Execute "Reload window" command.
                //   3. After VS Code loads, open the "Files" view.
                this.didChangeTreeData.fire(undefined)
                await this.didFocus(this.activeUri)
            }
        })
        treeView.onDidExpandElement(async event => {
            await this.isRepository(event.element)
            this.isExpandedNode.add(event.element)
        })
        treeView.onDidCollapseElement(async event => {
            await this.isRepository(event.element)
            this.isExpandedNode.delete(event.element)
        })
    }

    public async isRepository(selectedUri: string): Promise<void> {
        const isRepo = [...this.fs.allRepositoryUris()].includes(selectedUri)
        this.selectedRepository = isRepo ? selectedUri : undefined
        await vscode.commands.executeCommand('setContext', 'sourcegraph.removeRepository', isRepo)
    }

    public async getParent(uriString?: string): Promise<string | undefined> {
        // log.appendLine(`getParent(${uriString})`)
        try {
            // Implementation note: this method is not implemented as
            // `SourcegraphUri.parse(uri).parentUri()` because that would return
            // URIs to directories that don't exist because they have no siblings
            // and are therefore automatically merged with their parent. For example,
            // imagine the following folder structure:
            //   .gitignore
            //   .github/workflows/ci.yml
            //   src/command.ts
            //   src/browse.ts
            // The parent of `.github/workflows/ci.yml` is `.github/` because the `workflows/`
            // directory has no sibling.
            if (!uriString) {
                return undefined
            }
            const uri = SourcegraphUri.parse(uriString)
            if (!uri.path) {
                return undefined
            }
            let ancestor: string | undefined = uri.repositoryUri()
            let children = await this.getChildren(ancestor)
            while (ancestor) {
                const isParent = children?.includes(uriString)
                if (isParent) {
                    break
                }
                ancestor = children?.find(childUri => {
                    const child = SourcegraphUri.parse(childUri)
                    return child.path && uri.path?.startsWith(child.path + '/')
                })
                if (!ancestor) {
                    log.errorAndThrow(`getParent(${uriString || 'undefined'}) nothing startsWith`)
                }
                children = await this.getChildren(ancestor)
            }
            return ancestor
        } catch (error) {
            log.errorAndThrow(`getParent(${uriString || 'undefined'})`, error)
            return undefined
        }
    }

    public async getChildren(uriString?: string): Promise<string[] | undefined> {
        try {
            if (!uriString) {
                const repos = [...this.fs.allRepositoryUris()]
                return repos.map(repo => repo.replace('https://', 'sourcegraph://'))
            }
            const uri = SourcegraphUri.parse(uriString)
            const tree = await this.fs.getFileTree(uri)
            const directChildren = tree.directChildren(uri.path || '')
            for (const child of directChildren) {
                this.treeItemCache.set(child, this.newTreeItem(SourcegraphUri.parse(child), uri, directChildren.length))
            }
            return directChildren
        } catch (error) {
            return log.errorAndThrow(`getChildren(${uriString || ''})`, error)
        }
    }

    public async focusActiveFile(): Promise<void> {
        await vscode.commands.executeCommand('sourcegraph.files.focus')
        await this.didFocus(this.activeUri)
    }

    public async didFocus(vscodeUri: vscode.Uri | undefined): Promise<void> {
        this.didFocusToken.cancel()
        this.didFocusToken = new vscode.CancellationTokenSource()
        this.activeUri = vscodeUri
        await vscode.commands.executeCommand(
            'setContext',
            'sourcegraph.canFocusActiveDocument',
            vscodeUri?.scheme === 'sourcegraph'
        )
        if (vscodeUri && vscodeUri.scheme === 'sourcegraph' && this.treeView && this._isViewVisible) {
            const uri = this.fs.sourcegraphUri(vscodeUri)
            if (uri.uri === this.fs.emptyFileUri()) {
                return
            }
            await this.fs.downloadFiles(uri)
            await this.didFocusString(uri, true, this.didFocusToken.token)
        }
    }

    public isSourcegrapeRemoteFile(vscodeUri: vscode.Uri | undefined): boolean {
        if (vscodeUri && vscodeUri.scheme === 'sourcegraph' && this.treeView && this._isViewVisible) {
            return true
        }
        return false
    }

    public async getTreeItem(uriString: string): Promise<vscode.TreeItem> {
        try {
            const fromCache = this.treeItemCache.get(uriString)
            if (fromCache) {
                return fromCache
            }
            const uri = SourcegraphUri.parse(uriString)
            const parentUri = await this.getParent(uri.uri)
            return this.newTreeItem(uri, parentUri ? SourcegraphUri.parse(parentUri) : undefined, 0)
        } catch (error) {
            log.errorAndThrow(`getTreeItem(${uriString})`, error)
        }
        return {}
    }

    private async didFocusString(
        uri: SourcegraphUri,
        isDestinationNode: boolean,
        token: vscode.CancellationToken
    ): Promise<void> {
        try {
            if (this.treeView) {
                const parent = await this.getParent(uri.uri)
                if (parent) {
                    await this.didFocusString(SourcegraphUri.parse(parent), false, token)
                } else {
                    await this.getChildren(undefined)
                }
                if (token.isCancellationRequested) {
                    return
                }
                await this.treeView.reveal(uri.uri, {
                    focus: true,
                    select: isDestinationNode,
                    expand: !isDestinationNode,
                })
            }
        } catch (error) {
            log.error(`didFocusString(${uri.uri})`, error)
        }
    }

    // Remove selected repo from tree
    public async removeTreeItem(): Promise<void> {
        if (this.selectedRepository) {
            this.fs.removeRepository(this.selectedRepository)
        }
        this.selectedRepository = undefined
        await vscode.commands.executeCommand('setContext', 'sourcegraph.removeRepository', false)
        return this.didChangeTreeData.fire(undefined)
    }

    private newTreeItem(
        uri: SourcegraphUri,
        parent: SourcegraphUri | undefined,
        parentChildrenCount: number
    ): vscode.TreeItem {
        const command = uri.isFile()
            ? {
                  command: 'sourcegraph.openFile',
                  title: 'Open file',
                  toolbar: 'test',
                  arguments: [uri.uri],
              }
            : undefined
        // Check if this is a currently selected file
        let selectedFile = false
        if (
            vscode.window.activeTextEditor?.document &&
            uri.path === SourcegraphUri.parse(vscode.window.activeTextEditor?.document.uri.toString()).path
        ) {
            selectedFile = true
        }
        return {
            id: uri.uri,
            label: uri.treeItemLabel(parent),
            tooltip: uri.uri.replace('sourcegraph://', 'https://'),
            collapsibleState: uri.isFile()
                ? vscode.TreeItemCollapsibleState.None
                : parentChildrenCount === 0
                ? vscode.TreeItemCollapsibleState.Expanded
                : vscode.TreeItemCollapsibleState.Collapsed,
            command,
            resourceUri: vscode.Uri.parse(uri.uri),
            contextValue: !uri.isFile() ? 'directory' : selectedFile ? 'selected' : 'file',
        }
    }
}
