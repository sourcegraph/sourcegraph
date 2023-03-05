import React, { MouseEvent, KeyboardEvent, useCallback } from 'react'

import classNames from 'classnames'
import * as H from 'history'
import { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { LastSyncedIcon, FileMatchChildrenStyles as styles, CodeExcerpt } from '@sourcegraph/branded'
import {
    appendLineRangeQueryParameter,
    appendSubtreeQueryParameter,
    toPositionOrRangeQueryParameter,
} from '@sourcegraph/common'
import { FetchFileParameters } from '@sourcegraph/shared/src/backend/file'
import { MatchGroup } from '@sourcegraph/shared/src/components/ranking/PerFileResultRanking'
import { ContentMatch, SymbolMatch, PathMatch, getFileMatchUrl } from '@sourcegraph/shared/src/search/stream'
import { isSettingsValid, SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { SymbolKind } from '@sourcegraph/shared/src/symbols/SymbolKind'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, Code } from '@sourcegraph/wildcard'

import { HighlightLineRange } from '../../../graphql-operations'
import { useOpenSearchResultsContext } from '../MatchHandlersContext'

interface FileMatchProps extends SettingsCascadeProps, TelemetryProps {
    location?: H.Location
    result: ContentMatch | SymbolMatch | PathMatch
    grouped: MatchGroup[]
    /* Clicking on a match opens the link in a new tab */
    openInNewTab?: boolean
    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
}

/**
 * This helper function determines whether a mouse/click event was triggered as
 * a result of selecting text in search results.
 * There are at least to ways to do this:
 *
 * - Tracking `mouseup`, `mousemove` and `mousedown` events. The occurrence of
 * a `mousemove` event would indicate a text selection. However, users
 * might slightly move the mouse while clicking, and solutions that would
 * take this into account seem fragile.
 * - (implemented here) Inspect the Selection object returned by
 * `window.getSelection()`.
 *
 * CAVEAT: Chromium and Firefox (and maybe other browsers) behave
 * differently when a search result is clicked *after* text selection was
 * made:
 *
 * - Firefox will clear the selection before executing the click event
 * handler, i.e. the search result will be opened.
 * - Chrome will only clear the selection if the click happens *outside*
 * of the selected text (in which case the search result will be
 * opened). If the click happens inside the selected text the selection
 * will be cleared only *after* executing the click event handler.
 */
function isTextSelectionEvent(event: MouseEvent<HTMLElement>): boolean {
    const selection = window.getSelection()

    // Text selections are always ranges. Should the type not be set, verify
    // that the selection is not empty.
    if (selection && (selection.type === 'Range' || selection.toString() !== '')) {
        // Firefox specific: Because our code excerpts are implemented as tables,
        // CTRL+click would select the table cell. Since users don't know that we
        // use tables, the most likely wanted to open the search results in a new
        // tab instead though.
        if ((event.ctrlKey || event.metaKey) && selection.anchorNode?.nodeName === 'TR') {
            // Ugly side effect: We don't want the table cell to be highlighted.
            // The focus style that Firefox uses doesn't seem to be affected by
            // CSS so instead we clear the selection.
            selection.empty()
            return false
        }

        return true
    }

    return false
}

/**
 * A helper function to replicate browser behavior when clicking on links.
 * A very common interaction is to open links in a new in the _background_ via
 * CTRL/CMD + click or middle click.
 * Unfortunately `window.open` doesn't give us much control over how the new
 * window/tab should be opened, and the behavior is inconcistent between
 * browsers.
 * In order to replicate the standard behvior as much as possible this function
 * dynamically creates an `<a>` element and triggers a click event on it.
 */
function openLinkInNewTab(
    url: string,
    event: Pick<MouseEvent, 'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey'>,
    button: 'primary' | 'middle'
): void {
    const link = document.createElement('a')
    link.href = url
    link.style.display = 'none'
    link.target = '_blank'
    link.rel = 'noopener noreferrer'
    const clickEvent = new window.MouseEvent('click', {
        bubbles: false,
        altKey: event.altKey,
        shiftKey: event.shiftKey,
        // Regarding middle click: Setting "button: 1:" doesn't seem to suffice:
        // Firefox doesn't react to the event at all, Chromium opens the tab in
        // the foreground. So in order to simulate a middle click, we set
        // ctrlKey and metaKey to `true` instead.
        ctrlKey: button === 'middle' ? true : event.ctrlKey,
        metaKey: button === 'middle' ? true : event.metaKey,
        view: window,
    })

    // It looks the link has to be part of the document, otherwise Firefox won't
    // trigger the default behavior (it works without appending in Chromium).
    document.body.append(link)
    link.dispatchEvent(clickEvent)
    link.remove()
}

/**
 * Since we are not using a real link anymore, we have to simulate opening
 * the file in a new tab when the search result is clicked on with the
 * middle mouse button.
 * This handler is bound to the `mouseup` event because the `auxclick`
 * (https://w3c.github.io/uievents/#event-type-auxclick) event is not
 * support by all browsers yet (https://caniuse.com/?search=auxclick)
 */
function navigateToFileOnMiddleMouseButtonClick(event: MouseEvent<HTMLElement>): void {
    const href = event.currentTarget.getAttribute('data-href')
    if (href && event.button === 1) {
        openLinkInNewTab(href, event, 'middle')
    }
}

export const FileMatchChildren: React.FunctionComponent<React.PropsWithChildren<FileMatchProps>> = props => {
    const { result, grouped, fetchHighlightedFileLineRanges, telemetryService } = props

    const { openFile, openSymbol } = useOpenSearchResultsContext()

    const fetchHighlightedFileRangeLines = React.useCallback(
        (startLine: number, endLine: number) => {
            const startTime = Date.now()
            return fetchHighlightedFileLineRanges(
                {
                    repoName: result.repository,
                    commitID: result.commit || '',
                    filePath: result.path,
                    disableTimeout: false,
                    ranges: grouped.map(
                        (group): HighlightLineRange => ({
                            startLine: group.startLine,
                            endLine: group.endLine,
                        })
                    ),
                },
                false
            ).pipe(
                map(lines => {
                    telemetryService.log(
                        'search.latencies.frontend.code-load',
                        { durationMs: Date.now() - startTime },
                        { durationMs: Date.now() - startTime }
                    )
                    return lines[grouped.findIndex(group => group.startLine === startLine && group.endLine === endLine)]
                })
            )
        },
        [result, fetchHighlightedFileLineRanges, grouped, telemetryService]
    )

    const createCodeExcerptLink = (group: MatchGroup): string => {
        const positionOrRangeQueryParameter = toPositionOrRangeQueryParameter({ position: group.position })
        return appendLineRangeQueryParameter(
            appendSubtreeQueryParameter(getFileMatchUrl(result)),
            positionOrRangeQueryParameter
        )
    }

    /**
     * This handler implements the logic to simulate the click/keyboard
     * activation behavior of links, while also allowing the selection of text
     * inside the element.
     * Because a click event is dispatched in both cases (clicking the search
     * result to open it as well as selecting text within it), we have to be
     * able to distinguish between those two actions.
     * If we detect a text selection action, we don't have to do anything.
     *
     * CAVEATS:
     * - In Firefox, Shift+click will open the URL in a new tab instead of
     * a window (unlike Chromium which seems to show the same behavior as with
     * native links).
     * - Firefox will insert \t\n in between table rows, causing the copied
     * text to be different from what is in the file/search result.
     */
    const navigateToFile = useCallback(
        (
            event: KeyboardEvent<HTMLElement> | MouseEvent<HTMLElement>,
            { line, character }: { line: number; character: number }
        ): void => {
            // Testing for text selection is only necessary for mouse/click
            // events. Middle-click (event.button === 1) is already handled in the `onMouseUp` callback.
            if (
                (event.type === 'click' &&
                    !isTextSelectionEvent(event as MouseEvent<HTMLElement>) &&
                    (event as MouseEvent<HTMLElement>).button !== 1) ||
                (event as KeyboardEvent<HTMLElement>).key === 'Enter'
            ) {
                const href = event.currentTarget.getAttribute('data-href')
                if (!event.defaultPrevented && href) {
                    event.preventDefault()

                    openFile(result.repository, {
                        path: result.path,
                        revision: result.commit,
                        position: {
                            line: line - 1,
                            character: character - 1,
                        },
                    })
                }
            }
        },
        [openFile, result]
    )

    return (
        <div className={styles.fileMatchChildren} data-testid="file-match-children">
            {result.repoLastFetched && <LastSyncedIcon lastSyncedTime={result.repoLastFetched} />}
            {/* Path */}
            {result.type === 'path' && (
                <div className={styles.item} data-testid="file-match-children-item">
                    <small>Path match</small>
                </div>
            )}

            {/* Symbols */}
            {((result.type === 'symbol' && result.symbols) || []).map(symbol => (
                <Button
                    className={classNames('test-file-match-children-item', styles.item, 'btn-text-link')}
                    key={`symbol:${symbol.name}${String(symbol.containerName)}${symbol.url}`}
                    data-testid="file-match-children-item"
                    onClick={() => openSymbol(symbol.url)}
                >
                    <SymbolKind
                        kind={symbol.kind}
                        className="mr-1"
                        symbolKindTags={
                            isSettingsValid(props.settingsCascade) &&
                            props.settingsCascade.final.experimentalFeatures?.symbolKindTags
                        }
                    />
                    <Code>
                        {symbol.name}{' '}
                        {symbol.containerName && <span className="text-muted">{symbol.containerName}</span>}
                    </Code>
                </Button>
            ))}

            {/* Line matches */}
            {grouped && (
                <div>
                    {grouped.map(group => (
                        <div
                            key={`linematch:${getFileMatchUrl(result)}${group.position.line}:${
                                group.position.character
                            }`}
                            className={classNames('test-file-match-children-item-wrapper', styles.itemCodeWrapper)}
                        >
                            <div
                                data-href={createCodeExcerptLink(group)}
                                className={classNames(
                                    'test-file-match-children-item',
                                    styles.item,
                                    styles.itemClickable
                                )}
                                onClick={event =>
                                    navigateToFile(event, {
                                        line: group.position.line,
                                        character: group.position.character,
                                    })
                                }
                                onMouseUp={navigateToFileOnMiddleMouseButtonClick}
                                onKeyDown={event =>
                                    navigateToFile(event, {
                                        line: group.position.line,
                                        character: group.position.character,
                                    })
                                }
                                data-testid="file-match-children-item"
                                tabIndex={0}
                                role="link"
                            >
                                <CodeExcerpt
                                    repoName={result.repository}
                                    commitID={result.commit || ''}
                                    filePath={result.path}
                                    startLine={group.startLine}
                                    endLine={group.endLine}
                                    highlightRanges={group.matches}
                                    fetchHighlightedFileRangeLines={fetchHighlightedFileRangeLines}
                                    blobLines={group.blobLines}
                                />
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
