import React, { useMemo, useState } from 'react'

import { mdiArrowLeft, mdiFileDocumentOutline, mdiFolderOutline, mdiSourceRepository } from '@mdi/js'
import { VSCodeProgressRing } from '@vscode/webview-ui-toolkit/react'
import classNames from 'classnames'
import { catchError } from 'rxjs/operators'

import { fetchTreeEntries } from '@sourcegraph/shared/src/backend/repo'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import type { QueryState } from '@sourcegraph/shared/src/search'
import type { RepositoryMatch } from '@sourcegraph/shared/src/search/stream'
import { Icon, PageHeader, useObservable, H4, Text, Button } from '@sourcegraph/wildcard'

import type { WebviewPageProps } from '../platform/context'

import styles from './RepoView.module.scss'

interface RepoViewProps extends Pick<WebviewPageProps, 'extensionCoreAPI' | 'platformContext' | 'instanceURL'> {
    onBackToSearchResults: () => void
    // Debt: just use repository name and make GraphQL Repository query to get metadata.
    // This will enable more info (like description) when navigating here from file matches.
    repositoryMatch: Pick<RepositoryMatch, 'repository' | 'branches' | 'description'>
    setQueryState: (query: QueryState) => void
}

export const RepoView: React.FunctionComponent<React.PropsWithChildren<RepoViewProps>> = ({
    extensionCoreAPI,
    platformContext,
    repositoryMatch,
    onBackToSearchResults,
    instanceURL,
    setQueryState,
}) => {
    const [directoryStack, setDirectoryStack] = useState<string[]>([])

    // File tree results are memoized, so going back isn't expensive.
    const treeEntries = useObservable(
        useMemo(
            () =>
                fetchTreeEntries({
                    repoName: repositoryMatch.repository,
                    commitID: '',
                    revision: repositoryMatch.branches?.[0] ?? 'HEAD',
                    filePath: directoryStack.length > 0 ? directoryStack.at(-1)! : '',
                    requestGraphQL: platformContext.requestGraphQL,
                }).pipe(
                    catchError(error => {
                        console.error(error, { repositoryMatch })
                        // TODO: remove and add error boundary in searchresultsview
                        return []
                    })
                ),
            [platformContext, repositoryMatch, directoryStack]
        )
    )

    const onPreviousDirectory = (): void => {
        const newDirectoryStack = directoryStack.slice(0, -1)
        setQueryState({
            query: `repo:^${repositoryMatch.repository}$ ${
                newDirectoryStack.length > 0 ? `file:^${newDirectoryStack.at(-1)}` : ''
            }`,
        })
        setDirectoryStack(newDirectoryStack)
    }

    const onSelect = (isDirectory: boolean, path: string, url: string): void => {
        const host = new URL(instanceURL).host
        if (isDirectory) {
            setQueryState({ query: `repo:^${repositoryMatch.repository}$ file:^${path}` })
            setDirectoryStack([...directoryStack, path])
        } else {
            extensionCoreAPI.openSourcegraphFile(`sourcegraph://${host}${url}`).catch(error => {
                console.error('Error opening Sourcegraph file', error)
            })
        }
    }

    return (
        <section className="mb-3 p-2">
            <Button
                variant="link"
                outline={true}
                size="sm"
                onClick={onBackToSearchResults}
                className="test-back-to-search-view-btn shadow-none"
            >
                <Icon aria-hidden={true} className="mr-1" svgPath={mdiArrowLeft} />
                Back to search view
            </Button>
            {directoryStack.length > 0 && (
                <Button variant="link" outline={true} size="sm" onClick={onPreviousDirectory} className="shadow-none">
                    <Icon aria-hidden={true} className="mr-1" svgPath={mdiArrowLeft} />
                    Back to previous directory
                </Button>
            )}
            <PageHeader
                path={[{ icon: mdiSourceRepository, text: displayRepoName(repositoryMatch.repository) }]}
                className="mb-1 mt-3 test-tree-page-title"
            />
            {repositoryMatch.description && <Text className="mt-0 text-muted">{repositoryMatch.description}</Text>}
            <div className={classNames(styles.section)}>
                <H4>Files and directories</H4>
                {treeEntries === undefined ? (
                    <VSCodeProgressRing />
                ) : (
                    <div className={classNames('pr-2', styles.treeEntriesSectionColumns)}>
                        {treeEntries.entries.map(entry => (
                            <Button
                                variant="link"
                                size="sm"
                                key={entry.name}
                                className={classNames(
                                    'test-page-file-decorable shadow-none',
                                    styles.treeEntry,
                                    entry.isDirectory && 'font-weight-bold',
                                    `test-tree-entry-${entry.isDirectory ? 'directory' : 'file'}`,
                                    treeEntries.entries.length < 7 && styles.treeEntryNoColumns
                                )}
                                title={entry.path}
                                data-testid="tree-entry"
                                onClick={() => onSelect(entry.isDirectory, entry.path, entry.url)}
                            >
                                <div
                                    className={classNames(
                                        'd-flex align-items-center justify-content-between test-file-decorable-name overflow-hidden'
                                    )}
                                >
                                    <span>
                                        <Icon
                                            aria-label={entry.isDirectory ? 'Folder' : 'File'}
                                            className="mr-1 text-muted"
                                            svgPath={entry.isDirectory ? mdiFolderOutline : mdiFileDocumentOutline}
                                        />
                                        {entry.name}
                                        {entry.isDirectory && '/'}
                                    </span>
                                </div>
                            </Button>
                        ))}
                    </div>
                )}
            </div>
        </section>
    )
}
