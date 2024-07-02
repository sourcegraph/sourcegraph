import React from 'react'

import classNames from 'classnames'

import { H2 } from '@sourcegraph/wildcard'

import { AlertBadge } from './AlertBadge'
import { CodyProBadge } from './CodyProBadge'

import styles from './CodyAlert.module.scss'

interface CodyAlertProps extends React.HTMLAttributes<HTMLDivElement> {
    variant: 'purple' | 'green' | 'error'
    className?: string
    title?: string
    badge?: 'CodyPro' | 'Alert'
    children: React.ReactNode
}

export const CodyAlert: React.FunctionComponent<CodyAlertProps> = ({
    variant,
    className,
    title,
    children,
    badge,
    ...props
}) => {
    const alertClassName = classNames(
        styles.alert,
        {
            [styles.purple]: variant === 'purple',
            [styles.green]: variant === 'green',
            [styles.error]: variant === 'error',
        },
        className
    )

    return (
        // eslint-disable-next-line no-restricted-syntax
        <div className={alertClassName} {...props}>
            {badge && (
                <div className={styles.badge}>
                    <div className={styles.cardClip}>{badge === 'CodyPro' ? <CodyProBadge /> : <AlertBadge />}</div>
                </div>
            )}
            <div className={styles.content}>
                <H2>{title}</H2>
                <div>{children}</div>
            </div>
        </div>
    )
}
