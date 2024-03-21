import React from 'react'

import VisuallyHidden from '@reach/visually-hidden'

import { LegacyResultContainer, CommitSearchResultMatch } from '@sourcegraph/branded'
import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { type CommitMatch, getCommitMatchUrl } from '@sourcegraph/shared/src/search/stream'
import { Button, Code } from '@sourcegraph/wildcard'

import { useOpenSearchResultsContext } from '../MatchHandlersContext'

import styles from './SearchResultsStyles.module.scss'

interface Props extends PlatformContextProps<'requestGraphQL'> {
    result: CommitMatch
    repoName: string
    icon: React.ComponentType<{ className?: string }>
    onSelect: () => void
    openInNewTab?: boolean
    containerClassName?: string
    as?: React.ElementType
    index: number
}

export const CommitSearchResult: React.FunctionComponent<Props> = ({
    result,
    icon,
    platformContext,
    onSelect,
    openInNewTab,
    containerClassName,
    as,
    index,
}) => {
    /**
     * Use the custom hook useIsTruncated to check if overflow: ellipsis is activated for the element
     * We want to do it on mouse enter as browser window size might change after the element has been
     * loaded initially
     */
    const { openRepo, openCommit, instanceURL } = useOpenSearchResultsContext()

    const renderTitle = (): JSX.Element => (
        <div className={styles.title}>
            <span className="test-search-result-label ml-1 flex-shrink-past-contents text-truncate">
                <>
                    <Button
                        className="btn-text-link"
                        onClick={() =>
                            openRepo({
                                repository: result.repository,
                                branches: [result.oid],
                            })
                        }
                    >
                        {displayRepoName(result.repository)}
                    </Button>
                    {' › '}
                    <Button className="btn-text-link" onClick={() => openCommit(getCommitMatchUrl(result))}>
                        {result.authorName}
                    </Button>
                    <span aria-hidden={true}>{': '}</span>
                    <Button className="btn-text-link" onClick={() => openCommit(getCommitMatchUrl(result))}>
                        {result.message.split('\n', 1)[0]}
                    </Button>
                </>
            </span>
            <span className={styles.spacer} />
            {result.type === 'commit' && (
                <Button className="btn-text-link" onClick={() => openCommit(getCommitMatchUrl(result))}>
                    <Code className={styles.commitOid}>
                        <VisuallyHidden>Commit hash:</VisuallyHidden>
                        {result.oid.slice(0, 7)}
                        <VisuallyHidden>,</VisuallyHidden>
                    </Code>{' '}
                    <VisuallyHidden>Committed</VisuallyHidden>
                    <Timestamp date={result.authorDate} noAbout={true} strict={true} />
                </Button>
            )}
            {result.repoStars && <div className={styles.divider} />}
        </div>
    )

    const renderBody = (): JSX.Element => (
        <CommitSearchResultMatch
            key={result.url}
            item={{
                ...result,
                // Make it an absolute URL to open in browser.
                url: new URL(result.url, instanceURL).href,
            }}
            platformContext={platformContext}
            openInNewTab={openInNewTab}
        />
    )

    return (
        <LegacyResultContainer
            as={as}
            index={index}
            icon={icon}
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
