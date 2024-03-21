import React, { useMemo, useState } from 'react'

import { mdiChevronDown, mdiChevronLeft } from '@mdi/js'
import classNames from 'classnames'

import { type EventLogResult, fetchRecentFileViews } from '@sourcegraph/shared/src/search'
import { Icon, Link, H5, useObservable, Button } from '@sourcegraph/wildcard'

import type { HistorySidebarProps } from '../HistorySidebarView'

import styles from '../../search/SearchSidebarView.module.scss'

interface RecentFile {
    repoName: string
    filePath: string
    timestamp: string
    url: string
}

export const RecentFilesSection: React.FunctionComponent<React.PropsWithChildren<HistorySidebarProps>> = ({
    platformContext,
    authenticatedUser,
    extensionCoreAPI,
}) => {
    const itemsToLoad = 15
    const [collapsed, setCollapsed] = useState(false)

    // Debt: lift this shared query up to HistorySidebarView.
    const recentFilesResult = useObservable(
        useMemo(
            () => fetchRecentFileViews(authenticatedUser.id, itemsToLoad, platformContext),
            [authenticatedUser.id, itemsToLoad, platformContext]
        )
    )

    if (!recentFilesResult) {
        return null
    }

    const processedFiles = processRecentFiles(recentFilesResult)

    if (!processedFiles) {
        return null
    }

    const onRecentFileClick = (uri: string): void => {
        platformContext.telemetryService.log('VSCERecentFilesClick')
        extensionCoreAPI.openSourcegraphFile(uri).catch(error => {
            // TODO surface to user
            console.error('Error submitting search from Sourcegraph sidebar', error)
        })
    }

    return (
        <div className={styles.sidebarSection}>
            <Button
                variant="secondary"
                outline={true}
                className={styles.sidebarSectionCollapseButton}
                onClick={() => setCollapsed(!collapsed)}
                aria-label={`${collapsed ? 'Expand' : 'Collapse'} recent files`}
            >
                <H5 className="flex-grow-1">Recent Files</H5>
                <Icon aria-hidden={true} className="mr-1" svgPath={collapsed ? mdiChevronLeft : mdiChevronDown} />
            </Button>

            {!collapsed && (
                <div className={classNames('p-1', styles.sidebarSectionFileList)}>
                    {processedFiles
                        ?.filter((search, index) => index < itemsToLoad)
                        .map((recentFile, index) => (
                            <div key={index}>
                                <small key={index} className={styles.sidebarSectionListItem}>
                                    <Link
                                        data-testid="recent-files-item"
                                        to="/"
                                        onClick={() => onRecentFileClick(recentFile.url)}
                                    >
                                        {recentFile.repoName.split('@')[0]} › {recentFile.filePath}
                                    </Link>
                                </small>
                            </div>
                        ))}
                </div>
            )}
        </div>
    )
}

function processRecentFiles(eventLogResult?: EventLogResult): RecentFile[] | null {
    if (!eventLogResult) {
        return null
    }

    const recentFiles: RecentFile[] = []

    for (const node of eventLogResult.nodes) {
        if (node.argument && node.url) {
            const parsedArguments = JSON.parse(node.argument)
            let repoName = parsedArguments?.repoName as string
            let filePath = parsedArguments?.filePath as string

            if (!repoName || !filePath) {
                ;({ repoName, filePath } = extractFileInfoFromUrl(node.url))
            }

            if (
                filePath &&
                repoName &&
                !recentFiles.some(file => file.repoName === repoName && file.filePath === filePath) // Don't show the same file twice
            ) {
                recentFiles.push({
                    url: node.url.replace('https://', 'sourcegraph://'), // So that clicking on link would open the file directly
                    repoName,
                    filePath,
                    timestamp: node.timestamp,
                })
            }
        }
    }

    return recentFiles
}

function extractFileInfoFromUrl(url: string): { repoName: string; filePath: string } {
    const parsedUrl = new URL(url)

    // Remove first character as it's a '/'
    const [repoName, filePath] = parsedUrl.pathname.slice(1).split('/-/blob/')
    if (!repoName || !filePath) {
        return { repoName: '', filePath: '' }
    }
    return { repoName, filePath }
}
