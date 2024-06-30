import { useCallback, useState, type FunctionComponent } from 'react'

import { lastValueFrom } from 'rxjs'

import { asError, type ErrorLike } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
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

interface Props extends TelemetryV2Props {
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
    telemetryRecorder: TelemetryV2Props['telemetryRecorder'],
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

        EVENT_LOGGER.log('UserEmailAddressVerificationResent')
        telemetryRecorder.recordEvent('settings.email.verification', 'resend')

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
    telemetryRecorder,
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
            EVENT_LOGGER.log('UserEmailAddressDeleted')
            telemetryRecorder.recordEvent('settings.email', 'delete')
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
                EVENT_LOGGER.log('UserEmailAddressMarkedVerified')
                telemetryRecorder.recordEvent('settings.email', 'verify')
            } else {
                EVENT_LOGGER.log('UserEmailAddressMarkedUnverified')
                telemetryRecorder.recordEvent('settings.email', 'unverify')
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
        await resendVerificationEmail(user, email, telemetryRecorder, {
            onSuccess: () => {
                setIsLoading(false)
                onEmailResendVerification?.()
            },
            onError: handleError,
        })
    }, [user, email, onEmailResendVerification, handleError, telemetryRecorder])

    return (
        <>
            <div className="d-flex align-items-center justify-content-between">
                <div className="d-flex align-items-center flex-gap-2">
                    <span className="mr-2">{email}</span>
                    {/*
                        a11y-ignore
                        Rule: "color-contrast" (Elements must have sufficient color contrast)
                        GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
                    */}
                    {verified && (
                        <Badge variant="success" className="a11y-ignore">
                            Verified
                        </Badge>
                    )}
                    {!verified && !verificationPending && <Badge variant="secondary">Not verified</Badge>}
                    {isPrimary && <Badge variant="primary">Primary</Badge>}
                </div>
                <div className="d-flex align-items-center flex-gap-2">
                    {!verified && verificationPending && (
                        <Button onClick={resendEmail} disabled={isLoading || disableControls} variant="secondary">
                            Resend verification email
                        </Button>
                    )}
                    {viewerCanManuallyVerify && (
                        <Button
                            onClick={() => updateEmailVerification(!verified)}
                            disabled={isLoading || disableControls}
                            variant="secondary"
                        >
                            {verified ? 'Mark as unverified' : 'Mark as verified'}
                        </Button>
                    )}
                    {!isPrimary && (
                        <Button
                            onClick={removeEmail}
                            disabled={isLoading || disableControls}
                            variant="danger"
                            outline={true}
                        >
                            Remove
                        </Button>
                    )}
                </div>
            </div>
        </>
    )
}
