import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { mdiChevronDown, mdiChevronUp } from '@mdi/js'
import classNames from 'classnames'
import * as H from 'history'
import { Observable } from 'rxjs'

import { isErrorLike, pluralize } from '@sourcegraph/common'
import { AggregableBadge } from '@sourcegraph/extension-api-types'
import { FetchFileParameters } from '@sourcegraph/shared/src/backend/file'
import type { AggregableBadge } from '@sourcegraph/shared/src/codeintel/legacy-extensions/api'
import { LineRanking } from '@sourcegraph/shared/src/components/ranking/LineRanking'
import { MatchItem } from '@sourcegraph/shared/src/components/ranking/PerFileResultRanking'
import { ZoektRanking } from '@sourcegraph/shared/src/components/ranking/ZoektRanking'
import { ContentMatch, getFileMatchUrl, getRepositoryUrl, getRevision } from '@sourcegraph/shared/src/search/stream'
import { isSettingsValid, SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Icon, Badge } from '@sourcegraph/wildcard'

import { CopyPathAction } from './CopyPathAction'
import { FileMatchChildren } from './FileMatchChildren'
import { RepoFileLink } from './RepoFileLink'
import { ResultContainer } from './ResultContainer'

import resultContainerStyles from './ResultContainer.module.scss'
import styles from './SearchResult.module.scss'

interface Props extends SettingsCascadeProps, TelemetryProps {
    location: H.Location
    /**
     * The file match search result.
     */
    result: ContentMatch

    /**
     * Formatted repository name to be displayed in repository link. If not
     * provided, the default format will be displayed.
     */
    repoDisplayName?: string

    /**
     * Called when the file's search result is selected.
     */
    onSelect: () => void

    /**
     * Whether this file should be rendered as expanded by default.
     */
    defaultExpanded: boolean

    /**
     * Whether or not to show all matches for this file, or only a subset.
     */
    showAllMatches: boolean

    allExpanded?: boolean

    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>

    /**
     * CSS class name to be applied to the ResultContainer Component
     */
    containerClassName?: string

    /**
     * Clicking on a match opens the link in a new tab.
     */
    openInNewTab?: boolean

    index: number
}

const sumHighlightRanges = (count: number, item: MatchItem): number => count + item.highlightRanges.length

const BY_LINE_RANKING = 'by-line-number'
const DEFAULT_CONTEXT = 1

export const FileContentSearchResult: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    containerClassName,
    result,
    settingsCascade,
    location,
    index,
    repoDisplayName,
    defaultExpanded,
    allExpanded,
    showAllMatches,
    openInNewTab,
    telemetryService,
    fetchHighlightedFileLineRanges,
    onSelect,
}) => {
    const repoAtRevisionURL = getRepositoryUrl(result.repository, result.branches)
    const revisionDisplayName = getRevision(result.branches, result.commit)

    const ranking = useMemo(() => {
        const settings = settingsCascade.final
        if (!isErrorLike(settings) && settings?.experimentalFeatures?.clientSearchResultRanking === BY_LINE_RANKING) {
            return new LineRanking(5)
        }
        return new ZoektRanking(3)
    }, [settingsCascade])

    // The number of lines of context to show before and after each match.
    const context = useMemo(() => {
        if (location?.pathname === '/search') {
            // Check if search.contextLines is configured in settings.
            const contextLinesSetting =
                isSettingsValid(settingsCascade) &&
                settingsCascade.final &&
                settingsCascade.final['search.contextLines']

            if (typeof contextLinesSetting === 'number' && contextLinesSetting >= 0) {
                return contextLinesSetting
            }
        }
        return DEFAULT_CONTEXT
    }, [location, settingsCascade])

    const items: MatchItem[] = useMemo(
        () =>
            result.type === 'content'
                ? result.chunkMatches?.map(match => ({
                      highlightRanges: match.ranges.map(range => ({
                          startLine: range.start.line,
                          startCharacter: range.start.column,
                          endLine: range.end.line,
                          endCharacter: range.end.column,
                      })),
                      content: match.content,
                      startLine: match.contentStart.line,
                      endLine: match.ranges[match.ranges.length - 1].end.line,
                      aggregableBadges: match.aggregableBadges,
                  })) ||
                  result.lineMatches?.map(match => ({
                      highlightRanges: match.offsetAndLengths.map(offsetAndLength => ({
                          startLine: match.lineNumber,
                          startCharacter: offsetAndLength[0],
                          endLine: match.lineNumber,
                          endCharacter: offsetAndLength[0] + offsetAndLength[1],
                      })),
                      content: match.line,
                      startLine: match.lineNumber,
                      endLine: match.lineNumber,
                      aggregableBadges: match.aggregableBadges,
                  })) ||
                  []
                : [],
        [result]
    )

    const expandedMatchGroups = useMemo(() => ranking.expandedResults(items, context), [items, context, ranking])
    const collapsedMatchGroups = useMemo(() => ranking.collapsedResults(items, context), [items, context, ranking])
    const collapsedMatchCount = collapsedMatchGroups.matches.length

    const highlightRangesCount = useMemo(() => items.reduce(sumHighlightRanges, 0), [items])
    const collapsedHighlightRangesCount = useMemo(
        () => collapsedMatchGroups.matches.reduce(sumHighlightRanges, 0),
        [collapsedMatchGroups]
    )

    const hiddenMatchesCount = highlightRangesCount - collapsedHighlightRangesCount
    const collapsible = !showAllMatches && items.length > collapsedMatchCount

    const [expanded, setExpanded] = useState(allExpanded || defaultExpanded)
    useEffect(() => setExpanded(allExpanded || defaultExpanded), [allExpanded, defaultExpanded])

    const rootRef = useRef<HTMLDivElement>(null)
    const toggle = useCallback((): void => {
        if (collapsible) {
            setExpanded(expanded => !expanded)
        }

        // Scroll back to top of result when collapsing
        if (expanded) {
            setTimeout(() => {
                const reducedMotion = !window.matchMedia('(prefers-reduced-motion: no-preference)').matches
                rootRef.current?.scrollIntoView({ block: 'nearest', behavior: reducedMotion ? 'auto' : 'smooth' })
            }, 0)
        }
    }, [collapsible, expanded])

    const description =
        items.length > 0 ? (
            <>
                {aggregateBadges(items).map(badge => (
                    <Badge
                        key={badge.text}
                        href={badge.linkURL}
                        tooltip={badge.hoverMessage}
                        variant="secondary"
                        small={true}
                        className="text-muted text-uppercase file-match__badge"
                    >
                        {badge.text}
                    </Badge>
                ))}
            </>
        ) : undefined

    const title = (
        <>
            <span className="d-flex align-items-center">
                <RepoFileLink
                    repoName={result.repository}
                    repoURL={repoAtRevisionURL}
                    filePath={result.path}
                    pathMatchRanges={result.pathMatches ?? []}
                    fileURL={getFileMatchUrl(result)}
                    repoDisplayName={
                        repoDisplayName
                            ? `${repoDisplayName}${revisionDisplayName ? `@${revisionDisplayName}` : ''}`
                            : undefined
                    }
                    className={classNames(styles.titleInner, styles.mutedRepoFileLink)}
                />
                <CopyPathAction
                    className={styles.copyButton}
                    filePath={result.path}
                    telemetryService={telemetryService}
                />
            </span>
            {description && <span className={classNames('ml-2', styles.headerDescription)}>{description}</span>}
        </>
    )

    useEffect(() => {
        const ref = rootRef.current
        if (!ref) {
            return
        }

        const expand = (): void => setExpanded(true)
        const collapse = (): void => setExpanded(false)
        const toggle = (): void => setExpanded(expanded => !expanded)

        // Custom events triggered by search results keyboard navigation (from the useSearchResultsKeyboardNavigation hook).
        ref.addEventListener('expandSearchResultsGroup', expand)
        ref.addEventListener('collapseSearchResultsGroup', collapse)
        ref.addEventListener('toggleSearchResultsGroup', toggle)

        return () => {
            ref.removeEventListener('expandSearchResultsGroup', expand)
            ref.removeEventListener('collapseSearchResultsGroup', collapse)
            ref.removeEventListener('toggleSearchResultsGroup', toggle)
        }
    }, [rootRef, setExpanded])

    return (
        <ResultContainer
            index={index}
            title={title}
            resultType={result.type}
            onResultClicked={onSelect}
            repoName={result.repository}
            repoStars={result.repoStars}
            className={classNames(styles.copyButtonContainer, containerClassName)}
            resultClassName={resultContainerStyles.highlightResult}
            rankingDebug={result.debug}
            ref={rootRef}
        >
            <div data-testid="file-search-result" data-expanded={expanded}>
                <FileMatchChildren
                    result={result}
                    grouped={expanded ? expandedMatchGroups.grouped : collapsedMatchGroups.grouped}
                    fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
                    settingsCascade={settingsCascade}
                    telemetryService={telemetryService}
                    openInNewTab={openInNewTab}
                />
                {collapsible && (
                    <button
                        type="button"
                        className={classNames(
                            styles.toggleMatchesButton,
                            expanded && styles.toggleMatchesButtonExpanded
                        )}
                        onClick={toggle}
                        data-testid="toggle-matches-container"
                    >
                        <Icon aria-hidden={true} svgPath={expanded ? mdiChevronUp : mdiChevronDown} />
                        <span className={styles.toggleMatchesButtonText}>
                            {expanded
                                ? 'Show less'
                                : `Show ${hiddenMatchesCount} more ${pluralize(
                                      'match',
                                      hiddenMatchesCount,
                                      'matches'
                                  )}`}
                        </span>
                    </button>
                )}
            </div>
        </ResultContainer>
    )
}

function aggregateBadges(items: MatchItem[]): AggregableBadge[] {
    const aggregatedBadges = new Map<string, AggregableBadge>()
    for (const badge of items.flatMap(item => item.aggregableBadges || [])) {
        aggregatedBadges.set(badge.text, badge)
    }

    return [...aggregatedBadges.values()].sort((a, b) => a.text.localeCompare(b.text))
}
