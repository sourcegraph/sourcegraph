import React, { useEffect, useState } from 'react'

import { Navigate, useLocation, useParams } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Alert, Link, LoadingSpinner, ErrorAlert, Container } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { PageTitle } from '../components/PageTitle'
import type { SourcegraphContext } from '../jscontext'
import { eventLogger } from '../tracking/eventLogger'

import { AuthPageWrapper } from './AuthPageWrapper'
import { getReturnTo } from './SignInSignUpCommon'

import styles from './UnlockAccount.module.scss'

interface UnlockAccountPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    context: Pick<
        SourcegraphContext,
        'allowSignup' | 'authProviders' | 'sourcegraphDotComMode' | 'xhrHeaders' | 'resetPasswordEnabled'
    >
    /** Used for testing only. */
    mockSuccess?: boolean
}

export const UnlockAccountPage: React.FunctionComponent<React.PropsWithChildren<UnlockAccountPageProps>> = props => {
    const [error, setError] = useState<Error | null>(null)
    const [loading, setLoading] = useState(true)

    const location = useLocation()
    const { token } = useParams()

    const unlockAccount = React.useCallback(async (): Promise<void> => {
        try {
            setLoading(true)
            const response = await fetch('/-/unlock-account', {
                credentials: 'same-origin',
                method: 'POST',
                headers: {
                    ...props.context.xhrHeaders,
                    Accept: 'application/json',
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    token,
                }),
            })

            if (!response.ok) {
                throw new Error('The url you provided is either expired or invalid.')
            }

            eventLogger.log('OkUnlockAccount')
            props.telemetryRecorder.recordEvent('auth.unlockAccount', 'success')
        } catch (error) {
            setError(error)
            eventLogger.log('KoUnlockAccount')
            props.telemetryRecorder.recordEvent('auth.unlockAccount', 'fail')
        } finally {
            setLoading(false)
        }
    }, [token, props.context.xhrHeaders, props.telemetryRecorder])

    useEffect(() => {
        if (props.authenticatedUser) {
            return
        }
        eventLogger.logPageView('UnlockUserAccountRequest', null, false)
        props.telemetryRecorder.recordEvent('auth.unlockAccount', 'view')

        unlockAccount().catch(error => {
            setError(error)
        })
    }, [unlockAccount, props.authenticatedUser, props.telemetryRecorder])

    if (props.authenticatedUser) {
        const returnTo = getReturnTo(location)
        return <Navigate to={returnTo} replace={true} />
    }

    return (
        <>
            <PageTitle title="Unlock account" />
            <AuthPageWrapper
                title={
                    props.context.sourcegraphDotComMode
                        ? 'Unlock your Sourcegraph.com account'
                        : 'Unlock your Sourcegraph Server account'
                }
                sourcegraphDotComMode={props.context.sourcegraphDotComMode}
                className={styles.wrapper}
            >
                <Container>
                    {!props.mockSuccess && loading && <LoadingSpinner />}
                    {!props.mockSuccess && error && <ErrorAlert className="mb-0" error={error} />}
                    {((!loading && !error) || props.mockSuccess) && (
                        <>
                            <Alert variant="success" className="mb-0">
                                Your account was unlocked. Please try to <Link to="/sign-in">sign in</Link> to continue.
                            </Alert>
                        </>
                    )}
                </Container>
            </AuthPageWrapper>
        </>
    )
}
