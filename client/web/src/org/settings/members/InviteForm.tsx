import React, { useCallback, useState } from 'react'

import { mdiPlus, mdiEmailOpenOutline, mdiClose } from '@mdi/js'
import classNames from 'classnames'
import { lastValueFrom } from 'rxjs'
import { map } from 'rxjs/operators'

import { asError, createAggregateError, isErrorLike } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import type { Scalars } from '@sourcegraph/shared/src/graphql-operations'
import {
    LoadingSpinner,
    Button,
    Link,
    Alert,
    Icon,
    Input,
    Text,
    Code,
    Tooltip,
    ErrorAlert,
    Form,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../auth'
import { requestGraphQL } from '../../../backend/graphql'
import { CopyableText } from '../../../components/CopyableText'
import { DismissibleAlert } from '../../../components/DismissibleAlert'
import type {
    InviteUserToOrganizationResult,
    InviteUserToOrganizationVariables,
    AddUserToOrganizationResult,
    AddUserToOrganizationVariables,
    InviteUserToOrganizationFields,
} from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'

import styles from './InviteForm.module.scss'

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

export const InviteForm: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
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
    const [invited, setInvited] = useState<Invited>()
    const [isInviteShown, setShowInvitation] = useState<boolean>(false)

    const inviteUser = useCallback(() => {
        eventLogger.log('InviteOrgMemberClicked')
        ;(async () => {
            setLoading('inviteUserToOrganization')
            const { invitationURL, sentInvitationEmail } = await inviteUserToOrganization(username, orgID)
            setInvited({ username, sentInvitationEmail, invitationURL })
            setShowInvitation(true)
            onOrganizationUpdate()
            setUsername('')
            setLoading(undefined)
        })().catch(error => setLoading(asError(error)))
    }, [onOrganizationUpdate, orgID, setShowInvitation, username])

    const onInviteClick = useCallback<React.MouseEventHandler<HTMLButtonElement>>(() => {
        inviteUser()
    }, [inviteUser])

    const dismissNotification = (): void => {
        setShowInvitation(false)
    }

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
        <div>
            <div className={styles.container}>
                <Form className="form-inline align-items-end" onSubmit={onSubmit}>
                    <Input
                        inputClassName="mb-2 mr-sm-2"
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
                        className="mb-0 w-auto flex-column align-items-start"
                    />
                    <div className="d-block d-md-inline mb-sm-2">
                        {viewerCanAddUserToOrganization && (
                            <Tooltip content="Add immediately without sending invitation (site admins only)">
                                <Button
                                    type="submit"
                                    disabled={
                                        loading === 'addUserToOrganization' || loading === 'inviteUserToOrganization'
                                    }
                                    className="mr-2"
                                    variant="primary"
                                >
                                    {loading === 'addUserToOrganization' ? (
                                        <LoadingSpinner />
                                    ) : (
                                        <Icon aria-hidden={true} svgPath={mdiPlus} />
                                    )}{' '}
                                    Add member
                                </Button>
                            </Tooltip>
                        )}
                        {(emailInvitesEnabled || !viewerCanAddUserToOrganization) && (
                            <Tooltip
                                content={
                                    emailInvitesEnabled
                                        ? 'Send invitation email with link to join this organization'
                                        : 'Generate invitation link to manually send to user'
                                }
                            >
                                <Button
                                    type={viewerCanAddUserToOrganization ? 'button' : 'submit'}
                                    disabled={
                                        loading === 'addUserToOrganization' || loading === 'inviteUserToOrganization'
                                    }
                                    variant={viewerCanAddUserToOrganization ? 'secondary' : 'primary'}
                                    onClick={viewerCanAddUserToOrganization ? onInviteClick : undefined}
                                >
                                    {loading === 'inviteUserToOrganization' ? (
                                        <LoadingSpinner />
                                    ) : (
                                        <Icon aria-hidden={true} svgPath={mdiEmailOpenOutline} />
                                    )}{' '}
                                    {emailInvitesEnabled
                                        ? viewerCanAddUserToOrganization
                                            ? 'Send invitation to join'
                                            : 'Send invitation'
                                        : 'Generate invitation link'}
                                </Button>
                            </Tooltip>
                        )}
                    </div>
                </Form>
            </div>
            {authenticatedUser?.siteAdmin && !emailInvitesEnabled && (
                <DismissibleAlert variant="info" partialStorageKey="org-invite-email-config">
                    <Text className="mb-0">
                        Set <Code>email.smtp</Code> in <Link to="/site-admin/configuration">site configuration</Link> to
                        send email notifications about invitations.
                    </Text>
                </DismissibleAlert>
            )}
            {invited && isInviteShown && (
                <InvitedNotification
                    key={invited.username}
                    {...invited}
                    className={styles.alert}
                    onDismiss={dismissNotification}
                />
            )}
            {isErrorLike(loading) && <ErrorAlert className={styles.alert} error={loading} />}
        </div>
    )
}

function inviteUserToOrganization(
    username: string,
    organization: Scalars['ID']
): Promise<InviteUserToOrganizationResult['inviteUserToOrganization']> {
    return lastValueFrom(
        requestGraphQL<InviteUserToOrganizationResult, InviteUserToOrganizationVariables>(
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
        ).pipe(
            map(({ data, errors }) => {
                if (!data?.inviteUserToOrganization || (errors && errors.length > 0)) {
                    eventLogger.log('InviteOrgMemberFailed')
                    throw createAggregateError(errors)
                }
                eventLogger.log('OrgMemberInvited')
                return data.inviteUserToOrganization
            })
        )
    )
}

function addUserToOrganization(username: string, organization: Scalars['ID']): Promise<void> {
    return lastValueFrom(
        requestGraphQL<AddUserToOrganizationResult, AddUserToOrganizationVariables>(
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
        ).pipe(
            map(({ data, errors }) => {
                if (!data?.addUserToOrganization || (errors && errors.length > 0)) {
                    eventLogger.log('AddOrgMemberFailed')
                    throw createAggregateError(errors)
                }
                eventLogger.log('OrgMemberAdded')
            })
        )
    )
}

interface InvitedNotificationProps {
    username: string
    sentInvitationEmail: boolean
    invitationURL: string
    onDismiss: () => void
    className?: string
}

const InvitedNotification: React.FunctionComponent<React.PropsWithChildren<InvitedNotificationProps>> = ({
    className,
    username,
    sentInvitationEmail,
    invitationURL,
    onDismiss,
}) => (
    <Alert variant="success" className={classNames(styles.invitedNotification, className)}>
        <div className={styles.message}>
            {sentInvitationEmail ? (
                <>
                    Invitation sent to {username}. You can also send {username} the invitation link directly:
                </>
            ) : (
                <>Generated invitation link. Copy and send it to {username}:</>
            )}
            <CopyableText label="Invitation URL" text={invitationURL} size={40} className="mt-2" />
        </div>
        <Button variant="icon" title="Dismiss" onClick={onDismiss}>
            <Icon aria-hidden={true} svgPath={mdiClose} />
        </Button>
    </Alert>
)
