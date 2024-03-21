import * as React from 'react'

import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import { parseRepoRevision } from '@sourcegraph/shared/src/util/url'
import { Button, Tooltip, useIsTruncated } from '@sourcegraph/wildcard'

import { useOpenSearchResultsContext } from '../MatchHandlersContext'

/**
 * Splits the repository name into the dir and base components.
 */
export function splitPath(path: string): [string, string] {
    const components = path.split('/')
    return [components.slice(0, -1).join('/'), components.at(-1)!]
}

interface Props {
    repoName: string
    repoURL: string
    filePath: string
    fileURL: string
    repoDisplayName?: string
    className?: string
}

/**
 * A link to a repository or a file within a repository, formatted as "repo" or "repo > file". Unless you
 * absolutely need breadcrumb-like behavior, use this instead of FilePathBreadcrumb.
 */
export const RepoFileLink: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    repoDisplayName,
    repoName,
    repoURL,
    filePath,
    className,
}) => {
    /**
     * Use the custom hook useIsTruncated to check if overflow: ellipsis is activated for the element
     * We want to do it on mouse enter as browser window size might change after the element has been
     * loaded initially
     */
    const [titleReference, truncated, checkTruncation] = useIsTruncated()

    const [fileBase, fileName] = splitPath(filePath)

    const { openRepo, openFile } = useOpenSearchResultsContext()

    const getRepoAndRevision = (): { repoName: string; revision: string | undefined } => {
        // Example: `/github.com/sourcegraph/sourcegraph@main`
        const indexOfSeparator = repoURL.indexOf('/-/')
        let repoRevision: string
        if (indexOfSeparator === -1) {
            repoRevision = repoURL // the whole string
        } else {
            repoRevision = repoURL.slice(0, indexOfSeparator) // the whole string leading up to the separator (allows revision to be multiple path parts)
        }
        let { repoName, revision } = parseRepoRevision(repoRevision)
        // Remove leading slash
        if (repoName.startsWith('/')) {
            repoName = repoName.slice(1)
        }
        return { repoName, revision }
    }

    const onRepoClick = (): void => {
        const { repoName, revision } = getRepoAndRevision()

        openRepo({
            repository: repoName,
            branches: revision ? [revision] : undefined,
        })
    }

    const onFileClick = (): void => {
        const { repoName, revision } = getRepoAndRevision()
        openFile(repoName, { path: filePath, revision })
    }

    return (
        <Tooltip content={truncated ? (fileBase ? `${fileBase}/${fileName}` : fileName) : null}>
            <span>
                <div ref={titleReference} className={className} onMouseEnter={checkTruncation}>
                    <Button onClick={onRepoClick} className="btn-text-link">
                        {repoDisplayName || displayRepoName(repoName)}
                    </Button>{' '}
                    ›{' '}
                    <Button onClick={onFileClick} className="btn-text-link">
                        {fileBase ? `${fileBase}/` : null}
                        <strong>{fileName}</strong>
                    </Button>
                </div>
            </span>
        </Tooltip>
    )
}
