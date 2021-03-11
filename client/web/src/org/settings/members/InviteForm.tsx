import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import AddIcon from 'mdi-react/AddIcon'
import CloseIcon from 'mdi-react/CloseIcon'
import EmailOpenOutlineIcon from 'mdi-react/EmailOpenOutlineIcon'
import React, { useCallback, useState } from 'react'
import { map } from 'rxjs/operators'
import { gql } from '../../../../../shared/src/graphql/graphql'
import { asError, createAggregateError, isErrorLike } from '../../../../../shared/src/util/errors'
import { requestGraphQL } from '../../../backend/graphql'
import { CopyableText } from '../../../components/CopyableText'
import { DismissibleAlert } from '../../../components/DismissibleAlert'
import { Form } from '../../../../../branded/src/components/Form'
import { eventLogger } from '../../../tracking/eventLogger'
import { ErrorAlert } from '../../../components/alerts'
import { AuthenticatedUser } from '../../../auth'
import { Scalars } from '../../../../../shared/src/graphql-operations'
import {
    InviteUserToOrganizationResult,
    InviteUserToOrganizationVariables,
    AddUserToOrganizationResult,
    AddUserToOrganizationVariables,
    InviteUserToOrganizationFields,
} from '../../../graphql-operations'
import classNames from 'classnames'
import { Link } from '../../../../../shared/src/components/Link'

const emailInvitesEnabled = window.context.emailEnabled

interface Invited extends InviteUserToOrganizationFields {
    username: string
}

interface Props {
    orgID: Scalars['ID']
    authenticatedUser: AuthenticatedUser | null

    /** Called when the organization members list changes. */
    onDidUpdateOrganizationMembers: () => void

    onOrganizationUpdate: () => void
}

export const InviteForm: React.FunctionComponent<Props> = ({
    orgID,
    authenticatedUser,
    onDidUpdateOrganizationMembers,
    onOrganizationUpdate,
}) => {
    const [username, setUsername] = useState<string>('')
    const onUsernameChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setUsername(event.currentTarget.value)
    }, [])
    const [loading, setLoading] = useState<'inviteUserToOrganization' | 'addUserToOrganization' | Error>()
    const [invited, setInvited] = useState<Invited[]>([])

    const inviteUser = useCallback(() => {
        eventLogger.log('InviteOrgMemberClicked')
        ;(async () => {
            setLoading('inviteUserToOrganization')
            const { invitationURL, sentInvitationEmail } = await inviteUserToOrganization(username, orgID)
            setInvited(previous => [...previous, { username, sentInvitationEmail, invitationURL }])
            onOrganizationUpdate()
            setUsername('')
            setLoading(undefined)
        })().catch(error => setLoading(asError(error)))
    }, [onOrganizationUpdate, orgID, username])

    const onInviteClick = useCallback<React.MouseEventHandler<HTMLButtonElement>>(() => {
        inviteUser()
    }, [inviteUser])

    const dismissNotification = useCallback((dismissedIndex: number): void => {
        setInvited(previous => {
            previous.splice(dismissedIndex, 1)
            return previous
        })
    }, [])

    const viewerCanAddUserToOrganization = !!authenticatedUser && authenticatedUser.siteAdmin

    const onSubmit = useCallback<React.FormEventHandler<HTMLFormElement>>(
        async event => {
            event.preventDefault()
            if (!viewerCanAddUserToOrganization) {
                inviteUser()
            } else {
                setLoading('addUserToOrganization')
                try {
                    await addUserToOrganization(username, orgID)
                    onDidUpdateOrganizationMembers()
                    setUsername('')
                    setLoading(undefined)
                } catch (error) {
                    setLoading(asError(error))
                }
            }
        },
        [inviteUser, onDidUpdateOrganizationMembers, orgID, username, viewerCanAddUserToOrganization]
    )

    return (
        <div className="invite-form">
            <div className="card invite-form__container">
                <div className="card-body">
                    <h4 className="card-title">
                        {viewerCanAddUserToOrganization ? 'Add or invite member' : 'Invite member'}
                    </h4>
                    <Form className="form-inline align-items-start" onSubmit={onSubmit}>
                        <label className="sr-only" htmlFor="invite-form__username">
                            Username
                        </label>
                        <input
                            type="text"
                            className="form-control mb-2 mr-sm-2"
                            id="invite-form__username"
                            placeholder="Username"
                            onChange={onUsernameChange}
                            value={username}
                            autoComplete="off"
                            autoCapitalize="off"
                            autoCorrect="off"
                            required={true}
                            spellCheck={false}
                            size={30}
                        />
                        <div className="d-block d-md-inline">
                            {viewerCanAddUserToOrganization && (
                                <button
                                    type="submit"
                                    disabled={
                                        loading === 'addUserToOrganization' || loading === 'inviteUserToOrganization'
                                    }
                                    className="btn btn-primary mr-2"
                                    data-tooltip="Add immediately without sending invitation (site admins only)"
                                >
                                    {loading === 'addUserToOrganization' ? (
                                        <LoadingSpinner className="icon-inline" />
                                    ) : (
                                        <AddIcon className="icon-inline" />
                                    )}{' '}
                                    Add member
                                </button>
                            )}
                            {(emailInvitesEnabled || !viewerCanAddUserToOrganization) && (
                                <button
                                    type={viewerCanAddUserToOrganization ? 'button' : 'submit'}
                                    disabled={
                                        loading === 'addUserToOrganization' || loading === 'inviteUserToOrganization'
                                    }
                                    className={`btn ${
                                        viewerCanAddUserToOrganization ? 'btn-secondary' : 'btn-primary'
                                    }`}
                                    data-tooltip={
                                        emailInvitesEnabled
                                            ? 'Send invitation email with link to join this organization'
                                            : 'Generate invitation link to manually send to user'
                                    }
                                    onClick={viewerCanAddUserToOrganization ? onInviteClick : undefined}
                                >
                                    {loading === 'inviteUserToOrganization' ? (
                                        <LoadingSpinner className="icon-inline" />
                                    ) : (
                                        <EmailOpenOutlineIcon className="icon-inline" />
                                    )}{' '}
                                    {emailInvitesEnabled
                                        ? viewerCanAddUserToOrganization
                                            ? 'Send invitation to join'
                                            : 'Send invitation'
                                        : 'Generate invitation link'}
                                </button>
                            )}
                        </div>
                    </Form>
                </div>
            </div>
            {authenticatedUser?.siteAdmin && !emailInvitesEnabled && (
                <DismissibleAlert className="alert-info" partialStorageKey="org-invite-email-config">
                    <p className=" mb-0">
                        Set <code>email.smtp</code> in <Link to="/site-admin/configuration">site configuration</Link> to
                        send email notfications about invitations.
                    </p>
                </DismissibleAlert>
            )}
            {invited.map((invite, index) => (
                /* eslint-disable react/jsx-no-bind */
                <InvitedNotification
                    key={index}
                    {...invite}
                    className="alert alert-success invite-form__alert"
                    onDismiss={() => dismissNotification(index)}
                />
                /* eslint-enable react/jsx-no-bind */
            ))}
            {isErrorLike(loading) && <ErrorAlert className="invite-form__alert" error={loading} />}
        </div>
    )
}

function inviteUserToOrganization(
    username: string,
    organization: Scalars['ID']
): Promise<InviteUserToOrganizationResult['inviteUserToOrganization']> {
    return requestGraphQL<InviteUserToOrganizationResult, InviteUserToOrganizationVariables>(
        gql`
            mutation InviteUserToOrganization($organization: ID!, $username: String!) {
                inviteUserToOrganization(organization: $organization, username: $username) {
                    ...InviteUserToOrganizationFields
                }
            }

            fragment InviteUserToOrganizationFields on InviteUserToOrganizationResult {
                sentInvitationEmail
                invitationURL
            }
        `,
        {
            username,
            organization,
        }
    )
        .pipe(
            map(({ data, errors }) => {
                if (!data || !data.inviteUserToOrganization || (errors && errors.length > 0)) {
                    eventLogger.log('InviteOrgMemberFailed')
                    throw createAggregateError(errors)
                }
                eventLogger.log('OrgMemberInvited')
                return data.inviteUserToOrganization
            })
        )
        .toPromise()
}

function addUserToOrganization(username: string, organization: Scalars['ID']): Promise<void> {
    return requestGraphQL<AddUserToOrganizationResult, AddUserToOrganizationVariables>(
        gql`
            mutation AddUserToOrganization($organization: ID!, $username: String!) {
                addUserToOrganization(organization: $organization, username: $username) {
                    alwaysNil
                }
            }
        `,
        {
            username,
            organization,
        }
    )
        .pipe(
            map(({ data, errors }) => {
                if (!data || !data.addUserToOrganization || (errors && errors.length > 0)) {
                    eventLogger.log('AddOrgMemberFailed')
                    throw createAggregateError(errors)
                }
                eventLogger.log('OrgMemberAdded')
            })
        )
        .toPromise()
}

interface InvitedNotificationProps {
    username: string
    sentInvitationEmail: boolean
    invitationURL: string
    onDismiss: () => void
    className?: string
}

const InvitedNotification: React.FunctionComponent<InvitedNotificationProps> = ({
    className,
    username,
    sentInvitationEmail,
    invitationURL,
    onDismiss,
}) => (
    <div className={classNames('invited-notification', className)}>
        <div className="invited-notification__message">
            {sentInvitationEmail ? (
                <>
                    Invitation sent to {username}. You can also send {username} the invitation link directly:
                </>
            ) : (
                <>Generated invitation link. Copy and send it to {username}:</>
            )}
            <CopyableText text={invitationURL} size={40} className="mt-2" />
        </div>
        <button type="button" className="btn btn-icon" title="Dismiss" onClick={onDismiss}>
            <CloseIcon className="icon-inline" />
        </button>
    </div>
)
