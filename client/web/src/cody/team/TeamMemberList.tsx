import { type FunctionComponent, useMemo, useCallback, useState } from 'react'

import { mdiCheck } from '@mdi/js'
import classNames from 'classnames'
import { intlFormatDistance } from 'date-fns'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { H2, Text, Badge, Button, Modal, H3 } from '@sourcegraph/wildcard'

import { CodyAlert } from '../components/CodyAlert'
import { CodyContainer } from '../components/CodyContainer'
import { useCancelInvite, useResendInvite } from '../management/api/react-query/invites'
import { useUpdateTeamMember } from '../management/api/react-query/teams'
import type { TeamMember, TeamInvite } from '../management/api/types'
import { LoadingIconButton } from '../management/subscription/manage/LoadingIconButton'

import styles from './TeamMemberList.module.scss'

interface TeamMemberListProps extends TelemetryV2Props {
    teamId: string
    teamMembers: TeamMember[]
    invites: Omit<TeamInvite, 'sentBy'>[]
    isAdmin: boolean
}

// This tiny function is extracted to make it testable. Same for the "now" parameter.
export const formatInviteDate = (sentAt: string | null, now?: Date): string => {
    try {
        if (sentAt) {
            return intlFormatDistance(sentAt, now ?? new Date())
        }
        return ''
    } catch {
        return ''
    }
}

export const TeamMemberList: FunctionComponent<TeamMemberListProps> = ({
    teamId,
    teamMembers,
    invites,
    isAdmin,
    telemetryRecorder,
}) => {
    const [actionResult, setActionResult] = useState<{ message: string; isError: boolean } | null>(null)
    const updateTeamMemberMutation = useUpdateTeamMember()
    const cancelInviteMutation = useCancelInvite()
    const resendInviteMutation = useResendInvite()
    const [confirmationModal, setConfirmationModal] = useState<{
        action: 'removeMember' | 'revokeAdmin'
        member: TeamMember
    }>()

    const isLoading =
        updateTeamMemberMutation.status === 'pending' ||
        cancelInviteMutation.status === 'pending' ||
        resendInviteMutation.status === 'pending'

    const updateRole = useCallback(
        async (accountId: string, newRole: 'member' | 'admin'): Promise<void> => {
            if (isLoading) {
                return
            }

            telemetryRecorder.recordEvent('cody.team.revokeAdmin', 'click', {
                privateMetadata: { teamId, accountId },
            })
            try {
                await updateTeamMemberMutation.mutateAsync.call(undefined, {
                    updateMemberRole: { accountId, teamRole: newRole },
                })
                setActionResult({ message: 'Team role updated.', isError: false })
            } catch (error) {
                setActionResult({
                    message: `We couldn't modify the user's role. The error was: "${error}". Please try again later.`,
                    isError: true,
                })
            }
        },
        [isLoading, updateTeamMemberMutation.mutateAsync, telemetryRecorder, teamId]
    )

    const revokeInvite = useCallback(
        async (inviteId: string): Promise<void> => {
            if (isLoading) {
                return
            }
            telemetryRecorder.recordEvent('cody.team.revokeInvite', 'click', { privateMetadata: { teamId } })
            try {
                await cancelInviteMutation.mutateAsync.call(undefined, { teamId, inviteId })
                setActionResult({ message: 'Invite revoked.', isError: false })
            } catch (error) {
                setActionResult({
                    message: `We couldn't revoke the invite. The error was: "${error}". Please try again later.`,
                    isError: true,
                })
            }
        },
        [isLoading, cancelInviteMutation.mutateAsync, telemetryRecorder, teamId]
    )

    const resendInvite = useCallback(
        async (inviteId: string): Promise<void> => {
            if (isLoading) {
                return
            }
            telemetryRecorder.recordEvent('cody.team.resendInvite', 'click', { privateMetadata: { teamId } })

            try {
                await resendInviteMutation.mutateAsync.call(undefined, { inviteId })
                setActionResult({ message: 'Invite resent.', isError: false })
            } catch (error) {
                setActionResult({
                    message: `We couldn't resend the invite (${error}). Please try again later.`,
                    isError: true,
                })
            }

            telemetryRecorder.recordEvent('cody.team.resendInvite', 'click', { privateMetadata: { teamId } })
        },
        [isLoading, resendInviteMutation.mutateAsync, telemetryRecorder, teamId]
    )

    const removeMember = useCallback(
        async (accountId: string): Promise<void> => {
            if (isLoading) {
                return
            }
            telemetryRecorder.recordEvent('cody.team.removeMember', 'click', { privateMetadata: { teamId } })

            try {
                await updateTeamMemberMutation.mutateAsync.call(undefined, {
                    removeMember: { accountId, teamRole: 'member' },
                })
                setActionResult({ message: 'Team member removed.', isError: false })
            } catch (error) {
                setActionResult({
                    message: `We couldn't remove the team member. (${error}). Please try again later.`,
                    isError: true,
                })
            }
        },
        [isLoading, updateTeamMemberMutation.mutateAsync, telemetryRecorder, teamId]
    )

    const adminCount = useMemo(() => teamMembers?.filter(member => member.role === 'admin').length ?? 0, [teamMembers])

    if (!teamMembers) {
        return null
    }

    const renderConfirmActionModal = (): React.ReactNode => {
        if (!confirmationModal) {
            return null
        }

        const { action, member } = confirmationModal

        const dismissModal = (): void => setConfirmationModal(undefined)
        let title: string
        let comfirmationText: string
        let confirmButtonText: string
        let performAction: () => Promise<void>
        switch (action) {
            case 'revokeAdmin': {
                title = 'Revoke admin'
                comfirmationText = `Do you want to revoke admin rights for ${member.email}? The user will still have access to Cody Pro and remain on the team.`
                confirmButtonText = 'Revoke admin'
                performAction = () => updateRole(member.accountId, 'member')
                break
            }
            case 'removeMember': {
                title = 'Remove user'
                comfirmationText = `Do you want to remove ${member.email} from your Cody Pro team? The user will be downgraded to Cody Free.`
                confirmButtonText = 'Remove from team'
                performAction = () => removeMember(member.accountId)
                break
            }
            default: {
                return null
            }
        }

        return (
            <Modal aria-label="Confirmation modal" isOpen={!!confirmationModal} onDismiss={dismissModal}>
                <div className="pb-3">
                    <H3>{title}</H3>
                    <Text className="mt-4 mb-0">{comfirmationText}</Text>
                </div>
                <div className="d-flex mt-4 justify-content-end">
                    <Button
                        variant="secondary"
                        outline={true}
                        disabled={updateTeamMemberMutation.isPending}
                        onClick={dismissModal}
                        className="mr-3"
                    >
                        Cancel
                    </Button>
                    <LoadingIconButton
                        variant="danger"
                        disabled={updateTeamMemberMutation.isPending}
                        isLoading={updateTeamMemberMutation.isPending}
                        onClick={() => performAction().finally(dismissModal)}
                        iconSvgPath={mdiCheck}
                    >
                        {confirmButtonText}
                    </LoadingIconButton>
                </div>
            </Modal>
        )
    }

    return (
        <>
            {actionResult && (
                <CodyAlert variant={actionResult.isError ? 'error' : 'green'}>{actionResult.message}</CodyAlert>
            )}
            <CodyContainer className={classNames('p-4 border bg-1 d-flex flex-column')}>
                <H2 className="text-lg font-semibold mb-2">Team members</H2>
                <Text className="text-sm text-gray-500 mb-4">Manage invited and active users</Text>
                <ul className={classNames(styles.teamMemberList, 'list-none pl-0')}>
                    {teamMembers.map(member => (
                        <li key={member.accountId} className="d-contents">
                            <div className="align-content-center">
                                <div className="d-flex flex-row">
                                    {member.avatarUrl ? (
                                        <img
                                            src={member.avatarUrl}
                                            alt="avatar"
                                            width="40"
                                            height="40"
                                            className={classNames(styles.avatar)}
                                        />
                                    ) : (
                                        <div className={classNames(styles.avatar, styles.avatarPlaceholder)} />
                                    )}
                                    <div className="d-flex flex-column justify-content-center ml-2">
                                        {member.displayName && <strong>{member.displayName}</strong>}
                                        <Text className="mb-0">{member.email}</Text>
                                    </div>
                                </div>
                            </div>
                            <div className="align-content-center">
                                {member.role === 'admin' && (
                                    <Badge variant="primary" className="text-uppercase">
                                        admin
                                    </Badge>
                                )}
                            </div>
                            <div />
                            {isAdmin ? (
                                member.role === 'admin' ? (
                                    <>
                                        <div />
                                        <div className="align-content-center text-center">
                                            <Button
                                                variant="link"
                                                onClick={() => setConfirmationModal({ action: 'revokeAdmin', member })}
                                                className="ml-2"
                                                disabled={adminCount < 2}
                                            >
                                                Revoke admin
                                            </Button>
                                        </div>
                                    </>
                                ) : (
                                    <>
                                        <div className="align-content-center text-center">
                                            <Button
                                                variant="link"
                                                onClick={() => updateRole(member.accountId, 'admin')}
                                                className="ml-2"
                                            >
                                                Make admin
                                            </Button>
                                        </div>
                                        <div className="align-content-center text-center">
                                            <Button
                                                variant="link"
                                                onClick={() => setConfirmationModal({ action: 'removeMember', member })}
                                                className="ml-2"
                                            >
                                                Remove
                                            </Button>
                                        </div>
                                    </>
                                )
                            ) : (
                                <>
                                    <div />
                                    <div />
                                </>
                            )}
                        </li>
                    ))}
                    {invites
                        .filter(invite => invite.status === 'sent')
                        .map(invite => (
                            <li key={invite.id} className="d-contents">
                                <div className="align-content-center">
                                    <div className="d-flex flex-row">
                                        <div className={classNames(styles.avatar, styles.avatarPlaceholder)} />
                                        <div className="d-flex flex-column justify-content-center ml-2">
                                            <Text className="mb-0">{invite.email}</Text>
                                        </div>
                                    </div>
                                </div>
                                <div className="align-content-center">
                                    <Badge variant="secondary" className="mr-2 text-uppercase">
                                        invited
                                    </Badge>
                                    {invite.role === 'admin' && (
                                        <Badge variant="primary" className="text-uppercase">
                                            admin
                                        </Badge>
                                    )}
                                </div>
                                <div className="align-content-center">
                                    <em>Invite sent {formatInviteDate(invite.sentAt)}</em>
                                </div>
                                {isAdmin && (
                                    <>
                                        <div className="align-content-center text-center">
                                            <Button
                                                variant="link"
                                                onClick={() => revokeInvite(invite.id)}
                                                className="ml-2"
                                            >
                                                Revoke
                                            </Button>
                                        </div>
                                        <div className="align-content-center text-center">
                                            <Button
                                                variant="secondary"
                                                size="sm"
                                                onClick={() => resendInvite(invite.id)}
                                                className="ml-2"
                                            >
                                                Re-send invite
                                            </Button>
                                        </div>
                                    </>
                                )}
                            </li>
                        ))}
                </ul>

                {renderConfirmActionModal()}
            </CodyContainer>
        </>
    )
}
