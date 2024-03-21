import React from 'react'

import classNames from 'classnames'

import type { SearchMatch } from '@sourcegraph/shared/src/search/stream'
import type { ForwardReferenceExoticComponent } from '@sourcegraph/wildcard'

import { formatRepositoryStarCount } from '../../util/stars'
import { CodeHostIcon } from '../CodeHostIcon'
import { LastSyncedIcon } from '../LastSyncedIcon'
import { SearchResultStar } from '../SearchResultStar'

import styles from './OldResultContainer.module.scss'

export interface ResultContainerProps {
    index: number
    title: React.ReactNode
    titleClassName?: string
    resultClassName?: string
    repoStars?: number
    repoLastFetched?: string
    resultType?: SearchMatch['type']
    repoName?: string
    className?: string
    rankingDebug?: string
    onResultClicked?: () => void
}

const accessibleResultType: Record<SearchMatch['type'], string> = {
    symbol: 'symbol',
    content: 'file content',
    repo: 'repository',
    path: 'file path',
    commit: 'commit',
    person: 'person',
    team: 'team',
}

/**
 * The container component for a result in the SearchResults component.
 */
export const OldResultContainer: ForwardReferenceExoticComponent<
    React.ElementType,
    React.PropsWithChildren<ResultContainerProps>
> = React.forwardRef(function ResultContainer(props, reference) {
    const {
        children,
        title,
        titleClassName,
        resultClassName,
        index,
        repoStars,
        resultType,
        repoName,
        className,
        rankingDebug,
        as: Component = 'div',
        onResultClicked,
        repoLastFetched,
    } = props

    const formattedRepositoryStarCount = formatRepositoryStarCount(repoStars)

    const trackReferencePanelClick = (): void => onResultClicked?.()

    return (
        <Component
            className={classNames('test-search-result', styles.resultContainer, className)}
            data-testid="result-container"
            data-result-type={resultType}
            onClick={trackReferencePanelClick}
            ref={reference}
        >
            <article aria-labelledby={`result-container-${index}`}>
                <div className={styles.header} id={`result-container-${index}`}>
                    {/* Add a result type to be read out to screen readers only, so that screen reader users can
                    easily scan the search results list (for example, by navigating by landmarks). */}
                    <span className="sr-only">{resultType ? accessibleResultType[resultType] : 'search'} result,</span>
                    {repoName && <CodeHostIcon repoName={repoName} className="flex-shrink-0 mr-1" />}
                    <div
                        className={classNames(styles.headerTitle, titleClassName)}
                        data-testid="result-container-header"
                    >
                        {title}
                    </div>
                    {formattedRepositoryStarCount && (
                        <span className="d-flex align-items-center">
                            <SearchResultStar aria-label={`${repoStars} stars`} />
                            <span aria-hidden={true}>{formattedRepositoryStarCount}</span>
                        </span>
                    )}
                    {repoLastFetched && <LastSyncedIcon lastSyncedTime={repoLastFetched} className="ml-2" />}
                </div>
                {rankingDebug && <div>{rankingDebug}</div>}
                {children && <div className={classNames(styles.result, resultClassName)}>{children}</div>}
            </article>
        </Component>
    )
})
