import * as H from 'history'
import {
    IExternalChangeset,
    IChangesetPlan,
    ChangesetReviewState,
    IFileDiff,
    IPreviewFileDiff,
    ChangesetState,
    ChangesetCheckState,
} from '../../../../../../shared/src/graphql/schema'
import React, { useState, useEffect } from 'react'
import {
    changesetReviewStateColors,
    changesetReviewStateIcons,
    changesetStageLabels,
    changesetStatusColorClasses,
    changesetStateIcons,
    changesetCheckStateIcons,
    changesetCheckStateColors,
    changesetCheckStateTooltips,
} from './presentation'
import { Link } from '../../../../../../shared/src/components/Link'
import { LinkOrSpan } from '../../../../../../shared/src/components/LinkOrSpan'
import { ThemeProps } from '../../../../../../shared/src/theme'
import { Collapsible } from '../../../../components/Collapsible'
import { DiffStat } from '../../../../components/diff/DiffStat'
import { FileDiffNode } from '../../../../components/diff/FileDiffNode'
import { Markdown } from '../../../../../../shared/src/components/Markdown'
import { renderMarkdown } from '../../../../../../shared/src/util/markdown'
import { publishChangeset as _publishChangeset, syncChangeset } from '../backend'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { Subject } from 'rxjs'
import ErrorIcon from 'mdi-react/ErrorIcon'
import { asError } from '../../../../../../shared/src/util/errors'
import { ChangesetLabel } from './ChangesetLabel'
import classNames from 'classnames'
import { DraftBadge } from '../../DraftBadge'
import SyncIcon from 'mdi-react/SyncIcon'
import { parseISO, formatDistanceToNow } from 'date-fns'

export interface ChangesetNodeProps extends ThemeProps {
    node: IExternalChangeset | IChangesetPlan
    campaignUpdates?: Subject<void>
    history: H.History
    location: H.Location
    /** Shows the publish button for ChangesetPlans */
    enablePublishing: boolean
}

export const ChangesetNode: React.FunctionComponent<ChangesetNodeProps> = ({
    node,
    campaignUpdates,
    isLightTheme,
    history,
    location,
    enablePublishing,
}) => {
    const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null)
    const [isPublishing, setIsPublishing] = useState<boolean>()
    const publicationEnqueued = node.__typename === 'ChangesetPlan' && node.publicationEnqueued
    useEffect(() => {
        setIsPublishing(publicationEnqueued)
    }, [publicationEnqueued])
    const nodeUpdatedAt = node.__typename === 'ExternalChangeset' && node.updatedAt
    const lastUpdatedAtChanged = lastUpdatedAt && nodeUpdatedAt !== lastUpdatedAt
    useEffect(() => {
        if (lastUpdatedAtChanged && nodeUpdatedAt) {
            if (campaignUpdates) {
                campaignUpdates.next()
            }
            setLastUpdatedAt(null)
        }
    }, [campaignUpdates, lastUpdatedAtChanged, nodeUpdatedAt])
    const [publishError, setPublishError] = useState<Error>()
    const publishChangeset: React.MouseEventHandler = async () => {
        try {
            setPublishError(undefined)
            setIsPublishing(true)
            await _publishChangeset(node.id)
            if (campaignUpdates) {
                campaignUpdates.next()
            }
        } catch (error) {
            setPublishError(asError(error))
        } finally {
            setIsPublishing(false)
        }
    }
    const enqueueChangeset: React.MouseEventHandler = async () => {
        if (node.__typename === 'ExternalChangeset') {
            setLastUpdatedAt(node.updatedAt)
            await syncChangeset(node.id)
        }
    }
    const fileDiffs = node.diff?.fileDiffs
    const fileDiffNodes: (IFileDiff | IPreviewFileDiff)[] | undefined = fileDiffs ? fileDiffs.nodes : undefined
    const ChangesetStateIcon =
        node.__typename === 'ExternalChangeset'
            ? changesetStateIcons[node.state]
            : changesetStateIcons[ChangesetState.OPEN]
    const ReviewStateIcon =
        node.__typename === 'ExternalChangeset'
            ? changesetReviewStateIcons[node.reviewState]
            : changesetReviewStateIcons[ChangesetReviewState.PENDING]
    const ChangesetCheckStateIcon =
        node.__typename === 'ExternalChangeset' && node.checkState
            ? changesetCheckStateIcons[node.checkState]
            : changesetCheckStateIcons[ChangesetCheckState.PENDING]
    const changesetState = node.__typename === 'ExternalChangeset' ? node.state : ChangesetState.OPEN

    const stateIcon = (
        <ChangesetStateIcon
            className={classNames(
                'mr-1',
                node.__typename === 'ExternalChangeset'
                    ? `text-${changesetStatusColorClasses[changesetState]}`
                    : 'text-muted'
            )}
            data-tooltip={changesetStageLabels[changesetState]}
        />
    )

    const UpdateLoaderIcon =
        node.__typename === 'ExternalChangeset' && node.updatedAt !== lastUpdatedAt ? SyncIcon : LoadingSpinner

    const changesetNodeRow = (
        <div className="d-flex align-items-center m-1 ml-2">
            <div className="changeset-node__content flex-fill">
                <h3 className="m-0">
                    <div className="d-flex flex-column">
                        {node.__typename === 'ExternalChangeset' && (
                            <div>
                                {stateIcon}
                                <LinkOrSpan
                                    /* Deleted changesets most likely don't exist on the codehost anymore and would return 404 pages */
                                    to={
                                        node.externalURL && node.state !== ChangesetState.DELETED
                                            ? node.externalURL.url
                                            : undefined
                                    }
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    {node.title}
                                </LinkOrSpan>
                                {node.__typename === 'ExternalChangeset' && node.checkState && (
                                    <ChangesetCheckStateIcon
                                        className={classNames(
                                            'ml-1 changeset-node__check-state',
                                            changesetCheckStateColors[node.checkState]
                                        )}
                                        data-tooltip={changesetCheckStateTooltips[node.checkState]}
                                    />
                                )}
                                {node.labels.length > 0 && (
                                    <span className="ml-2">
                                        {node.labels.map(label => (
                                            <ChangesetLabel label={label} key={label.text} />
                                        ))}
                                    </span>
                                )}
                            </div>
                        )}
                        <div>
                            {node.__typename === 'ChangesetPlan' && stateIcon}
                            <Link
                                to={node.repository.url}
                                className="text-muted"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                {node.repository.name}
                            </Link>
                            {node.__typename === 'ChangesetPlan' && !isPublishing && <DraftBadge className="ml-2" />}
                        </div>
                    </div>
                </h3>
                {node.__typename === 'ExternalChangeset' && (
                    <Markdown
                        className="text-truncate"
                        dangerousInnerHTML={renderMarkdown(node.body, { plainText: true })}
                    />
                )}
            </div>
            <div className="flex-shrink-0 flex-grow-0 ml-1 d-flex flex-column align-items-end">
                {node.__typename === 'ExternalChangeset' && (
                    <span
                        data-tooltip={`Last synced ${formatDistanceToNow(parseISO(node.updatedAt))} ago. ${
                            node.updatedAt === lastUpdatedAt ? 'Currently refreshing' : 'Click to prioritize refresh'
                        }`}
                    >
                        <UpdateLoaderIcon
                            className={classNames('icon-inline', node.updatedAt !== lastUpdatedAt && 'cursor-pointer')}
                            onClick={enqueueChangeset}
                        />
                    </span>
                )}
                {fileDiffs && <DiffStat {...fileDiffs.diffStat} expandedCounts={true} />}
                {node.__typename === 'ExternalChangeset' && (
                    <ReviewStateIcon
                        className={
                            node.state === ChangesetState.DELETED
                                ? 'text-muted'
                                : `text-${changesetReviewStateColors[node.reviewState]}`
                        }
                        data-tooltip={changesetStageLabels[node.reviewState]}
                    />
                )}
            </div>
            {enablePublishing && node.__typename === 'ChangesetPlan' && (
                <>
                    {publishError && <ErrorIcon data-tooltip={publishError.message} className="ml-2" />}
                    <button
                        type="button"
                        className="flex-shrink-0 flex-grow-0 btn btn-sm btn-secondary ml-2"
                        disabled={isPublishing}
                        onClick={publishChangeset}
                    >
                        {isPublishing && <LoadingSpinner className="mr-1 icon-inline" />}{' '}
                        {isPublishing ? 'Publishing' : 'Publish'}
                    </button>
                </>
            )}
        </div>
    )
    return (
        <li className="list-group-item e2e-changeset-node">
            {fileDiffNodes ? (
                <Collapsible
                    titleClassName="changeset-node__content flex-fill"
                    title={changesetNodeRow}
                    wholeTitleClickable={false}
                >
                    {fileDiffNodes.map(fileDiffNode => (
                        <FileDiffNode
                            isLightTheme={isLightTheme}
                            node={fileDiffNode}
                            lineNumbers={true}
                            location={location}
                            history={history}
                            persistLines={node.__typename === 'ExternalChangeset'}
                            key={fileDiffNode.internalID}
                            className="mb-1"
                        />
                    ))}
                </Collapsible>
            ) : (
                <div className="changeset-node__content changeset-node__content--no-collapse flex-fill">
                    {changesetNodeRow}
                </div>
            )}
        </li>
    )
}
