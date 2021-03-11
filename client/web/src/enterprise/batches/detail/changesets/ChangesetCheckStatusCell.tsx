import React from 'react'
import { ExternalChangesetFields, ChangesetCheckState } from '../../../../graphql-operations'
import CheckCircleIcon from 'mdi-react/CheckCircleIcon'
import CloseCircleIcon from 'mdi-react/CloseCircleIcon'
import TimerSandIcon from 'mdi-react/TimerSandIcon'
import classNames from 'classnames'

export interface ChangesetCheckStatusCellProps {
    className?: string
    checkState: NonNullable<ExternalChangesetFields['checkState']>
}

export const ChangesetCheckStatusCell: React.FunctionComponent<ChangesetCheckStatusCellProps> = ({
    className,
    checkState,
}) => {
    switch (checkState) {
        case ChangesetCheckState.PENDING:
            return <ChangesetCheckStatusPending className={className} />
        case ChangesetCheckState.PASSED:
            return <ChangesetCheckStatusPassed className={className} />
        case ChangesetCheckState.FAILED:
            return <ChangesetCheckStatusFailed className={className} />
    }
}

export const ChangesetCheckStatusPending: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-warning m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <TimerSandIcon data-tooltip="Check state is pending" />
        <span className="text-muted">Pending</span>
    </div>
)
export const ChangesetCheckStatusPassed: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-success m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <CheckCircleIcon data-tooltip="All checks complete" />
        <span className="text-muted">Passed</span>
    </div>
)
export const ChangesetCheckStatusFailed: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div
        className={classNames(
            'text-danger m-0 text-nowrap d-flex flex-column align-items-center justify-content-center',
            className
        )}
    >
        <CloseCircleIcon data-tooltip="Some checks failed" />
        <span className="text-muted">Failed</span>
    </div>
)
