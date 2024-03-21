import { of, type Subscription } from 'rxjs'
import { map, switchMap, throttleTime } from 'rxjs/operators'
import type * as vscode from 'vscode'

import { SearchMode } from '@sourcegraph/shared/src/search'
import { appendContextFilter } from '@sourcegraph/shared/src/search/query/transformer'
import { aggregateStreamingSearch } from '@sourcegraph/shared/src/search/stream'

import type { ExtensionCoreAPI } from '../contract'
import { SearchPatternType } from '../graphql-operations'
import type { VSCEStateMachine } from '../state'
import { focusSearchPanel } from '../webview/commands'

import { isOlderThan, observeInstanceVersionNumber } from './instanceVersion'

export function createStreamSearch({
    context,
    stateMachine,
    sourcegraphURL,
    session,
}: {
    context: vscode.ExtensionContext
    stateMachine: VSCEStateMachine
    sourcegraphURL: string
    session: vscode.AuthenticationSession | undefined
}): ExtensionCoreAPI['streamSearch'] {
    // Ensure only one search is active at a time
    let previousSearchSubscription: Subscription | null

    context.subscriptions.push({
        dispose: () => {
            previousSearchSubscription?.unsubscribe()
        },
    })
    const token = session?.accessToken === undefined ? '' : session?.accessToken
    const instanceVersionNumber = observeInstanceVersionNumber(token, sourcegraphURL)

    return function streamSearch(query, options) {
        previousSearchSubscription?.unsubscribe()

        stateMachine.emit({
            type: 'submit_search_query',
            submittedSearchQueryState: {
                queryState: { query },
                searchCaseSensitivity: options.caseSensitive,
                searchPatternType: options.patternType,
                searchMode: options.searchMode || SearchMode.Precise,
            },
        })
        // Focus search panel if not already focused
        // (in case e.g. user initiates search from search sidebar when panel is hidden).
        focusSearchPanel()

        previousSearchSubscription = instanceVersionNumber
            .pipe(
                map(version => {
                    let patternType = options.patternType

                    if (
                        patternType === SearchPatternType.standard &&
                        version &&
                        isOlderThan(version, { major: 3, minor: 43 })
                    ) {
                        /**
                         * SearchPatternType.standard support was added in Sourcegraph v3.43.0.
                         * Use SearchPatternType.literal for earlier versions instead (it was the default before v3.43.0).
                         * See: https://docs.sourcegraph.com/CHANGELOG#3-43-0, https://github.com/sourcegraph/sourcegraph/pull/38141.
                         */
                        patternType = SearchPatternType.literal
                    }

                    return patternType
                }),
                switchMap(patternType =>
                    aggregateStreamingSearch(
                        of(appendContextFilter(query, stateMachine.state.context.selectedSearchContextSpec)),
                        { ...options, patternType, sourcegraphURL }
                    ).pipe(throttleTime(500, undefined, { leading: true, trailing: true }))
                )
            )
            .subscribe(searchResults => {
                if (searchResults.state === 'error') {
                    // Pass only primitive copied values because Error object is not cloneable
                    const { name, message, stack } = searchResults.error
                    searchResults.error = { name, message, stack }
                }

                stateMachine.emit({ type: 'received_search_results', searchResults })
            })
    }
}
