import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import AddIcon from 'mdi-react/AddIcon'
import React, { useCallback, useMemo, useState } from 'react'
import { RouteComponentProps } from 'react-router'
import { concat, Observable, Subject } from 'rxjs'
import { catchError, concatMap, map, tap } from 'rxjs/operators'
import { gql } from '../../../../../shared/src/graphql/graphql'
import { asError, createAggregateError, isErrorLike } from '../../../../../shared/src/util/errors'
import { AccessTokenScopes } from '../../../auth/accessToken'
import { requestGraphQL } from '../../../backend/graphql'
import { Form } from '../../../../../branded/src/components/Form'
import { PageTitle } from '../../../components/PageTitle'
import { SiteAdminAlert } from '../../../site-admin/SiteAdminAlert'
import { eventLogger } from '../../../tracking/eventLogger'
import { ErrorAlert } from '../../../components/alerts'
import { CreateAccessTokenResult, CreateAccessTokenVariables, Scalars } from '../../../graphql-operations'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { UserSettingsAreaRouteContext } from '../UserSettingsArea'
import { Link } from '../../../../../shared/src/components/Link'

function createAccessToken(
    user: Scalars['ID'],
    scopes: string[],
    note: string
): Observable<CreateAccessTokenResult['createAccessToken']> {
    return requestGraphQL<CreateAccessTokenResult, CreateAccessTokenVariables>(
        gql`
            mutation CreateAccessToken($user: ID!, $scopes: [String!]!, $note: String!) {
                createAccessToken(user: $user, scopes: $scopes, note: $note) {
                    id
                    token
                }
            }
        `,
        { user, scopes, note }
    ).pipe(
        map(({ data, errors }) => {
            if (!data || !data.createAccessToken || (errors && errors.length > 0)) {
                eventLogger.log('CreateAccessTokenFailed')
                throw createAggregateError(errors)
            }
            eventLogger.log('AccessTokenCreated')
            return data.createAccessToken
        })
    )
}

interface Props
    extends Pick<UserSettingsAreaRouteContext, 'authenticatedUser' | 'user'>,
        Pick<RouteComponentProps<{}>, 'history' | 'match'>,
        TelemetryProps {
    /**
     * Called when a new access token is created and should be temporarily displayed to the user.
     */
    onDidCreateAccessToken: (value: CreateAccessTokenResult['createAccessToken']) => void
}

/**
 * A page with a form to create an access token for a user.
 */
export const UserSettingsCreateAccessTokenPage: React.FunctionComponent<Props> = ({
    telemetryService,
    onDidCreateAccessToken,
    authenticatedUser,
    user,
    history,
    match,
}) => {
    useMemo(() => {
        telemetryService.logViewEvent('NewAccessToken')
    }, [telemetryService])

    /** The contents of the note input field. */
    const [note, setNote] = useState<string>('')
    /** The selected scopes checkboxes. */
    const [scopes, setScopes] = useState<string[]>([AccessTokenScopes.UserAll])

    const onNoteChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setNote(event.currentTarget.value)
    }, [])

    const onScopesChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        const checked = event.currentTarget.checked
        const value = event.currentTarget.value
        setScopes(previous => (checked ? [...previous, value] : previous.filter(scope => scope !== value)))
    }, [])

    const submits = useMemo(() => new Subject<React.FormEvent<HTMLFormElement>>(), [])
    const onSubmit = useCallback<React.FormEventHandler<HTMLFormElement>>(event => submits.next(event), [submits])

    const creationOrError = useObservable(
        useMemo(
            () =>
                submits.pipe(
                    tap(event => event.preventDefault()),
                    concatMap(() =>
                        concat(
                            ['loading'],
                            createAccessToken(user.id, scopes, note).pipe(
                                tap(result => {
                                    // Go back to access tokens list page and display the token secret value.
                                    history.push(`${match.url.replace(/\/new$/, '')}`)
                                    onDidCreateAccessToken(result)
                                }),
                                catchError(error => [asError(error)])
                            )
                        )
                    )
                ),
            [history, match.url, note, onDidCreateAccessToken, scopes, submits, user.id]
        )
    )

    const siteAdminViewingOtherUser = authenticatedUser && authenticatedUser.id !== user.id

    return (
        <div className="user-settings-create-access-token-page">
            <PageTitle title="Create access token" />
            <h2>New access token</h2>

            {siteAdminViewingOtherUser && (
                <SiteAdminAlert className="sidebar__alert">
                    Creating access token for other user <strong>{user.username}</strong>
                </SiteAdminAlert>
            )}

            <Form onSubmit={onSubmit}>
                <div className="form-group">
                    <label htmlFor="user-settings-create-access-token-page__note">Token description</label>
                    <input
                        type="text"
                        className="form-control test-create-access-token-description"
                        id="user-settings-create-access-token-page__note"
                        onChange={onNoteChange}
                        required={true}
                        autoFocus={true}
                        placeholder="Description"
                    />
                    <small className="form-help text-muted">What's this token for?</small>
                </div>
                <div className="form-group">
                    <div className="mb-1">Token scope</div>
                    <div>
                        <small className="form-help text-muted">
                            Tokens with limited user scopes are not yet supported.
                        </small>
                    </div>
                    <div className="form-check">
                        <input
                            className="form-check-input"
                            type="checkbox"
                            id="user-settings-create-access-token-page__scope-user:all"
                            checked={true}
                            value={AccessTokenScopes.UserAll}
                            onChange={onScopesChange}
                            disabled={true}
                        />
                        <label
                            className="form-check-label"
                            htmlFor="user-settings-create-access-token-page__scope-user:all"
                        >
                            <strong>{AccessTokenScopes.UserAll}</strong> — Full control of all resources accessible to
                            the user account
                        </label>
                    </div>
                    {user.siteAdmin && (
                        <div className="form-check">
                            <input
                                className="form-check-input"
                                type="checkbox"
                                id="user-settings-create-access-token-page__scope-site-admin:sudo"
                                checked={scopes.includes(AccessTokenScopes.SiteAdminSudo)}
                                value={AccessTokenScopes.SiteAdminSudo}
                                onChange={onScopesChange}
                            />
                            <label
                                className="form-check-label"
                                htmlFor="user-settings-create-access-token-page__scope-site-admin:sudo"
                            >
                                <strong>{AccessTokenScopes.SiteAdminSudo}</strong> — Ability to perform any action as
                                any other user
                            </label>
                        </div>
                    )}
                </div>
                <button
                    type="submit"
                    disabled={creationOrError === 'loading'}
                    className="btn btn-success test-create-access-token-submit"
                >
                    {creationOrError === 'loading' ? (
                        <LoadingSpinner className="icon-inline" />
                    ) : (
                        <AddIcon className="icon-inline" />
                    )}{' '}
                    Generate token
                </button>
                <Link
                    className="btn btn-secondary ml-1 test-create-access-token-cancel"
                    to={match.url.replace(/\/new$/, '')}
                >
                    Cancel
                </Link>
            </Form>

            {isErrorLike(creationOrError) && <ErrorAlert className="invite-form__alert" error={creationOrError} />}
        </div>
    )
}
