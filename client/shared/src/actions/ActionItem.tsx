import * as React from 'react'

import { mdiHelpCircleOutline, mdiOpenInNew } from '@mdi/js'
import classNames from 'classnames'
import * as H from 'history'
import { from, Subject, Subscription } from 'rxjs'
import { catchError, map, mapTo, mergeMap, startWith, tap } from 'rxjs/operators'

import { ActionContribution } from '@sourcegraph/client-api'
import { asError, ErrorLike, isErrorLike, isExternalLink, logger } from '@sourcegraph/common'
import {
    LoadingSpinner,
    Button,
    ButtonLink,
    ButtonLinkProps,
    WildcardThemeContext,
    Icon,
    Tooltip,
} from '@sourcegraph/wildcard'

import { ExecuteCommandParameters } from '../api/client/mainthread-api'
import { urlForOpenPanel } from '../commands/commands'
import type { ExtensionsControllerProps } from '../extensions/controller'
import { PlatformContextProps } from '../platform/context'
import { TelemetryProps } from '../telemetry/telemetryService'

import styles from './ActionItem.module.scss'

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

export interface ActionItemComponentProps
    extends ExtensionsControllerProps<'executeCommand'>,
        PlatformContextProps<'settings'> {
    location: H.Location

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

    /**
     * Whether to show the error (if any) from executing the command inline on this component and NOT in the global
     * notifications UI component.
     *
     * This inline error display behavior is intended for actions that are scoped to a particular component. If the
     * error were displayed in the global notifications UI component, it might not be clear which of the many
     * possible scopes the error applies to.
     *
     * For example, the hover actions ("Go to definition", "Find references", etc.) use showInlineError == true
     * because those actions are scoped to a specific token in a file. The command palette uses showInlineError ==
     * false because it is a global UI component (and because showing tooltips on menu items would look strange).
     */
    showInlineError?: boolean

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

interface State {
    /** The executed action: undefined while loading, null when done or not started, or an error. */
    actionOrError: typeof LOADING | null | ErrorLike
}

export class ActionItem extends React.PureComponent<ActionItemProps, State, typeof WildcardThemeContext> {
    public static contextType = WildcardThemeContext
    public context!: React.ContextType<typeof WildcardThemeContext>

    public state: State = { actionOrError: null }

    private commandExecutions = new Subject<ExecuteCommandParameters>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.commandExecutions
                .pipe(
                    mergeMap(parameters =>
                        from(
                            this.props.extensionsController
                                ? this.props.extensionsController.executeCommand(parameters, this.props.showInlineError)
                                : Promise.reject(
                                      new Error(
                                          'ActionItems commands other than open and invokeFunction-new are deprecated'
                                      )
                                  )
                        ).pipe(
                            mapTo(null),
                            catchError(error => [asError(error)]),
                            map(actionOrError => ({ actionOrError })),
                            tap(() => {
                                if (this.props.onDidExecute) {
                                    this.props.onDidExecute(this.props.action.id)
                                }
                            }),
                            startWith<Pick<State, 'actionOrError'>>({ actionOrError: LOADING })
                        )
                    )
                )
                .subscribe(
                    stateUpdate => this.setState(stateUpdate),
                    error => logger.error(error)
                )
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        let content: JSX.Element | string | undefined
        let tooltip: string | undefined
        if (this.props.title) {
            content = this.props.title
            tooltip = this.props.action.description
        } else if (this.props.variant === 'actionItem' && this.props.action.actionItem) {
            content = (
                <>
                    {this.props.action.actionItem.iconURL && (
                        <img
                            src={this.props.action.actionItem.iconURL}
                            alt={this.props.action.actionItem.iconDescription || ''}
                            className={this.props.iconClassName}
                        />
                    )}
                    {!this.props.hideLabel &&
                        this.props.action.actionItem.label &&
                        ` ${this.props.action.actionItem.label}`}
                </>
            )
            tooltip = this.props.action.actionItem.description
        } else if (this.props.disabledWhen) {
            content = this.props.action.disabledTitle
        } else {
            content = (
                <>
                    {this.props.action.iconURL && (
                        <img
                            src={this.props.action.iconURL}
                            alt={this.props.action.description || ''}
                            className={this.props.iconClassName}
                        />
                    )}{' '}
                    {this.props.action.title}
                </>
            )
            tooltip = this.props.action.description
        }

        if (!this.props.active && tooltip) {
            tooltip += ' (inactive)'
        }

        // Simple display if the action is a noop.
        if (!this.props.action.command) {
            return (
                <Tooltip content={tooltip}>
                    <span
                        data-content={this.props.dataContent}
                        className={this.props.className}
                        tabIndex={this.props.tabIndex}
                    >
                        {this.props.action?.title === '?' ? (
                            <Icon aria-hidden={true} svgPath={mdiHelpCircleOutline} />
                        ) : (
                            content
                        )}
                    </span>
                </Tooltip>
            )
        }

        const showLoadingSpinner = this.props.showLoadingSpinnerDuringExecution && this.state.actionOrError === LOADING
        const pressed =
            this.props.variant === 'actionItem' && this.props.action.actionItem
                ? this.props.action.actionItem.pressed
                : undefined

        const altTo = this.props.altAction && urlForClientCommandOpen(this.props.altAction, this.props.location.hash)
        const primaryTo = urlForClientCommandOpen(this.props.action, this.props.location.hash)
        const to = primaryTo || altTo
        // Open in new tab if an external link
        const newTabProps =
            to && isExternalLink(to)
                ? {
                      target: '_blank',
                      rel: 'noopener noreferrer',
                  }
                : {}
        const buttonLinkProps: Pick<ButtonLinkProps, 'variant' | 'size' | 'outline'> = this.context.isBranded
            ? {
                  variant: this.props.actionItemStyleProps?.actionItemVariant ?? 'link',
                  size: this.props.actionItemStyleProps?.actionItemSize,
                  outline: this.props.actionItemStyleProps?.actionItemOutline,
              }
            : {}
        const disabled = this.isDisabled()

        // TODO don't run action when disabled

        // Props shared between button and link
        const sharedProps = {
            disabled:
                !this.props.active ||
                ((this.props.disabledDuringExecution || this.props.showLoadingSpinnerDuringExecution) &&
                    this.state.actionOrError === LOADING) ||
                this.props.disabledWhen,
            tabIndex: this.props.tabIndex,
        }

        const tooltipOrErrorMessage =
            this.props.showInlineError && isErrorLike(this.state.actionOrError)
                ? `Error: ${this.state.actionOrError.message}`
                : tooltip

        if (!to) {
            return (
                <Tooltip content={tooltipOrErrorMessage}>
                    <Button
                        {...sharedProps}
                        {...buttonLinkProps}
                        className={classNames(
                            'test-action-item',
                            this.props.className,
                            showLoadingSpinner && styles.actionItemLoading,
                            pressed && [this.props.pressedClassName],
                            sharedProps.disabled && this.props.inactiveClassName
                        )}
                        onClick={this.runAction}
                        data-action-item-pressed={pressed}
                        aria-pressed={pressed}
                        aria-label={tooltipOrErrorMessage}
                    >
                        {content}{' '}
                        {showLoadingSpinner && (
                            <div className={styles.loader} data-testid="action-item-spinner">
                                <LoadingSpinner inline={false} className={this.props.iconClassName} />
                            </div>
                        )}
                    </Button>
                </Tooltip>
            )
        }

        return (
            <Tooltip content={tooltipOrErrorMessage}>
                <span>
                    <ButtonLink
                        data-content={this.props.dataContent}
                        disabledClassName={this.props.inactiveClassName}
                        aria-disabled={disabled}
                        data-action-item-pressed={pressed}
                        className={classNames(
                            'test-action-item',
                            this.props.className,
                            showLoadingSpinner && styles.actionItemLoading,
                            pressed && [this.props.pressedClassName],
                            buttonLinkProps.variant === 'link' && styles.actionItemLink,
                            disabled && this.props.inactiveClassName
                        )}
                        pressed={pressed}
                        onSelect={this.runAction}
                        // If the command is 'open' or 'openXyz' (builtin commands), render it as a link. Otherwise render
                        // it as a button that executes the command.
                        to={to}
                        {...newTabProps}
                        {...buttonLinkProps}
                        {...sharedProps}
                    >
                        {content}{' '}
                        {!this.props.hideExternalLinkIcon && primaryTo && isExternalLink(primaryTo) && (
                            <Icon
                                className={this.props.iconClassName}
                                svgPath={mdiOpenInNew}
                                inline={false}
                                aria-hidden={true}
                            />
                        )}
                        {showLoadingSpinner && (
                            <div className={styles.loader} data-testid="action-item-spinner">
                                <LoadingSpinner inline={false} className={this.props.iconClassName} />
                            </div>
                        )}
                    </ButtonLink>
                </span>
            </Tooltip>
        )
    }

    public runAction = (event: React.MouseEvent<HTMLElement> | React.KeyboardEvent<HTMLElement>): void => {
        const action = (isAltEvent(event) && this.props.altAction) || this.props.action

        if (!action.command) {
            // Unexpectedly arrived here; noop actions should not have event handlers that trigger
            // this.
            return
        }

        if (this.isDisabled()) {
            return
        }

        // Record action ID (but not args, which might leak sensitive data).
        this.props.telemetryService.log(action.id)

        const emitDidExecute = (): void => {
            if (this.props.onDidExecute) {
                // Defer calling onRun until after the URL has been opened. If we call it immediately, then in
                // CommandList it immediately updates the (most-recent-first) ordering of the ActionItems, and
                // the URL actually changes underneath us before the URL is opened. There is no harm to
                // deferring this call; onRun's documentation allows this.
                const onDidExecute = this.props.onDidExecute
                setTimeout(() => onDidExecute(action.id))
            }
        }

        if (urlForClientCommandOpen(action, this.props.location.hash)) {
            if (event.currentTarget.tagName === 'A' && event.currentTarget.hasAttribute('href')) {
                // Do not execute the command. The <LinkOrButton>'s default event handler will do what we want (which
                // is to open a URL). The only case where this breaks is if both the action and alt action are "open"
                // commands; in that case, this only ever opens the (non-alt) action.
                emitDidExecute()
                return
            }
        }

        // A special-case to support invokeFunction style actions without the extensions controller
        if (action.command === 'invokeFunction-new') {
            const args = action.commandArguments || []
            for (const arg of args) {
                if (typeof arg === 'function') {
                    arg()
                }
            }

            emitDidExecute()
            return
        }

        // If the action we're running is *not* opening a URL by using the event target's default handler, then
        // ensure the default event handler for the <LinkOrButton> doesn't run (which might open the URL).
        event.preventDefault()

        this.commandExecutions.next({
            command: action.command,
            args: action.commandArguments,
        })
    }

    private isDisabled = (): boolean | undefined =>
        !this.props.active ||
        ((this.props.disabledDuringExecution || this.props.showLoadingSpinnerDuringExecution) &&
            this.state.actionOrError === LOADING) ||
        this.props.disabledWhen
}

export function urlForClientCommandOpen(
    action: Pick<ActionContribution, 'command' | 'commandArguments'>,
    locationHash: string
): string | undefined {
    if (action.command === 'open' && action.commandArguments) {
        const url = action.commandArguments[0]
        if (typeof url !== 'string') {
            return undefined
        }
        return url
    }

    if (action.command === 'openPanel' && action.commandArguments) {
        const url = action.commandArguments[0]
        if (typeof url !== 'string') {
            return undefined
        }
        return urlForOpenPanel(url, locationHash)
    }

    return undefined
}

function isAltEvent(event: React.KeyboardEvent | React.MouseEvent): boolean {
    return event.altKey || event.metaKey || event.ctrlKey || ('button' in event && event.button === 1)
}
