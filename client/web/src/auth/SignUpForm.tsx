import React, { useCallback, useMemo, useState } from 'react'

import classNames from 'classnames'
import { type Observable, of } from 'rxjs'
import { fromFetch } from 'rxjs/fetch'
import { catchError, switchMap } from 'rxjs/operators'

import { asError } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import {
    useInputValidation,
    type ValidationOptions,
    deriveInputClassName,
} from '@sourcegraph/shared/src/util/useInputValidation'
import { Link, Label, Text, LoaderInput, ErrorAlert } from '@sourcegraph/wildcard'

import { LoaderButton } from '../components/LoaderButton'
import type { SourcegraphContext } from '../jscontext'
import { V2AuthProviderTypes } from '../util/constants'
import { validatePassword, getPasswordRequirements } from '../util/security'

import { PasswordInput, UsernameInput } from './SignInSignUpCommon'
import { SignupEmailField } from './SignupEmailField'

export interface SignUpArguments {
    email: string
    username: string
    password: string
    anonymousUserId?: string
    firstSourceUrl?: string
    lastSourceUrl?: string
}

interface SignUpFormProps extends TelemetryV2Props {
    className?: string

    /** Called to perform the signup on the server. */
    onSignUp: (args: SignUpArguments) => Promise<void>

    buttonLabel?: string
    context: Pick<SourcegraphContext, 'authProviders' | 'authPasswordPolicy' | 'authMinPasswordLength'>

    // For use in ExperimentalSignUpPage. Modifies styling and removes terms of service.
    experimental?: boolean
}

const preventDefault = (event: React.FormEvent): void => event.preventDefault()

/**
 * The form for creating an account
 */
export const SignUpForm: React.FunctionComponent<React.PropsWithChildren<SignUpFormProps>> = ({
    onSignUp,
    buttonLabel,
    className,
    context,
    experimental = false,
    telemetryRecorder,
}) => {
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<Error | null>(null)

    const signUpFieldValidators: Record<'email' | 'username' | 'password', ValidationOptions> = useMemo(
        () => ({
            email: {
                synchronousValidators: [],
                asynchronousValidators: [],
            },
            username: {
                synchronousValidators: [],
                asynchronousValidators: [isUsernameUnique],
            },
            password: {
                synchronousValidators: [password => validatePassword(context, password)],
                asynchronousValidators: [],
            },
        }),
        [context]
    )

    const [emailState, nextEmailFieldChange, emailInputReference] = useInputValidation(signUpFieldValidators.email)

    const [usernameState, nextUsernameFieldChange, usernameInputReference] = useInputValidation(
        signUpFieldValidators.username
    )

    const [passwordState, nextPasswordFieldChange, passwordInputReference] = useInputValidation(
        signUpFieldValidators.password
    )

    const canRegister = emailState.kind === 'VALID' && usernameState.kind === 'VALID' && passwordState.kind === 'VALID'

    const disabled = loading || !canRegister

    const handleSubmit = useCallback(
        (event: React.FormEvent<HTMLFormElement>): void => {
            event.preventDefault()
            if (disabled) {
                return
            }

            setLoading(true)
            onSignUp({
                email: emailState.value,
                username: usernameState.value,
                password: passwordState.value,
                anonymousUserId: EVENT_LOGGER.user.anonymousUserID,
                firstSourceUrl: EVENT_LOGGER.session.getFirstSourceURL(),
                lastSourceUrl: EVENT_LOGGER.session.getLastSourceURL(),
            }).catch(error => {
                setError(asError(error))
                setLoading(false)
            })
            EVENT_LOGGER.log('InitiateSignUp')
            telemetryRecorder.recordEvent('auth', 'initiate', { metadata: { type: V2AuthProviderTypes.builtin } })
        },
        [onSignUp, disabled, emailState, usernameState, passwordState, telemetryRecorder]
    )

    return (
        <>
            {error && <ErrorAlert error={error} />}

            {/* Using  <form /> to set 'valid' + 'is-invaild' at the input level */}
            {/* eslint-disable-next-line react/forbid-elements */}
            <form className={classNames('test-signup-form', className)} onSubmit={handleSubmit} noValidate={true}>
                <SignupEmailField
                    label="Email"
                    loading={loading}
                    nextEmailFieldChange={nextEmailFieldChange}
                    emailState={emailState}
                    emailInputReference={emailInputReference}
                />
                <div className="form-group d-flex flex-column align-content-start">
                    <Label>
                        Username
                        <LoaderInput
                            className={classNames(deriveInputClassName(usernameState))}
                            loading={usernameState.kind === 'LOADING'}
                        >
                            <UsernameInput
                                className={deriveInputClassName(usernameState)}
                                onChange={nextUsernameFieldChange}
                                value={usernameState.value}
                                required={true}
                                disabled={loading}
                                placeholder=" "
                                inputRef={usernameInputReference}
                                aria-describedby="username-input-invalid-feedback"
                                error={usernameState.kind === 'INVALID' ? usernameState.reason : undefined}
                            />
                        </LoaderInput>
                    </Label>
                </div>
                <div className="form-group d-flex flex-column align-content-start">
                    <Label>
                        Password
                        <LoaderInput
                            className={classNames(deriveInputClassName(passwordState))}
                            loading={passwordState.kind === 'LOADING'}
                        >
                            <PasswordInput
                                className={deriveInputClassName(passwordState)}
                                onChange={nextPasswordFieldChange}
                                value={passwordState.value}
                                required={true}
                                disabled={loading}
                                autoComplete="new-password"
                                minLength={context.authMinPasswordLength}
                                placeholder=" "
                                onInvalid={preventDefault}
                                inputRef={passwordInputReference}
                                formNoValidate={true}
                                aria-describedby="password-input-invalid-feedback password-requirements"
                                message={getPasswordRequirements(context)}
                                error={passwordState.kind === 'INVALID' ? passwordState.reason : undefined}
                            />
                        </LoaderInput>
                    </Label>
                </div>
                <div className="form-group mb-0">
                    <LoaderButton
                        loading={loading}
                        label={buttonLabel || 'Register'}
                        type="submit"
                        disabled={disabled}
                        variant="primary"
                        display="block"
                    />
                </div>

                {!experimental && (
                    <Text className="mt-3 mb-0">
                        <small className="form-text text-muted">
                            By signing up, you agree to our{' '}
                            <Link to="https://sourcegraph.com/terms" target="_blank" rel="noopener">
                                Terms of Service
                            </Link>{' '}
                            and{' '}
                            <Link to="https://sourcegraph.com/privacy" target="_blank" rel="noopener">
                                Privacy Policy
                            </Link>
                            .
                        </small>
                    </Text>
                )}
            </form>
        </>
    )
}

// Asynchronous Validators

function isUsernameUnique(username: string): Observable<string | undefined> {
    return fromFetch(`/-/check-username-taken/${username}`).pipe(
        switchMap(response => {
            switch (response.status) {
                case 200: {
                    return of('Username is already taken.')
                }
                case 404: {
                    // Username is unique
                    return of(undefined)
                }

                default: {
                    return of('Unknown error validating username')
                }
            }
        }),
        catchError(() => of('Unknown error validating username'))
    )
}
