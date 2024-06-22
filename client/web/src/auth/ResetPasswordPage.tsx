import * as React from 'react'

import { mdiArrowLeftBoldBoxOutline } from '@mdi/js'

import { useLocation } from 'react-router-dom'

import { asError, type ErrorLike, isErrorLike, logger } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Button, Link, LoadingSpinner, Alert, Text, Input, ErrorAlert, Form, Container, Icon } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { LoaderButton } from '../components/LoaderButton'
import { PageTitle } from '../components/PageTitle'
import type { SourcegraphContext } from '../jscontext'

import { AuthPageWrapper } from './AuthPageWrapper'
import { PasswordInput } from './SignInSignUpCommon'

import styles from './ResetPasswordPage.module.scss'

interface ResetPasswordInitFormState {
    /** The user's email input value. */
    email: string

    /**
     * The state of the form submission. If undefined, the form has not been submitted. If null, the form was
     * submitted successfully.
     */
    submitOrError: undefined | 'loading' | ErrorLike | null
}

/**
 * A form where the user can initiate the reset-password flow. This is the 1st step in the
 * reset-password flow; ResetPasswordCodePage is the 2nd step.
 */
class ResetPasswordInitForm extends React.PureComponent<TelemetryV2Props, ResetPasswordInitFormState> {
    public state: ResetPasswordInitFormState = {
        email: '',
        submitOrError: undefined,
    }

    public render(): JSX.Element | null {
        if (this.state.submitOrError === null) {
            return (
                <>
                    <Container className="w-100 mb-3" data-testid="reset-password-page-form">
                        <Text className="mb-0">Check your email for a link to reset your password.</Text>
                    </Container>
                    <Text className="text-center">
                        <Link to="/sign-in">Return to sign in</Link>
                    </Text>
                </>
            )
        }

        return (
            <>
                {isErrorLike(this.state.submitOrError) && (
                    <ErrorAlert className="mt-2" error={this.state.submitOrError} />
                )}
                <Container className="w-100 mb-3">
                    <Form data-testid="reset-password-page-form" onSubmit={this.handleSubmitResetPasswordInit}>
                        <Input
                            onChange={this.onEmailFieldChange}
                            value={this.state.email}
                            type="email"
                            name="email"
                            autoFocus={true}
                            spellCheck={false}
                            required={true}
                            autoComplete="email"
                            disabled={this.state.submitOrError === 'loading'}
                            className="form-group"
                            label="Enter your account email address and we will send you a password reset link"
                        />
                        <Button
                            type="submit"
                            disabled={this.state.submitOrError === 'loading'}
                            variant="primary"
                            display="block"
                        >
                            {this.state.submitOrError === 'loading' ? <LoadingSpinner /> : 'Send reset password link'}
                        </Button>
                    </Form>
                </Container>
                <Text className="text-center">
                    <Link to="/sign-in">Return to sign in</Link>
                </Text>
            </>
        )
    }

    private onEmailFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ email: event.target.value })
    }

    private handleSubmitResetPasswordInit = (event: React.FormEvent<HTMLFormElement>): void => {
        event.preventDefault()
        this.setState({ submitOrError: 'loading' })
        this.props.telemetryRecorder.recordEvent('auth.resetPassword.init', 'submit')
        fetch('/-/reset-password-init', {
            credentials: 'same-origin',
            method: 'POST',
            headers: {
                ...window.context.xhrHeaders,
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                email: this.state.email,
            }),
        })
            .then(response => {
                if (response.status === 200) {
                    this.setState({ submitOrError: null })
                } else if (response.status === 429) {
                    this.setState({
                        submitOrError: new Error('Too many password reset requests. Try again in a few minutes.'),
                    })
                } else {
                    response
                        .text()
                        .catch(() => null)
                        .then(text => this.setState({ submitOrError: new Error(text || 'Unknown error') }))
                        .catch(error => logger.error(error))
                }
            })
            .catch(error => this.setState({ submitOrError: asError(error) }))
    }
}

interface ResetPasswordCodeFormProps extends TelemetryV2Props {
    userID: number
    code: string
    email: string | null
    emailVerifyCode: string | null
}

interface ResetPasswordCodeFormState {
    /** The user's new password input value. */
    password: string

    /**
     * The state of the form submission. If undefined, the form has not been submitted. If null, the form was
     * submitted successfully.
     */
    submitOrError: undefined | 'loading' | ErrorLike | null
}

class ResetPasswordCodeForm extends React.PureComponent<ResetPasswordCodeFormProps, ResetPasswordCodeFormState> {
    public state: ResetPasswordCodeFormState = {
        password: '',
        submitOrError: undefined,
    }

    public render(): JSX.Element | null {
        const { email } = this.props

        if (this.state.submitOrError === null) {
            return (
                <Alert variant="success">
                    Your password was reset. <Link to={`/sign-in?email=${email}`}>Sign in with your new password</Link> to continue.
                </Alert>
            )
        }

        return (
            <>
                {isErrorLike(this.state.submitOrError) && <ErrorAlert error={this.state.submitOrError} />}
                <Container className="w-100">
                    <Link to='/password-reset'><Icon className="mr-1" aria-hidden={true} svgPath={mdiArrowLeftBoldBoxOutline} />Raise request for a different account</Link>
                    <Text className="mt-1 text-center text-muted font-weight-bold mb-5">{email}</Text>
                    <Form data-testid="reset-password-page-form" onSubmit={this.handleSubmitResetPassword}>
                        <PasswordInput
                            name="password"
                            onChange={this.onPasswordFieldChange}
                            value={this.state.password}
                            className="form-group"
                            required={true}
                            autoFocus={true}
                            autoComplete="new-password"
                            placeholder=" "
                            disabled={this.state.submitOrError === 'loading'}
                            label="Enter a new password for your account"
                        />
                        <LoaderButton
                            display="block"
                            type="submit"
                            variant="primary"
                            loading={this.state.submitOrError === 'loading'}
                            label="Reset Password"
                        />
                    </Form>
                </Container>
            </>
        )
    }

    private onPasswordFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ password: event.target.value })
    }

    private handleSubmitResetPassword = (event: React.FormEvent<HTMLFormElement>): void => {
        event.preventDefault()
        this.props.telemetryRecorder.recordEvent('auth.resetPassword', 'submit')
        this.setState({ submitOrError: 'loading' })
        fetch('/-/reset-password-code', {
            credentials: 'same-origin',
            method: 'POST',
            headers: {
                ...window.context.xhrHeaders,
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                userID: this.props.userID,
                code: this.props.code,
                password: this.state.password,
                email: this.props.email,
                emailVerifyCode: this.props.emailVerifyCode,
            }),
        })
            .then(async response => {
                if (response.status === 200) {
                    this.setState({ submitOrError: null })
                } else if (response.status >= 400 && response.status < 500) {
                    this.setState({ submitOrError: new Error(await response.text()) })
                } else {
                    this.setState({ submitOrError: new Error('Password reset failed.') })
                }
            })
            .catch(error => this.setState({ submitOrError: asError(error) }))
    }
}

interface ResetPasswordPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    context: Pick<SourcegraphContext, 'xhrHeaders' | 'sourcegraphDotComMode' | 'resetPasswordEnabled'>
}

/**
 * A page that implements the reset-password flow for a user: (1) initiate the flow by providing the email address
 * of the account whose password to reset, and (2) complete the flow by providing the password-reset code.
 */
export const ResetPasswordPage: React.FunctionComponent<ResetPasswordPageProps> = props => {
    const location = useLocation()

    React.useEffect(() => {
        EVENT_LOGGER.logViewEvent('ResetPassword', false)
        props.telemetryRecorder.recordEvent('auth.resetPassword', 'view')
    }, [props.telemetryRecorder])

    let body: JSX.Element
    if (props.authenticatedUser) {
        body = <Alert variant="danger">Authenticated users may not perform password reset.</Alert>
    } else if (props.context.resetPasswordEnabled) {
        const searchParameters = new URLSearchParams(location.search)
        if (searchParameters.has('code') || searchParameters.has('userID')) {
            const code = searchParameters.get('code')
            const userID = parseInt(searchParameters.get('userID') || '', 10)
            const email = searchParameters.get('email')
            const emailVerifyCode = searchParameters.get('emailVerifyCode')
            if (code && !isNaN(userID)) {
                body = (
                    <ResetPasswordCodeForm
                        code={code}
                        userID={userID}
                        email={email}
                        emailVerifyCode={emailVerifyCode}
                        telemetryRecorder={props.telemetryRecorder}
                    />
                )
            } else {
                body = <Alert variant="danger">The password reset link you followed is invalid.</Alert>
            }
        } else {
            body = <ResetPasswordInitForm telemetryRecorder={props.telemetryRecorder} />
        }
    } else {
        body = (
            <Alert variant="warning">
                Password reset is disabled. Ask a site administrator to manually reset your password.
            </Alert>
        )
    }

    return (
        <>
            <PageTitle title="Reset your password" />
            <AuthPageWrapper
                title="Reset your password"
                sourcegraphDotComMode={props.context.sourcegraphDotComMode}
                className={styles.wrapper}
            >
                {body}
            </AuthPageWrapper>
        </>
    )
}
