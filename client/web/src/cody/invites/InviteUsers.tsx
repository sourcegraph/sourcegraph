import React, { useState, useCallback, useMemo } from 'react'

import { pluralize } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { H2, Link, Text, H3, TextArea, Button, H1 } from '@sourcegraph/wildcard'

import { CodyAlert } from '../components/CodyAlert'
import { CodyContainer } from '../components/CodyContainer'
import { CodyProBadgeDeck } from '../components/CodyProBadgeDeck'
import { useSendInvite, useTeamInvites } from '../management/api/react-query/invites'
import { useCurrentSubscription, useUpdateCurrentSubscription } from '../management/api/react-query/subscriptions'
import { useTeamMembers } from '../management/api/react-query/teams'
import type { SubscriptionSummary } from '../management/api/teamSubscriptions'
import { isValidEmailAddress } from '../util'

import styles from './InviteUsers.module.scss'

interface InviteUsersProps extends TelemetryV2Props {
    subscriptionSummary: SubscriptionSummary
}

export const InviteUsers: React.FunctionComponent<InviteUsersProps> = ({ telemetryRecorder, subscriptionSummary }) => {
    const subscriptionQueryResult = useCurrentSubscription()
    const isAdmin = subscriptionSummary.userRole === 'admin'
    const teamId = subscriptionSummary.teamId
    const teamMembersQueryResult = useTeamMembers()
    const teamMembers = teamMembersQueryResult.data?.members
    const teamInvitesQueryResult = useTeamInvites()
    const teamInvites = teamInvitesQueryResult.data

    const remainingInviteCount = useMemo(() => {
        const memberCount = teamMembers?.length ?? 0
        const invitesUsed = (teamInvites ?? []).filter(invite => invite.status === 'sent').length
        return Math.max((subscriptionQueryResult.data?.maxSeats ?? 0) - (memberCount + invitesUsed), 0)
    }, [subscriptionQueryResult.data?.maxSeats, teamMembers, teamInvites])

    const [emailAddressesString, setEmailAddressesString] = useState<string>('')
    const emailAddresses = emailAddressesString.split(',').map(email => email.trim())
    const [emailAddressErrorMessage, setEmailAddressErrorMessage] = useState<string | null>(null)

    const sendInviteMutation = useSendInvite()
    const updateSubscriptionMutation = useUpdateCurrentSubscription()

    const verifyEmailList = useCallback((): Error | void => {
        if (emailAddresses.length === 0) {
            return new Error('Please enter at least one email address.')
        }

        if (emailAddresses.length > remainingInviteCount) {
            return new Error(
                `${emailAddresses.length} email addresses entered, but you only have ${remainingInviteCount} seats.`
            )
        }

        const invalidEmails = emailAddresses.filter(email => !isValidEmailAddress(email))

        if (invalidEmails.length > 0) {
            return new Error(
                `Invalid email address${invalidEmails.length > 1 ? 'es' : ''}: ${invalidEmails.join(', ')}`
            )
        }
    }, [emailAddresses, remainingInviteCount])

    const onSendInvitesClicked = useCallback(async () => {
        const emailListError = verifyEmailList()
        if (emailListError) {
            setEmailAddressErrorMessage(emailListError.message)
            return
        }
        telemetryRecorder.recordEvent('cody.team.sendInvites', 'click', {
            metadata: { count: emailAddresses.length },
            privateMetadata: { teamId, emailAddresses },
        })

        const results = await Promise.allSettled(
            emailAddresses.map(emailAddress =>
                sendInviteMutation.mutateAsync.call(undefined, { email: emailAddress, role: 'member' })
            )
        )

        const failures = results
            .map((result, index) => ({
                emailAddress: emailAddresses[index],
                errorMessage: result.status === 'rejected' ? (result.reason as Error).message : null,
            }))
            .filter(({ errorMessage }) => errorMessage)
        if (failures.length) {
            const failureList = failures
                .map(({ emailAddress, errorMessage }) => `"${emailAddress}": ${errorMessage}`)
                .join(', ')
            const errorMessage = `We couldn't send${
                failures.length < emailAddresses.length ? ` ${failures.length} of` : ''
            } the ${pluralize('invite', emailAddresses.length)}. This is what we got: ${failureList}`
            telemetryRecorder.recordEvent('cody.team.sendInvites', 'error', {
                metadata: { count: emailAddresses.length, softError: 0 },
                privateMetadata: { teamId, emailAddresses, error: errorMessage },
            })
            setEmailAddressErrorMessage(errorMessage)
            return
        }

        telemetryRecorder.recordEvent('cody.team.sendInvites', 'success', {
            metadata: { count: emailAddresses.length },
            privateMetadata: { teamId, emailAddresses },
        })
    }, [emailAddresses, sendInviteMutation.mutateAsync, teamId, telemetryRecorder, verifyEmailList])

    if (updateSubscriptionMutation.isSuccess) {
        return (
            <CodyAlert title="Remaining invites removed from plan" variant="green">
                <Text>You can add more seats at any time with the "Add seats" button.</Text>
            </CodyAlert>
        )
    }

    if (!isAdmin || !remainingInviteCount || !subscriptionQueryResult.data) {
        return null
    }

    const { maxSeats } = subscriptionQueryResult.data

    return (
        <>
            {sendInviteMutation.status === 'success' && (
                <CodyAlert
                    title={`${emailAddresses.length} ${pluralize('invite', emailAddresses.length)} sent!`}
                    variant="green"
                >
                    <Text size="small" className="mb-0">
                        Invitees will receive an email from cody@sourcegraph.com.
                    </Text>
                </CodyAlert>
            )}
            {sendInviteMutation.status === 'error' && (
                <CodyAlert title="Invites not sent." variant="error">
                    <Text size="small">Error sending invites: {sendInviteMutation.error?.message}</Text>
                    <Text size="small" className="mt-2">
                        If you encounter this issue repeatedly, please contact support at{' '}
                        <Link to="mailto:support@sourcegraph.com">support@sourcegraph.com</Link>.
                    </Text>
                </CodyAlert>
            )}

            <CodyContainer className="p-4 border bg-1 mb-4 d-flex flex-row">
                <div className="d-flex justify-content-between align-items-center w-100">
                    <CodyProBadgeDeck className={styles.codyProBadgeDeck} />
                    <div className="flex-1 d-flex flex-column ml-4">
                        <H2 className="mb-4 font-weight-normal">
                            <strong>Invite users</strong> – {remainingInviteCount}{' '}
                            {pluralize('seat', remainingInviteCount)} remaining
                        </H2>
                        <TextArea
                            className="mb-2"
                            placeholder="Example: someone@sourcegraph.com, another.user@sourcegraph.com"
                            rows={4}
                            onChange={event => {
                                setEmailAddressErrorMessage(null)
                                setEmailAddressesString(event.target.value)
                                sendInviteMutation.reset()
                            }}
                            isValid={emailAddressErrorMessage ? false : undefined}
                        />
                        {emailAddressErrorMessage ? (
                            <Text className="text-danger">{emailAddressErrorMessage}</Text>
                        ) : (
                            <Text className="text-muted">Enter email addresses separated by a comma.</Text>
                        )}

                        <div>
                            <Button
                                disabled={updateSubscriptionMutation.isPending || sendInviteMutation.isPending}
                                variant="success"
                                onClick={onSendInvitesClicked}
                                className="mr-2"
                            >
                                Send
                            </Button>
                            <Button
                                variant="link"
                                disabled={updateSubscriptionMutation.isPending || sendInviteMutation.isPending}
                                onClick={() =>
                                    updateSubscriptionMutation.mutate({
                                        subscriptionUpdate: {
                                            newSeatCount: maxSeats - remainingInviteCount,
                                        },
                                    })
                                }
                            >
                                Remove invites from plan
                            </Button>
                        </div>
                    </div>
                </div>
            </CodyContainer>
        </>
    )
}
