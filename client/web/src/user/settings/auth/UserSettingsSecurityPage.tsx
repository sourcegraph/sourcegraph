import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import * as React from 'react'
import { Subject, Subscription } from 'rxjs'
import { catchError, filter, mergeMap, tap } from 'rxjs/operators'
import { PasswordInput } from '../../../auth/SignInSignUpCommon'
import { Form } from '../../../../../branded/src/components/Form'
import { PageTitle } from '../../../components/PageTitle'
import { eventLogger } from '../../../tracking/eventLogger'
import { updatePassword, createPassword } from '../backend'
import { ErrorAlert } from '../../../components/alerts'
import { AuthenticatedUser } from '../../../auth'
import {
    UserAreaUserFields,
    ExternalServiceKind,
    ExternalAccountFields,
    MinExternalAccountsVariables,
} from '../../../graphql-operations'
import { ExternalAccountsSignIn } from './ExternalAccountsSignIn'
import { Link } from '../../../../../shared/src/components/Link'
import { SourcegraphContext } from '../../../jscontext'
import { gql, dataOrThrowErrors } from '../../../../../shared/src/graphql/graphql'
import { requestGraphQL } from '../../../backend/graphql'
import { ErrorLike, asError } from '../../../../../shared/src/util/errors'

// pick only the fields we need
type MinExternalAccount = Pick<ExternalAccountFields, 'id' | 'serviceID' | 'serviceType' | 'accountData'>
type UserExternalAccount = UserExternalAccountsResult['user']['externalAccounts']['nodes'][0]
type ServiceType = AuthProvider['serviceType']

export type AuthProvider = SourcegraphContext['authProviders'][0]
export type ExternalAccountsByType = Partial<Record<ServiceType, UserExternalAccount>>
export type AuthProvidersByType = Partial<Record<ServiceType, AuthProvider>>

interface UserExternalAccountsResult {
    user: {
        externalAccounts: {
            nodes: MinExternalAccount[]
        }
    }
}

interface Props {
    user: UserAreaUserFields
    authenticatedUser: AuthenticatedUser
    context: Pick<SourcegraphContext, 'authProviders'>
}

interface State {
    error?: ErrorLike
    loading?: boolean
    saved?: boolean
    accounts: { fetched?: MinExternalAccount[]; lastRemoved?: string }
    oldPassword: string
    newPassword: string
    newPasswordConfirmation: string
}

const fetchUserExternalAccountsByType = async (username: string): Promise<MinExternalAccount[]> => {
    const result = dataOrThrowErrors(
        await requestGraphQL<UserExternalAccountsResult, MinExternalAccountsVariables>(
            gql`
                query MinExternalAccounts($username: String!) {
                    user(username: $username) {
                        externalAccounts {
                            nodes {
                                id
                                serviceID
                                serviceType
                                accountData
                            }
                        }
                    }
                }
            `,
            { username }
        ).toPromise()
    )
    // if user doesn't have external accounts API will return an empty array
    return result.user.externalAccounts.nodes
}

const accountsByType = (accounts: MinExternalAccount[]): ExternalAccountsByType =>
    accounts.reduce((accumulator: ExternalAccountsByType, account) => {
        accumulator[account.serviceType as ServiceType] = account
        return accumulator
    }, {})

export class UserSettingsSecurityPage extends React.Component<Props, State> {
    public state: State = {
        oldPassword: '',
        newPassword: '',
        newPasswordConfirmation: '',
        accounts: {},
    }

    private submits = new Subject<React.FormEvent<HTMLFormElement>>()
    private subscriptions = new Subscription()

    private newPasswordConfirmationField: HTMLInputElement | null = null
    private setNewPasswordConfirmationField = (element: HTMLInputElement | null): void => {
        this.newPasswordConfirmationField = element
    }

    // auth providers by service type
    private authProvidersByType = this.props.context.authProviders.reduce(
        (accumulator: AuthProvidersByType, provider) => {
            accumulator[provider.serviceType] = provider
            return accumulator
        },
        {}
    )

    private shouldShowOldPasswordInput = (): boolean =>
        /**
         * Show old password form only when all items are true
         * 1. user has a password set
         * 2. user doesn't have external accounts
         */
        this.props.user.builtinAuth && this.state.accounts.fetched?.length === 0

    private fetchAccounts = (): void => {
        fetchUserExternalAccountsByType(this.props.user.username)
            .then(accounts => {
                this.setState({ accounts: { fetched: accounts } })

                this.subscriptions.add(
                    this.submits
                        .pipe(
                            tap(event => {
                                event.preventDefault()
                                eventLogger.log('UpdatePasswordClicked')
                            }),
                            filter(event => event.currentTarget.checkValidity()),
                            tap(() => this.setState({ loading: true })),
                            mergeMap(() =>
                                (this.shouldShowOldPasswordInput()
                                    ? updatePassword({
                                          oldPassword: this.state.oldPassword,
                                          newPassword: this.state.newPassword,
                                      })
                                    : createPassword({
                                          newPassword: this.state.newPassword,
                                      })
                                ).pipe(
                                    // Sign the user out after their password is changed.
                                    // We do this because the backend will no longer accept their current session
                                    // and failing to sign them out will leave them in a confusing state
                                    tap(() => (window.location.href = '/-/sign-out')),
                                    catchError(error => this.handleError(error))
                                )
                            )
                        )
                        .subscribe(
                            () =>
                                this.setState({
                                    loading: false,
                                    error: undefined,
                                    oldPassword: '',
                                    newPassword: '',
                                    newPasswordConfirmation: '',
                                    saved: true,
                                    accounts: {},
                                }),
                            error => this.handleError(error)
                        )
                )
            })
            .catch(error => {
                this.setState({ error: asError(error) })
            })
    }

    public componentDidMount(): void {
        eventLogger.logViewEvent('UserSettingsPassword')
        this.fetchAccounts()
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <>
                <PageTitle title="Account security" />

                {this.props.authenticatedUser.id !== this.props.user.id && (
                    <div className="alert alert-danger">
                        Only the user may change their password. Site admins may{' '}
                        <Link to={`/site-admin/users?query=${encodeURIComponent(this.props.user.username)}`}>
                            reset a user's password
                        </Link>
                        .
                    </div>
                )}

                {this.state.accounts.lastRemoved && (
                    <div className="alert alert-warning" role="alert">
                        Sign in connection for {this.state.accounts.lastRemoved} removed. Please set a new password for
                        your account.
                    </div>
                )}

                {this.state.error && <ErrorAlert className="mb-3" error={this.state.error} />}

                {this.state.saved && <div className="alert alert-success mb-3">Password changed!</div>}

                <h2 className="mb-4">Account security</h2>
                <h3>Sign in connections</h3>
                <span className="text-muted">
                    Connect your account on Sourcegraph with a third-party login service to make signing in easier. This
                    will be used to sign in to Sourcegraph in the future.
                </span>

                {/* external accounts not fetched yet */}
                {!this.state.accounts.fetched && this.state.error && (
                    <div className="d-flex justify-content-center mt-4">
                        <LoadingSpinner className="icon-inline" />
                    </div>
                )}

                {/* fetched external accounts */}
                {this.state.accounts.fetched && (
                    <ExternalAccountsSignIn
                        supported={[ExternalServiceKind.GITHUB, ExternalServiceKind.GITLAB]}
                        accounts={accountsByType(this.state.accounts.fetched)}
                        authProviders={this.authProvidersByType}
                        onDidError={console.log}
                        onDidRemove={this.onAccountRemoval}
                    />
                )}

                {/* fetched external accounts but user doesn't have any */}
                {this.state.accounts.fetched?.length === 0 && (
                    <>
                        <hr className="my-4" />
                        <h3 className="pt-2">Password</h3>
                        <Form className="mt-3 user-settings-security__passwords-container" onSubmit={this.handleSubmit}>
                            {/* Include a username field as a hint for password managers to update the saved password. */}
                            <input
                                type="text"
                                value={this.props.user.username}
                                name="username"
                                autoComplete="username"
                                readOnly={true}
                                hidden={true}
                            />
                            {this.shouldShowOldPasswordInput() && (
                                <div className="form-group">
                                    <label htmlFor="oldPassword">Old password</label>
                                    <PasswordInput
                                        value={this.state.oldPassword}
                                        onChange={this.onOldPasswordFieldChange}
                                        disabled={this.state.loading}
                                        id="oldPassword"
                                        name="oldPassword"
                                        aria-label="old password"
                                        placeholder=" "
                                        autoComplete="current-password"
                                    />
                                </div>
                            )}

                            <div className="form-group">
                                <label htmlFor="newPassword">New password</label>
                                <PasswordInput
                                    value={this.state.newPassword}
                                    onChange={this.onNewPasswordFieldChange}
                                    disabled={this.state.loading}
                                    id="newPassword"
                                    name="newPassword"
                                    aria-label="new password"
                                    placeholder=" "
                                    autoComplete="new-password"
                                />
                                <small className="form-help text-muted">At least 12 characters</small>
                            </div>
                            <div className="form-group">
                                <label htmlFor="newPasswordConfirmation">Confirm new password</label>
                                <PasswordInput
                                    value={this.state.newPasswordConfirmation}
                                    onChange={this.onNewPasswordConfirmationFieldChange}
                                    disabled={this.state.loading}
                                    id="newPasswordConfirmation"
                                    name="newPasswordConfirmation"
                                    aria-label="new password confirmation"
                                    placeholder=" "
                                    inputRef={this.setNewPasswordConfirmationField}
                                    autoComplete="new-password"
                                />
                            </div>
                            <button
                                className="btn btn-primary user-settings-password-page__button"
                                type="submit"
                                disabled={this.state.loading}
                            >
                                {this.shouldShowOldPasswordInput() ? 'Update password' : 'Set password'}
                            </button>
                            {this.state.loading && (
                                <div className="icon-inline">
                                    <LoadingSpinner className="icon-inline" />
                                </div>
                            )}
                        </Form>
                    </>
                )}
            </>
        )
    }

    private onAccountRemoval = (removeId: string, name: string): void => {
        // keep every account that doesn't match removeId
        this.setState(previousState => ({
            accounts: {
                fetched: previousState.accounts.fetched?.filter(({ id }) => id !== removeId),
                lastRemoved: name,
            },
        }))
    }

    private onOldPasswordFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ oldPassword: event.target.value })
    }

    private onNewPasswordFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ newPassword: event.target.value }, () => this.validateForm())
    }

    private onNewPasswordConfirmationFieldChange = (event: React.ChangeEvent<HTMLInputElement>): void => {
        this.setState({ newPasswordConfirmation: event.target.value }, () => this.validateForm())
    }

    private validateForm(): void {
        if (this.newPasswordConfirmationField) {
            if (this.state.newPassword === this.state.newPasswordConfirmation) {
                this.newPasswordConfirmationField.setCustomValidity('') // valid
            } else {
                this.newPasswordConfirmationField.setCustomValidity("New passwords don't match.")
            }
        }
    }

    private handleSubmit = (event: React.FormEvent<HTMLFormElement>): void => {
        this.submits.next(event)
    }

    private handleError = (error: Error): [] => {
        this.setState({ loading: false, saved: false, error })
        return []
    }
}
