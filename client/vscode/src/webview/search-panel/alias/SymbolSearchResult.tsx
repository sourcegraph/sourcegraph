import React, { useCallback } from 'react'

import classNames from 'classnames'
import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import {
    SymbolSearchResultStyles as styles,
    navigateToFileOnMiddleMouseButtonClick,
    OldResultContainer,
    CopyPathAction,
} from '@sourcegraph/branded'
import type { FetchFileParameters } from '@sourcegraph/shared/src/backend/file'
import { getFileMatchUrl, getRepositoryUrl, getRevision, type SymbolMatch } from '@sourcegraph/shared/src/search/stream'
import { isSettingsValid, type SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { SymbolKind } from '@sourcegraph/shared/src/symbols/SymbolKind'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { codeCopiedEvent } from '@sourcegraph/shared/src/tracking/event-log-creators'

import { type HighlightLineRange, HighlightResponseFormat } from '../../../graphql-operations'
import { CodeExcerpt } from '../components/CodeExcerpt'
import { useOpenSearchResultsContext } from '../MatchHandlersContext'

import { RepoFileLink } from './RepoFileLink'

import searchResultStyles from './SearchResultsStyles.module.scss'

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
    repoDisplayName,
    onSelect,
    containerClassName,
    index,
    telemetryService,
    settingsCascade,
    fetchHighlightedFileLineRanges,
}) => {
    const enableLazyFileResultSyntaxHighlighting =
        isSettingsValid(settingsCascade) &&
        settingsCascade.final.experimentalFeatures?.enableLazyFileResultSyntaxHighlighting

    const repoAtRevisionURL = getRepositoryUrl(result.repository, result.branches)
    const revisionDisplayName = getRevision(result.branches, result.commit)

    const { openSymbol } = useOpenSearchResultsContext()

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
                className={classNames(searchResultStyles.titleInner)}
            />
            <CopyPathAction
                filePath={result.path}
                className={searchResultStyles.copyButton}
                telemetryRecorder={noOpTelemetryRecorder}
                telemetryService={telemetryService}
            />
        </span>
    )

    const logEventOnCopy = useCallback(() => {
        telemetryService.log(...codeCopiedEvent('file-match'))
    }, [telemetryService])

    const fetchSymbolMatchLineRanges = useCallback(
        (startLine: number, endLine: number, format: HighlightResponseFormat) => {
            const startTime = Date.now()
            return fetchHighlightedFileLineRanges({
                repoName: result.repository,
                commitID: result.commit || '',
                filePath: result.path,
                disableTimeout: false,
                format,
                ranges: result.symbols.map(
                    (symbol): HighlightLineRange => ({
                        startLine: symbol.line - 1,
                        endLine: symbol.line,
                    })
                ),
            }).pipe(
                map(lines => {
                    const endTime = Date.now()
                    telemetryService.log(
                        'search.latencies.frontend.code-load',
                        { durationMs: endTime - startTime },
                        { durationMs: endTime - startTime }
                    )
                    return lines[
                        result.symbols.findIndex(symbol => symbol.line - 1 === startLine && symbol.line === endLine)
                    ]
                })
            )
        },
        [result, fetchHighlightedFileLineRanges, telemetryService]
    )

    const fetchHighlightedSymbolMatchLineRanges = useCallback(
        (startLine: number, endLine: number) =>
            fetchSymbolMatchLineRanges(startLine, endLine, HighlightResponseFormat.HTML_HIGHLIGHT),
        [fetchSymbolMatchLineRanges]
    )

    const fetchPlainTextSymbolMatchLineRanges = useCallback(
        (startLine: number, endLine: number) =>
            fetchSymbolMatchLineRanges(startLine, endLine, HighlightResponseFormat.HTML_PLAINTEXT),
        [fetchSymbolMatchLineRanges]
    )

    return (
        <OldResultContainer
            index={index}
            title={title}
            resultType={result.type}
            onResultClicked={onSelect}
            repoName={result.repository}
            repoStars={result.repoStars}
            className={classNames(searchResultStyles.copyButtonContainer, containerClassName)}
            repoLastFetched={result.repoLastFetched}
        >
            <div className={styles.symbols}>
                {result.symbols.map(symbol => (
                    <div
                        key={`symbol:${symbol.name}${String(symbol.containerName)}${symbol.url}`}
                        className={styles.symbol}
                        data-href={symbol.url}
                        role="link"
                        data-testid="symbol-search-result"
                        tabIndex={0}
                        onClick={() => openSymbol(symbol.url)}
                        onMouseUp={navigateToFileOnMiddleMouseButtonClick}
                        onKeyDown={() => openSymbol(symbol.url)}
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
                                repoName={result.repository}
                                commitID={result.commit || ''}
                                filePath={result.path}
                                startLine={symbol.line - 1}
                                endLine={symbol.line}
                                fetchHighlightedFileRangeLines={fetchHighlightedSymbolMatchLineRanges}
                                fetchPlainTextFileRangeLines={
                                    enableLazyFileResultSyntaxHighlighting
                                        ? fetchPlainTextSymbolMatchLineRanges
                                        : undefined
                                }
                                onCopy={logEventOnCopy}
                                highlightRanges={[]}
                            />
                        </div>
                    </div>
                ))}
            </div>
        </OldResultContainer>
    )
}
