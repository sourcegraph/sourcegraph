import React, { type FunctionComponent, useEffect, useState, useCallback } from 'react'

import classNames from 'classnames'
import { lastValueFrom } from 'rxjs'

import { asError, type ErrorLike, isErrorLike } from '@sourcegraph/common'
import { gql, dataOrThrowErrors, useQuery } from '@sourcegraph/http-client'
import { Container, PageHeader, LoadingSpinner, Alert, ErrorAlert } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../backend/graphql'
import { PageTitle } from '../../../components/PageTitle'
import type {
    Scalars,
    UserEmail as UserEmailType,
    UserEmailsResult,
    UserEmailsVariables,
    UserSettingsAreaUserFields,
    UserSettingsEmailsSiteFlagsResult,
    UserSettingsEmailsSiteFlagsVariables,
} from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'
import { ScimAlert } from '../ScimAlert'

import { AddUserEmailForm } from './AddUserEmailForm'
import { SetUserPrimaryEmailForm } from './SetUserPrimaryEmailForm'
import { UserEmail } from './UserEmail'

import styles from './UserSettingsEmailsPage.module.scss'

interface Props {
    user: UserSettingsAreaUserFields
}

type Status = undefined | 'loading' | 'loaded' | ErrorLike
type EmailActionError = undefined | ErrorLike

// NOTE: The name of the query is also added in the refreshSiteFlags() function
// found in client/web/src/site/backend.tsx
const FLAGS_QUERY = gql`
    query UserSettingsEmailsSiteFlags {
        site {
            id
            sendsEmailVerificationEmails
        }
    }
`

export const UserSettingsEmailsPage: FunctionComponent<React.PropsWithChildren<Props>> = ({ user }) => {
    const [emails, setEmails] = useState<UserEmailType[]>([])
    const [statusOrError, setStatusOrError] = useState<Status>()
    const [emailActionError, setEmailActionError] = useState<EmailActionError>()

    const { data } = useQuery<UserSettingsEmailsSiteFlagsResult, UserSettingsEmailsSiteFlagsVariables>(FLAGS_QUERY, {})
    const flags = data?.site

    const onEmailRemove = useCallback(
        (deletedEmail: string): void => {
            setEmails(emails => emails.filter(({ email }) => email !== deletedEmail))
            // always cleanup email action errors when removing emails
            setEmailActionError(undefined)
        },
        [setEmailActionError]
    )

    const fetchEmails = useCallback(async (): Promise<void> => {
        setStatusOrError('loading')

        const fetchedEmails = await fetchUserEmails(user.id)

        // always cleanup email action errors when re-fetching emails
        setEmailActionError(undefined)

        if (fetchedEmails?.node?.__typename === 'User' && fetchedEmails.node.emails) {
            setEmails(fetchedEmails.node.emails)
            setStatusOrError('loaded')
        } else {
            setStatusOrError(asError("Sorry, we couldn't fetch user emails. Try again?"))
        }
    }, [user, setStatusOrError, setEmails])

    useEffect(() => {
        eventLogger.logViewEvent('UserSettingsEmails')
    }, [])

    useEffect(() => {
        fetchEmails().catch(error => {
            setStatusOrError(asError(error))
        })
    }, [fetchEmails])

    if (statusOrError === 'loading') {
        return <LoadingSpinner />
    }

    return (
        <div className={styles.userSettingsEmailsPage} data-testid="user-settings-emails-page">
            {user.scimControlled && <ScimAlert />}
            <PageTitle title="Emails" />
            <PageHeader headingElement="h2" path={[{ text: 'Emails' }]} className="mb-3" />

            {flags && !flags.sendsEmailVerificationEmails && (
                <Alert variant="warning">
                    Sourcegraph is not configured to send email verifications. Newly added email addresses must be
                    manually verified by a site admin.
                </Alert>
            )}

            {isErrorLike(statusOrError) && <ErrorAlert className="mt-2" error={statusOrError} />}
            {isErrorLike(emailActionError) && <ErrorAlert className="mt-2" error={emailActionError} />}

            <Container>
                <ul className="list-group">
                    {emails.map(email => (
                        <li key={email.email} className={classNames('list-group-item', styles.listItem)}>
                            <UserEmail
                                user={user.id}
                                email={email}
                                onEmailVerify={fetchEmails}
                                onEmailResendVerification={fetchEmails}
                                onDidRemove={onEmailRemove}
                                onError={setEmailActionError}
                                disableControls={user.scimControlled}
                            />
                        </li>
                    ))}
                    {emails.length === 0 && (
                        <li className={classNames('list-group-item text-muted', styles.listItem)}>No emails</li>
                    )}
                </ul>
            </Container>
            {/* re-fetch emails on onDidAdd to guarantee correct state */}
            <AddUserEmailForm
                className={styles.emailForm}
                user={user}
                onDidAdd={fetchEmails}
                emails={new Set(emails.map(userEmail => userEmail.email))}
            />
            <hr className="my-4" aria-hidden="true" />
            <SetUserPrimaryEmailForm user={user} emails={emails} onDidSet={fetchEmails} />
        </div>
    )
}

async function fetchUserEmails(userID: Scalars['ID']): Promise<UserEmailsResult> {
    return dataOrThrowErrors(
        await lastValueFrom(
            requestGraphQL<UserEmailsResult, UserEmailsVariables>(
                gql`
                    fragment UserEmail on UserEmail {
                        email
                        isPrimary
                        verified
                        verificationPending
                        viewerCanManuallyVerify
                    }
                    query UserEmails($user: ID!) {
                        node(id: $user) {
                            ... on User {
                                __typename
                                emails {
                                    ...UserEmail
                                }
                            }
                        }
                    }
                `,
                { user: userID }
            )
        )
    )
}
