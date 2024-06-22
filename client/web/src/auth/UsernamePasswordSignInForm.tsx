import React, { useCallback, useState } from 'react'

import classNames from 'classnames'
import { useLocation } from 'react-router-dom'

import { asError, logger } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Label, Button, LoadingSpinner, Link, Text, Input, Form } from '@sourcegraph/wildcard'

import type { SourcegraphContext } from '../jscontext'
import { V2AuthProviderTypes } from '../util/constants'

import { getReturnTo, PasswordInput } from './SignInSignUpCommon'

interface Props extends TelemetryV2Props {
    onAuthError: (error: Error | null) => void
    email: string | null
    context: Pick<
        SourcegraphContext,
        'allowSignup' | 'authProviders' | 'sourcegraphDotComMode' | 'xhrHeaders' | 'resetPasswordEnabled'
    >
    className?: string
}

/**
 * The form for signing in with a username and password.
 */
export const UsernamePasswordSignInForm: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    onAuthError,
    email,
    className,
    context,
    telemetryRecorder,
}) => {
    const location = useLocation()

    // To populate the username/email text-box with the user's email value on sign-in screen after successful password change request
    const [usernameOrEmail, setUsernameOrEmail] = useState(email || '')
    const [password, setPassword] = useState('')
    const [loading, setLoading] = useState(false)

    const onUsernameOrEmailFieldChange = useCallback((event: React.ChangeEvent<HTMLInputElement>): void => {
        setUsernameOrEmail(event.target.value)
    }, [])

    const onPasswordFieldChange = useCallback((event: React.ChangeEvent<HTMLInputElement>): void => {
        setPassword(event.target.value)
    }, [])

    const handleSubmit = useCallback(
        async (event: React.FormEvent<HTMLFormElement>): Promise<void> => {
            event.preventDefault()
            if (loading) {
                return
            }

            setLoading(true)
            EVENT_LOGGER.log('InitiateSignIn')
            telemetryRecorder.recordEvent('auth', 'initiate', { metadata: { type: V2AuthProviderTypes.builtin } })
            try {
                const response = await fetch('/-/sign-in', {
                    credentials: 'same-origin',
                    method: 'POST',
                    headers: {
                        ...context.xhrHeaders,
                        Accept: 'application/json',
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        email: usernameOrEmail,
                        password,
                    }),
                })
                if (response.status === 200) {
                    if (new URLSearchParams(location.search).get('close') === 'true') {
                        window.close()
                    } else {
                        const returnTo = getReturnTo(location)
                        window.location.replace(returnTo)
                    }
                } else if (response.status === 401) {
                    throw new Error('User or password was incorrect')
                } else if (response.status === 422) {
                    throw new Error('The account has been locked out')
                } else {
                    const text = await response.text()
                    throw new Error(text === '' ? 'Unknown error' : text)
                }
            } catch (error) {
                logger.error('Auth error:', error)
                setLoading(false)
                onAuthError(asError(error))
            }
        },
        [usernameOrEmail, loading, location, password, onAuthError, context, telemetryRecorder]
    )

    return (
        <>
            <Form onSubmit={handleSubmit} className={className}>
                <Input
                    id="username-or-email"
                    label={<Text alignment="left">Username or email</Text>}
                    onChange={onUsernameOrEmailFieldChange}
                    required={true}
                    value={usernameOrEmail}
                    disabled={loading}
                    autoCapitalize="off"
                    autoFocus={true}
                    className="form-group"
                    // There is no well supported way to declare username OR email here.
                    // Using username seems to be the best approach and should still support this behaviour.
                    // See: https://github.com/whatwg/html/issues/4445
                    autoComplete="username"
                />

                <div className="form-group d-flex flex-column align-content-start position-relative">
                    <Label htmlFor="password" className="align-self-start">
                        Password
                    </Label>
                    <PasswordInput
                        onChange={onPasswordFieldChange}
                        value={password}
                        required={true}
                        disabled={loading}
                        autoComplete="current-password"
                        placeholder=" "
                    />
                    {context.resetPasswordEnabled && (
                        <small className="form-text text-muted align-self-end position-absolute">
                            <Link to="/password-reset">Forgot password?</Link>
                        </small>
                    )}
                </div>

                <div className={classNames('form-group', 'mb-0')}>
                    <Button display="block" type="submit" disabled={loading} variant="primary">
                        {loading ? <LoadingSpinner /> : 'Sign in'}
                    </Button>
                </div>
            </Form>
        </>
    )
}
