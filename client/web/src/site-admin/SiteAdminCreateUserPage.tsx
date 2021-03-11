import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Subject, Subscription } from 'rxjs'
import { catchError, mergeMap, tap } from 'rxjs/operators'
import * as GQL from '../../../shared/src/graphql/schema'
import { EmailInput, UsernameInput } from '../auth/SignInSignUpCommon'
import { CopyableText } from '../components/CopyableText'
import { Form } from '../../../branded/src/components/Form'
import { PageTitle } from '../components/PageTitle'
import { eventLogger } from '../tracking/eventLogger'
import { createUser } from './backend'
import { ErrorAlert } from '../components/alerts'
import { asError } from '../../../shared/src/util/errors'

interface State {
    errorDescription?: string
    loading: boolean

    /**
     * The result of creating the user.
     */
    createUserResult?: GQL.ICreateUserResult

    // Form
    username: string
    email: string
}

/**
 * A page with a form to create a user account.
 */
export class SiteAdminCreateUserPage extends React.Component<RouteComponentProps<{}>, State> {
    public state: State = {
        loading: false,
        username: '',
        email: '',
    }

    private submits = new Subject<{ username: string; email: string }>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        eventLogger.logViewEvent('SiteAdminCreateUser')

        this.subscriptions.add(
            this.submits
                .pipe(
                    tap(() =>
                        this.setState({
                            createUserResult: undefined,
                            loading: true,
                            errorDescription: undefined,
                        })
                    ),
                    mergeMap(({ username, email }) =>
                        createUser(username, email).pipe(
                            catchError(error => {
                                console.error(error)
                                this.setState({
                                    createUserResult: undefined,
                                    loading: false,
                                    errorDescription: asError(error).message,
                                })
                                return []
                            })
                        )
                    )
                )
                .subscribe(
                    createUserResult =>
                        this.setState({
                            loading: false,
                            errorDescription: undefined,
                            createUserResult,
                        }),
                    error => console.error(error)
                )
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div className="site-admin-create-user-page">
                <PageTitle title="Create user - Admin" />
                <h2>Create user account</h2>
                <p>
                    Create a new user account
                    {window.context.resetPasswordEnabled
                        ? ' and generate a password reset link. If sending emails is not configured, you must manually send the link to the new user.'
                        : '. New users must authenticate using a configured authentication provider.'}
                </p>
                <p className="mb-4">
                    For information about configuring SSO authentication, see{' '}
                    <Link to="/help/admin/auth">User authentication</Link> in the Sourcegraph documentation.
                </p>
                {this.state.createUserResult ? (
                    <div className="alert alert-success">
                        <p>
                            Account created for <strong>{this.state.username}</strong>.
                        </p>
                        {this.state.createUserResult.resetPasswordURL !== null ? (
                            <>
                                <p>You must manually send this password reset link to the new user:</p>
                                <CopyableText text={this.state.createUserResult.resetPasswordURL} size={40} />
                            </>
                        ) : (
                            <p>The user must authenticate using a configured authentication provider.</p>
                        )}
                        <button
                            type="button"
                            className="btn btn-primary mt-2"
                            onClick={this.dismissAlert}
                            autoFocus={true}
                        >
                            Create another user
                        </button>
                    </div>
                ) : (
                    <Form onSubmit={this.onSubmit} className="site-admin-create-user-page__form">
                        <div className="form-group site-admin-create-user-page__form-group">
                            <label htmlFor="site-admin-create-user-page__form-username">Username</label>
                            <UsernameInput
                                id="site-admin-create-user-page__form-username"
                                onChange={this.onUsernameFieldChange}
                                value={this.state.username}
                                required={true}
                                disabled={this.state.loading}
                                autoFocus={true}
                            />
                            <small className="form-text text-muted">
                                A username consists of letters, numbers, hyphens (-), dots (.) and may not begin or end
                                with a dot, nor begin with a hyphen.
                            </small>
                        </div>
                        <div className="form-group site-admin-create-user-page__form-group">
                            <label htmlFor="site-admin-create-user-page__form-email">Email</label>
                            <EmailInput
                                id="site-admin-create-user-page__form-email"
                                onChange={this.onEmailFieldChange}
                                value={this.state.email}
                                disabled={this.state.loading}
                                aria-describedby="site-admin-create-user-page__form-email-help"
                            />
                            <small id="site-admin-create-user-page__form-email-help" className="form-text text-muted">
                                Optional verified email for the user.
                            </small>
                        </div>
                        {this.state.errorDescription && (
                            <ErrorAlert className="my-2" error={this.state.errorDescription} />
                        )}
                        <button className="btn btn-primary" disabled={this.state.loading} type="submit">
                            {window.context.resetPasswordEnabled
                                ? 'Create account & generate password reset link'
                                : 'Create account'}
                        </button>
                    </Form>
                )}
            </div>
        )
    }

    private onEmailFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ email: event.target.value, errorDescription: undefined })
    }

    private onUsernameFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ username: event.target.value, errorDescription: undefined })
    }

    private onSubmit = (event: React.FormEvent<HTMLFormElement>): void => {
        event.preventDefault()
        event.stopPropagation()
        this.submits.next({ username: this.state.username, email: this.state.email })
    }

    private dismissAlert = (): void =>
        this.setState({
            createUserResult: undefined,
            errorDescription: undefined,
            username: '',
            email: '',
        })
}
