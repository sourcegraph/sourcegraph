import React, { useCallback, useMemo, useState } from 'react'

import { mdiStar, mdiStarOutline, mdiLock, mdiDotsHorizontal, mdiWeb, mdiDomain } from '@mdi/js'
import classNames from 'classnames'
import { Observable } from 'rxjs'
import { catchError, switchMap, tap } from 'rxjs/operators'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Menu,
    MenuButton,
    MenuDivider,
    MenuItem,
    Button,
    useEventObservable,
    MenuList,
    MenuHeader,
    Position,
    Icon,
} from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../../auth'
import { NotebookFields } from '../../graphql-operations'
import { OrgAvatar } from '../../org/OrgAvatar'
import {
    deleteNotebook as _deleteNotebook,
    createNotebookStar as _createNotebookStar,
    deleteNotebookStar as _deleteNotebookStar,
} from '../backend'

import { DeleteNotebookModal } from './DeleteNotebookModal'
import { ShareOption } from './NotebookShareOptionsDropdown'
import { ShareNotebookModal } from './ShareNotebookModal'

import styles from './NotebookPageHeaderActions.module.scss'

export interface NotebookPageHeaderActionsProps extends TelemetryProps {
    isSourcegraphDotCom: boolean
    authenticatedUser: AuthenticatedUser | null
    namespace: NotebookFields['namespace']
    notebookId: string
    viewerCanManage: boolean
    isPublic: boolean
    onUpdateVisibility: (isPublic: boolean, namespace: string) => void
    deleteNotebook: typeof _deleteNotebook
    starsCount: number
    viewerHasStarred: boolean
    createNotebookStar: typeof _createNotebookStar
    deleteNotebookStar: typeof _deleteNotebookStar
}

export const NotebookPageHeaderActions: React.FunctionComponent<
    React.PropsWithChildren<NotebookPageHeaderActionsProps>
> = ({
    isSourcegraphDotCom,
    authenticatedUser,
    notebookId,
    viewerCanManage,
    isPublic,
    namespace,
    onUpdateVisibility,
    deleteNotebook,
    starsCount,
    viewerHasStarred,
    createNotebookStar,
    deleteNotebookStar,
    telemetryService,
}) => {
    const [showShareModal, setShowShareModal] = useState(false)
    const toggleShareModal = useCallback(() => setShowShareModal(show => !show), [setShowShareModal])
    const [selectedShareOption, setSelectedShareOption] = useState<ShareOption | null>(
        namespace
            ? {
                  namespaceType: namespace.__typename,
                  namespaceId: namespace.id,
                  namespaceName: namespace.namespaceName,
                  isPublic,
              }
            : null
    )

    const shareIcon = useMemo(() => {
        if (!selectedShareOption) {
            return <></>
        }
        if (selectedShareOption.namespaceType === 'User') {
            return selectedShareOption.isPublic ? (
                <Icon
                    className="mr-1"
                    svgPath={isSourcegraphDotCom ? mdiWeb : mdiDomain}
                    inline={false}
                    aria-hidden={true}
                    height="1.15rem"
                    width="1.15rem"
                />
            ) : (
                <Icon
                    className="mr-1"
                    svgPath={mdiLock}
                    inline={false}
                    aria-hidden={true}
                    height="1.15rem"
                    width="1.15rem"
                />
            )
        }
        return (
            <OrgAvatar org={selectedShareOption.namespaceName} className="d-inline-flex mr-1" size="sm" light={true} />
        )
    }, [selectedShareOption, isSourcegraphDotCom])

    return (
        <div className="d-flex align-items-center">
            <NotebookStarsButton
                disabled={authenticatedUser === null}
                notebookId={notebookId}
                starsCount={starsCount}
                viewerHasStarred={viewerHasStarred}
                createNotebookStar={createNotebookStar}
                deleteNotebookStar={deleteNotebookStar}
                telemetryService={telemetryService}
            />
            {authenticatedUser && viewerCanManage && namespace && selectedShareOption && (
                <>
                    <Button
                        variant="primary"
                        onClick={toggleShareModal}
                        className="d-flex align-items-center"
                        data-testid="share-notebook-button"
                    >
                        {shareIcon} Share
                    </Button>
                    <ShareNotebookModal
                        isOpen={showShareModal}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        toggleModal={toggleShareModal}
                        telemetryService={telemetryService}
                        authenticatedUser={authenticatedUser}
                        selectedShareOption={selectedShareOption}
                        setSelectedShareOption={setSelectedShareOption}
                        onUpdateVisibility={onUpdateVisibility}
                    />
                </>
            )}
            {viewerCanManage && (
                <NotebookSettingsDropdown
                    notebookId={notebookId}
                    deleteNotebook={deleteNotebook}
                    telemetryService={telemetryService}
                />
            )}
        </div>
    )
}

interface NotebookSettingsDropdownProps extends TelemetryProps {
    notebookId: string
    deleteNotebook: typeof _deleteNotebook
}

const NotebookSettingsDropdown: React.FunctionComponent<React.PropsWithChildren<NotebookSettingsDropdownProps>> = ({
    notebookId,
    deleteNotebook,
    telemetryService,
}) => {
    const [showDeleteModal, setShowDeleteModal] = useState(false)
    const toggleDeleteModal = useCallback(() => setShowDeleteModal(show => !show), [setShowDeleteModal])

    return (
        <>
            <Menu>
                <MenuButton outline={true} aria-label="Notebook action">
                    <Icon svgPath={mdiDotsHorizontal} inline={false} aria-hidden={true} />
                </MenuButton>
                <MenuList position={Position.bottomEnd}>
                    <MenuHeader>Settings</MenuHeader>
                    <MenuDivider />
                    <MenuItem
                        as={Button}
                        variant="danger"
                        className={styles.dangerMenuItem}
                        onSelect={() => setShowDeleteModal(true)}
                    >
                        Delete notebook
                    </MenuItem>
                </MenuList>
            </Menu>
            <DeleteNotebookModal
                notebookId={notebookId}
                isOpen={showDeleteModal}
                toggleDeleteModal={toggleDeleteModal}
                deleteNotebook={deleteNotebook}
                telemetryService={telemetryService}
            />
        </>
    )
}

interface NotebookStarsButtonProps extends TelemetryProps {
    notebookId: string
    disabled: boolean
    starsCount: number
    viewerHasStarred: boolean
    createNotebookStar: typeof _createNotebookStar
    deleteNotebookStar: typeof _deleteNotebookStar
}

const NotebookStarsButton: React.FunctionComponent<React.PropsWithChildren<NotebookStarsButtonProps>> = ({
    notebookId,
    disabled,
    starsCount: initialStarsCount,
    viewerHasStarred: initialViewerHasStarred,
    createNotebookStar,
    deleteNotebookStar,
    telemetryService,
}) => {
    const [starsCount, setStarsCount] = useState(initialStarsCount)
    const [viewerHasStarred, setViewerHasStarred] = useState(initialViewerHasStarred)

    const [onStarToggle] = useEventObservable(
        useCallback(
            (viewerHasStarred: Observable<boolean>) =>
                viewerHasStarred.pipe(
                    // Immediately update the UI.
                    tap(viewerHasStarred => {
                        telemetryService.log(`SearchNotebook${viewerHasStarred ? 'Remove' : 'Add'}Star`)
                        if (viewerHasStarred) {
                            setStarsCount(starsCount => starsCount - 1)
                            setViewerHasStarred(() => false)
                        } else {
                            setStarsCount(starsCount => starsCount + 1)
                            setViewerHasStarred(() => true)
                        }
                    }),
                    switchMap(viewerHasStarred =>
                        viewerHasStarred ? deleteNotebookStar(notebookId) : createNotebookStar(notebookId)
                    ),
                    catchError(() => {
                        setStarsCount(initialStarsCount)
                        setViewerHasStarred(initialViewerHasStarred)
                        return []
                    })
                ),
            [
                deleteNotebookStar,
                notebookId,
                createNotebookStar,
                initialStarsCount,
                initialViewerHasStarred,
                telemetryService,
            ]
        )
    )

    return (
        <Button
            className="d-flex align-items-center pl-0"
            outline={true}
            disabled={disabled}
            onClick={() => onStarToggle(viewerHasStarred)}
            aria-label={viewerHasStarred ? 'Unstar notebook' : 'Star notebook'}
        >
            {viewerHasStarred ? (
                <Icon
                    aria-hidden={true}
                    className={classNames(styles.notebookStarIcon, styles.notebookStarIconActive)}
                    svgPath={mdiStar}
                />
            ) : (
                <Icon aria-hidden={true} className={styles.notebookStarIcon} svgPath={mdiStarOutline} />
            )}
            <span className="ml-1">{starsCount}</span>
        </Button>
    )
}
