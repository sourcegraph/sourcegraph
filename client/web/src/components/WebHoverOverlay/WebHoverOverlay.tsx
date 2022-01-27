import classNames from 'classnames'
import React, { useCallback, useEffect } from 'react'
import { fromEvent } from 'rxjs'
import { finalize, tap } from 'rxjs/operators'

import { HoveredToken } from '@sourcegraph/codeintellify'
import { isErrorLike } from '@sourcegraph/common'
import { NotificationType } from '@sourcegraph/shared/src/api/extension/extensionHostApi'
import { HoverOverlay, HoverOverlayProps } from '@sourcegraph/shared/src/hover/HoverOverlay'
import { useLocalStorage } from '@sourcegraph/wildcard'

import { HoverThresholdProps } from '../../repo/RepoContainer'

import styles from './WebHoverOverlay.module.scss'

const iconKindToAlertKind = {
    [NotificationType.Info]: 'secondary',
    [NotificationType.Error]: 'danger',
    [NotificationType.Warning]: 'warning',
}

const getAlertClassName: HoverOverlayProps['getAlertClassName'] = iconKind =>
    `alert alert-${iconKindToAlertKind[iconKind]}`

export const WebHoverOverlay: React.FunctionComponent<
    HoverOverlayProps &
        HoverThresholdProps & {
            hoveredTokenElement?: HTMLElement
            nav?: (url: string) => void
            onHoverToken: (hoveredToken: HoveredToken) => void
        }
> = props => {
    const [dismissedAlerts, setDismissedAlerts] = useLocalStorage<string[]>('WebHoverOverlay.dismissedAlerts', [])
    const onAlertDismissed = useCallback(
        (alertType: string) => {
            if (!dismissedAlerts.includes(alertType)) {
                setDismissedAlerts([...dismissedAlerts, alertType])
            }
        },
        [dismissedAlerts, setDismissedAlerts]
    )

    let propsToUse = props
    if (props.hoverOrError && props.hoverOrError !== 'loading' && !isErrorLike(props.hoverOrError)) {
        const filteredAlerts = (props.hoverOrError?.alerts || []).filter(
            alert => !alert.type || !dismissedAlerts.includes(alert.type)
        )
        propsToUse = { ...props, hoverOrError: { ...props.hoverOrError, alerts: filteredAlerts } }
    }

    const { hoverOrError } = propsToUse
    const { onHoverShown, onHoverToken, hoveredToken } = props

    /** Whether the hover has actual content (that provides value to the user) */
    const hoverHasValue = hoverOrError !== 'loading' && !isErrorLike(hoverOrError) && !!hoverOrError?.contents?.length

    useEffect(() => {
        if (hoverHasValue) {
            onHoverShown?.()
        }
        // if (hoveredToken) {
        //     onHoverToken(hoveredToken)
        // }
    }, [
        hoveredToken,
        // hoveredToken?.filePath,
        // hoveredToken?.line,
        // hoveredToken?.character,
        onHoverShown,
        hoverHasValue,
        onHoverToken,
    ])

    useEffect(() => {
        const token = props.hoveredTokenElement

        // const definitionAction =
        //     Array.isArray(props.actionsOrError) &&
        //     props.actionsOrError.find(a => a.action.id === 'goToDefinition.preloaded' && !a.disabledWhen)

        // const referenceAction =
        //     Array.isArray(props.actionsOrError) &&
        //     props.actionsOrError.find(a => a.action.id === 'findReferences' && !a.disabledWhen)

        // const action = definitionAction || referenceAction
        // if (!action) {
        //     return undefined
        // }
        // const url = urlForClientCommandOpen(action.action, props.location.hash)

        // if (!token || !url || !props.nav) {
        if (!token || !hoveredToken) {
            return
        }

        // const nav = props.nav

        const oldCursor = token.style.cursor
        token.style.cursor = 'pointer'

        const subscription = fromEvent(token, 'click')
            .pipe(
                tap(() => {
                    const selection = window.getSelection()
                    if (selection !== null && selection.toString() !== '') {
                        return
                    }

                    // const actionType = action === definitionAction ? 'definition' : 'reference'
                    // props.telemetryService.log(`${actionType}HoverOverlay.click`)
                    // nav(url)

                    onHoverToken(hoveredToken)
                }),
                finalize(() => (token.style.cursor = oldCursor))
            )
            .subscribe()

        return () => subscription.unsubscribe()
    }, [
        props.actionsOrError,
        props.hoveredTokenElement,
        props.location.hash,
        props.nav,
        props.telemetryService,
        props.onHoverToken,
        hoveredToken,
        onHoverToken,
    ])

    return (
        <HoverOverlay
            {...propsToUse}
            className={classNames('card', styles.webHoverOverlay)}
            actionItemClassName="btn btn-sm btn-secondary border-0"
            onAlertDismissed={onAlertDismissed}
            getAlertClassName={getAlertClassName}
        />
    )
}

WebHoverOverlay.displayName = 'WebHoverOverlay'
