import React, { useCallback, useMemo, useState } from 'react'

import {
    mdiAccountReactivate,
    mdiArchive,
    mdiBadgeAccount,
    mdiChevronDown,
    mdiClipboardMinus,
    mdiClipboardPlus,
    mdiClose,
    mdiDelete,
    mdiLock,
    mdiLockOpen,
    mdiLockReset,
    mdiLogoutVariant,
    mdiSecurity,
} from '@mdi/js'
import classNames from 'classnames'
import { endOfDay, formatDistanceToNowStrict, startOfDay } from 'date-fns'

import { logger } from '@sourcegraph/common'
import { useQuery } from '@sourcegraph/http-client'
import {
    Alert,
    Badge,
    Button,
    ErrorAlert,
    H2,
    Icon,
    Link,
    LoadingSpinner,
    Popover,
    PopoverContent,
    type PopoverOpenEvent,
    PopoverTrigger,
    Position,
    Text,
    Tooltip,
    useDebounce,
} from '@sourcegraph/wildcard'

import {
    SiteUserOrderBy,
    type UsersManagementUsersListResult,
    type UsersManagementUsersListVariables,
} from '../../../graphql-operations'
import { useURLSyncedState } from '../../../hooks'
import { USERS_MANAGEMENT_USERS_LIST } from '../queries'

import { Table } from './Table'
import { useUserListActions } from './useUserListActions'

import styles from '../index.module.scss'

export type SiteUser = UsersManagementUsersListResult['site']['users']['nodes'][0]

const LIMIT = 25

interface UsersListProps {
    onActionEnd?: () => void
    renderAssignmentModal: (
        onCancel: () => void,
        onSuccess: (user: { username: string }) => void,
        users: SiteUser[]
    ) => React.ReactNode
}

interface DateRangeQueryParameter {
    range?: [Date, Date]
    isNegated?: boolean
}

function parseDateRangeQueryParameter(rawValue: string): DateRangeQueryParameter {
    if (!rawValue) {
        return {}
    }
    const { range, isNegated } = JSON.parse(rawValue) as DateRangeQueryParameter

    if (Array.isArray(range) && range.length === 2) {
        const [start, end] = range
        return {
            isNegated,
            range: [new Date(start), new Date(end)],
        }
    }
    return {
        isNegated,
    }
}

function stringifyDateRangeQueryParameter(dateRange: DateRangeQueryParameter): string {
    return JSON.stringify(dateRange)
}

const DEFAULT_FILTERS = {
    searchText: '',
    offset: '0',
    limit: LIMIT.toString(),
    orderBy: SiteUserOrderBy.EVENTS_COUNT,
    descending: 'false',
    isAdmin: '',
    maxEventsCount: '',
    lastActiveAt: '',
    createdAt: '',
    deletedAt: '{"isNegated":true}',
}

const dateRangeQueryParameterToVariable = (
    dateRange?: DateRangeQueryParameter
): { empty?: boolean | null; gte?: string | null; lte?: string | null; not?: boolean | null } | null => {
    if (!dateRange) {
        return null
    }
    if (dateRange.range) {
        const {
            range: [start, end],
            isNegated,
        } = dateRange
        return {
            not: isNegated === undefined ? null : isNegated,
            gte: startOfDay(start).toISOString(),
            lte: endOfDay(end).toISOString(),
        }
    }

    return {
        empty: typeof dateRange.isNegated === 'boolean' ? dateRange.isNegated : null,
    }
}

export const UsersList: React.FunctionComponent<UsersListProps> = ({ onActionEnd, renderAssignmentModal }) => {
    const [filters, setFilters] = useURLSyncedState(DEFAULT_FILTERS)
    const debouncedSearchText = useDebounce(filters.searchText, 300)

    const lastActiveAt = useMemo(() => parseDateRangeQueryParameter(filters.lastActiveAt), [filters.lastActiveAt])
    const deletedAt = useMemo(() => parseDateRangeQueryParameter(filters.deletedAt), [filters.deletedAt])
    const createdAt = useMemo(() => parseDateRangeQueryParameter(filters.createdAt), [filters.createdAt])

    const offset = Number(filters.offset)
    const limit = Number(filters.limit)
    const descending = filters.descending ? (JSON.parse(filters.descending) as boolean) : null
    const siteAdmin = filters.isAdmin ? (JSON.parse(filters.isAdmin) as boolean) : null

    const { data, previousData, refetch, variables, error, loading } = useQuery<
        UsersManagementUsersListResult,
        UsersManagementUsersListVariables
    >(USERS_MANAGEMENT_USERS_LIST, {
        variables: {
            limit,
            offset,
            query: debouncedSearchText || null,
            lastActiveAt: dateRangeQueryParameterToVariable(lastActiveAt),
            deletedAt: dateRangeQueryParameterToVariable(deletedAt),
            createdAt: dateRangeQueryParameterToVariable(createdAt),
            eventsCount: !filters.maxEventsCount
                ? null
                : {
                      lte: Number(filters.maxEventsCount),
                  },
            orderBy: filters.orderBy,
            descending,
            siteAdmin,
        },
    })

    const handleActionEnd = useCallback(
        (error?: any) => {
            if (!error) {
                // reload data
                refetch(variables).catch(logger.error)
                onActionEnd?.()
            }
        },
        [onActionEnd, refetch, variables]
    )

    const [roleAssignmentModal, setRoleAssignmentModal] = useState<React.ReactNode>(null)

    const openRoleAssignmentModal = (selectedUsers: SiteUser[]): void => {
        setRoleAssignmentModal(renderAssignmentModal(closeRoleAssignmentModal, onRoleAssignmentSuccess, selectedUsers))
    }
    const closeRoleAssignmentModal = (): void => setRoleAssignmentModal(null)

    const {
        notification,
        handleForceSignOutUsers,
        handleDeleteUsers,
        handleDeleteUsersForever,
        handlePromoteToSiteAdmin,
        handleUnlockUser,
        handleRecoverUsers,
        handleRevokeSiteAdmin,
        handleResetUserPassword,
        handleDismissNotification,
        handleDisplayNotification,
    } = useUserListActions(handleActionEnd)

    const setFiltersWithOffset = useCallback(
        (newFilters: Partial<typeof filters>) => {
            setFilters({ ...newFilters, ...newFilters, offset: DEFAULT_FILTERS.offset })
        },
        [setFilters]
    )
    const onLimitChange = useCallback((newLimit: number) => setFilters({ limit: newLimit.toString() }), [setFilters])

    const users = (data || previousData)?.site.users
    const onPreviousPage = useCallback(
        () => setFilters({ offset: Math.max(0, offset - limit).toString() }),
        [limit, offset, setFilters]
    )
    const onNextPage = useCallback(() => {
        const newOffset = offset + limit
        if (users?.totalCount && users?.totalCount >= newOffset) {
            setFilters({ offset: newOffset.toString() })
        }
    }, [limit, offset, setFilters, users?.totalCount])

    const onRoleAssignmentSuccess = (user: { username: string }): void => {
        handleDisplayNotification(
            <Text as="span">
                Role(s) successfully updated for user <strong>{user.username}</strong>.
            </Text>
        )
        closeRoleAssignmentModal()
    }

    return (
        <div className="position-relative">
            <H2 className="my-4 ml-2">Users</H2>
            {roleAssignmentModal}
            {notification && (
                <Alert
                    className="mt-2 d-flex justify-content-between align-items-center"
                    variant={notification.isError ? 'danger' : 'success'}
                >
                    {notification.text}
                    <Button variant="link" onClick={handleDismissNotification}>
                        <Icon aria-label="Close notification" svgPath={mdiClose} />
                    </Button>
                </Alert>
            )}
            {loading && (
                <div
                    className={classNames(
                        'position-absolute w-100 h-100 d-flex justify-content-center align-items-center',
                        styles.loadingSpinnerContainer
                    )}
                >
                    <LoadingSpinner />
                </div>
            )}
            {error && <ErrorAlert error={error} />}
            {users && (
                <>
                    <Table
                        selectable={true}
                        sortBy={{ key: filters.orderBy, descending: !!descending }}
                        data={users.nodes}
                        pagination={{
                            onPrevious: onPreviousPage,
                            onNext: onNextPage,
                            onLimitChange,
                            formatLabel: (start: number, end: number, total: number) =>
                                `Showing ${start}-${end} of ${total} users`,
                            limitOptions: [
                                {
                                    label: 'Show 25 per page',
                                    value: LIMIT,
                                },
                                {
                                    label: 'Show 50 per page',
                                    value: LIMIT * 2,
                                },
                                {
                                    label: 'Show 100 per page',
                                    value: LIMIT * 4,
                                },
                            ],
                            total: users.totalCount,
                            offset,
                            limit,
                        }}
                        onSortByChange={value => {
                            setFiltersWithOffset({
                                orderBy: value.key as SiteUserOrderBy,
                                descending: value.descending.toString(),
                            })
                        }}
                        onClearAllFiltersClick={() => setFiltersWithOffset(DEFAULT_FILTERS)}
                        getRowId={({ id }) => id}
                        actions={[
                            {
                                key: 'force-sign-out',
                                label: 'Force sign-out',
                                icon: mdiLogoutVariant,
                                onClick: handleForceSignOutUsers,
                                bulk: true,
                                condition: users => users.some(user => !user.deletedAt),
                            },
                            {
                                key: 'reset-password',
                                label: 'Reset password',
                                icon: mdiLockReset,
                                onClick: handleResetUserPassword,
                                condition: ([user]) => !user?.deletedAt,
                            },
                            {
                                key: 'revoke-site-admin',
                                label: 'Revoke site admin',
                                icon: mdiClipboardMinus,
                                onClick: handleRevokeSiteAdmin,
                                condition: ([user]) => user?.siteAdmin && !user?.deletedAt,
                            },
                            {
                                key: 'promote-to-site-admin',
                                label: 'Promote to site admin',
                                icon: mdiClipboardPlus,
                                onClick: handlePromoteToSiteAdmin,
                                condition: ([user]) => !user?.siteAdmin && !user?.deletedAt,
                            },
                            {
                                key: 'manage-roles',
                                label: 'Manage roles',
                                icon: mdiBadgeAccount,
                                onClick: openRoleAssignmentModal,
                                condition: ([user]) => !user?.deletedAt,
                                bulk: true,
                            },
                            {
                                key: 'unlock-user',
                                label: 'Unlock user',
                                icon: mdiLockOpen,
                                onClick: handleUnlockUser,
                                condition: ([user]) => !user?.deletedAt && user?.locked,
                            },
                            {
                                key: 'view-permissions',
                                label: 'View permissions',
                                icon: mdiSecurity,
                                href: ([user]) => `/users/${user.username}/settings/permissions`,
                                target: '_blank',
                                condition: ([user]) => !!user,
                            },
                            {
                                key: 'delete',
                                label: 'Delete',
                                icon: mdiArchive,
                                iconColor: 'danger',
                                onClick: handleDeleteUsers,
                                bulk: true,
                                condition: users => users.some(user => !user.deletedAt),
                            },
                            {
                                key: 'delete-forever',
                                label: 'Delete forever',
                                icon: mdiDelete,
                                iconColor: 'danger',
                                labelColor: 'danger',
                                onClick: handleDeleteUsersForever,
                                bulk: true,
                            },
                            {
                                key: 'recover',
                                label: 'Recover',
                                icon: mdiAccountReactivate,
                                onClick: handleRecoverUsers,
                                bulk: true,
                                condition: users => users.some(user => user.deletedAt),
                            },
                        ]}
                        columns={[
                            {
                                key: SiteUserOrderBy.USERNAME,
                                accessor: 'username',
                                header: 'User',
                                sortable: true,
                                render: RenderUsernameAndEmail,
                                filter: {
                                    type: 'text',
                                    placeholder: 'Username, email, display name',
                                    onChange: value => {
                                        setFiltersWithOffset({ searchText: value })
                                    },
                                    value: filters.searchText,
                                },
                            },
                            {
                                key: SiteUserOrderBy.SITE_ADMIN,
                                accessor: item => (item.siteAdmin ? 'Yes' : 'No'),
                                header: { label: 'Is Admin', align: 'left' },
                                sortable: true,
                                align: 'center',
                                filter: {
                                    type: 'select',
                                    options: [
                                        { value: 'null', label: 'All' },
                                        { value: 'true', label: 'Yes' },
                                        { value: 'false', label: 'No' },
                                    ],
                                    onChange: value => {
                                        setFiltersWithOffset({ isAdmin: value })
                                    },
                                    value: filters.isAdmin,
                                },
                            },
                            {
                                key: SiteUserOrderBy.EVENTS_COUNT,
                                accessor: 'eventsCount',
                                header: {
                                    label: 'Events',
                                    align: 'left',
                                    tooltip:
                                        '"Events" count is cached and updated every 12 hours. It is based on event logs table and available for the last 93 days.',
                                },
                                sortable: true,
                                align: 'right',
                                filter: {
                                    type: 'select',
                                    options: [
                                        { value: '', label: 'All' },
                                        { value: '0', label: '= 0' },
                                        { value: '10', label: '< 10' },
                                        { value: '100', label: '< 100' },
                                        { value: '1000', label: '< 1000' },
                                    ],
                                    onChange: value => {
                                        setFiltersWithOffset({ maxEventsCount: value })
                                    },
                                    value: filters.maxEventsCount,
                                },
                            },
                            {
                                key: SiteUserOrderBy.LAST_ACTIVE_AT,
                                accessor: item =>
                                    item.lastActiveAt
                                        ? formatDistanceToNowStrict(new Date(item.lastActiveAt), { addSuffix: true })
                                        : '',
                                header: {
                                    label: 'Last Active',
                                    align: 'left',
                                    tooltip:
                                        '"Last Active" is cached and updated every 12 hours. It is based on event logs table and available for the last 93 days.',
                                },
                                sortable: true,
                                align: 'right',
                                filter: {
                                    type: 'date-range',
                                    placeholder: 'Select',
                                    onChange: (range, isNegated) => {
                                        setFiltersWithOffset({
                                            lastActiveAt: stringifyDateRangeQueryParameter({ range, isNegated }),
                                        })
                                    },
                                    value: lastActiveAt.range,
                                    negation: {
                                        label: 'Find inactive users',
                                        value: lastActiveAt.isNegated,
                                        message:
                                            'When checked will show users who have NOT been active in the selected range/all time.',
                                    },
                                },
                            },
                            {
                                key: SiteUserOrderBy.CREATED_AT,
                                accessor: item =>
                                    formatDistanceToNowStrict(new Date(item.createdAt), { addSuffix: true }),
                                header: { label: 'Created', align: 'left' },
                                sortable: true,
                                align: 'right',
                                filter: {
                                    type: 'date-range',
                                    placeholder: 'Select',
                                    onChange: (range, isNegated) => {
                                        setFiltersWithOffset({
                                            createdAt: stringifyDateRangeQueryParameter({ range, isNegated }),
                                        })
                                    },
                                    value: createdAt.range,
                                    isRequired: true,
                                    negation: {
                                        label: 'Find created NOT in range',
                                        value: createdAt.isNegated,
                                        message:
                                            'When checked will show users who have NOT been created in the selected range.',
                                    },
                                },
                            },
                            {
                                key: SiteUserOrderBy.DELETED_AT,
                                accessor: item =>
                                    item.deletedAt
                                        ? formatDistanceToNowStrict(new Date(item.deletedAt), { addSuffix: true })
                                        : '',
                                header: { label: 'Deleted', align: 'left' },
                                sortable: true,
                                align: 'right',
                                filter: {
                                    type: 'date-range',
                                    placeholder: 'Select',
                                    onChange: (range, isNegated) => {
                                        setFiltersWithOffset({
                                            deletedAt: stringifyDateRangeQueryParameter({ range, isNegated }),
                                        })
                                    },
                                    value: deletedAt.range,
                                    negation: {
                                        label: 'Find deleted NOT in range/all time',
                                        value: deletedAt.isNegated,
                                        message:
                                            'When checked will show users who have NOT been deleted in the selected range/all time.',
                                    },
                                },
                            },
                        ]}
                        note={
                            <Text as="span">
                                Note: Events is the count of <Link to="/help/admin/pricing">all billable events</Link>{' '}
                                performed by a user.
                            </Text>
                        }
                    />
                </>
            )}
        </div>
    )
}

function RenderUsernameAndEmail({
    username,
    email,
    displayName,
    deletedAt,
    locked,
    scimControlled,
}: SiteUser): JSX.Element {
    const [isOpen, setIsOpen] = useState<boolean>(false)
    const handleOpenChange = useCallback((event: PopoverOpenEvent): void => {
        setIsOpen(event.isOpen)
    }, [])

    return (
        <div
            className={classNames('d-flex p-2 align-items-center justify-content-between', styles.usernameColumn, {
                [styles.visibleActionsOnHover]: !isOpen,
            })}
        >
            <div className="d-flex align-items-center text-truncate">
                {!deletedAt ? (
                    <>
                        {locked && (
                            <Tooltip content="This user is locked and cannot sign in.">
                                <Icon aria-label="Account locked" svgPath={mdiLock} />
                            </Tooltip>
                        )}{' '}
                        <Link to={`/users/${username}`} className="text-truncate">
                            @{username}
                        </Link>
                    </>
                ) : (
                    <Text className="mb-0 text-truncate">@{username}</Text>
                )}
                <Popover isOpen={isOpen} onOpenChange={handleOpenChange}>
                    <PopoverTrigger
                        as={Button}
                        className={classNames('ml-1 border-0 p-1', styles.actionsButton)}
                        variant="secondary"
                        outline={true}
                    >
                        <Icon aria-label="Show details" svgPath={mdiChevronDown} />
                    </PopoverTrigger>
                    <PopoverContent position={Position.bottom} focusLocked={false}>
                        <div className="p-2">
                            <Text className="mb-0">{displayName}</Text>
                            <Text className="mb-0">{email}</Text>
                        </div>
                    </PopoverContent>
                </Popover>
            </div>
            {scimControlled && (
                <Tooltip
                    content={
                        <Text>
                            This user is{' '}
                            <Link to="/help/admin/scim" target="_blank" rel="noopener">
                                SCIM
                            </Link>
                            -controlled—an external system controls some of its attributes.
                        </Text>
                    }
                >
                    <Badge variant="secondary" className="mr-1">
                        SCIM
                    </Badge>
                </Tooltip>
            )}
        </div>
    )
}

type ActionHandler = (users: SiteUser[]) => void

export interface UseUserListActionReturnType {
    notification: { text: React.ReactNode; isError?: boolean } | undefined
    handleForceSignOutUsers: ActionHandler
    handleDeleteUsers: ActionHandler
    handleDeleteUsersForever: ActionHandler
    handlePromoteToSiteAdmin: ActionHandler
    handleUnlockUser: ActionHandler
    handleRecoverUsers: ActionHandler
    handleRevokeSiteAdmin: ActionHandler
    handleResetUserPassword: ActionHandler
    handleDismissNotification: () => void
    handleDisplayNotification: (text: React.ReactNode) => void
}

export const getUsernames = (users: SiteUser[]): string => users.map(user => user.username).join(', ')
