import React, { FunctionComponent, useEffect, useState, useCallback } from 'react'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'

import { requestGraphQL } from '../../../backend/graphql'
import { UserAreaUserFields, UserEmailsResult, UserEmailsVariables } from '../../../graphql-operations'
import { gql, dataOrThrowErrors } from '../../../../../shared/src/graphql/graphql'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { siteFlags } from '../../../site/backend'
import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { eventLogger } from '../../../tracking/eventLogger'

import { ErrorAlert } from '../../../components/alerts'
import { PageTitle } from '../../../components/PageTitle'
import { UserEmail } from './UserEmail'
import { AddUserEmailForm } from './AddUserEmailForm'
import { SetUserPrimaryEmailForm } from './SetUserPrimaryEmailForm'

interface Props {
    user: UserAreaUserFields
}

type UserEmail = NonNullable<UserEmailsResult['node']>['emails'][number]
type Status = undefined | 'loading' | 'loaded' | ErrorLike
type EmailActionError = undefined | ErrorLike

export const UserSettingsEmailsPage: FunctionComponent<Props> = ({ user }) => {
    const [emails, setEmails] = useState<UserEmail[]>([])
    const [statusOrError, setStatusOrError] = useState<Status>()
    const [emailActionError, setEmailActionError] = useState<EmailActionError>()

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

        const fetchedEmails = dataOrThrowErrors(
            await requestGraphQL<UserEmailsResult, UserEmailsVariables>(
                gql`
                    query UserEmails($user: ID!) {
                        node(id: $user) {
                            ... on User {
                                emails {
                                    email
                                    isPrimary
                                    verified
                                    verificationPending
                                    viewerCanManuallyVerify
                                }
                            }
                        }
                    }
                `,
                { user: user.id }
            ).toPromise()
        )

        // always cleanup email action errors when re-fetching emails
        setEmailActionError(undefined)

        if (fetchedEmails?.node?.emails) {
            setEmails(fetchedEmails.node.emails)
            setStatusOrError('loaded')
        } else {
            setStatusOrError(asError("Sorry, we couldn't fetch user emails. Try again?"))
        }
    }, [user, setStatusOrError, setEmails])

    const flags = useObservable(siteFlags)

    useEffect(() => {
        eventLogger.logViewEvent('UserSettingsEmails')
    }, [])

    useEffect(() => {
        fetchEmails().catch(error => {
            setStatusOrError(asError(error))
        })
    }, [fetchEmails])

    return (
        <div className="user-settings-emails-page">
            <PageTitle title="Emails" />

            {flags && !flags.sendsEmailVerificationEmails && (
                <div className="alert alert-warning mt-2">
                    Sourcegraph is not configured to send email verifications. Newly added email addresses must be
                    manually verified by a site admin.
                </div>
            )}

            {isErrorLike(statusOrError) && <ErrorAlert className="mt-2" error={statusOrError} />}
            {isErrorLike(emailActionError) && <ErrorAlert className="mt-2" error={emailActionError} />}

            <h2>Emails</h2>

            {statusOrError === 'loading' ? (
                <div className="d-flex justify-content-center">
                    <LoadingSpinner className="icon-inline" />
                </div>
            ) : (
                <div className="mt-4">
                    <ul className="list-group">
                        {emails.map(email => (
                            <li key={email.email} className="list-group-item p-3">
                                <UserEmail
                                    user={user.id}
                                    email={email}
                                    onEmailVerify={fetchEmails}
                                    onEmailResendVerification={fetchEmails}
                                    onDidRemove={onEmailRemove}
                                    onError={setEmailActionError}
                                />
                            </li>
                        ))}
                    </ul>
                </div>
            )}

            {/* re-fetch emails on onDidAdd to guarantee correct state */}
            <AddUserEmailForm className="mt-4" user={user.id} onDidAdd={fetchEmails} />
            <hr className="my-4" />
            {statusOrError === 'loaded' && (
                <SetUserPrimaryEmailForm user={user.id} emails={emails} onDidSet={fetchEmails} />
            )}
        </div>
    )
}
