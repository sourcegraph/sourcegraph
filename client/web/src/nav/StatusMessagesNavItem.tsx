import React, { useState, useMemo } from 'react'

import {
    mdiInformation,
    mdiAlert,
    mdiSync,
    mdiCheckboxMarkedCircle,
    mdiDatabaseSyncOutline,
    mdiInformationOutline,
} from '@mdi/js'
import classNames from 'classnames'

import { useQuery } from '@sourcegraph/http-client'
import {
    CloudAlertIconRefresh,
    CloudSyncIconRefresh,
    CloudCheckIconRefresh,
    CloudInfoIconRefresh,
} from '@sourcegraph/shared/src/components/icons'
import {
    Button,
    Link,
    Popover,
    PopoverContent,
    PopoverTail,
    PopoverTrigger,
    Position,
    Icon,
    H4,
    Text,
    Tooltip,
    ErrorAlert,
} from '@sourcegraph/wildcard'

import type { StatusAndRepoCountResult } from '../graphql-operations'

import { STATUS_AND_REPO_COUNT } from './StatusMessagesNavItemQueries'

import styles from './StatusMessagesNavItem.module.scss'

type EntryType = 'progress' | 'warning' | 'success' | 'error' | 'indexing' | 'info'

interface StatusMessageEntryProps {
    title: string
    message: string
    linkTo: string
    linkText: string
    entryType: EntryType
    linkOnClick: (event: React.MouseEvent<HTMLAnchorElement, MouseEvent>) => void
    messageHint?: string
}

function entryIcon(entryType: EntryType): JSX.Element {
    const sharedProps = { height: 14, width: 14, inline: false }

    switch (entryType) {
        case 'error': {
            return (
                <Icon
                    {...sharedProps}
                    className={classNames('text-danger', styles.icon)}
                    svgPath={mdiInformation}
                    aria-label="Error"
                />
            )
        }
        case 'warning': {
            return (
                <Icon
                    {...sharedProps}
                    className={classNames('text-warning', styles.icon)}
                    svgPath={mdiAlert}
                    aria-label="Warning"
                />
            )
        }
        case 'success': {
            return (
                <Icon
                    {...sharedProps}
                    className={classNames('text-success', styles.icon)}
                    svgPath={mdiCheckboxMarkedCircle}
                    aria-label="Success"
                />
            )
        }
        case 'progress': {
            return (
                <Icon
                    {...sharedProps}
                    className={classNames('text-primary', styles.icon)}
                    svgPath={mdiSync}
                    aria-label="In progress"
                />
            )
        }
        case 'indexing': {
            return (
                <Icon
                    {...sharedProps}
                    className={classNames('text-primary', styles.icon)}
                    svgPath={mdiDatabaseSyncOutline}
                    aria-label="Indexing"
                />
            )
        }
        case 'info': {
            return (
                <Icon
                    {...sharedProps}
                    className={classNames('text-primary', styles.icon)}
                    svgPath={mdiInformationOutline}
                    aria-label="Information"
                />
            )
        }
    }
}

const getMessageColor = (entryType: EntryType): string => {
    switch (entryType) {
        case 'error': {
            return styles.messageError
        }
        case 'warning': {
            return styles.messageWarning
        }
        default: {
            return ''
        }
    }
}

const getBorderClassname = (entryType: EntryType): string => {
    switch (entryType) {
        case 'error': {
            return styles.entryBorderError
        }
        case 'warning': {
            return styles.entryBorderWarning
        }
        case 'success': {
            return styles.entryBorderSuccess
        }
        case 'progress': {
            return styles.entryBorderProgress
        }
        case 'indexing': {
            return styles.entryBorderProgress
        }
        case 'info': {
            return styles.entryBorderInfo
        }
        default: {
            return ''
        }
    }
}

const StatusMessagesNavItemEntry: React.FunctionComponent<React.PropsWithChildren<StatusMessageEntryProps>> = props => (
    <div key={props.message} className={styles.entry}>
        <H4 className="d-flex align-items-center mb-0">
            {entryIcon(props.entryType)}
            {props.title ? props.title : 'Your repositories'}
        </H4>
        <div
            className={classNames(
                'status-messages-nav-item__entry-card',
                styles.cardActive,
                getBorderClassname(props.entryType)
            )}
        >
            <Text className={classNames(styles.message, getMessageColor(props.entryType))}>{props.message}</Text>
            {props.messageHint && (
                <>
                    <small className="text-muted d-inline-block mb-1">{props.messageHint}</small>
                    <br />
                </>
            )}
            <Link className="text-primary" to={props.linkTo} onClick={props.linkOnClick}>
                {props.linkText}
            </Link>
        </div>
    </div>
)

const STATUS_MESSAGES_POLL_INTERVAL = 10000

interface Props {
    disablePolling?: boolean
}
/**
 * Displays a status icon in the navbar reflecting the completion of backend
 * tasks such as repository cloning, and exposes a dropdown menu containing
 * more information on these tasks.
 */
export const StatusMessagesNavItem: React.FunctionComponent<React.PropsWithChildren<Props>> = props => {
    const [isOpen, setIsOpen] = useState(false)
    const toggleIsOpen = (): void => setIsOpen(old => !old)

    const { data, error } = useQuery<StatusAndRepoCountResult>(STATUS_AND_REPO_COUNT, {
        fetchPolicy: 'no-cache',
        pollInterval: props.disablePolling !== true ? STATUS_MESSAGES_POLL_INTERVAL : undefined,
    })

    const icon: JSX.Element | null = useMemo(() => {
        if (!data) {
            return null
        }

        let codeHostMessage
        let iconProps
        if (
            data.statusMessages?.some(
                ({ __typename: type }) => type === 'ExternalServiceSyncError' || type === 'SyncError'
            )
        ) {
            codeHostMessage = 'Syncing repositories failed'
            iconProps = { as: CloudAlertIconRefresh }
        } else if (data.statusMessages?.some(({ __typename: type }) => type === 'GitUpdatesDisabled')) {
            codeHostMessage = 'Syncing repositories disabled'
            iconProps = { as: CloudAlertIconRefresh }
        } else if (data.statusMessages?.some(({ __typename: type }) => type === 'CloningProgress')) {
            codeHostMessage = 'Cloning repositories...'
            iconProps = { as: CloudSyncIconRefresh }
        } else if (data.statusMessages?.some(({ __typename: type }) => type === 'IndexingProgress')) {
            codeHostMessage = 'Indexing repositories...'
            iconProps = { as: CloudSyncIconRefresh }
        } else if (data.statusMessages?.some(({ __typename: type }) => type === 'NoRepositoriesDetected')) {
            codeHostMessage = 'No repositories'
            iconProps = { as: CloudInfoIconRefresh }
        } else {
            if (data.repositories.totalCount) {
                codeHostMessage = `${data.repositories.totalCount} repo(s) updated`
                iconProps = { as: CloudCheckIconRefresh }
            } else {
                codeHostMessage = 'No repositories connected'
                iconProps = { as: CloudAlertIconRefresh }
            }
        }

        return (
            <Tooltip content={isOpen ? undefined : codeHostMessage}>
                <Icon
                    {...iconProps}
                    size="md"
                    {...(isOpen ? { 'aria-hidden': true } : { 'aria-label': codeHostMessage })}
                />
            </Tooltip>
        )
    }, [data, isOpen])

    const messages: JSX.Element | null = useMemo(() => {
        if (!data) {
            return null
        }

        // no status messages
        if (data.statusMessages.length === 0) {
            if (data.repositories.totalCount) {
                return (
                    <StatusMessagesNavItemEntry
                        key="up-to-date"
                        title="Repositories up to date"
                        message="Repositories synced from code host and available for search."
                        linkTo="/site-admin/repositories"
                        linkText="View repositories"
                        linkOnClick={toggleIsOpen}
                        entryType="success"
                    />
                )
            } else {
                return (
                    <StatusMessagesNavItemEntry
                        key="no-repositories-connected"
                        title="No Repositories connected"
                        message="Unlock the power of Sourcegraph by connecting repos."
                        linkTo="site-admin/external-services/new"
                        linkText="Connect a repo"
                        linkOnClick={toggleIsOpen}
                        entryType="warning"
                    />
                )
            }
        }

        return (
            <>
                {data.statusMessages.map(status => {
                    if (status.__typename === 'GitUpdatesDisabled') {
                        return (
                            <StatusMessagesNavItemEntry
                                key={status.message}
                                message={status.message}
                                title="Code syncing disabled"
                                messageHint="Remove disableGitAutoUpdates or set it to false in the site configuration"
                                linkTo="/site-admin/configuration"
                                linkText="View site configuration"
                                linkOnClick={toggleIsOpen}
                                entryType="warning"
                            />
                        )
                    }
                    if (status.__typename === 'NoRepositoriesDetected') {
                        return (
                            <StatusMessagesNavItemEntry
                                key="no-repositories"
                                title="No repositories"
                                message="Connect a code host to connect repositories to Sourcegraph."
                                linkTo="/setup"
                                linkText="Setup code hosts"
                                linkOnClick={toggleIsOpen}
                                entryType="info"
                            />
                        )
                    }
                    if (status.__typename === 'CloningProgress') {
                        return (
                            <StatusMessagesNavItemEntry
                                key={status.message}
                                message={status.message}
                                title="Cloning repositories"
                                messageHint="Not all repositories available for search yet."
                                linkTo="/site-admin/repositories"
                                linkText="View repositories"
                                linkOnClick={toggleIsOpen}
                                entryType="progress"
                            />
                        )
                    }
                    if (status.__typename === 'IndexingProgress') {
                        return (
                            <StatusMessagesNavItemEntry
                                key="indexing-progress"
                                message={`Indexing repositories. ${status.indexed} out of ${
                                    status.indexed + status.notIndexed
                                } indexed.`}
                                title="Indexing repositories"
                                messageHint="Indexing repositories speeds up search."
                                linkTo="/site-admin/repositories"
                                linkText="View repositories"
                                linkOnClick={toggleIsOpen}
                                entryType="indexing"
                            />
                        )
                    }
                    if (status.__typename === 'ExternalServiceSyncError') {
                        return (
                            <StatusMessagesNavItemEntry
                                key={status.externalService.id}
                                title="Code host connection"
                                message={`Failed to connect to "${status.externalService.displayName}".`}
                                messageHint="Repositories synced to Sourcegraph may not be up to date."
                                linkTo={`/site-admin/external-services/${status.externalService.id}`}
                                linkText="View code host configuration"
                                linkOnClick={toggleIsOpen}
                                entryType="error"
                            />
                        )
                    }
                    if (status.__typename === 'SyncError') {
                        return (
                            <StatusMessagesNavItemEntry
                                key={status.message}
                                message={status.message}
                                title="Syncing repositories from code hosts"
                                messageHint="Repository contents may not be up to date."
                                linkTo="/site-admin/repositories?status=failed-fetch"
                                linkText="View affected repositories"
                                linkOnClick={toggleIsOpen}
                                entryType="warning"
                            />
                        )
                    }
                    if (status.__typename === 'GitserverDiskThresholdReached') {
                        return (
                            <StatusMessagesNavItemEntry
                                key="disk-threshold-reached"
                                title="Gitserver disk threshold reached"
                                message={status.message}
                                messageHint="Search and cloning may be impacted until disk usage is reduced."
                                linkTo="/site-admin/gitservers"
                                linkText="Manage Gitservers"
                                linkOnClick={toggleIsOpen}
                                entryType="warning"
                            />
                        )
                    }
                    return null
                })}
            </>
        )
    }, [data])

    return (
        <Popover isOpen={isOpen} onOpenChange={event => setIsOpen(event.isOpen)}>
            <PopoverTrigger
                className="nav-link py-0 px-0"
                as={Button}
                variant="link"
                aria-label={isOpen ? 'Hide status messages' : 'Show status messages'}
            >
                {error ? (
                    <Tooltip content="Sorry, we couldn’t fetch notifications!">
                        <Icon
                            aria-label="Sorry, we couldn’t fetch notifications!"
                            as={CloudAlertIconRefresh}
                            size="md"
                        />
                    </Tooltip>
                ) : (
                    icon
                )}
            </PopoverTrigger>

            <PopoverContent position={Position.bottom} className={classNames('p-0', styles.dropdownMenu)}>
                <div className={styles.dropdownMenuContent}>
                    <small className={classNames('d-inline-block text-muted', styles.sync)}>Status</small>
                    {error && (
                        <ErrorAlert className={styles.entry} prefix="Failed to load status messages" error={error} />
                    )}
                    {messages}
                </div>
            </PopoverContent>

            <PopoverTail />
        </Popover>
    )
}
