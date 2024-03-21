import React, { type CSSProperties, useCallback, useState } from 'react'

import { mdiClose } from '@mdi/js'
import classNames from 'classnames'

import { isErrorLike, sanitizeClass } from '@sourcegraph/common'
import { Card, Icon, Button } from '@sourcegraph/wildcard'

import { ActionItem, type ActionItemComponentProps } from '../actions/ActionItem'
import type { TelemetryProps } from '../telemetry/telemetryService'

import { CopyLinkIcon } from './CopyLinkIcon'
import { toNativeEvent } from './helpers'
import type { HoverContext, HoverOverlayBaseProps } from './HoverOverlay.types'
import { HoverOverlayContents } from './HoverOverlayContents'
import { HoverOverlayLogo } from './HoverOverlayLogo'
import { useLogTelemetryEvent } from './useLogTelemetryEvent'

import hoverOverlayStyle from './HoverOverlay.module.scss'
import style from './HoverOverlayContents.module.scss'

const LOADING = 'loading' as const

const transformMouseEvent = (handler: (event: MouseEvent) => void) => (event: React.MouseEvent<HTMLElement>) =>
    handler(toNativeEvent(event))

export type { HoverContext }

export interface HoverOverlayClassProps {
    /** An optional class name to apply to the outermost element of the HoverOverlay */
    className?: string
    closeButtonClassName?: string

    iconClassName?: string
    badgeClassName?: string

    actionItemClassName?: string
    actionItemPressedClassName?: string

    contentClassName?: string
    actionsClassName?: string
}

export interface HoverOverlayProps
    extends HoverOverlayBaseProps,
        ActionItemComponentProps,
        HoverOverlayClassProps,
        TelemetryProps {
    /** A ref callback to get the root overlay element. Use this to calculate the position. */
    hoverRef?: React.Ref<HTMLDivElement>

    pinOptions?: PinOptions

    /** Show Sourcegraph logo alongside prompt */
    useBrandedLogo?: boolean
}

export interface PinOptions {
    /** Whether to show the close button for the hover overlay */
    showCloseButton: boolean

    /** Called when the close button is clicked */
    onCloseButtonClick?: () => void

    /** Called when the copy link button is clicked */
    onCopyLinkButtonClick?: () => void
}

const getOverlayStyle = (overlayPosition: HoverOverlayProps['overlayPosition']): CSSProperties => {
    if (!overlayPosition) {
        return {
            opacity: 0,
            visibility: 'hidden',
        }
    }

    const topOrBottom = 'top' in overlayPosition ? 'top' : 'bottom'
    const topOrBottomValue = 'top' in overlayPosition ? overlayPosition.top : overlayPosition.bottom

    return {
        opacity: 1,
        visibility: 'visible',
        left: `${overlayPosition.left}px`,
        [topOrBottom]: `${topOrBottomValue}px`,
    }
}

export const HoverOverlay: React.FunctionComponent<React.PropsWithChildren<HoverOverlayProps>> = props => {
    const {
        hoverOrError,
        hoverRef,
        overlayPosition,
        actionsOrError,
        telemetryService,
        extensionsController,
        pinOptions,
        location,

        className,
        closeButtonClassName,
        iconClassName,
        badgeClassName,
        actionItemClassName,
        actionItemPressedClassName,
        contentClassName,
        actionsClassName,

        actionItemStyleProps,

        useBrandedLogo,
    } = props

    useLogTelemetryEvent(props)

    const [copyLinkText, setCopyLinkText] = useState('Copy link')

    const onCopyLink = useCallback(() => {
        setCopyLinkText('Copied!')
        setTimeout(() => setCopyLinkText('Copy link'), 3000)
        pinOptions?.onCopyLinkButtonClick?.()
    }, [pinOptions])

    if (!hoverOrError && (!actionsOrError || isErrorLike(actionsOrError))) {
        return null
    }

    return (
        <Card
            // needed for dynamic styling
            data-testid="hover-overlay"
            style={getOverlayStyle(overlayPosition)}
            className={classNames(hoverOverlayStyle.card, hoverOverlayStyle.hoverOverlay, className)}
            ref={hoverRef}
        >
            <div
                data-testid="hover-overlay-contents"
                className={classNames(
                    style.hoverOverlayContents,
                    hoverOrError === LOADING && style.hoverOverlayContentsLoading,
                    pinOptions?.showCloseButton && style.hoverOverlayContentsWithCloseButton
                )}
            >
                {pinOptions?.showCloseButton && (
                    <Button
                        variant="icon"
                        onClick={
                            pinOptions.onCloseButtonClick
                                ? transformMouseEvent(pinOptions.onCloseButtonClick)
                                : undefined
                        }
                        className={classNames(
                            hoverOverlayStyle.closeButton,
                            closeButtonClassName,
                            hoverOrError === LOADING && hoverOverlayStyle.closeButtonLoading
                        )}
                    >
                        <Icon className={iconClassName} svgPath={mdiClose} inline={false} aria-label="Close" />
                    </Button>
                )}
                <HoverOverlayContents
                    hoverOrError={hoverOrError}
                    iconClassName={iconClassName}
                    badgeClassName={badgeClassName}
                    contentClassName={contentClassName}
                />
            </div>
            <div className={hoverOverlayStyle.actionsContainer}>
                {actionsOrError !== undefined &&
                    actionsOrError !== null &&
                    actionsOrError !== LOADING &&
                    !isErrorLike(actionsOrError) &&
                    actionsOrError.length > 0 && (
                        <div className={hoverOverlayStyle.actions}>
                            <div className={classNames(hoverOverlayStyle.actionsInner, actionsClassName)}>
                                {actionsOrError.map((action, index) => (
                                    <ActionItem
                                        key={index}
                                        {...action}
                                        className={classNames(
                                            hoverOverlayStyle.action,
                                            actionItemClassName,
                                            `test-tooltip-${sanitizeClass(action.action.title || 'untitled')}`,
                                            index !== 0 && 'ml-1'
                                        )}
                                        iconClassName={iconClassName}
                                        pressedClassName={actionItemPressedClassName}
                                        variant="actionItem"
                                        disabledDuringExecution={true}
                                        showLoadingSpinnerDuringExecution={true}
                                        telemetryService={telemetryService}
                                        extensionsController={extensionsController}
                                        location={location}
                                        actionItemStyleProps={actionItemStyleProps}
                                    />
                                ))}
                            </div>

                            {useBrandedLogo && <HoverOverlayLogo className={hoverOverlayStyle.overlayLogo} />}
                        </div>
                    )}

                {pinOptions && (
                    <button
                        data-testid="hover-copy-link"
                        className={classNames('d-flex', 'align-items-center', hoverOverlayStyle.actionsCopyLink)}
                        onClick={onCopyLink}
                        onKeyPress={onCopyLink}
                        type="button"
                    >
                        <Icon className="mr-1" as={CopyLinkIcon} aria-hidden={true} />
                        <span className="inline-block">{copyLinkText}</span>
                    </button>
                )}
            </div>
        </Card>
    )
}
