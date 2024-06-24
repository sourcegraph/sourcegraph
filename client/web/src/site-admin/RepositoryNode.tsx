import React, { useState } from 'react'

import {
    mdiBrain,
    mdiClose,
    mdiCog,
    mdiConnection,
    mdiDatabaseRefresh,
    mdiDotsVertical,
    mdiInformation,
    mdiListStatus,
    mdiRefresh,
    mdiSearchWeb,
    mdiSecurity,
} from '@mdi/js'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import { RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import {
    Alert,
    Badge,
    Button,
    H4,
    Icon,
    LinkOrSpan,
    LoadingSpinner,
    Menu,
    MenuButton,
    MenuDivider,
    MenuItem,
    MenuList,
    Popover,
    PopoverContent,
    PopoverTail,
    PopoverTrigger,
    Position,
} from '@sourcegraph/wildcard'

import type {
    RecloneRepositoryResult,
    RecloneRepositoryVariables,
    SiteAdminRepositoryFields,
    UpdateMirrorRepositoryResult,
    UpdateMirrorRepositoryVariables,
} from '../graphql-operations'
import { LogsPageTabs } from '../repo/constants'

import { RECLONE_REPOSITORY_MUTATION, UPDATE_MIRROR_REPOSITORY } from './backend'
import { ExternalRepositoryIcon } from './components/ExternalRepositoryIcon'
import { RepoMirrorInfo } from './components/RepoMirrorInfo'

import styles from './RepositoryNode.module.scss'

const RepositoryStatusBadge: React.FunctionComponent<{ status: string }> = ({ status }) => (
    <Badge className={classNames(styles[status as keyof typeof styles], 'py-0 px-1 text-uppercase font-weight-normal')}>
        {status}
    </Badge>
)

const parseRepositoryStatus = (repo: SiteAdminRepositoryFields): string => {
    let status = 'queued'
    if (repo.mirrorInfo.cloned && !repo.mirrorInfo.lastError) {
        status = 'cloned'
    } else if (repo.mirrorInfo.cloneInProgress) {
        status = 'cloning'
    } else if (repo.mirrorInfo.lastError) {
        status = 'failed'
    }
    return status
}

const repoClonedAndHealthy = (repo: SiteAdminRepositoryFields): boolean =>
    repo.mirrorInfo.cloned && !repo.mirrorInfo.lastError && !repo.mirrorInfo.cloneInProgress

interface RepositoryNodeProps {
    node: SiteAdminRepositoryFields
    refetchAllRepos: () => Promise<void>
}

export const RepositoryNode: React.FunctionComponent<React.PropsWithChildren<RepositoryNodeProps>> = ({
    node,
    refetchAllRepos,
}) => {
    const [isPopoverOpen, setIsPopoverOpen] = useState(false)
    const navigate = useNavigate()
    const [recloneRepository] = useMutation<RecloneRepositoryResult, RecloneRepositoryVariables>(
        RECLONE_REPOSITORY_MUTATION,
        {
            variables: { repo: node.id },
        }
    )
    const [updateRepo] = useMutation<UpdateMirrorRepositoryResult, UpdateMirrorRepositoryVariables>(
        UPDATE_MIRROR_REPOSITORY,
        { variables: { repository: node.id } }
    )

    const recloneAndFetch = async (): Promise<void> => {
        await recloneRepository()
        await refetchAllRepos()
    }

    return (
        <li
            className="repository-node list-group-item px-0 py-2"
            data-test-repository={node.name}
            data-test-cloned={node.mirrorInfo.cloned}
        >
            <div className="d-flex flex-row">
                {/* that col-md-8 is a little too large for the two buttons in a row */}
                <div className="d-flex flex-column justify-content-between flex-md-row col px-0">
                    <div className="d-flex col align-items-center px-0">
                        <ExternalRepositoryIcon externalRepo={node.externalRepository} className={styles.repoIcon} />
                        <RepoLink repoName={node.name} to={node.url} />
                    </div>

                    <div className="d-flex align-items-center col justify-content-start px-0 px-md-2 mt-2 mt-md-0">
                        <RepositoryStatusBadge status={parseRepositoryStatus(node)} />
                        {node.mirrorInfo.cloneInProgress && <LoadingSpinner className="ml-2" />}
                        {node.mirrorInfo.lastError && (
                            <Popover isOpen={isPopoverOpen} onOpenChange={event => setIsPopoverOpen(event.isOpen)}>
                                <PopoverTrigger
                                    as={Button}
                                    className="p-0 ml-2"
                                    variant="icon"
                                    size="sm"
                                    aria-label="See errors"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiInformation} />
                                </PopoverTrigger>

                                <PopoverContent position={Position.left} className={styles.errorContent}>
                                    <div className="d-flex">
                                        <H4 className="m-2">
                                            <RepositoryStatusBadge status={parseRepositoryStatus(node)} />
                                            <ExternalRepositoryIcon
                                                externalRepo={node.externalRepository}
                                                className="mx-2"
                                            />
                                            <RepoLink repoName={node.name} to={null} />
                                        </H4>

                                        <Button
                                            aria-label="Dismiss error"
                                            variant="icon"
                                            className="ml-auto mr-2"
                                            onClick={() => setIsPopoverOpen(false)}
                                        >
                                            <Icon aria-hidden={true} svgPath={mdiClose} />
                                        </Button>
                                    </div>

                                    <MenuDivider />

                                    <Alert variant="warning" className={classNames('m-2', styles.alertOverflow)}>
                                        <H4>Error syncing repository:</H4>
                                        {node.mirrorInfo.lastError}
                                    </Alert>
                                </PopoverContent>
                                <PopoverTail size="sm" />
                            </Popover>
                        )}
                    </div>
                </div>

                <div className="d-flex align-items-start justify-content-start col-1 px-0 mt-0">
                    {!window.location.pathname.includes('/setup') && (
                        <Menu>
                            <MenuButton outline={true} aria-label="Repository action">
                                <Icon svgPath={mdiDotsVertical} inline={false} aria-hidden={true} />
                            </MenuButton>
                            <MenuList position={Position.bottomEnd}>
                                <MenuItem
                                    as={Button}
                                    disabled={!repoClonedAndHealthy(node)}
                                    onSelect={() => updateRepo()}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiRefresh} className="mr-1" />
                                    Sync
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    disabled={!repoClonedAndHealthy(node)}
                                    onSelect={() => recloneAndFetch()}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiDatabaseRefresh} className="mr-1" />
                                    Reclone
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    onSelect={() =>
                                        navigate(`/${node.name}/-/settings/logs?activeTab=${LogsPageTabs.SYNCLOGS}`)
                                    }
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiListStatus} className="mr-1" />
                                    Last sync log
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    disabled={!repoClonedAndHealthy(node)}
                                    onSelect={() => navigate(`/${node.name}/-/code-graph`)}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiBrain} className="mr-1" />
                                    Code graph data
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    disabled={!repoClonedAndHealthy(node)}
                                    onSelect={() => navigate(`/${node.name}/-/settings/index`)}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiSearchWeb} className="mr-1" />
                                    Search indexing
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    disabled={!repoClonedAndHealthy(node)}
                                    onSelect={() => navigate(`/${node.name}/-/settings/permissions`)}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiSecurity} className="mr-1" />
                                    Permissions
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    onSelect={() => navigate(`/site-admin/external-services?repoID=${node.id}`)}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiConnection} className="mr-1" />
                                    Code host connections
                                </MenuItem>
                                <MenuItem
                                    as={Button}
                                    onSelect={() => navigate(`/${node.name}/-/settings`)}
                                    className="p-2"
                                >
                                    <Icon aria-hidden={true} svgPath={mdiCog} className="mr-1" />
                                    Settings
                                </MenuItem>
                            </MenuList>
                        </Menu>
                    )}
                </div>
            </div>

            <div className="w-100">
                <RepoMirrorInfo mirrorInfo={node.mirrorInfo} />
            </div>

            {node.mirrorInfo.isCorrupted && (
                <div className={styles.alertWrapper}>
                    <Alert variant="danger">
                        Repository is corrupt.{' '}
                        <LinkOrSpan to={`/${node.name}/-/settings/mirror`}>More details</LinkOrSpan>
                    </Alert>
                </div>
            )}
        </li>
    )
}
