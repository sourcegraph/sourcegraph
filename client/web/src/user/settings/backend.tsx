import { EMPTY, type Observable, Subject, lastValueFrom } from 'rxjs'
import { bufferTime, catchError, concatMap, map } from 'rxjs/operators'

import { createAggregateError } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import { EventSource, type Scalars } from '@sourcegraph/shared/src/graphql-operations'

import { requestGraphQL } from '../../backend/graphql'
import type {
    SetUserEmailVerifiedResult,
    SetUserEmailVerifiedVariables,
    UpdatePasswordResult,
    UpdatePasswordVariables,
    LogEventsResult,
    LogEventsVariables,
    Event,
} from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'

export const UPDATE_PASSWORD = gql`
    mutation UpdatePassword($oldPassword: String!, $newPassword: String!) {
        updatePassword(oldPassword: $oldPassword, newPassword: $newPassword) {
            alwaysNil
        }
    }
`

export const CREATE_PASSWORD = gql`
    mutation CreatePassword($newPassword: String!) {
        createPassword(newPassword: $newPassword) {
            alwaysNil
        }
    }
`

export const userExternalAccountFragment = gql`
    fragment UserExternalAccountFields on ExternalAccount {
        id
        serviceID
        serviceType
        clientID
        publicAccountData {
            displayName
            login
            url
        }
    }
`

export const USER_EXTERNAL_ACCOUNTS = gql`
    query UserExternalAccountsWithAccountData($username: String!) {
        user(username: $username) {
            externalAccounts {
                nodes {
                    id
                    serviceID
                    serviceType
                    clientID
                    publicAccountData {
                        displayName
                        login
                        url
                    }
                }
            }
        }
    }
`

export function updatePassword(args: UpdatePasswordVariables): Observable<void> {
    return requestGraphQL<UpdatePasswordResult, UpdatePasswordVariables>(
        gql`
            mutation UpdatePassword($oldPassword: String!, $newPassword: String!) {
                updatePassword(oldPassword: $oldPassword, newPassword: $newPassword) {
                    alwaysNil
                }
            }
        `,
        args
    ).pipe(
        map(({ data, errors }) => {
            if (!data?.updatePassword) {
                eventLogger.log('UpdatePasswordFailed')
                throw createAggregateError(errors)
            }
            eventLogger.log('PasswordUpdated')
        })
    )
}

/**
 * Set the verification state for a user email address.
 *
 * @param user the user's GraphQL ID
 * @param email the email address to edit
 * @param verified the new verification state for the user email
 */
export function setUserEmailVerified(user: Scalars['ID'], email: string, verified: boolean): Observable<void> {
    return requestGraphQL<SetUserEmailVerifiedResult, SetUserEmailVerifiedVariables>(
        gql`
            mutation SetUserEmailVerified($user: ID!, $email: String!, $verified: Boolean!) {
                setUserEmailVerified(user: $user, email: $email, verified: $verified) {
                    alwaysNil
                }
            }
        `,
        { user, email, verified }
    ).pipe(
        map(({ data, errors }) => {
            if (!data || (errors && errors.length > 0)) {
                throw createAggregateError(errors)
            }
        })
    )
}

// Log events in batches.
const batchedEvents = new Subject<Event>()

export const logEventsMutation = gql`
    mutation LogEvents($events: [Event!]) {
        logEvents(events: $events) {
            alwaysNil
        }
    }
`

function sendEvents(events: Event[]): Promise<void> {
    return lastValueFrom(
        requestGraphQL<LogEventsResult, LogEventsVariables>(logEventsMutation, {
            events,
        })
    )
        .then(dataOrThrowErrors)
        .then(() => {})
}

function sendEvent(event: Event): Promise<void> {
    return sendEvents([event])
}

batchedEvents
    .pipe(
        bufferTime(1000),
        concatMap(events => {
            if (events.length > 0) {
                return sendEvents(events)
            }
            return EMPTY
        }),
        // TODO: log errors to Sentry
        catchError(() => [])
    )
    // eslint-disable-next-line rxjs/no-ignored-subscription
    .subscribe()

/**
 * Log a raw user action (used to allow site admins on a Sourcegraph instance
 * to see a count of unique users on a daily, weekly, and monthly basis).
 *
 * When invoked on a non-Sourcegraph.com instance, this data is stored in the
 * instance's database, and not sent to Sourcegraph.com.
 *
 * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
 * src/telemetry instead.
 */
export function logEvent(event: string, eventProperties?: unknown, publicArgument?: unknown): void {
    batchedEvents.next(createEvent(event, eventProperties, publicArgument))
}

/**
 * Log a raw user action and return a promise that resolves once the event has
 * been sent. This method will not attempt to batch requests, so it should be
 * used only when low event latency is necessary (e.g., on an external link).
 *
 * See logEvent for additional details.
 *
 * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
 * src/telemetry instead.
 */
export function logEventSynchronously(
    event: string,
    eventProperties?: unknown,
    publicArgument?: unknown
): Promise<void> {
    return sendEvent(createEvent(event, eventProperties, publicArgument))
}

function createEvent(event: string, eventProperties?: unknown, publicArgument?: unknown): Event {
    return {
        event,
        userCookieID: eventLogger.user.anonymousUserID,
        cohortID: eventLogger.user.cohortID || null,
        firstSourceURL: eventLogger.session.getFirstSourceURL(),
        lastSourceURL: eventLogger.session.getLastSourceURL(),
        referrer: eventLogger.session.getReferrer(),
        originalReferrer: eventLogger.session.getOriginalReferrer(),
        sessionReferrer: eventLogger.session.getSessionReferrer(),
        sessionFirstURL: eventLogger.session.getSessionFirstURL(),
        deviceSessionID: eventLogger.user.deviceSessionID,
        url: window.location.href,
        source: EventSource.WEB,
        argument: eventProperties ? JSON.stringify(eventProperties) : null,
        publicArgument: publicArgument ? JSON.stringify(publicArgument) : null,
        deviceID: eventLogger.user.deviceID,
        eventID: eventLogger.getEventID(),
        insertID: eventLogger.getInsertID(),
        client: eventLogger.getClient(),
        connectedSiteID: window.context?.siteID,
        hashedLicenseKey: window.context?.hashedLicenseKey,
    }
}
