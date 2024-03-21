import React from 'react'

import { mdiSourceFork, mdiArchive, mdiLock } from '@mdi/js'
import classNames from 'classnames'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'

import { LegacyResultContainer } from '@sourcegraph/branded'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import { getRepoMatchLabel, type RepositoryMatch } from '@sourcegraph/shared/src/search/stream'
import { Button, Icon } from '@sourcegraph/wildcard'

import { useOpenSearchResultsContext } from '../MatchHandlersContext'

import styles from './SearchResultsStyles.module.scss'

export interface RepoSearchResultProps {
    result: RepositoryMatch
    repoName: string
    onSelect: () => void
    containerClassName?: string
    as?: React.ElementType
    index: number
}

export const RepoSearchResult: React.FunctionComponent<RepoSearchResultProps> = ({
    result,
    onSelect,
    containerClassName,
    as,
    index,
}) => {
    /**
     * Use the custom hook useIsTruncated to check if overflow: ellipsis is activated for the element
     * We want to do it on mouse enter as browser window size might change after the element has been
     * loaded initially
     */
    const { openRepo } = useOpenSearchResultsContext()

    const renderTitle = (): JSX.Element => (
        <div className={styles.title}>
            <span className="test-search-result-label ml-1 flex-shrink-past-contents text-truncate">
                <Button className="btn-text-link" onClick={() => openRepo(result)}>
                    {displayRepoName(getRepoMatchLabel(result))}
                </Button>
            </span>
        </div>
    )

    const renderBody = (): JSX.Element => (
        <div data-testid="search-repo-result">
            <div className={classNames(styles.searchResultMatch, 'p-2 flex-column')}>
                <div className="d-flex align-items-center flex-row">
                    <div className={styles.matchType}>
                        <small>Repository match</small>
                    </div>
                    {result.fork && (
                        <>
                            <div className={styles.divider} />
                            <div>
                                <Icon
                                    className={classNames('flex-shrink-0 text-muted', styles.icon)}
                                    aria-label="Forked repository"
                                    svgPath={mdiSourceFork}
                                />
                            </div>
                            <div>
                                <small>Fork</small>
                            </div>
                        </>
                    )}
                    {result.archived && (
                        <>
                            <div className={styles.divider} />
                            <div>
                                <Icon
                                    className={classNames('flex-shrink-0 text-muted', styles.icon)}
                                    aria-label="Archived repository"
                                    svgPath={mdiArchive}
                                />
                            </div>
                            <div>
                                <small>Archived</small>
                            </div>
                        </>
                    )}
                    {result.private && (
                        <>
                            <div className={styles.divider} />
                            <div>
                                <Icon
                                    className={classNames('flex-shrink-0 text-muted', styles.icon)}
                                    aria-label="Private repository"
                                    svgPath={mdiLock}
                                />
                            </div>
                            <div>
                                <small>Private</small>
                            </div>
                        </>
                    )}
                </div>
                {result.description && (
                    <>
                        <div className={styles.dividerVertical} />
                        <div>
                            <small>
                                <em>{result.description}</em>
                            </small>
                        </div>
                    </>
                )}
            </div>
        </div>
    )

    return (
        <LegacyResultContainer
            as={as}
            index={index}
            icon={SourceRepositoryIcon}
            collapsible={false}
            defaultExpanded={true}
            title={renderTitle()}
            resultType={result.type}
            onResultClicked={onSelect}
            expandedChildren={renderBody()}
            className={containerClassName}
            repoName={result.repository}
            repoStars={result.repoStars}
        />
    )
}
