import React from 'react'

import classNames from 'classnames'
import { upperFirst } from 'lodash'

import { isErrorLike } from '@sourcegraph/common'
import { Alert, AlertProps, LoadingSpinner } from '@sourcegraph/wildcard'

import { HoverOverlayBaseProps } from '../HoverOverlay.types'

import { HoverOverlayContent } from './HoverOverlayContent'

import hoverOverlayStyle from '../HoverOverlay.module.scss'

interface HoverOverlayContentsProps extends Pick<HoverOverlayBaseProps, 'hoverOrError'> {
    iconClassName?: string
    badgeClassName?: string
    errorAlertClassName?: string
    errorAlertVariant?: AlertProps['variant']
    contentClassName?: string
}

export const HoverOverlayContents: React.FunctionComponent<
    React.PropsWithChildren<HoverOverlayContentsProps>
> = props => {
    const { hoverOrError, iconClassName, errorAlertClassName, errorAlertVariant, badgeClassName, contentClassName } =
        props

    if (hoverOrError === 'loading') {
        return (
            <div className={classNames(hoverOverlayStyle.loaderRow)}>
                <LoadingSpinner inline={false} className={iconClassName} />
            </div>
        )
    }

    if (isErrorLike(hoverOrError)) {
        return (
            <Alert
                className={classNames(errorAlertClassName, hoverOverlayStyle.hoverError)}
                variant={errorAlertVariant}
            >
                {upperFirst(hoverOrError.message)}
            </Alert>
        )
    }

    if (hoverOrError === undefined) {
        return null
    }

    if (hoverOrError === null || hoverOrError.contents.length === 0) {
        return (
            // Show some content to give the close button space and communicate to the user we couldn't find a hover.
            <small className={classNames(hoverOverlayStyle.hoverEmpty)}>No hover information available.</small>
        )
    }

    return (
        <>
            {hoverOrError.contents.map((content, index) => (
                <HoverOverlayContent
                    key={index}
                    index={index}
                    content={content}
                    aggregatedBadges={hoverOrError.aggregatedBadges}
                    errorAlertClassName={errorAlertClassName}
                    errorAlertVariant={errorAlertVariant}
                    badgeClassName={badgeClassName}
                    contentClassName={contentClassName}
                />
            ))}
        </>
    )
}
