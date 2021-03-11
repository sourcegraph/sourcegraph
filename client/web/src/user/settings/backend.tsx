import { Observable } from 'rxjs'
import { map } from 'rxjs/operators'
import { gql, dataOrThrowErrors } from '../../../../shared/src/graphql/graphql'
import { createAggregateError } from '../../../../shared/src/util/errors'
import { requestGraphQL } from '../../backend/graphql'
import { eventLogger } from '../../tracking/eventLogger'
import { UserEvent, EventSource, Scalars } from '../../../../shared/src/graphql-operations'
import {
    LogEventResult,
    LogEventVariables,
    LogUserEventResult,
    LogUserEventVariables,
    SetUserEmailVerifiedResult,
    SetUserEmailVerifiedVariables,
    UpdatePasswordResult,
    UpdatePasswordVariables,
    CreatePasswordResult,
    CreatePasswordVariables,
} from '../../graphql-operations'

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
            if (!data || !data.updatePassword) {
                eventLogger.log('UpdatePasswordFailed')
                throw createAggregateError(errors)
            }
            eventLogger.log('PasswordUpdated')
        })
    )
}

export function createPassword(args: CreatePasswordVariables): Observable<void> {
    return requestGraphQL<CreatePasswordResult, CreatePasswordVariables>(
        gql`
            mutation CreatePassword($newPassword: String!) {
                createPassword(newPassword: $newPassword) {
                    alwaysNil
                }
            }
        `,
        args
    ).pipe(
        map(({ data, errors }) => {
            if (!data || !data.createPassword) {
                eventLogger.log('CreatePasswordFailed')
                throw createAggregateError(errors)
            }
            eventLogger.log('PasswordCreated')
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

/**
 * Log a user action (used to allow site admins on a Sourcegraph instance
 * to see a count of unique users on a daily, weekly, and monthly basis).
 *
 * Not used at all for public/sourcegraph.com usage.
 *
 * @deprecated Use logEvent
 */
export function logUserEvent(event: UserEvent): void {
    requestGraphQL<LogUserEventResult, LogUserEventVariables>(
        gql`
            mutation LogUserEvent($event: UserEvent!, $userCookieID: String!) {
                logUserEvent(event: $event, userCookieID: $userCookieID) {
                    alwaysNil
                }
            }
        `,
        { event, userCookieID: eventLogger.getAnonymousUserID() }
    )
        .pipe(
            map(({ data, errors }) => {
                if (!data || (errors && errors.length > 0)) {
                    throw createAggregateError(errors)
                }
                return
            })
        )
        // Event logs are best-effort and non-blocking
        // eslint-disable-next-line rxjs/no-ignored-subscription
        .subscribe()
}

/**
 * Log a raw user action (used to allow site admins on a Sourcegraph instance
 * to see a count of unique users on a daily, weekly, and monthly basis).
 *
 * When invoked on a non-Sourcegraph.com instance, this data is stored in the
 * instance's database, and not sent to Sourcegraph.com.
 */
export function logEvent(event: string, eventProperties?: unknown): void {
    requestGraphQL<LogEventResult, LogEventVariables>(
        gql`
            mutation LogEvent(
                $event: String!
                $userCookieID: String!
                $url: String!
                $source: EventSource!
                $argument: String
            ) {
                logEvent(event: $event, userCookieID: $userCookieID, url: $url, source: $source, argument: $argument) {
                    alwaysNil
                }
            }
        `,
        {
            event,
            userCookieID: eventLogger.getAnonymousUserID(),
            url: window.location.href,
            source: EventSource.WEB,
            argument: eventProperties ? JSON.stringify(eventProperties) : null,
        }
    )
        .pipe(map(dataOrThrowErrors))
        // Event logs are best-effort and non-blocking
        // eslint-disable-next-line rxjs/no-ignored-subscription
        .subscribe()
}
