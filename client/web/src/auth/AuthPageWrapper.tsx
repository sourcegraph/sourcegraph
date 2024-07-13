import React from 'react'

import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { H1, Text } from '@sourcegraph/wildcard'

import { BrandLogo } from '../components/branding/BrandLogo'

import styles from './AuthPageWrapper.module.scss'

interface Props {
    /** Title of the page. */
    title: string
    /** Optional second line item. */
    description?: string
    className?: string
}

export type AuthPageWrapperProps = React.PropsWithChildren<Props>

export const AuthPageWrapper: React.FunctionComponent<AuthPageWrapperProps> = ({
    title,
    description,
    className,
    children,
}) => {
    const isLightTheme = useIsLightTheme()

    return (
        <>
            <div className={styles.wrapper}>
                <span>
                    <BrandLogo
                        className={styles.logo}
                        isLightTheme={isLightTheme}
                        variant="symbol"
                        disableSymbolSpin={true}
                    />
                </span>
                <H1>{title}</H1>
                {description && <Text className="text-muted">{description}</Text>}
                <div className={className}>{children}</div>
            </div>
        </>
    )
}
