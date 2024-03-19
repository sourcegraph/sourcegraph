import isAbsoluteURL from 'is-absolute-url'
import { memoize, noop } from 'lodash'
import { type Subscriber, type Subscription, fromEvent, of, lastValueFrom } from 'rxjs'
import { map } from 'rxjs/operators'

import {
    LATEST_VERSION,
    type MessageHandlers,
    type SearchMatch,
    messageHandlers,
    search,
    switchAggregateSearchResults,
    observeMessages,
    type SearchEvent,
} from '@sourcegraph/shared/src/search/stream'
import type { TourTaskStepType } from '@sourcegraph/shared/src/settings/temporary'
import type { UserOnboardingConfig } from '@sourcegraph/shared/src/settings/temporary/TemporarySettings'

import { SearchPatternType } from '../../../graphql-operations'

export function isNotNullOrUndefined<T>(value: T): value is NonNullable<T> {
    return value !== null && value !== undefined
}

/**
 * Returns a new URL w/ tour state tracking query parameters. This is used to show/hide tour task info box.
 *
 * @param href original URL
 * @param stepId TourTaskStepType id
 * @example /search?q=context:global+repo:my-repo&patternType=literal + &tour=true&stepId=DiffSearch
 */
export const buildURIMarkers = (href: string, stepId: string): string => {
    const isRelative = !isAbsoluteURL(href)

    try {
        const url = new URL(href, isRelative ? `${location.protocol}//${location.host}` : undefined)
        url.searchParams.set('tour', 'true')
        url.searchParams.set('stepId', stepId)
        return isRelative ? url.toString().slice(url.origin.length) : url.toString()
    } catch {
        return '#'
    }
}

/**
 * Returns tour URL state tracking query parameters from a URL search parameter that was build using "buildURIMarkers" function.
 */
export const parseURIMarkers = (searchParameters: string): { isTour: boolean; stepId: string | null } => {
    const parameters = new URLSearchParams(searchParameters)
    const isTour = parameters.has('tour')
    const stepId = parameters.get('stepId')
    return { isTour, stepId }
}

const noopHandler = <T extends SearchEvent>(
    type: T['type'],
    eventSource: EventSource,
    _observer: Subscriber<SearchEvent>
): Subscription => fromEvent(eventSource, type).subscribe(noop)

const firstMatchMessageHandlers: MessageHandlers = {
    ...messageHandlers,
    matches: (type, eventSource, observer) =>
        observeMessages(type, eventSource).subscribe(data => {
            observer.next(data)
            // Once we observer the first `matches` event, complete the stream and close the event source.
            observer.complete()
            eventSource.close()
        }),
    progress: noopHandler,
    filters: noopHandler,
    alert: noopHandler,
}

/**
 * Initiates a streaming search, stop at the first `matches` event, and aggregate the results.
 */
const fetchStreamSuggestions = memoize(
    (query: string, sourcegraphURL?: string): Promise<SearchMatch[]> =>
        lastValueFrom(
            search(
                of(query),
                {
                    version: LATEST_VERSION,
                    patternType: SearchPatternType.standard,
                    caseSensitive: false,
                    trace: undefined,
                    sourcegraphURL,
                },
                firstMatchMessageHandlers
            ).pipe(
                switchAggregateSearchResults,
                map(suggestions => suggestions.results)
            )
        ),
    (query, sourcegraphURL) => `${query}|${sourcegraphURL}`
)

/**
 * Executes the a "restricted" version of the provided query to determine whether
 * the query returns results or not.
 */
export function isQuerySuccessful(query: string): Promise<boolean> {
    let dynamicQuery = `${query} timeout:3s count:1`

    if (!query.includes('type:')) {
        dynamicQuery += ' select:content'
    }

    return fetchStreamSuggestions(dynamicQuery)
        .then(results => results.length > 0)
        .catch(() => false)
}

export enum QueryPlaceholder {
    Snippet = '$$snippet',
    Repo = '$$userrepo',
    Lang = '$$userlang',
    Email = '$$useremail',
}

export function containsPlaceholder(queryTemplate: string, placeholder: QueryPlaceholder): boolean {
    return queryTemplate.includes(placeholder)
}

/**
 * Returns true if the provided step can be executed by the user. This is true for all steps except
 * for search query steps which can depend on user specific information (preferred repo, language, ...).
 * If this information is used in a query but not provided by the user then we don't want to show this step.
 */
export function canRunStep(step: TourTaskStepType, userInfo: UserOnboardingConfig['userinfo']): boolean {
    switch (step.action.type) {
        case 'search-query': {
            const action = step.action
            const placeholders: [QueryPlaceholder, string | undefined][] = [
                [QueryPlaceholder.Repo, userInfo?.repo],
                [QueryPlaceholder.Lang, userInfo?.language],
                [QueryPlaceholder.Email, userInfo?.email],
            ]
            return placeholders.every(
                ([placeholder, value]) => !containsPlaceholder(action.query, placeholder) || !!value
            )
        }
        default: {
            return true
        }
    }
}
