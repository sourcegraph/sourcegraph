import React from 'react'

import classNames from 'classnames'
import * as H from 'history'

import type { ActionContribution } from '@sourcegraph/client-api'
import { ButtonLink, ButtonLinkProps, Tooltip } from '@sourcegraph/wildcard'

import { TelemetryProps } from '../telemetry/telemetryService'

export interface ActionItemAction {
    /**
     * The action specified in the menu item's {@link module:sourcegraph.module/protocol.MenuItemContribution#action}
     * property.
     */
    action: ActionContribution

    /**
     * The alternative action specified in the menu item's
     * {@link module:sourcegraph.module/protocol.MenuItemContribution#alt} property.
     */
    altAction?: ActionContribution

    /** Whether the action item is active in the given context */
    active: boolean

    /** Whether the action item should be disabled in the given context */
    disabledWhen?: boolean
}

export interface ActionItemStyleProps {
    actionItemVariant?: ButtonLinkProps['variant']
    actionItemSize?: ButtonLinkProps['size']
    actionItemOutline?: ButtonLinkProps['outline']
}

export interface ActionItemComponentProps {
    location: H.Location // TODO(sqs): remove?

    iconClassName?: string

    actionItemStyleProps?: ActionItemStyleProps
}

export interface ActionItemProps extends ActionItemAction, ActionItemComponentProps, TelemetryProps {
    variant?: 'actionItem'

    hideLabel?: boolean

    className?: string

    /**
     * Added _in addition_ to `className` if the action item is a toggle in the "pressed" state.
     */
    pressedClassName?: string

    /**
     * Added _in addition_ to `className` if the action item is not active in the given context
     */
    inactiveClassName?: string

    /** Called after executing the action (for both success and failure). */
    onDidExecute?: (actionID: string) => void

    /**
     * Whether to set the disabled attribute on the element when execution is started and not yet finished.
     */
    disabledDuringExecution?: boolean

    /**
     * Whether to show an animated loading spinner when execution is started and not yet finished.
     */
    showLoadingSpinnerDuringExecution?: boolean

    /** Instead of showing the icon and/or title, show this element. */
    title?: JSX.Element | null

    dataContent?: string

    /** Override default tab index */
    tabIndex?: number

    hideExternalLinkIcon?: boolean

    /**
     * Class applied to tooltip trigger `<span>`,
     * which is wrapped around action item content.
     */
    tooltipClassName?: string
}

const LOADING = 'loading' as const

export const ActionItem: React.FunctionComponent<ActionItemProps> = props => {
    const actionTODO = 'x'

    let content: JSX.Element | string | undefined
    let tooltip: string | undefined
    if (props.title) {
        content = props.title
        tooltip = props.action.description
    } else if (props.variant === 'actionItem' && props.action.actionItem) {
        content = (
            <>
                {props.action.actionItem.iconURL && (
                    <img
                        src={props.action.actionItem.iconURL}
                        alt={props.action.actionItem.iconDescription || ''}
                        className={props.iconClassName}
                    />
                )}
                {!props.hideLabel && props.action.actionItem.label && ` ${props.action.actionItem.label}`}
            </>
        )
        tooltip = props.action.actionItem.description
    } else if (props.disabledWhen) {
        content = props.action.disabledTitle
    } else {
        content = (
            <>
                {props.action.iconURL && (
                    <img
                        src={props.action.iconURL}
                        alt={props.action.description || ''}
                        className={props.iconClassName}
                    />
                )}{' '}
                {props.action.title}
            </>
        )
        tooltip = props.action.description
    }

    if (!props.active && tooltip) {
        tooltip += ' (inactive)'
    }

    const isBranded = true // TODO(sqs)
    const buttonLinkProps: Pick<ButtonLinkProps, 'variant' | 'size' | 'outline'> = isBranded
        ? {
              variant: props.actionItemStyleProps?.actionItemVariant ?? 'link',
              size: props.actionItemStyleProps?.actionItemSize,
              outline: props.actionItemStyleProps?.actionItemOutline,
          }
        : {}

    // Props shared between button and link
    const sharedProps = {
        disabled:
            !props.active ||
            ((props.disabledDuringExecution || props.showLoadingSpinnerDuringExecution) && actionTODO === LOADING) ||
            props.disabledWhen,
        tabIndex: props.tabIndex,
    }

    return (
        <Tooltip content={tooltip}>
            <span>
                <ButtonLink
                    data-content={props.dataContent}
                    disabledClassName={props.inactiveClassName}
                    className={classNames('test-action-item', props.className)}
                    // TODO(sqs): action

                    // If the command is 'open' or 'openXyz' (builtin commands), render it as a link. Otherwise render
                    // it as a button that executes the command.
                    to="TODO(sqs)"
                    {...buttonLinkProps}
                    {...sharedProps}
                >
                    {content}{' '}
                </ButtonLink>
            </span>
        </Tooltip>
    )
}
