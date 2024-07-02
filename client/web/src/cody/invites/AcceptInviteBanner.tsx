import { Button, ButtonLink, Text } from '@sourcegraph/wildcard'

import { CodyProRoutes } from '../codyProRoutes'
import { CodyAlert } from '../components/CodyAlert'
import { useAcceptInvite, useCancelInvite } from '../management/api/react-query/invites'

import { useInviteParams } from './useInviteParams'
import { UserInviteStatus, useInviteState } from './useInviteState'

export const AcceptInviteBanner: React.FC<{ onSuccess: () => unknown }> = ({ onSuccess }) => {
    const { inviteParams, clearInviteParams } = useInviteParams()
    if (!inviteParams) {
        return null
    }
    return (
        <AcceptInviteBannerContent
            teamId={inviteParams.teamId}
            inviteId={inviteParams.inviteId}
            onSuccess={onSuccess}
            clearInviteParams={clearInviteParams}
        />
    )
}

const AcceptInviteBannerContent: React.FC<{
    teamId: string
    inviteId: string
    onSuccess: () => unknown
    clearInviteParams: () => void
}> = ({ teamId, inviteId, onSuccess, clearInviteParams }) => {
    const inviteState = useInviteState(teamId, inviteId)
    const acceptInviteMutation = useAcceptInvite()
    const cancelInviteMutation = useCancelInvite()

    if (inviteState.status === 'loading') {
        return null
    }

    if (
        inviteState.status === 'error' ||
        inviteState.initialInviteStatus !== 'sent' ||
        inviteState.initialUserStatus === UserInviteStatus.Error
    ) {
        return (
            <CodyAlert title="Issue with invite" variant="error">
                <Text>The invitation is no longer valid. Contact your team admin.</Text>
            </CodyAlert>
        )
    }

    switch (inviteState.initialUserStatus) {
        case UserInviteStatus.NoCurrentTeam:
        case UserInviteStatus.AnotherTeamMember: {
            // Invite has been canceled. Remove the banner.
            if (cancelInviteMutation.isSuccess || cancelInviteMutation.isError) {
                return null
            }

            switch (acceptInviteMutation.status) {
                case 'error': {
                    return (
                        <CodyAlert title="Issue with invite" variant="error" badge="Alert">
                            <Text>Accepting invite failed with error: {acceptInviteMutation.error.message}.</Text>
                        </CodyAlert>
                    )
                }
                case 'success': {
                    return (
                        <CodyAlert title="Pro team change complete!" variant="green" badge="CodyPro">
                            <Text>
                                {inviteState.initialUserStatus === UserInviteStatus.NoCurrentTeam
                                    ? 'You successfully joined the new Cody Pro team.'
                                    : 'Your pro team has been successfully changed.'}
                            </Text>
                        </CodyAlert>
                    )
                }
                case 'idle':
                case 'pending':
                default: {
                    return (
                        <CodyAlert title="Join new Cody Pro team?" variant="purple">
                            <Text>
                                You've been invited to a new Cody Pro team by {inviteState.sentBy}. <br />
                                {inviteState.initialUserStatus === UserInviteStatus.NoCurrentTeam
                                    ? 'You will get unlimited autocompletions, chat messages and commands.'
                                    : 'This will terminate your current Cody Pro plan, and place you on the new Cody Pro team. You will not lose access to your Cody Pro benefits.'}
                            </Text>
                            <div className="mt-3">
                                <Button
                                    variant="primary"
                                    disabled={acceptInviteMutation.isPending || cancelInviteMutation.isPending}
                                    className="mr-3"
                                    onClick={() =>
                                        acceptInviteMutation.mutate(
                                            { teamId, inviteId },
                                            { onSuccess, onSettled: clearInviteParams }
                                        )
                                    }
                                >
                                    Accept
                                </Button>
                                <Button
                                    variant="link"
                                    disabled={acceptInviteMutation.isPending || cancelInviteMutation.isPending}
                                    onClick={() =>
                                        cancelInviteMutation.mutate(
                                            { teamId, inviteId },
                                            { onSettled: clearInviteParams }
                                        )
                                    }
                                >
                                    Decline
                                </Button>
                            </div>
                        </CodyAlert>
                    )
                }
            }
        }
        case UserInviteStatus.InvitedTeamMember: {
            if (cancelInviteMutation.isIdle) {
                void cancelInviteMutation.mutate({ teamId, inviteId }, { onSettled: clearInviteParams })
            }
            return (
                <CodyAlert title="Issue with invite" variant="error">
                    <Text>
                        You've been invited to a Cody Pro team by {inviteState.sentBy}.<br />
                        You cannot accept this invite as as you are already on this team.
                    </Text>
                </CodyAlert>
            )
        }
        case UserInviteStatus.AnotherTeamSoleAdmin: {
            return (
                <CodyAlert title="Issue with invite" variant="error">
                    <Text>
                        You've been invited to a new Cody Pro team by {inviteState.sentBy}. <br />
                        To accept this invite you need to transfer your administrative role to another member of your
                        team and click the invite link again.
                    </Text>
                    <div className="mt-3">
                        <ButtonLink variant="primary" to={CodyProRoutes.ManageTeam}>
                            Manage team
                        </ButtonLink>
                    </div>
                </CodyAlert>
            )
        }
        default: {
            return null
        }
    }
}
