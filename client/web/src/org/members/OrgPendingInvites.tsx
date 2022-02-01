import React, { useCallback, useEffect, useState } from 'react'

import { Container, PageHeader, LoadingSpinner } from '@sourcegraph/wildcard'

import { PageTitle } from '../../components/PageTitle'
import { eventLogger } from '../../tracking/eventLogger'
import { OrgAreaPageProps } from '../area/OrgArea'
import { IModalInviteResult, InvitedNotification, InviteMemberModal } from './InviteMemberModal'
import { gql, useQuery } from '@apollo/client'
import { OrganizationMembersResult, OrganizationMembersVariables } from '../../graphql-operations'
import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'

interface Props extends Pick<OrgAreaPageProps, 'org' | 'authenticatedUser' | 'isSourcegraphDotCom'> {}

const ORG_MEMBERS_QUERY = gql`
    query OrganizationMembers($id: ID!) {
        node(id: $id) {
            ... on Org {
                viewerCanAdminister
                members {
                    nodes {
                        id
                        username
                        displayName
                        avatarURL
                    }
                    totalCount
                }
            }
        }
    }
`

/**
 * The organization members list page.
 */
export const OrgPendingInvitesPage: React.FunctionComponent<Props> = ({ org, authenticatedUser }) => {
    const orgId = org.id
    useEffect(() => {
        eventLogger.logViewEvent('OrgPendingInvites', { orgId })
    }, [orgId])

    const [invite, setInvite] = useState<IModalInviteResult>()
    const { data, loading, error, refetch } = useQuery<OrganizationMembersResult, OrganizationMembersVariables>(
        ORG_MEMBERS_QUERY,
        {
            variables: { id: orgId },
        }
    )

    const onInviteSent = useCallback(
        async (result: IModalInviteResult) => {
            setInvite(result)
            await refetch({ id: orgId })
        },
        [setInvite, orgId]
    )

    const onInviteSentMessageDismiss = useCallback(() => {
        setInvite(undefined)
    }, [setInvite])

    const viewerCanAddUserToOrganization = !!authenticatedUser && authenticatedUser.siteAdmin

    return (
        <>
            <div className="org-pendinginvites-page">
                <PageTitle title={`${org.name} pending invites`} />
                {invite && (
                    <InvitedNotification
                        orgName={org.name}
                        username={invite.username}
                        onDismiss={onInviteSentMessageDismiss}
                        invitationURL={invite.inviteResult.inviteUserToOrganization.invitationURL}
                    />
                )}
                <div className="d-flex flex-0 justify-content-between align-items-center mb-3">
                    <PageHeader path={[{ text: 'Pending Invites' }]} headingElement="h2" />
                    <div>
                        {viewerCanAddUserToOrganization && (
                            <InviteMemberModal orgName={org.name} orgId={org.id} onInviteSent={onInviteSent} />
                        )}
                    </div>
                </div>

                <Container>
                    {loading && <LoadingSpinner />}
                    {data && <pre>{JSON.stringify(data, null, 2)}</pre>}
                    {error && (
                        <ErrorAlert
                            className="mt-2"
                            error={`Error loading ${org.name} members. Please, try refreshing the page.`}
                        />
                    )}
                </Container>
            </div>
        </>
    )
}
