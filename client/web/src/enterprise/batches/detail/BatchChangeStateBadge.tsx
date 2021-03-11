import classNames from 'classnames'
import React from 'react'

export const BatchChangeStateBadge: React.FunctionComponent<{ isClosed: boolean; className?: string }> = ({
    isClosed,
    className,
}) => {
    if (isClosed) {
        return <span className={classNames('badge badge-danger text-uppercase', className)}>Closed</span>
    }
    return <span className={classNames('badge badge-success text-uppercase', className)}>Open</span>
}
