import { type FunctionComponent, useState, useCallback } from 'react'

import { lastValueFrom } from 'rxjs'

import { asError, type ErrorLike } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { Badge, Button, screenReaderAnnounce } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../backend/graphql'
import type {
    RemoveUserEmailResult,
    RemoveUserEmailVariables,
    ResendVerificationEmailResult,
    ResendVerificationEmailVariables,
    SetUserEmailVerifiedResult,
    SetUserEmailVerifiedVariables,
    UserEmailsResult,
} from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'

import styles from './UserEmail.module.scss'

interface Props {
    user: string
    email: (NonNullable<UserEmailsResult['node']> & { __typename: 'User' })['emails'][number]
    disableControls: boolean
    onError: (error: ErrorLike) => void
    onDidRemove?: (email: string) => void
    onEmailVerify?: () => void
    onEmailResendVerification?: () => void
}

export const resendVerificationEmail = async (
    userID: string,
    email: string,
    options?: { onSuccess: () => void; onError: (error: ErrorLike) => void }
): Promise<void> => {
    try {
        dataOrThrowErrors(
            await lastValueFrom(
                requestGraphQL<ResendVerificationEmailResult, ResendVerificationEmailVariables>(
                    gql`
                        mutation ResendVerificationEmail($userID: ID!, $email: String!) {
                            resendVerificationEmail(user: $userID, email: $email) {
                                alwaysNil
                            }
                        }
                    `,
                    { userID, email }
                )
            )
        )

        eventLogger.log('UserEmailAddressVerificationResent')

        options?.onSuccess?.()
    } catch (error) {
        options?.onError?.(error)
    }
}

export const UserEmail: FunctionComponent<React.PropsWithChildren<Props>> = ({
    user,
    email: { email, isPrimary, verified, verificationPending, viewerCanManuallyVerify },
    disableControls,
    onError,
    onDidRemove,
    onEmailVerify,
    onEmailResendVerification,
}) => {
    const [isLoading, setIsLoading] = useState(false)

    const handleError = useCallback(
        (error: ErrorLike): void => {
            onError(asError(error))
            setIsLoading(false)
        },
        [onError, setIsLoading]
    )

    const removeEmail = async (): Promise<void> => {
        setIsLoading(true)

        try {
            dataOrThrowErrors(
                await lastValueFrom(
                    requestGraphQL<RemoveUserEmailResult, RemoveUserEmailVariables>(
                        gql`
                            mutation RemoveUserEmail($user: ID!, $email: String!) {
                                removeUserEmail(user: $user, email: $email) {
                                    alwaysNil
                                }
                            }
                        `,
                        { user, email }
                    )
                )
            )

            setIsLoading(false)
            eventLogger.log('UserEmailAddressDeleted')
            screenReaderAnnounce('Email address removed')

            if (onDidRemove) {
                onDidRemove(email)
            }
        } catch (error) {
            handleError(error)
        }
    }

    const updateEmailVerification = async (verified: boolean): Promise<void> => {
        setIsLoading(true)

        try {
            dataOrThrowErrors(
                await lastValueFrom(
                    requestGraphQL<SetUserEmailVerifiedResult, SetUserEmailVerifiedVariables>(
                        gql`
                            mutation SetUserEmailVerified($user: ID!, $email: String!, $verified: Boolean!) {
                                setUserEmailVerified(user: $user, email: $email, verified: $verified) {
                                    alwaysNil
                                }
                            }
                        `,
                        { user, email, verified }
                    )
                )
            )

            setIsLoading(false)

            if (verified) {
                eventLogger.log('UserEmailAddressMarkedVerified')
            } else {
                eventLogger.log('UserEmailAddressMarkedUnverified')
            }

            if (onEmailVerify) {
                onEmailVerify()
            }
        } catch (error) {
            handleError(error)
        }
    }

    const resendEmail = useCallback(async () => {
        setIsLoading(true)
        await resendVerificationEmail(user, email, {
            onSuccess: () => {
                setIsLoading(false)
                onEmailResendVerification?.()
            },
            onError: handleError,
        })
    }, [user, email, onEmailResendVerification, handleError])

    return (
        <>
            <div className="d-flex align-items-center justify-content-between">
                <div className="d-flex align-items-center">
                    <span className="mr-2">{email}</span>
                    {/*
                        a11y-ignore
                        Rule: "color-contrast" (Elements must have sufficient color contrast)
                        GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
                    */}
                    {verified && (
                        <Badge variant="success" className="mr-1 a11y-ignore">
                            Verified
                        </Badge>
                    )}
                    {!verified && !verificationPending && (
                        <Badge variant="secondary" className="mr-1">
                            Not verified
                        </Badge>
                    )}
                    {isPrimary && (
                        <Badge variant="primary" className="mr-1">
                            Primary
                        </Badge>
                    )}
                    {!verified && verificationPending && (
                        <span>
                            <span className={styles.dot}>&bull;&nbsp;</span>
                            <Button
                                className="p-0"
                                onClick={resendEmail}
                                disabled={isLoading || disableControls}
                                variant="link"
                            >
                                Resend verification email
                            </Button>
                        </span>
                    )}
                </div>
                <div className="d-flex align-items-center">
                    {viewerCanManuallyVerify && (
                        <Button
                            className="p-0"
                            onClick={() => updateEmailVerification(!verified)}
                            disabled={isLoading || disableControls}
                            variant="link"
                        >
                            {verified ? 'Mark as unverified' : 'Mark as verified'}
                        </Button>
                    )}{' '}
                    {!isPrimary && (
                        <Button
                            className="text-danger p-0"
                            onClick={removeEmail}
                            disabled={isLoading || disableControls}
                            variant="link"
                        >
                            Remove
                        </Button>
                    )}
                </div>
            </div>
        </>
    )
}
