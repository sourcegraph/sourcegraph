import React, { useState, useCallback } from 'react'

import { mdiDotsHorizontal, mdiContentCopy, mdiFileDocument } from '@mdi/js'
import classNames from 'classnames'
import copy from 'copy-to-clipboard'
import { capitalize } from 'lodash'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { pluralize } from '@sourcegraph/common'
import { useSettings } from '@sourcegraph/shared/src/settings/settings'
import { Button, ButtonGroup, ErrorAlert, Link, Icon, Code, screenReaderAnnounce, Tooltip } from '@sourcegraph/wildcard'

import { type GitCommitFields, RepositoryType } from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'
import { CommitMessageWithLinks } from '../commit/CommitMessageWithLinks'
import { DiffModeSelector } from '../commit/DiffModeSelector'
import type { DiffMode } from '../commit/RepositoryCommitPage'
import { Linkified } from '../linkifiy/Linkified'
import { getCanonicalURL, getRefType, isPerforceChangelistMappingEnabled, isPerforceDepotSource } from '../utils'

import { GitCommitNodeByline } from './GitCommitNodeByline'

import styles from './GitCommitNode.module.scss'

const TRUNCATED_COMMIT_MESSAGE_LENGTH = 240
export interface GitCommitNodeProps {
    node: GitCommitFields

    /** An optional additional CSS class name to apply to this element. */
    className?: string

    /** Display in a single line (more compactly). */
    compact?: boolean

    /** Display in a single line, with less spacing between elements and no SHA. */
    extraCompact?: boolean

    /** Expand the commit message body. */
    expandCommitMessageBody?: boolean

    /** Hide the button to expand the commit message body. */
    hideExpandCommitMessageBody?: boolean

    /** Show the full 40-character SHA and parents on their own row. */
    showSHAAndParentsRow?: boolean

    /**
     * Show the absolute timestamp and move relative time to tooltip.
     * If not explicitly set, the user's preference from settings
     * (history.preferAbsoluteTimestamps) is used.
     */
    preferAbsoluteTimestamps?: boolean

    /** Fragment to show at the end to the right of the SHA. */
    afterElement?: React.ReactNode

    /** Determine the git diff visualization UI */
    diffMode?: DiffMode

    /** Handler for change the diff mode */
    onHandleDiffMode?: (mode: DiffMode) => void

    /** An optional additional css class name to apply this to commit node message subject */
    messageSubjectClassName?: string

    /**
     * Element that should wrap the commit data. Only use 'li' when rendering the component in a list
     *
     * Note: This is primarily required for support when using this component in `FilteredConnection`
     * Tracking issue to migrate away from this component: https://github.com/sourcegraph/sourcegraph/issues/23157
     * */
    wrapperElement?: 'div' | 'li'
}

/** Displays a Git commit. */
export const GitCommitNode: React.FunctionComponent<React.PropsWithChildren<GitCommitNodeProps>> = ({
    node,
    afterElement,
    className,
    compact,
    extraCompact,
    expandCommitMessageBody,
    hideExpandCommitMessageBody,
    messageSubjectClassName,
    showSHAAndParentsRow,
    preferAbsoluteTimestamps,
    diffMode,
    onHandleDiffMode,
    wrapperElement: WrapperElement = 'div',
}) => {
    const settings = useSettings()

    const [showCommitMessageBody, setShowCommitMessageBody] = useState<boolean>(false)
    const [truncateCommitMessage, setTruncateCommitMessage] = useState<boolean>(true)
    const [flashCopiedToClipboardMessage, setFlashCopiedToClipboardMessage] = useState<boolean>(false)

    const sourceType = node.perforceChangelist ? RepositoryType.PERFORCE_DEPOT : RepositoryType.GIT_REPOSITORY
    const isPerforceDepot = isPerforceDepotSource(sourceType)
    const abbreviatedRefID = node.perforceChangelist?.cid ?? node.abbreviatedOID
    const refID = node.perforceChangelist?.cid ?? node.oid
    const canonicalURL = getCanonicalURL(sourceType, node)
    // Fall back to user preference if not explicitly set
    preferAbsoluteTimestamps = preferAbsoluteTimestamps ?? Boolean(settings?.['history.preferAbsoluteTimestamps'])

    const toggleShowCommitMessageBody = useCallback((): void => {
        eventLogger.log('CommitBodyToggled')
        setShowCommitMessageBody(!showCommitMessageBody)
    }, [showCommitMessageBody])

    const copyToClipboard = useCallback(
        (oid: string): void => {
            eventLogger.log(isPerforceDepot ? 'ChangelistIDCopiedToClipboard' : 'CommitSHACopiedToClipboard')
            copy(oid)
            setFlashCopiedToClipboardMessage(true)
            screenReaderAnnounce('Copied!')

            setTimeout(() => {
                setFlashCopiedToClipboardMessage(false)
            }, 1500)
        },
        [isPerforceDepot]
    )

    if (extraCompact) {
        // Implied by extraCompact
        compact = true
    }

    const messageElement = (
        <div
            className={classNames(
                !extraCompact && 'flex-grow-1',
                styles.message,
                compact && styles.messageSmall,
                extraCompact && styles.messageExtraSmall
            )}
            data-testid="git-commit-node-message"
        >
            <span className={classNames('mr-2', styles.messageSubject)}>
                <CommitMessageWithLinks
                    to={canonicalURL}
                    className={classNames(messageSubjectClassName, styles.messageLink)}
                    message={node.subject}
                    externalURLs={node.externalURLs}
                />
            </span>

            {node.body && !hideExpandCommitMessageBody && !expandCommitMessageBody && (
                <Button
                    className={styles.messageToggle}
                    onClick={toggleShowCommitMessageBody}
                    variant="secondary"
                    size="sm"
                    aria-label={showCommitMessageBody ? 'Hide commit message body' : 'Show commit message body'}
                >
                    <Icon aria-hidden={true} svgPath={mdiDotsHorizontal} />
                </Button>
            )}
            {compact && (
                <small className={classNames('text-muted', styles.messageTimestamp)}>
                    <Timestamp
                        noAbout={true}
                        preferAbsolute={preferAbsoluteTimestamps}
                        date={node.committer ? node.committer.date : node.author.date}
                    />
                </small>
            )}
        </div>
    )

    const commitMessage = node.body ?? ''
    const truncationNeeded = commitMessage.length > TRUNCATED_COMMIT_MESSAGE_LENGTH
    const truncatedCommitMessage =
        truncateCommitMessage && truncationNeeded
            ? `${commitMessage.slice(0, TRUNCATED_COMMIT_MESSAGE_LENGTH)}...`
            : commitMessage

    const commitMessageBody = (
        <div className="w-100">
            <pre className={styles.messageBody}>
                <Linkified input={truncatedCommitMessage} externalURLs={node.externalURLs} />
                {truncationNeeded && (
                    <Button
                        variant="link"
                        size="sm"
                        display="inline"
                        onClick={() => setTruncateCommitMessage(!truncateCommitMessage)}
                    >
                        {truncateCommitMessage ? 'see more' : 'see less'}
                    </Button>
                )}
            </pre>
        </div>
    )

    const bylineElement = (
        <GitCommitNodeByline
            className={classNames(styles.byline, 'd-flex text-muted')}
            avatarClassName={compact ? undefined : styles.signatureUserAvatar}
            author={node.author}
            committer={node.committer}
            // TODO compact needs to be always a boolean
            compact={Boolean(compact)}
            messageElement={messageElement}
            commitMessageBody={commitMessageBody}
            preferAbsoluteTimestamps={preferAbsoluteTimestamps}
            isPerforceDepot={isPerforceDepot}
        />
    )

    // Handling commits as git-commits is the default behaviour.
    const refType = getRefType(sourceType)
    const copyMessage = isPerforceDepot ? 'Copy changelist ID' : 'Copy full SHA'

    const shaDataElement = showSHAAndParentsRow && (
        <div className={classNames('w-100', styles.shaAndParents)}>
            <div className="d-flex mb-1">
                <span className={styles.shaAndParentsLabel}>{capitalize(refType)}:</span>
                <Code className={styles.shaAndParentsSha}>
                    {refID}{' '}
                    <Tooltip content={flashCopiedToClipboardMessage ? 'Copied!' : copyMessage}>
                        <Button
                            variant="icon"
                            className={styles.shaAndParentsCopy}
                            onClick={() => copyToClipboard(refID)}
                            aria-label={copyMessage}
                        >
                            <Icon aria-hidden={true} svgPath={mdiContentCopy} />
                        </Button>
                    </Tooltip>
                </Code>
            </div>
            <div className="align-items-center d-flex">
                {node.parents.length > 0 ? (
                    <>
                        <span className={styles.shaAndParentsLabel}>
                            {node.parents.length === 1
                                ? 'Parent'
                                : `${node.parents.length} ${pluralize('parent', node.parents.length)}`}
                            :
                        </span>{' '}
                        {node.parents.map(parent => (
                            <div className="d-flex" key={parent.oid}>
                                <Link
                                    className={styles.shaAndParentsParent}
                                    to={parent.perforceChangelist?.canonicalURL ?? parent.url}
                                >
                                    <Code>{parent.perforceChangelist?.cid ?? parent.oid}</Code>
                                </Link>
                                <Tooltip content={flashCopiedToClipboardMessage ? 'Copied!' : copyMessage}>
                                    <Button
                                        variant="icon"
                                        className={styles.shaAndParentsCopy}
                                        onClick={() => copyToClipboard(parent.perforceChangelist?.cid ?? parent.oid)}
                                        aria-label={copyMessage}
                                    >
                                        <Icon aria-hidden={true} svgPath={mdiContentCopy} />
                                    </Button>
                                </Tooltip>
                            </div>
                        ))}
                    </>
                ) : (
                    `(root ${refType})`
                )}
            </div>
        </div>
    )

    const diffModeSelector = (): JSX.Element | null => {
        if (diffMode && onHandleDiffMode) {
            return <DiffModeSelector onHandleDiffMode={onHandleDiffMode} diffMode={diffMode} small={true} />
        }

        return null
    }

    if (!node.tree) {
        return <ErrorAlert error={new Error('missing information about tree')} />
    }

    const treeCanonicalURL =
        isPerforceChangelistMappingEnabled() && isPerforceDepot
            ? node.tree.canonicalURL.replace(node.oid, refID)
            : node.tree.canonicalURL

    const viewFilesCommitElement = node.tree && (
        <div className="d-flex justify-content-between align-items-start">
            <Tooltip content="Browse files in the repository at this point in history">
                <Button
                    className="align-center d-inline-flex"
                    to={treeCanonicalURL}
                    variant="secondary"
                    outline={true}
                    size="sm"
                    as={Link}
                >
                    <Icon className="mr-1" aria-hidden={true} svgPath={mdiFileDocument} />
                    Browse files at @{abbreviatedRefID}
                </Button>
            </Tooltip>
            {diffModeSelector()}
        </div>
    )

    const oidElement = (
        <Code className={styles.oid} data-testid="git-commit-node-oid">
            {abbreviatedRefID}
        </Code>
    )

    return (
        <WrapperElement
            key={node.id}
            className={classNames(
                styles.gitCommitNode,
                compact && styles.gitCommitNodeCompact,
                extraCompact && styles.gitCommitNodeExtraCompact,
                className
            )}
        >
            <>
                {!compact ? (
                    <>
                        <div className="w-100 d-flex justify-content-between align-items-start">
                            <div className={styles.signature}>{bylineElement}</div>
                            <div className={styles.actions}>
                                {!showSHAAndParentsRow && (
                                    <div>
                                        <ButtonGroup className="mr-2">
                                            <Tooltip content={`View this ${refType}`}>
                                                <Button to={canonicalURL} variant="secondary" as={Link} size="sm">
                                                    <strong>{oidElement}</strong>
                                                </Button>
                                            </Tooltip>
                                            <Tooltip content={flashCopiedToClipboardMessage ? 'Copied!' : copyMessage}>
                                                <Button
                                                    onClick={() => copyToClipboard(refID)}
                                                    variant="secondary"
                                                    size="sm"
                                                    aria-label={copyMessage}
                                                >
                                                    <Icon
                                                        className="small"
                                                        aria-hidden={true}
                                                        svgPath={mdiContentCopy}
                                                    />
                                                </Button>
                                            </Tooltip>
                                        </ButtonGroup>
                                        {node.tree && (
                                            <Tooltip content={`View files at this ${refType}`}>
                                                <Button
                                                    aria-label="View files"
                                                    to={treeCanonicalURL}
                                                    variant="secondary"
                                                    size="sm"
                                                    as={Link}
                                                >
                                                    <Icon
                                                        className="mr-1"
                                                        aria-hidden={true}
                                                        svgPath={mdiFileDocument}
                                                    />
                                                </Button>
                                            </Tooltip>
                                        )}
                                    </div>
                                )}
                                {shaDataElement}
                            </div>
                        </div>
                        <div>{showSHAAndParentsRow ? viewFilesCommitElement : shaDataElement}</div>
                    </>
                ) : (
                    <div>
                        <div className={styles.innerWrapper}>
                            {bylineElement}
                            {messageElement}
                            {!extraCompact && <Link to={canonicalURL}>{oidElement}</Link>}
                            {afterElement}
                        </div>
                    </div>
                )}
            </>
        </WrapperElement>
    )
}
