import React, { type FC, useEffect, useState, useCallback } from 'react'

import { mdiChevronDown, mdiInformationOutline } from '@mdi/js'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { UserAvatar } from '@sourcegraph/shared/src/components/UserAvatar'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Container,
    PageHeader,
    LoadingSpinner,
    Text,
    Alert,
    Button,
    Link,
    Input,
    Badge,
    useDebounce,
    PageSwitcher,
    Icon,
    PopoverTrigger,
    PopoverContent,
    Popover,
    Position,
    type PopoverOpenEvent,
    Tooltip,
} from '@sourcegraph/wildcard'

import { usePageSwitcherPagination } from '../../../components/FilteredConnection/hooks/usePageSwitcherPagination'
import { PageTitle } from '../../../components/PageTitle'
import type {
    SettingsAreaRepositoryFields,
    RepoPermissionsInfoResult,
    RepoPermissionsInfoVariables,
    PermissionsInfoUserFields as INode,
    RepoPermissionsInfoRepoNode as IRepo,
} from '../../../graphql-operations'
import { useURLSyncedState } from '../../../hooks'
import { ActionContainer } from '../../../repo/settings/components/ActionContainer'
import { scheduleRepositoryPermissionsSync } from '../../../site-admin/backend'
import { PermissionsSyncJobsTable } from '../../../site-admin/permissions-center/PermissionsSyncJobsTable'
import { Table, type IColumn } from '../../../site-admin/UserManagement/components/Table'
import { PermissionReasonBadgeProps } from '../../settings/permissons'

import { RepoPermissionsInfoQuery } from './backend'

import styles from './RepoSettingsPermissionsPage.module.scss'

type IUser = INode['user']

export interface RepoSettingsPermissionsPageProps extends TelemetryProps, TelemetryV2Props {
    repo: SettingsAreaRepositoryFields
}

/**
 * The repository settings permissions page.
 */
export const RepoSettingsPermissionsPage: FC<RepoSettingsPermissionsPageProps> = ({
    repo,
    telemetryService,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('repo.settings.permissions', 'view')
    }, [telemetryRecorder])

    const [{ query }, setSearchQuery] = useURLSyncedState({ query: '' })
    const debouncedQuery = useDebounce(query, 300)

    const { connection, data, loading, error, refetch, variables, ...paginationProps } = usePageSwitcherPagination<
        RepoPermissionsInfoResult,
        RepoPermissionsInfoVariables,
        INode
    >({
        query: RepoPermissionsInfoQuery,
        variables: {
            repoID: repo.id,
            query: debouncedQuery,
        },
        getConnection: ({ data }) => {
            if (data?.node?.__typename === 'Repository') {
                const node = data.node as IRepo

                return node.permissionsInfo?.users || undefined
            }

            return undefined
        },
    })

    const permissionsInfo = data?.node?.__typename === 'Repository' ? (data.node as IRepo).permissionsInfo : undefined

    if (!permissionsInfo || !connection) {
        return <LoadingSpinner inline={false} />
    }

    return (
        <>
            <PageTitle title="Repo Permissions" />
            <PageHeader
                path={[{ text: 'Repo Permissions' }]}
                headingElement="h2"
                className="mb-3"
                description={
                    <>
                        Learn more about <Link to="/help/admin/permissions/syncing">permission syncing</Link>.
                    </>
                }
            />
            <Container className="repo-settings-permissions-page">
                {!repo.isPrivate ? (
                    <Alert className="mb-0" variant="info">
                        Access to this repository is <strong>not restricted</strong>, all Sourcegraph users have access.
                    </Alert>
                ) : !permissionsInfo ? (
                    <Alert className="mb-0" variant="info">
                        This repository is queued to sync permissions, only site admins will have access to it until
                        syncing is finished.
                    </Alert>
                ) : permissionsInfo.unrestricted ? (
                    <Alert className="mb-0" variant="info">
                        This repository has been explicitly flagged as unrestricted, all Sourcegraph users have access.
                    </Alert>
                ) : (
                    <div>
                        <table className="table">
                            <tbody>
                                <tr>
                                    <th>
                                        Last complete sync{' '}
                                        <Tooltip content="Syncs repository permissions from the code host. All users that have access to the repository on the code host will have access on Sourcegraph as well.">
                                            <Icon aria-label="more-info" svgPath={mdiInformationOutline} />
                                        </Tooltip>
                                    </th>
                                    <td>
                                        {permissionsInfo.syncedAt ? (
                                            <Timestamp date={permissionsInfo.syncedAt} />
                                        ) : (
                                            'Never'
                                        )}
                                    </td>
                                    <td className="text-muted">Updated by repository-centric permission sync.</td>
                                </tr>
                                <tr>
                                    <th>
                                        Last partial sync{' '}
                                        <Tooltip content="Syncs user permissions from the code host. If a user-centric sync returns this repository as accessible, it is noted here as partial sync. Partial syncs do not show in the list of permission sync jobs below.">
                                            <Icon aria-label="more-info" svgPath={mdiInformationOutline} />
                                        </Tooltip>
                                    </th>
                                    <td>
                                        {permissionsInfo.updatedAt === null ? (
                                            'Never'
                                        ) : (
                                            <Timestamp date={permissionsInfo.updatedAt} />
                                        )}
                                    </td>
                                    <td className="text-muted">Partial update done by user-centric permission sync.</td>
                                </tr>
                            </tbody>
                        </table>
                        <ScheduleRepositoryPermissionsSyncActionContainer repo={repo} />
                    </div>
                )}
            </Container>
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Permissions Sync Jobs' }]}
                description={
                    <>
                        List of permissions sync jobs. A permission sync job fetches the newest permissions for the
                        given repository.
                    </>
                }
                className="my-3 pt-3"
            />
            <Container className="mb-3">
                <PermissionsSyncJobsTable
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    minimal={true}
                    repoID={repo.id}
                />
            </Container>
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Users' }]}
                description="List of users who have access to the repository."
                className="my-3 pt-3"
            />
            <Container className="mb-3">
                <div className="d-flex mb-3">
                    <Input
                        type="search"
                        placeholder="Search users..."
                        name="query"
                        value={query}
                        onChange={event => setSearchQuery({ query: event.currentTarget.value })}
                        autoComplete="off"
                        autoCorrect="off"
                        autoCapitalize="off"
                        spellCheck={false}
                        aria-label="Search users..."
                        variant="regular"
                    />
                    <div className="flex-1" />
                </div>
                <Table<INode> columns={TableColumns} getRowId={node => node.id} data={connection.nodes} />
                <PageSwitcher {...paginationProps} totalCount={null} className="mt-4" />
            </Container>
        </>
    )
}

const TableColumns: IColumn<INode>[] = [
    {
        key: 'user',
        header: 'User',
        render: ({ user }: INode) => user && <RenderUsernameAndEmail key={user.id} user={user} />,
    },
    {
        key: 'reason',
        header: 'Reason',
        render: ({ reason }: INode) => <Badge {...(PermissionReasonBadgeProps[reason] || {})}>{reason}</Badge>,
    },
    {
        key: 'updatedAt',
        header: 'Updated At',
        render: ({ updatedAt }: INode) => (
            <div className={styles.updatedAtCell}>{updatedAt ? <Timestamp date={updatedAt} /> : '-'}</div>
        ),
    },
]

export const RenderUsernameAndEmail: FC<{ user: IUser }> = ({ user }) => {
    const [isOpen, setIsOpen] = useState<boolean>(false)
    const handleOpenChange = useCallback((event: PopoverOpenEvent): void => {
        setIsOpen(event.isOpen)
    }, [])

    if (!user) {
        return null
    }

    return (
        <div className="d-flex p-2 align-items-center">
            <UserAvatar inline={true} user={user} />
            <Link to={`/users/${user.username}`} className="text-truncate ml-2">
                @{user.username}
            </Link>
            <Popover isOpen={isOpen} onOpenChange={handleOpenChange}>
                <PopoverTrigger as={Button} className="ml-1 border-0 p-1" variant="secondary" outline={true}>
                    <Icon aria-label="Show details" svgPath={mdiChevronDown} />
                </PopoverTrigger>
                <PopoverContent position={Position.bottom} focusLocked={false}>
                    <div className="p-2">
                        <Text className="mb-0">{user.displayName}</Text>
                        <Text className="mb-0">{user.email}</Text>
                    </div>
                </PopoverContent>
            </Popover>
        </div>
    )
}

interface ScheduleRepositoryPermissionsSyncActionContainerProps {
    repo: SettingsAreaRepositoryFields
}

class ScheduleRepositoryPermissionsSyncActionContainer extends React.PureComponent<ScheduleRepositoryPermissionsSyncActionContainerProps> {
    public render(): JSX.Element | null {
        return (
            <ActionContainer
                title="Manually schedule a permissions sync"
                description={
                    <div>Site admins are able to manually schedule a permissions sync for this repository.</div>
                }
                buttonLabel="Schedule now"
                flashText="Added to queue"
                run={this.scheduleRepositoryPermissions}
            />
        )
    }

    private scheduleRepositoryPermissions = async (): Promise<void> => {
        await scheduleRepositoryPermissionsSync({ repository: this.props.repo.id })
    }
}
