import React from 'react'
import classnames from 'classnames'
import { Badge, BadgeProps } from './Badge'

interface FeedbackBadgeProps extends BadgeProps {
    /** Render a mailto href to share feedback */
    feedback: {
        mailto: string
        /** Defaults to 'Share feedback' */
        text?: string
    }
    className?: string
}

export const FeedbackBadge: React.FC<FeedbackBadgeProps> = props => {
    const {
        className,
        status,
        tooltip,
        feedback: { mailto, text },
    } = props

    return (
        <div className={classnames('d-flex', 'align-items-center', className)}>
            <Badge tooltip={tooltip} status={status} className="text-uppercase" />
            <a href={`mailto:${mailto}`} className="ml-2" target="_blank" rel="noopener noreferrer">
                {text || 'Share feedback'}
            </a>
        </div>
    )
}
