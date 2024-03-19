import React, { useCallback, type KeyboardEvent, type MouseEvent, useState, useEffect } from 'react'

import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'
import VisibilitySensor from 'react-visibility-sensor'
import type { Observable, Subscription } from 'rxjs'
import { catchError } from 'rxjs/operators'

import { asError, isErrorLike } from '@sourcegraph/common'
import type { FetchFileParameters } from '@sourcegraph/shared/src/backend/file'
import { HighlightResponseFormat } from '@sourcegraph/shared/src/graphql-operations'
import { getFileMatchUrl, getRepositoryUrl, getRevision, type SymbolMatch } from '@sourcegraph/shared/src/search/stream'
import { isSettingsValid, type SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { SymbolKind } from '@sourcegraph/shared/src/symbols/SymbolKind'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { codeCopiedEvent } from '@sourcegraph/shared/src/tracking/event-log-creators'

import { CodeExcerpt } from './CodeExcerpt'
import { navigateToCodeExcerpt, navigateToFileOnMiddleMouseButtonClick } from './codeLinkNavigation'
import { CopyPathAction } from './CopyPathAction'
import { RepoFileLink } from './RepoFileLink'
import { ResultContainer } from './ResultContainer'

import resultStyles from './ResultContainer.module.scss'
import styles from './SymbolSearchResult.module.scss'

const DEFAULT_VISIBILITY_OFFSET = { bottom: -500 }

export interface SymbolSearchResultProps extends TelemetryProps, SettingsCascadeProps {
    result: SymbolMatch
    openInNewTab?: boolean
    repoDisplayName: string
    containerClassName?: string
    index: number
    onSelect: () => void
    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
}

export const SymbolSearchResult: React.FunctionComponent<SymbolSearchResultProps> = ({
    result,
    openInNewTab,
    repoDisplayName,
    onSelect,
    containerClassName,
    index,
    telemetryService,
    settingsCascade,
    fetchHighlightedFileLineRanges,
}) => {
    const repoAtRevisionURL = getRepositoryUrl(result.repository, result.branches)
    const revisionDisplayName = getRevision(result.branches, result.commit)

    const title = (
        <span className="d-flex align-items-center">
            <RepoFileLink
                repoName={result.repository}
                repoURL={repoAtRevisionURL}
                filePath={result.path}
                fileURL={getFileMatchUrl(result)}
                repoDisplayName={
                    repoDisplayName
                        ? `${repoDisplayName}${revisionDisplayName ? `@${revisionDisplayName}` : ''}`
                        : undefined
                }
                className={resultStyles.titleInner}
            />
            <CopyPathAction
                filePath={result.path}
                className={resultStyles.copyButton}
                telemetryService={telemetryService}
                // TODO (dadlerj): update to use a real telemetry recorder
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </span>
    )

    const navigate = useNavigate()
    const navigateToFile = useCallback(
        (event: KeyboardEvent<HTMLElement> | MouseEvent<HTMLElement>): void =>
            navigateToCodeExcerpt(event, openInNewTab ?? false, navigate),
        [openInNewTab, navigate]
    )

    const logEventOnCopy = useCallback(() => {
        telemetryService.log(...codeCopiedEvent('search-result'))
    }, [telemetryService])

    const [hasBeenVisible, setHasBeenVisible] = useState(false)
    const [highlightedLines, setHighlightedLines] = useState<string[] | undefined>(undefined)
    useEffect(() => {
        let subscription: Subscription | undefined
        if (hasBeenVisible) {
            const startTime = Date.now()
            subscription = fetchHighlightedFileLineRanges(
                {
                    repoName: result.repository,
                    commitID: result.commit || '',
                    filePath: result.path,
                    disableTimeout: false,
                    format: HighlightResponseFormat.HTML_HIGHLIGHT,
                    ranges: result.symbols.map(symbol => ({
                        startLine: symbol.line - 1,
                        endLine: symbol.line,
                    })),
                },
                false
            )
                .pipe(catchError(error => [asError(error)]))
                .subscribe(res => {
                    const endTime = Date.now()
                    telemetryService.log(
                        'search.latencies.frontend.code-load',
                        { durationMs: endTime - startTime },
                        { durationMs: endTime - startTime }
                    )
                    if (!isErrorLike(res)) {
                        setHighlightedLines(res.map(arr => arr[0]))
                    }
                })
        }
        return () => subscription?.unsubscribe()
    }, [result, hasBeenVisible, fetchHighlightedFileLineRanges, telemetryService])

    return (
        <ResultContainer
            index={index}
            title={title}
            resultType={result.type}
            onResultClicked={onSelect}
            repoName={result.repository}
            repoStars={result.repoStars}
            className={classNames(resultStyles.copyButtonContainer, containerClassName)}
            repoLastFetched={result.repoLastFetched}
        >
            <VisibilitySensor
                onChange={(visible: boolean) => setHasBeenVisible(visible || hasBeenVisible)}
                partialVisibility={true}
                offset={DEFAULT_VISIBILITY_OFFSET}
            >
                <div className={styles.symbols}>
                    {result.symbols.map((symbol, i) => (
                        <div
                            key={`symbol:${symbol.name}${String(symbol.containerName)}${symbol.url}`}
                            className={classNames(
                                styles.symbol,
                                resultStyles.clickable,
                                resultStyles.focusableBlock,
                                resultStyles.horizontalDividerBetween
                            )}
                            data-href={symbol.url}
                            role="link"
                            data-testid="symbol-search-result"
                            tabIndex={0}
                            onClick={navigateToFile}
                            onMouseUp={navigateToFileOnMiddleMouseButtonClick}
                            onKeyDown={navigateToFile}
                            data-selectable-search-result="true"
                        >
                            <div className="mr-2 flex-shrink-0">
                                <SymbolKind
                                    kind={symbol.kind}
                                    symbolKindTags={
                                        isSettingsValid(settingsCascade) &&
                                        settingsCascade.final.experimentalFeatures?.symbolKindTags
                                    }
                                />
                            </div>
                            <div className={styles.symbolCodeExcerpt}>
                                <CodeExcerpt
                                    className="a11y-ignore"
                                    plaintextLines={['']}
                                    highlightedLines={highlightedLines && [highlightedLines[i]]}
                                    repoName={result.repository}
                                    commitID={result.commit || ''}
                                    filePath={result.path}
                                    startLine={symbol.line - 1}
                                    endLine={symbol.line}
                                    onCopy={logEventOnCopy}
                                    highlightRanges={[]}
                                />
                            </div>
                        </div>
                    ))}
                </div>
            </VisibilitySensor>
        </ResultContainer>
    )
}
