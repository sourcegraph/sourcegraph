import React, { useEffect } from 'react'

import classNames from 'classnames'
import type { Observable } from 'rxjs'

import { isErrorLike } from '@sourcegraph/common'
import { urlForClientCommandOpen } from '@sourcegraph/shared/src/actions/ActionItem'
import { HoverOverlay, type HoverOverlayProps } from '@sourcegraph/shared/src/hover/HoverOverlay'

import type { HoverThresholdProps } from '../../repo/RepoContainer'

import styles from './WebHoverOverlay.module.scss'

export interface WebHoverOverlayProps
    extends Omit<
            HoverOverlayProps,
            'className' | 'closeButtonClassName' | 'actionItemClassName' | 'actionItemStyleProps'
        >,
        HoverThresholdProps {
    hoveredTokenElement?: HTMLElement
    /**
     * If the hovered token doesn't have a corresponding DOM element, this prop
     * can be used to trigger the "click to go to definition" functionality.
     */
    hoveredTokenClick?: Observable<unknown>
    nav?: (url: string) => void

    hoverOverlayContainerClassName?: string
}

export const WebHoverOverlay: React.FunctionComponent<React.PropsWithChildren<WebHoverOverlayProps>> = props => {
    const { hoverOrError, onHoverShown, hoveredToken } = props

    /** Whether the hover has actual content (that provides value to the user) */
    const hoverHasValue = hoverOrError !== 'loading' && !isErrorLike(hoverOrError) && !!hoverOrError?.contents?.length

    useEffect(() => {
        if (hoverHasValue) {
            onHoverShown?.()
        }
    }, [hoveredToken?.filePath, hoveredToken?.line, hoveredToken?.character, onHoverShown, hoverHasValue])

    return (
        <HoverOverlay
            {...props}
            className={classNames(styles.webHoverOverlay, props.hoverOverlayContainerClassName)}
            closeButtonClassName={styles.webHoverOverlayCloseButton}
            actionItemClassName="border-0"
            actionItemStyleProps={{
                actionItemSize: 'sm',
                actionItemVariant: 'secondary',
            }}
        />
    )
}

WebHoverOverlay.displayName = 'WebHoverOverlay'

/**
 * Returns the URL and type to perform the "go to ..." navigation.
 */
export const getGoToURL = (
    actionsOrError: WebHoverOverlayProps['actionsOrError'],
    location: Location
): {
    url: string
    actionType: 'definition' | 'reference'
} | null => {
    const definitionAction =
        Array.isArray(actionsOrError) &&
        actionsOrError.find(a => a.action.id === 'goToDefinition.preloaded' && !a.disabledWhen)

    const referenceAction =
        Array.isArray(actionsOrError) && actionsOrError.find(a => a.action.id === 'findReferences' && !a.disabledWhen)

    const action = definitionAction || referenceAction
    if (!action) {
        return null
    }

    const url = urlForClientCommandOpen(action.action, location.hash)
    if (!url) {
        return null
    }

    return { url, actionType: action === definitionAction ? 'definition' : 'reference' }
}
