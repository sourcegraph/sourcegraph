import type * as Comlink from 'comlink'
import { EMPTY, of } from 'rxjs'
import { first, switchMap } from 'rxjs/operators'
import * as vscode from 'vscode'

import { finallyReleaseProxy, wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { makeRepoGitURI } from '@sourcegraph/shared/src/util/url'

import type { SearchSidebarAPI } from '../contract'
import type { SourcegraphFileSystemProvider } from '../file-system/SourcegraphFileSystemProvider'

export class SourcegraphHoverProvider implements vscode.HoverProvider {
    constructor(
        private readonly fs: SourcegraphFileSystemProvider,
        private readonly sourcegraphExtensionHostAPI: Comlink.Remote<SearchSidebarAPI>
    ) {}

    public async provideHover(
        document: vscode.TextDocument,
        position: vscode.Position,
        token: vscode.CancellationToken
    ): Promise<vscode.Hover | undefined> {
        const uri = this.fs.sourcegraphUri(document.uri)
        const extensionHostUri = makeRepoGitURI({
            repoName: uri.repositoryName,
            revision: uri.revision,
            filePath: uri.path,
        })

        const definitions = wrapRemoteObservable(
            this.sourcegraphExtensionHostAPI.getHover({
                textDocument: {
                    uri: extensionHostUri,
                },
                position: {
                    line: position.line,
                    character: position.character,
                },
            })
        )
            .pipe(
                finallyReleaseProxy(),
                switchMap(({ isLoading, result }) => {
                    if (isLoading) {
                        return EMPTY
                    }

                    const prefix =
                        result?.aggregatedBadges?.reduce((prefix, badge) => {
                            if (badge.linkURL) {
                                return prefix + `[${badge.text}](${badge.linkURL})\n`
                            }
                            return prefix + `${badge.text}\n`
                        }, `![*](${sourcegraphLogoDataURI}) `) || ''

                    return of<vscode.Hover>({
                        contents: [
                            ...(result?.contents ?? []).map(
                                content => new vscode.MarkdownString(prefix + content.value)
                            ),
                        ],
                    })
                }),
                first()
            )
            .toPromise()

        token.onCancellationRequested(() => {
            // Debt: manually create promise so we can cancel request.
        })

        return definitions
    }
}

const sourcegraphLogoDataURI =
    'data:image/svg+xml;base64,PHN2ZyBoZWlnaHQ9IjE0IiB2aWV3Qm94PSIwIDAgNTIgNTIiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+PHBhdGggZD0iTTMwLjggNTEuOGMtMi44LjUtNS41LTEuMy02LTQuMUwxNy4yIDYuMmMtLjUtMi44IDEuMy01LjUgNC4xLTZzNS41IDEuMyA2IDQuMWw3LjYgNDEuNWMuNSAyLjgtMS40IDUuNS00LjEgNnoiIGZpbGw9IiNGRjU1NDMiLz48cGF0aCBkPSJNMTAuOSA0NC43QzkuMSA0NSA3LjMgNDQuNCA2IDQzYy0xLjgtMi4yLTEuNi01LjQuNi03LjJMMzguNyA4LjVjMi4yLTEuOCA1LjQtMS42IDcuMi42IDEuOCAyLjIgMS42IDUuNC0uNiA3LjJsLTMyIDI3LjNjLS43LjYtMS42IDEtMi40IDEuMXoiIGZpbGw9IiNBMTEyRkYiLz48cGF0aCBkPSJNNDYuOCAzOC4xYy0uOS4yLTEuOC4xLTIuNi0uMkw0LjQgMjMuOGMtMi43LTEtNC4xLTMuOS0zLjEtNi42IDEtMi43IDMuOS00LjEgNi42LTMuMWwzOS43IDE0LjFjMi43IDEgNC4xIDMuOSAzLjEgNi42LS42IDEuOC0yLjIgMy0zLjkgMy4zeiIgZmlsbD0iIzAwQ0JFQyIvPjwvc3ZnPg=='
