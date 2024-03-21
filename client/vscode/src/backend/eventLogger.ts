import { EMPTY, Subject } from 'rxjs'
import { bufferTime, catchError, concatMap } from 'rxjs/operators'

import { gql } from '@sourcegraph/http-client'

import type { LogEventsResult, LogEventsVariables, Event } from '../graphql-operations'

import { requestGraphQLFromVSCode } from './requestGraphQl'

// Log events in batches.
const events = new Subject<Event>()

events
    .pipe(
        bufferTime(1000),
        concatMap(events => {
            if (events.length > 0) {
                return requestGraphQLFromVSCode<LogEventsResult, LogEventsVariables>(logEventsMutation, {
                    events,
                })
            }
            return EMPTY
        }),
        catchError(error => {
            console.error('Error logging events:', error)
            return []
        })
    )
    // eslint-disable-next-line rxjs/no-ignored-subscription
    .subscribe()

export const logEventsMutation = gql`
    mutation LogEvents($events: [Event!]) {
        logEvents(events: $events) {
            alwaysNil
        }
    }
`

/**
 * Log a raw user action (used to allow site admins on a Sourcegraph instance
 * to see a count of unique users on a daily, weekly, and monthly basis).
 *
 * When invoked on a non-Sourcegraph.com instance, this data is stored in the
 * instance's database, and not sent to Sourcegraph.com.
 */

export function logEvent(eventVariable: Event): void {
    events.next(eventVariable)
}
