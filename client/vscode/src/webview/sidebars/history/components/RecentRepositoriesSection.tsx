import React, { useMemo, useState } from 'react'

import { mdiChevronDown, mdiChevronLeft } from '@mdi/js'
import classNames from 'classnames'

import { SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import { type EventLogResult, fetchRecentSearches } from '@sourcegraph/shared/src/search'
import { scanSearchQuery } from '@sourcegraph/shared/src/search/query/scanner'
import { isRepoFilter } from '@sourcegraph/shared/src/search/query/validate'
import { LATEST_VERSION } from '@sourcegraph/shared/src/search/stream'
import { Icon, H5, useObservable, Button } from '@sourcegraph/wildcard'

import { SearchPatternType } from '../../../../graphql-operations'
import type { HistorySidebarProps } from '../HistorySidebarView'

import styles from '../../search/SearchSidebarView.module.scss'

export const RecentRepositoriesSection: React.FunctionComponent<React.PropsWithChildren<HistorySidebarProps>> = ({
    platformContext,
    authenticatedUser,
    extensionCoreAPI,
}) => {
    const itemsToLoad = 15
    const [collapsed, setCollapsed] = useState(false)

    // Debt: lift this shared query up to HistorySidebarView.
    const recentRepositoriesResult = useObservable(
        useMemo(
            () => fetchRecentSearches(authenticatedUser.id, itemsToLoad, platformContext),
            [authenticatedUser.id, itemsToLoad, platformContext]
        )
    )

    if (!recentRepositoriesResult) {
        return null
    }

    const processedRepositories = processRepositories(recentRepositoriesResult)

    if (!processedRepositories) {
        return null
    }

    const onRecentRepositoryClick = (query: string): void => {
        platformContext.telemetryService.log('VSCERecentRepositoryClick')
        extensionCoreAPI
            .streamSearch(query, {
                // Debt: using defaults here. The recent repository should override these, though.
                caseSensitive: false,
                patternType: SearchPatternType.standard,
                version: LATEST_VERSION,
                trace: undefined,
            })
            .catch(error => {
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
                <H5 className="flex-grow-1">Recent Repositories</H5>
                <Icon aria-hidden={true} className="mr-1" svgPath={collapsed ? mdiChevronLeft : mdiChevronDown} />
            </Button>

            {!collapsed && (
                <div className={classNames('p-1', styles.sidebarSectionList)}>
                    {processedRepositories
                        .filter((search, index) => index < itemsToLoad)
                        .map((repository, index) => (
                            <div key={`${repository}-${index}`}>
                                <small className={styles.sidebarSectionListItem}>
                                    <Button
                                        variant="link"
                                        className="p-0 text-left text-decoration-none"
                                        onClick={() => onRecentRepositoryClick(`repo:${repository}`)}
                                    >
                                        <SyntaxHighlightedSearchQuery query={`r:${repository}`} />
                                    </Button>
                                </small>
                            </div>
                        ))}
                </div>
            )}
        </div>
    )
}

export function parseSearchURLQuery(query: string): string | undefined {
    const searchParameters = new URLSearchParams(query)
    return searchParameters.get('q') || undefined
}

function processRepositories(eventLogResult: EventLogResult): string[] | null {
    if (!eventLogResult) {
        return null
    }
    const recentlySearchedRepos: string[] = []
    for (const node of eventLogResult.nodes) {
        if (node.url) {
            const url = new URL(node.url)
            const queryFromURL = parseSearchURLQuery(url.search)
            const scannedQuery = scanSearchQuery(queryFromURL || '')
            if (scannedQuery.type === 'success') {
                for (const token of scannedQuery.term) {
                    if (isRepoFilter(token)) {
                        if (token.value && !recentlySearchedRepos.includes(token.value.value)) {
                            recentlySearchedRepos.push(token.value.value)
                        }
                    }
                }
            }
        }
    }
    return recentlySearchedRepos
}
