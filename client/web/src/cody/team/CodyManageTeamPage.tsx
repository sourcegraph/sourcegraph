import React, { useEffect } from 'react'

import { mdiPlusThick } from '@mdi/js'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Icon, PageHeader, Button, Link, Text, H3, useSearchParameters } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { withAuthenticatedUser } from '../../auth/withAuthenticatedUser'
import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import { CodyProRoutes } from '../codyProRoutes'
import { CodyAlert } from '../components/CodyAlert'
import { PageHeaderIcon } from '../components/PageHeaderIcon'
import { InviteUsers } from '../invites/InviteUsers'
import { useTeamInvites } from '../management/api/react-query/invites'
import { useCurrentSubscription, useSubscriptionSummary } from '../management/api/react-query/subscriptions'
import { useTeamMembers } from '../management/api/react-query/teams'

import { TeamMemberList } from './TeamMemberList'

interface CodyManageTeamPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser
}

const AuthenticatedCodyManageTeamPage: React.FunctionComponent<CodyManageTeamPageProps> = ({ telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('cody.team.management', 'view')
    }, [telemetryRecorder])

    const navigate = useNavigate()

    // Process query params
    const parameters = useSearchParameters()
    const newSeatsPurchasedParam = parameters.get('newSeatsPurchased')
    const newSeatsPurchased: number | null = newSeatsPurchasedParam ? parseInt(newSeatsPurchasedParam, 10) : null

    // Load data
    const subscriptionQueryResult = useCurrentSubscription()
    const subscriptionSummaryQueryResult = useSubscriptionSummary()
    const isAdmin = subscriptionSummaryQueryResult.data?.userRole === 'admin'
    const teamMembersQueryResult = useTeamMembers()
    const teamMembers = teamMembersQueryResult.data?.members
    const teamInvitesQueryResult = useTeamInvites()
    const teamInvites = teamInvitesQueryResult.data
    const errorMessage =
        subscriptionQueryResult.error?.message ||
        subscriptionSummaryQueryResult.error?.message ||
        teamMembersQueryResult.error?.message ||
        teamInvitesQueryResult.error?.message

    useEffect(() => {
        if (subscriptionQueryResult.data?.subscriptionStatus === 'canceled') {
            navigate(CodyProRoutes.Subscription)
        }
    }, [navigate, subscriptionQueryResult.data])

    return (
        <>
            <Page className={classNames('d-flex flex-column')}>
                <PageTitle title="Manage Cody team" />
                <PageHeader
                    className="mb-4 mt-4"
                    actions={
                        isAdmin && (
                            <div className="d-flex">
                                <Link
                                    to={CodyProRoutes.Manage}
                                    className="d-inline-flex align-items-center mr-3"
                                    onClick={() =>
                                        telemetryRecorder.recordEvent('cody.team.manage.subscription', 'click', {
                                            metadata: {
                                                tier:
                                                    subscriptionQueryResult.data?.subscriptionStatus !== 'canceled'
                                                        ? 1
                                                        : 0,
                                            },
                                        })
                                    }
                                >
                                    Manage subscription
                                </Link>
                                <Button
                                    as={Link}
                                    to={`${CodyProRoutes.NewProSubscription}?addSeats=1`}
                                    variant="success"
                                    className="text-nowrap"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiPlusThick} /> Add seats
                                </Button>
                            </div>
                        )
                    }
                >
                    <PageHeader.Heading as="h2" styleAs="h1">
                        <div className="d-inline-flex align-items-center">
                            <PageHeaderIcon name="mdi-account-multiple-plus-gradient" className="mr-3" />
                            Manage team
                        </div>
                    </PageHeader.Heading>
                </PageHeader>

                {errorMessage ? (
                    <CodyAlert title="We couldn't load team data this time. Please try a bit later." variant="error">
                        <Text size="small" className="text-muted">
                            {errorMessage}
                        </Text>
                    </CodyAlert>
                ) : null}

                {newSeatsPurchased && (
                    <CodyAlert title={`${newSeatsPurchased} Cody teams seats purchased!`} variant="purple">
                        <Text size="small">
                            Invited users will receive unlimited autocompletions and unlimited chat messages.
                        </Text>
                    </CodyAlert>
                )}

                {isAdmin && !!subscriptionSummaryQueryResult.data && (
                    <InviteUsers
                        telemetryRecorder={telemetryRecorder}
                        subscriptionSummary={subscriptionSummaryQueryResult.data}
                    />
                )}
                {!!subscriptionSummaryQueryResult.data && (
                    <TeamMemberList
                        teamId={subscriptionSummaryQueryResult.data.teamId}
                        teamMembers={teamMembers || []}
                        invites={teamInvites || []}
                        isAdmin={isAdmin}
                        telemetryRecorder={telemetryRecorder}
                    />
                )}
            </Page>
        </>
    )
}

export const CodyManageTeamPage = withAuthenticatedUser(AuthenticatedCodyManageTeamPage)
