import { Hoverifier } from '@sourcegraph/codeintellify'
import * as H from 'history'
import React, { useState, useCallback } from 'react'
import ChevronRightIcon from 'mdi-react/ChevronRightIcon'
import ChevronDownIcon from 'mdi-react/ChevronDownIcon'
import { ThemeProps } from '../../../../../shared/src/theme'
import { ExternalChangesetFields } from '../../../graphql-operations'
import { RepoSpec, RevisionSpec, FileSpec, ResolvedRevisionSpec } from '../../../../../shared/src/util/url'
import { HoverMerged } from '../../../../../shared/src/api/client/types/hover'
import { ActionItemAction } from '../../../../../shared/src/actions/ActionItem'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { ChangesetCheckStatusCell } from '../detail/changesets/ChangesetCheckStatusCell'
import { ChangesetReviewStatusCell } from '../detail/changesets/ChangesetReviewStatusCell'
import { DiffStat } from '../../../components/diff/DiffStat'
import { ErrorAlert } from '../../../components/alerts'
import { ChangesetFileDiff } from '../detail/changesets/ChangesetFileDiff'
import { queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs } from '../detail/backend'
import { ChangesetCloseActionClose, ChangesetCloseActionKept } from './ChangesetCloseAction'
import { ExternalChangesetInfoCell } from '../detail/changesets/ExternalChangesetInfoCell'

export interface ExternalChangesetCloseNodeProps extends ThemeProps {
    node: ExternalChangesetFields
    willClose: boolean
    viewerCanAdminister: boolean
    history: H.History
    location: H.Location
    extensionInfo?: {
        hoverifier: Hoverifier<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec, HoverMerged, ActionItemAction>
    } & ExtensionsControllerProps
    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
}

export const ExternalChangesetCloseNode: React.FunctionComponent<ExternalChangesetCloseNodeProps> = ({
    node,
    willClose,
    viewerCanAdminister,
    isLightTheme,
    history,
    location,
    extensionInfo,
    queryExternalChangesetWithFileDiffs,
}) => {
    const [isExpanded, setIsExpanded] = useState(false)
    const toggleIsExpanded = useCallback<React.MouseEventHandler<HTMLButtonElement>>(
        event => {
            event.preventDefault()
            setIsExpanded(!isExpanded)
        },
        [isExpanded]
    )

    return (
        <>
            <button
                type="button"
                className="btn btn-icon test-batches-expand-changeset d-none d-sm-block"
                aria-label={isExpanded ? 'Collapse section' : 'Expand section'}
                onClick={toggleIsExpanded}
            >
                {isExpanded ? (
                    <ChevronDownIcon className="icon-inline" aria-label="Close section" />
                ) : (
                    <ChevronRightIcon className="icon-inline" aria-label="Expand section" />
                )}
            </button>
            {willClose ? (
                <ChangesetCloseActionClose className="external-changeset-close-node__action" />
            ) : (
                <ChangesetCloseActionKept className="external-changeset-close-node__action" />
            )}
            <ExternalChangesetInfoCell
                node={node}
                viewerCanAdminister={viewerCanAdminister}
                className="external-changeset-close-node__information"
            />
            <div className="d-flex d-md-none justify-content-start external-changeset-close-node__statuses">
                {node.checkState && <ChangesetCheckStatusCell checkState={node.checkState} className="mr-3" />}
                {node.reviewState && <ChangesetReviewStatusCell reviewState={node.reviewState} className="mr-3" />}
                {node.diffStat && <DiffStat {...node.diffStat} expandedCounts={true} separateLines={true} />}
            </div>
            <span className="d-none d-md-inline">
                {node.checkState && <ChangesetCheckStatusCell checkState={node.checkState} />}
            </span>
            <span className="d-none d-md-inline">
                {node.reviewState && <ChangesetReviewStatusCell reviewState={node.reviewState} />}
            </span>
            <div className="d-none d-md-flex justify-content-center">
                {node.diffStat && <DiffStat {...node.diffStat} expandedCounts={true} separateLines={true} />}
            </div>
            {/* The button for expanding the information used on xs devices. */}
            <button
                type="button"
                aria-label={isExpanded ? 'Collapse section' : 'Expand section'}
                onClick={toggleIsExpanded}
                className="external-changeset-close-node__show-details btn btn-outline-secondary d-block d-sm-none test-batches-expand-changeset"
            >
                {isExpanded ? (
                    <ChevronDownIcon className="icon-inline" aria-label="Close section" />
                ) : (
                    <ChevronRightIcon className="icon-inline" aria-label="Expand section" />
                )}{' '}
                {isExpanded ? 'Hide' : 'Show'} details
            </button>
            {isExpanded && (
                <div className="external-changeset-close-node__expanded-section p-2">
                    {node.error && <ErrorAlert error={node.error} />}
                    <ChangesetFileDiff
                        changesetID={node.id}
                        isLightTheme={isLightTheme}
                        history={history}
                        location={location}
                        repositoryID={node.repository.id}
                        repositoryName={node.repository.name}
                        extensionInfo={extensionInfo}
                        queryExternalChangesetWithFileDiffs={queryExternalChangesetWithFileDiffs}
                    />
                </div>
            )}
        </>
    )
}
