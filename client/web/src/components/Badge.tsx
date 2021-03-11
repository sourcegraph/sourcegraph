import React from 'react'
import classnames from 'classnames'

export type BadgeStatus = 'beta' | 'prototype' | 'wip' | 'new'

const statusStyleMapping: Record<BadgeStatus, string> = {
    prototype: 'badge-warning',
    wip: 'badge-warning',
    beta: 'badge-info',
    new: 'badge-info',
}

export interface BadgeProps {
    status: BadgeStatus
    tooltip?: string
    className?: string
}

export const Badge: React.FC<BadgeProps> = props => {
    const { className, status, tooltip } = props

    return (
        <span
            data-tooltip={tooltip}
            className={classnames(
                'badge',
                'badge--capitalized',
                'd-inline-flex',
                'align-items-center',
                statusStyleMapping[status],
                className
            )}
        >
            {status}
        </span>
    )
}
