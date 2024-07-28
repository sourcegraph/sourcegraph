import { FC } from 'react'

import classNames from 'classnames'

import styles from './Progress.module.scss'

interface ProgressProps {
    value: number
    className?: string
    'aria-labelledby'?: string
}

/**
 * Simple progress UI component, use it only for progress like
 * UI elements, for percentage status use Meter component instead.
 */
export const Progress: FC<ProgressProps> = props => {
    const { value, className, 'aria-labelledby': ariaLabelledBy } = props
    const normalizedValue = Math.min(Math.max(value, 0), 100)

    return (
        <div
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={normalizedValue}
            aria-labelledby={ariaLabelledBy}
            className={classNames(className, styles.root, {
                [styles.rootWithProgress]: normalizedValue !== 0,
                [styles.rootWithNoProgress]: normalizedValue === 0,
            })}
        >
            <div
                // eslint-disable-next-line react/forbid-dom-props
                style={{ width: `${normalizedValue}%` }}
                className={styles.bar}
            />
        </div>
    )
}
