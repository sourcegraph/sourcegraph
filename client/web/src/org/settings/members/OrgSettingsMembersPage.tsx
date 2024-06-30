import * as React from 'react'

import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { pluralize } from '@sourcegraph/common'
import { useMutation } from '@sourcegraph/http-client'
import { UserAvatar } from '@sourcegraph/shared/src/components/UserAvatar'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import {
    Alert,
    Button,
    Container,
    ErrorAlert,
    Input,
    Link,
    LoadingSpinner,
    PageHeader,
    PageSwitcher,
    Text,
    Tooltip,
    useDebounce,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../auth'
import { usePageSwitcherPagination } from '../../../components/FilteredConnection/hooks/usePageSwitcherPagination'
import { PageTitle } from '../../../components/PageTitle'
import type {
    OrgAreaOrganizationFields,
    OrganizationMemberNode,
    OrganizationSettingsMembersResult,
    OrganizationSettingsMembersVariables,
    RemoveUserFromOrganizationResult,
    RemoveUserFromOrganizationVariables,
} from '../../../graphql-operations'
import { userURL } from '../../../user'
import type { OrgAreaRouteContext } from '../../area/OrgArea'
import { ORGANIZATION_MEMBERS_QUERY, REMOVE_USER_FROM_ORGANIZATION_QUERY } from '../../backend'

import { InviteForm } from './InviteForm'

import styles from './OrgSettingsMembersPage.module.scss'

interface UserNodeProps {
    /** The user to display in this list item. */
    node: OrganizationMemberNode

    /** The organization being displayed. */
    org: OrgAreaOrganizationFields

    /** If there is only one member in the organization. */
    hasOneMember: boolean

    /** The currently authenticated user. */
    authenticatedUser: AuthenticatedUser | null

    /** Called when the user is updated by an action in this list item. */
    onDidUpdate?: (didRemoveSelf: boolean) => void
    blockRemoveOnlyMember?: () => boolean
}

const UserNode: React.FunctionComponent<UserNodeProps> = ({
    node,
    org,
    authenticatedUser,
    hasOneMember,
    onDidUpdate,
    blockRemoveOnlyMember,
}) => {
    const isSelf = React.useMemo(() => node.id === authenticatedUser?.id, [node, authenticatedUser])
    const [removeUserFromOrganisation, { error, loading }] = useMutation<
        RemoveUserFromOrganizationResult,
        RemoveUserFromOrganizationVariables
    >(REMOVE_USER_FROM_ORGANIZATION_QUERY, {
        variables: { user: node.id, organization: org.id },
        onCompleted: () => onDidUpdate?.(isSelf),
    })

    const remove = (): any => {
        if (hasOneMember && blockRemoveOnlyMember?.()) {
            return
        }

        if (window.confirm(isSelf ? 'Leave the organization?' : `Remove the user ${node.username}?`)) {
            return removeUserFromOrganisation()
        }
    }

    return (
        <li className={classNames(styles.container, 'list-group-item')} data-test-username={node.username}>
            <div className="d-flex align-items-center justify-content-between">
                <div className="d-flex align-items-center flex-1">
                    <Tooltip content={node.displayName || node.username}>
                        <UserAvatar size={36} className={styles.avatar} user={node} />
                    </Tooltip>
                    <div className="ml-2">
                        <Link to={userURL(node.username)}>
                            <strong>{node.displayName || node.username}</strong>
                        </Link>
                        {node.displayName && (
                            <>
                                <br />
                                <span className="text-muted">{node.username}</span>
                            </>
                        )}
                    </div>
                </div>
                {authenticatedUser && org.viewerCanAdminister && (
                    <Button
                        className="site-admin-detail-list__action test-remove-org-member"
                        onClick={remove}
                        disabled={loading}
                        variant="secondary"
                        size="sm"
                    >
                        {isSelf ? 'Leave organization' : 'Remove from organization'}
                    </Button>
                )}
            </div>
            {error && <ErrorAlert className="mt-2" error={error} />}
        </li>
    )
}

interface Props extends OrgAreaRouteContext {}

/**
 * The organizations members page
 */
export const OrgSettingsMembersPage: React.FunctionComponent<Props> = ({
    org,
    authenticatedUser,
    onOrganizationUpdate,
    telemetryRecorder,
}) => {
    React.useEffect(() => {
        EVENT_LOGGER.logViewEvent('OrgMembers')
        telemetryRecorder.recordEvent('org.members', 'view')
    }, [telemetryRecorder])

    const navigate = useNavigate()
    const [onlyMemberRemovalAttempted, setOnlyMemberRemovalAttempted] = React.useState(false)
    const [searchQuery, setSearchQuery] = React.useState<string>('')
    const debouncedSearchQuery = useDebounce<string>(searchQuery, 300)

    const {
        data,
        connection,
        loading,
        error,
        refetch: refetchQuery,
        ...paginationProps
    } = usePageSwitcherPagination<
        OrganizationSettingsMembersResult,
        OrganizationSettingsMembersVariables,
        OrganizationMemberNode
    >({
        query: ORGANIZATION_MEMBERS_QUERY,
        variables: { id: org.id, query: debouncedSearchQuery },
        getConnection: ({ data }) => {
            const org = data?.node?.__typename === 'Org' ? data.node : undefined

            return org?.members
        },
    })

    const viewerCanAdminister = React.useMemo(() => {
        const orgResult = data?.node?.__typename === 'Org' ? data.node : undefined

        return orgResult?.viewerCanAdminister || org.viewerCanAdminister
    }, [data, org.viewerCanAdminister])

    const hasOneMember = React.useMemo(() => connection?.totalCount === 1, [connection])

    const refetch = React.useCallback(() => {
        refetchQuery()
        setOnlyMemberRemovalAttempted(false)
    }, [refetchQuery, setOnlyMemberRemovalAttempted])

    const blockRemoveOnlyMember = React.useCallback(() => {
        if (!authenticatedUser.siteAdmin) {
            setOnlyMemberRemovalAttempted(true)
            return true
        }
        return false
    }, [authenticatedUser, setOnlyMemberRemovalAttempted])

    const onDidUpdate = React.useCallback(
        (didRemoveSelf: boolean) => {
            telemetryRecorder.recordEvent('org.members', 'remove')
            if (didRemoveSelf) {
                navigate('/user/settings')
            } else {
                refetch()
            }
        },
        [refetch, navigate, telemetryRecorder]
    )

    const totalCount = connection?.totalCount || 0

    return (
        <div className="org-settings-members-page">
            <PageTitle title={`Members - ${org.name}`} />
            <PageHeader
                path={[{ text: authenticatedUser?.siteAdmin ? 'Add or invite member' : 'Invite member' }]}
                headingElement="h2"
                className="mb-3"
            />
            {viewerCanAdminister && (
                <InviteForm
                    orgID={org.id}
                    authenticatedUser={authenticatedUser}
                    onOrganizationUpdate={onOrganizationUpdate}
                    onDidUpdateOrganizationMembers={refetch}
                    telemetryRecorder={telemetryRecorder}
                />
            )}
            <Container className="mt-3">
                <PageHeader
                    path={[{ text: 'Members' }]}
                    headingElement="h2"
                    className="mb-3"
                    actions={
                        <Input
                            placeholder="Search by username or display name"
                            onChange={event => setSearchQuery(event.target.value || '')}
                            value={searchQuery}
                            autoComplete="off"
                            autoCapitalize="off"
                            autoCorrect="off"
                            spellCheck={false}
                            size={30}
                            className="mb-0"
                        />
                    }
                />
                {onlyMemberRemovalAttempted && (
                    <Alert variant="warning">You can’t remove the only member of an organization</Alert>
                )}
                {connection?.totalCount === 0 ? (
                    <Alert variant="warning">No members found based on the search query.</Alert>
                ) : null}
                {loading && <LoadingSpinner />}
                {error && <ErrorAlert className="mb-3" error={error} />}

                {totalCount > 0 && (
                    <Text>
                        {`${totalCount} ${debouncedSearchQuery ? 'matching' : ''} ${pluralize('member', totalCount)}`}
                    </Text>
                )}
                <ul className="list-group list-group-flush test-org-members mt-4">
                    {(connection?.nodes || []).map(node => (
                        <UserNode
                            key={node.id}
                            node={node}
                            org={org}
                            hasOneMember={hasOneMember}
                            authenticatedUser={authenticatedUser}
                            blockRemoveOnlyMember={blockRemoveOnlyMember}
                            onDidUpdate={onDidUpdate}
                        />
                    ))}
                </ul>
                <PageSwitcher {...paginationProps} className="mt-4" totalCount={connection?.totalCount || 0} />
            </Container>
        </div>
    )
}
