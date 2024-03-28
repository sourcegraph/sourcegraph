/* eslint rxjs/no-ignored-subscription: warn */
import { ReplaySubject, Subject, forkJoin } from 'rxjs'
import { debounceTime, distinctUntilChanged, map, repeat, switchMap, share } from 'rxjs/operators'

import type { SearchMatch } from '@sourcegraph/shared/src/search/stream'
import { fetchStreamSuggestions } from '@sourcegraph/shared/src/search/suggestions'

interface BaseSuggestion {
    title: string
    description?: string

    /**
     * The URL that is navigated to when the user selects this
     * suggestion.
     */
    url: string

    /**
     * A label describing the action taken when navigating to
     * the URL (e.g., "go to repository").
     */
    urlLabel: string
}

interface SymbolSuggestion extends BaseSuggestion {
    type: 'symbol'
    kind: string
}

interface RepoSuggestion extends BaseSuggestion {
    type: 'repo'
}

interface FileSuggestion extends BaseSuggestion {
    type: 'file'
}

export type Suggestion = SymbolSuggestion | RepoSuggestion | FileSuggestion

/**
 * Returns all but the last element of path, or "." if that would be the empty path.
 */
function dirname(path: string): string | undefined {
    return path.split('/').slice(0, -1).join('/') || '.'
}

/**
 * Returns the last element of path, or "." if path is empty.
 */
function basename(path: string): string {
    return path.split('/').at(-1) || '.'
}

function createSuggestions(item: SearchMatch): Suggestion[] {
    switch (item.type) {
        case 'repo': {
            return [
                {
                    type: 'repo',
                    title: item.repository,
                    url: `/${item.repository}`,
                    urlLabel: 'go to repository',
                },
            ]
        }
        case 'path': {
            const descriptionParts: string[] = []
            const directory = dirname(item.path)
            if (directory !== undefined && directory !== '.') {
                descriptionParts.push(`${directory}/`)
            }
            descriptionParts.push(basename(item.repository))

            return [
                {
                    type: 'file',
                    title: item.path,
                    description: descriptionParts.join(' — '),
                    url: `/${item.repository}/-/blob/${item.path}`,
                    urlLabel: 'go to file',
                },
            ]
        }
        case 'symbol': {
            return item.symbols.map(symbol => ({
                type: 'symbol',
                kind: symbol.kind,
                title: symbol.name,
                description: `${item.path} — ${basename(item.repository)}`,
                url: symbol.url,
                urlLabel: 'go to definition',
            }))
        }
        default: {
            return []
        }
    }
}

export interface SuggestionInput {
    sourcegraphURL: string
    queries: string[]
    handler: (suggestion: Suggestion[]) => void
}

export const createSuggestionFetcher = (): ((input: SuggestionInput) => void) => {
    const fetcher = new Subject<SuggestionInput>()

    fetcher
        .pipe(
            distinctUntilChanged(),
            debounceTime(200),
            switchMap(({ sourcegraphURL, queries, handler }) =>
                forkJoin(queries.map(query => fetchStreamSuggestions(query, sourcegraphURL))).pipe(
                    map(suggestions => ({
                        suggestions: suggestions.flat().flatMap(suggestion => createSuggestions(suggestion)),
                        handler,
                    })),
                    share({
                        connector: () => new ReplaySubject(),
                        resetOnError: false,
                        resetOnComplete: false,
                        resetOnRefCountZero: false,
                    })
                )
            ),
            // But resubscribe afterwards
            repeat()
        )
        .subscribe(({ suggestions, handler }) => handler(suggestions))

    return input => fetcher.next(input)
}
