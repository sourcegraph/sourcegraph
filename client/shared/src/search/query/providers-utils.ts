// IMPORTANT: This module contains code used by the CodeMirror query input
// implementation and therefore shouldn't have any runtime dependencies on
// Monaco

import { Observable, lastValueFrom, of } from 'rxjs'
import { delay, takeUntil, switchMap } from 'rxjs/operators'

import type { SearchMatch } from '../stream'

import { FilterType } from './filters'
import { type Filter, KeywordKind, type Token } from './token'
import { isFilterType } from './validate'

const MAX_SUGGESTION_COUNT = 50
const REPO_SUGGESTION_FILTERS = [FilterType.fork, FilterType.visibility, FilterType.archived]
const FILE_SUGGESTION_FILTERS = [...REPO_SUGGESTION_FILTERS, FilterType.repo, FilterType.rev, FilterType.lang]

function serializeFilters(tokens: Token[], filterTypes: FilterType[]): string {
    return tokens
        .filter((token): token is Filter => filterTypes.some(filterType => isFilterType(token, filterType)))
        .map(filter => (filter.value ? `${filter.field.value}:${filter.value.value}` : ''))
        .filter(serialized => !!serialized)
        .join(' ')
}

/**
 * getSuggestionsQuery might return an empty query. The caller is responsible
 * for handling this accordingly.
 */
export function getSuggestionQuery(tokens: Token[], tokenAtColumn: Token, suggestionType: SearchMatch['type']): string {
    const hasAndOrOperators = tokens.some(
        token => token.type === 'keyword' && (token.kind === KeywordKind.Or || token.kind === KeywordKind.And)
    )

    let tokenValue = ''

    switch (tokenAtColumn.type) {
        case 'filter': {
            tokenValue = tokenAtColumn.value?.value ?? ''
            break
        }
        case 'pattern': {
            tokenValue = tokenAtColumn.value
            break
        }
    }

    if (!tokenValue) {
        return ''
    }

    if (suggestionType === 'repo') {
        const relevantFilters = !hasAndOrOperators ? serializeFilters(tokens, REPO_SUGGESTION_FILTERS) : ''
        return `${relevantFilters} repo:${tokenValue} type:repo count:${MAX_SUGGESTION_COUNT}`.trimStart()
    }

    // For the cases below, we are not handling queries with and/or operators. This is because we would need to figure out
    // for each filter which filters from the surrounding expression apply to it. For example, if we have a query: `repo:x file:y z OR repo:xx file:yy`
    // and we want to get suggestions for the `file:yy` filter. We would only want to include file suggestions from the `xx` repo and not the `x` repo, because it
    // is a part of a different expression.
    if (hasAndOrOperators) {
        return ''
    }

    if (suggestionType === 'path') {
        const relevantFilters = serializeFilters(tokens, FILE_SUGGESTION_FILTERS)
        return `${relevantFilters} file:${tokenValue} type:path count:${MAX_SUGGESTION_COUNT}`.trimStart()
    }
    if (suggestionType === 'symbol') {
        const relevantFilters = serializeFilters(tokens, [...FILE_SUGGESTION_FILTERS, FilterType.file])
        return `${relevantFilters} ${tokenValue} type:symbol count:${MAX_SUGGESTION_COUNT}`.trimStart()
    }

    return ''
}

export function createCancelableFetchSuggestions(
    fetchSuggestions: (query: string) => Observable<SearchMatch[]>
): (query: string, onAbort: (hander: () => void) => void) => Promise<SearchMatch[]> {
    return (query, onAbort) => {
        if (!query) {
            // Don't fetch suggestions if the query is empty. This would result
            // in arbitrary result types being returned, which is unexpected.
            return Promise.resolve([])
        }

        let aborted = false

        // By listeing to the abort event of the autocompletion we
        // can close the connection to server early and don't have to download
        // data sent by the server.
        const abort = new Observable(subscriber => {
            onAbort(() => {
                aborted = true
                subscriber.next(null)
                subscriber.complete()
            })
        })

        return lastValueFrom(
            of(query).pipe(
                // We use a delay here to implement a custom debounce. In the
                // next step we check if the current completion request was
                // cancelled in the meantime.
                // This prevents us from needlessly running multiple suggestion
                // queries.
                delay(150),
                switchMap(query => (aborted ? Promise.resolve([]) : fetchSuggestions(query))),
                takeUntil(abort)
            ),
            { defaultValue: [] }
        )
    }
}
