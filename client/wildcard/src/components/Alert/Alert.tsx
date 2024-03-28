import React from 'react'

import classNames from 'classnames'

import { useWildcardTheme } from '../../hooks'
import type { ForwardReferenceComponent } from '../../types'

import type { ALERT_VARIANTS } from './constants'
import { getAlertStyle } from './utils'

import styles from './Alert.module.scss'

type AlertVariant = typeof ALERT_VARIANTS[number]

export interface AlertProps extends React.HTMLAttributes<HTMLDivElement> {
    variant?: AlertVariant

    /**
     * Setting to control alert icon appearance, has true value
     * be default.
     */
    withIcon?: boolean

    styleOverrides?: {
        backgroundColor?: string
        textColor?: string
        textCentered?: boolean
    }
}

const userShouldBeImmediatelyNotified = (variant?: AlertVariant): boolean =>
    variant === 'danger' || variant === 'warning'

/**
 * Renders a styled alert on the page.
 *
 * Note: These alerts will be automatically read out by screen readers.
 * If this is not desired behavior, you should pass `aria-live="off"` to this component.
 * Further details: https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Roles/alert_role
 */
export const Alert = React.forwardRef(function Alert(
    {
        children,
        withIcon = true,
        as: Component = 'div',
        variant,
        className,
        role = 'alert',
        styleOverrides,
        style,
        ...attributes
    },
    reference
) {
    const { isBranded } = useWildcardTheme()
    const brandedClassName = isBranded && classNames(styles.alert, variant && getAlertStyle({ variant }))

    /**
     * Set the assertiveness setting on the alert.
     * Assertive: The alert will interrupt any current screen reader announcements.
     * Polite: The alert will be read out by the screen reader when the user is idle.
     */
    const alertAssertiveness = userShouldBeImmediatelyNotified(variant) ? 'assertive' : 'polite'

    // Merge styles with overrides
    const { backgroundColor, textColor, textCentered } = styleOverrides || {}
    const mergedStyles: React.CSSProperties = {
        ...style,
        ...(!!backgroundColor && { backgroundColor }),
        ...(!!textColor && { color: textColor }),
        ...(!!textCentered && { textAlign: 'center' }),
    }

    return (
        <Component
            ref={reference}
            className={classNames(brandedClassName, className, { [styles.alertWithNoIcon]: !withIcon })}
            role={role}
            aria-live={alertAssertiveness}
            style={mergedStyles}
            {...attributes}
        >
            {children}
        </Component>
    )
}) as ForwardReferenceComponent<'div', AlertProps>
