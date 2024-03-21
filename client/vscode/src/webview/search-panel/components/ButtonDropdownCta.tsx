import React, { useCallback, useEffect, useState } from 'react'

import { VSCodeButton } from '@vscode/webview-ui-toolkit/react'
import classNames from 'classnames'

import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, Popover, PopoverContent, PopoverTrigger, Position } from '@sourcegraph/wildcard'

import type { WebviewPageProps } from '../../platform/context'

import styles from './ButtonDropdownCta.module.scss'

// Debt: this is a fork of the web <ButtonDropdownCta>.

export interface ButtonDropdownCtaProps extends TelemetryProps, Pick<WebviewPageProps, 'extensionCoreAPI'> {
    button: JSX.Element
    icon: JSX.Element
    title: string
    copyText: string
    source: string
    viewEventName: string
    returnTo: string
    onToggle?: () => void
    className?: string
    instanceURL?: string
}

export const ButtonDropdownCta: React.FunctionComponent<React.PropsWithChildren<ButtonDropdownCtaProps>> = ({
    button,
    icon,
    title,
    copyText,
    telemetryService,
    source,
    viewEventName,
    returnTo,
    onToggle,
    className,
    extensionCoreAPI,
}) => {
    const [isDropdownOpen, setIsDropdownOpen] = useState(false)

    const toggleDropdownOpen = useCallback(() => {
        setIsDropdownOpen(isOpen => !isOpen)
        onToggle?.()
    }, [onToggle])

    // Whenever dropdown opens, log view event
    useEffect(() => {
        if (isDropdownOpen) {
            telemetryService.log(viewEventName)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isDropdownOpen])

    // Use cloud url instead of instance url
    // This will only display for non private uers
    // Because private users are required use with token = signed up
    const signUpURL = `https://sourcegraph.com/sign-up?src=${source}&returnTo=${encodeURIComponent(
        returnTo
    )}&utm_medium=VSCODE&utm_source=sidebar&utm_campaign=vsce-sign-up&utm_content=sign-up`

    const onClick = (): void => {
        telemetryService.log(`VSCE${source}SignUpModalClick`)
        extensionCoreAPI.openLink(signUpURL).catch(() => {
            console.error('Error opening sign up link')
        })
    }

    return (
        <Popover isOpen={isDropdownOpen} onOpenChange={toggleDropdownOpen}>
            <PopoverTrigger
                as={Button}
                variant="secondary"
                outline={true}
                size="sm"
                className={classNames('text-decoration-none', className, styles.toggle)}
            >
                {button}
            </PopoverTrigger>
            <PopoverContent className={styles.container} position={Position.bottomEnd}>
                <div className="d-flex mb-3">
                    <div className="d-flex align-items-center mr-3">
                        <div className={styles.icon}>{icon}</div>
                    </div>
                    <div>
                        <div className={styles.title}>
                            <strong>{title}</strong>
                        </div>
                        <div className={classNames('text-muted', styles.copyText)}>{copyText}</div>
                    </div>
                </div>
                <VSCodeButton type="button" onClick={onClick} autofocus={true}>
                    Sign up for Sourcegraph
                </VSCodeButton>
            </PopoverContent>
        </Popover>
    )
}
