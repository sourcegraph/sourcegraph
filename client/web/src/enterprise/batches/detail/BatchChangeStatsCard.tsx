import React from 'react'
import ProgressCheckIcon from 'mdi-react/ProgressCheckIcon'
import CheckCircleOutlineIcon from 'mdi-react/CheckCircleOutlineIcon'
import classNames from 'classnames'
import { BatchChangeFields, ChangesetsStatsFields, DiffStatFields } from '../../../graphql-operations'
import { BatchChangeStateBadge } from './BatchChangeStateBadge'
import {
    ChangesetStatusUnpublished,
    ChangesetStatusOpen,
    ChangesetStatusClosed,
    ChangesetStatusMerged,
    ChangesetStatusDraft,
} from './changesets/ChangesetStatusCell'
import { pluralize } from '../../../../../shared/src/util/strings'
import { DiffStat } from '../../../components/diff/DiffStat'

interface BatchChangeStatsCardProps {
    stats: ChangesetsStatsFields
    diff: DiffStatFields
    closedAt: BatchChangeFields['closedAt']
    className?: string
}

export const BatchChangeStatsCard: React.FunctionComponent<BatchChangeStatsCardProps> = ({
    stats,
    diff,
    closedAt,
    className,
}) => {
    const percentComplete =
        stats.total === 0 ? 0 : (((stats.closed + stats.merged + stats.deleted) / stats.total) * 100).toFixed(0)
    const isCompleted = stats.closed + stats.merged + stats.deleted === stats.total
    let BatchChangeStatusIcon = ProgressCheckIcon
    if (isCompleted) {
        BatchChangeStatusIcon = CheckCircleOutlineIcon
    }
    return (
        <div className={classNames(className)}>
            <div className="d-flex flex-wrap align-items-center flex-grow-1">
                <h2 className="m-0">
                    <BatchChangeStateBadge isClosed={!!closedAt} className="batch-change-stats-card__state-badge" />
                </h2>
                <div className="batch-change-stats-card__divider mx-4" />
                <div className="d-flex align-items-center">
                    <h1 className="d-inline mb-0">
                        <BatchChangeStatusIcon
                            className={classNames(
                                'icon-inline mr-2',
                                isCompleted && 'text-success',
                                !isCompleted && 'text-muted'
                            )}
                        />
                    </h1>{' '}
                    <span className="lead text-nowrap batch-change-stats-card__completeness">
                        {percentComplete}% complete
                    </span>
                </div>
                <div className="batch-change-stats-card__divider d-none d-md-block mx-4" />
                <DiffStat
                    {...diff}
                    expandedCounts={true}
                    separateLines={true}
                    className="batch-change-stats-card__diff-stat"
                />
                <div className="d-flex flex-wrap justify-content-end flex-grow-1">
                    <BatchChangeStatsTotalAction count={stats.total} />
                    <ChangesetStatusUnpublished
                        label={<span className="text-muted">{stats.unpublished} unpublished</span>}
                        className="d-flex flex-grow-0 px-2 text-truncate batch-change-stats-card__stat"
                    />
                    <ChangesetStatusDraft
                        label={<span className="text-muted">{stats.draft} draft</span>}
                        className="d-flex flex-grow-0 px-2 text-truncate batch-change-stats-card__stat"
                    />
                    <ChangesetStatusOpen
                        label={<span className="text-muted">{stats.open} open</span>}
                        className="d-flex flex-grow-0 px-2 text-truncate batch-change-stats-card__stat"
                    />
                    <ChangesetStatusClosed
                        label={<span className="text-muted">{stats.closed} closed</span>}
                        className="d-flex flex-grow-0 px-2 text-truncate batch-change-stats-card__stat"
                    />
                    <ChangesetStatusMerged
                        label={<span className="text-muted">{stats.merged} merged</span>}
                        className="d-flex flex-grow-0 pl-2 text-truncate batch-change-stats-card__stat"
                    />
                </div>
            </div>
        </div>
    )
}

export const BatchChangeStatsTotalAction: React.FunctionComponent<{ count: number }> = ({ count }) => (
    <div className="m-0 flex-grow-0 pr-2 text-truncate batch-change-stats-card__stat text-nowrap d-flex flex-column align-items-center justify-content-center">
        <span className="batch-change-stats-card__changesets-pill">
            <span className="badge badge-pill badge-secondary">{count}</span>
        </span>
        <span className="text-muted">{pluralize('changeset', count, 'changesets')}</span>
    </div>
)
