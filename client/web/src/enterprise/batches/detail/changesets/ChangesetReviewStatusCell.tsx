import React from 'react'
import { ExternalChangesetFields, ChangesetReviewState } from '../../../../graphql-operations'
import DeltaIcon from 'mdi-react/DeltaIcon'
import TimerSandIcon from 'mdi-react/TimerSandIcon'
import CheckCircleIcon from 'mdi-react/CheckCircleIcon'
import GateArrowRightIcon from 'mdi-react/GateArrowRightIcon'
import CommentOutlineIcon from 'mdi-react/CommentOutlineIcon'
import classNames from 'classnames'

export interface ChangesetReviewStatusCellProps {
    className?: string
    reviewState: NonNullable<ExternalChangesetFields['reviewState']>
}

export const ChangesetReviewStatusCell: React.FunctionComponent<ChangesetReviewStatusCellProps> = ({
    className,
    reviewState,
}) => {
    switch (reviewState) {
        case ChangesetReviewState.APPROVED:
            return <ChangesetReviewStatusApproved className={className} />
        case ChangesetReviewState.CHANGES_REQUESTED:
            return <ChangesetReviewStatusChangesRequested className={className} />
        case ChangesetReviewState.COMMENTED:
            return <ChangesetReviewStatusCommented className={className} />
        case ChangesetReviewState.DISMISSED:
            return <ChangesetReviewStatusDismissed className={className} />
        case ChangesetReviewState.PENDING:
            return <ChangesetReviewStatusPending className={className} />
    }
}

export const ChangesetReviewStatusPending: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-warning m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <TimerSandIcon />
        <span className="text-muted">Pending</span>
    </div>
)
export const ChangesetReviewStatusDismissed: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-muted m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <GateArrowRightIcon />
        <span className="text-muted">Dismissed</span>
    </div>
)
export const ChangesetReviewStatusCommented: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-muted m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <CommentOutlineIcon />
        <span className="text-muted">Commented</span>
    </div>
)
export const ChangesetReviewStatusChangesRequested: React.FunctionComponent<{ className?: string }> = ({
    className,
}) => (
    <div
        className={classNames(
            'text-warning m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <DeltaIcon />
        <span className="text-muted">Changes requested</span>
    </div>
)
export const ChangesetReviewStatusApproved: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-success m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <CheckCircleIcon />
        <span className="text-muted">Approved</span>
    </div>
)
