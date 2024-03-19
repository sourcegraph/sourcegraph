import { useCallback, useEffect } from 'react'

import classNames from 'classnames'
import SourceCommitIcon from 'mdi-react/SourceCommitIcon'
import { BehaviorSubject } from 'rxjs'

import { UserAvatar } from '@sourcegraph/shared/src/components/UserAvatar'
import {
    createRectangle,
    Icon,
    Link,
    Popover,
    PopoverContent,
    type PopoverOpenEvent,
    PopoverTrigger,
    Position,
    useObservable,
} from '@sourcegraph/wildcard'

import { eventLogger } from '../../tracking/eventLogger'
import { replaceRevisionInURL } from '../../util/url'
import type { BlameHunk, BlameHunkData } from '../blame/useBlameHunks'
import { CommitMessageWithLinks } from '../commit/CommitMessageWithLinks'

import styles from './BlameDecoration.module.scss'

const currentPopoverId = new BehaviorSubject<string | null>(null)
let closeTimeoutId: NodeJS.Timeout | null = null
const resetCloseTimeout = (): void => {
    if (closeTimeoutId) {
        clearTimeout(closeTimeoutId)
        closeTimeoutId = null
    }
}
let openTimeoutId: NodeJS.Timeout | null = null
const resetOpenTimeout = (): void => {
    if (openTimeoutId) {
        clearTimeout(openTimeoutId)
        openTimeoutId = null
    }
}
const resetAllTimeouts = (): void => {
    resetOpenTimeout()
    resetCloseTimeout()
}

const usePopover = ({
    id,
    timeout,
    onOpen,
    onClose,
}: {
    id: string
    timeout: number
    onOpen?: () => void
    onClose?: () => void
}): {
    isOpen: boolean
    open: () => void
    close: () => void
    openWithTimeout: () => void
    closeWithTimeout: () => void
} => {
    const popoverId = useObservable(currentPopoverId)

    const isOpen = popoverId === id
    useEffect(() => {
        if (isOpen) {
            onOpen?.()
        }

        return () => {
            if (isOpen) {
                onClose?.()
            }
        }
    }, [isOpen, onOpen, onClose])

    const open = useCallback(() => {
        resetCloseTimeout()
        currentPopoverId.next(id)
    }, [id])

    const close = useCallback(() => {
        if (currentPopoverId.getValue() === id) {
            currentPopoverId.next(null)
        }
    }, [id])

    const openWithTimeout = useCallback(() => {
        if (currentPopoverId.getValue() === null) {
            open()
            return
        }
        resetOpenTimeout()
        openTimeoutId = setTimeout(open, timeout)
    }, [open, timeout])

    const closeWithTimeout = useCallback(() => {
        resetCloseTimeout()
        closeTimeoutId = setTimeout(close, timeout)
    }, [close, timeout])

    return { isOpen, open, close, openWithTimeout, closeWithTimeout }
}

interface BlameDecorationProps {
    line: number // 1-based line number
    blameHunk?: BlameHunk
    externalURLs?: BlameHunkData['externalURLs']
    onSelect?: (line: number) => void
    onDeselect?: (line: number) => void
}

export const BlameDecoration: React.FunctionComponent<BlameDecorationProps> = ({
    line,
    blameHunk,
    onSelect,
    onDeselect,
    externalURLs,
}) => {
    const hunkStartLine = blameHunk?.startLine ?? line
    const id = hunkStartLine?.toString() || ''
    const onOpen = useCallback(() => {
        onSelect?.(hunkStartLine)
        eventLogger.log('GitBlamePopupViewed')
    }, [onSelect, hunkStartLine])
    const onClose = useCallback(() => onDeselect?.(hunkStartLine), [onDeselect, hunkStartLine])
    const { isOpen, open, close, closeWithTimeout, openWithTimeout } = usePopover({
        id,
        timeout: 50,
        onOpen,
        onClose,
    })

    const onPopoverOpenChange = useCallback(
        (event: PopoverOpenEvent) => (event.isOpen ? close() : open()),
        [close, open]
    )

    if (!blameHunk) {
        return null
    }
    const displayInfo = blameHunk.displayInfo

    return (
        <div className={classNames(styles.blame)}>
            <Popover isOpen={isOpen} onOpenChange={onPopoverOpenChange} key={id}>
                <PopoverTrigger
                    as={Link}
                    to={blameHunk.displayInfo.linkURL}
                    target="_blank"
                    rel="noreferrer noopener"
                    className={classNames(styles.popoverTrigger, 'px-2')}
                    onFocus={open}
                    onBlur={close}
                    onMouseEnter={openWithTimeout}
                    onMouseLeave={closeWithTimeout}
                >
                    <span className={styles.date}>{displayInfo.dateString}</span>
                    {blameHunk.author.person ? (
                        <>
                            <span className={styles.author}>
                                <UserAvatar
                                    inline={true}
                                    className={styles.avatar}
                                    style={{ top: 1 }}
                                    user={
                                        blameHunk.author.person.user
                                            ? blameHunk.author.person.user
                                            : blameHunk.author.person
                                    }
                                    size={16}
                                />
                            </span>
                        </>
                    ) : (
                        <span className={styles.author}>{`${displayInfo.username}${displayInfo.displayName}`}</span>
                    )}
                    <span className={styles.content}>
                        {blameHunk.author.person ? (
                            <>
                                {`${displayInfo.displayName}${displayInfo.username}`.split(' ')[0]}
                                {' • '}
                            </>
                        ) : null}
                        {displayInfo.message}
                    </span>
                </PopoverTrigger>

                <PopoverContent
                    constraintPadding={createRectangle(150, 0, 0, 0)}
                    position={Position.topStart}
                    focusLocked={false}
                    returnTargetFocus={false}
                    onMouseEnter={resetAllTimeouts}
                    onMouseLeave={close}
                    className={styles.popoverContent}
                >
                    <div className="py-1">
                        <div className={classNames(styles.head, 'px-3 my-2')}>
                            <span className={styles.author}>{blameHunk.displayInfo.displayName}</span>{' '}
                            {blameHunk.displayInfo.timestampString}
                        </div>
                        <hr className={classNames(styles.separator, 'm-0')} />
                        <div className={classNames('d-flex align-items-center', styles.block, styles.body)}>
                            <Icon
                                aria-hidden={true}
                                as={SourceCommitIcon}
                                className={classNames('mr-2 flex-shrink-0', styles.icon)}
                            />
                            <div>
                                <CommitMessageWithLinks
                                    message={blameHunk.message}
                                    to={blameHunk.displayInfo.linkURL}
                                    className={styles.link}
                                    onClick={logCommitClick}
                                    externalURLs={externalURLs}
                                />
                            </div>
                        </div>
                        {blameHunk.commit.parents.length > 0 && (
                            <>
                                <hr className={classNames(styles.separator, 'm-0')} />
                                <div className={classNames('px-3', styles.block)}>
                                    <Link
                                        to={replaceRevisionInURL(window.location.href, blameHunk.commit.parents[0].oid)}
                                        className={styles.footerLink}
                                    >
                                        View blame prior to this change
                                    </Link>
                                </div>
                            </>
                        )}
                    </div>
                </PopoverContent>
            </Popover>
        </div>
    )
}

const logCommitClick = (): void => {
    eventLogger.log('GitBlamePopupClicked', { target: 'commit' }, { target: 'commit' })
}
