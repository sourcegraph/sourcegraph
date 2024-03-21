import React from 'react'

import classNames from 'classnames'

import styles from '../index.module.scss'

interface BrandHeaderProps {
    isLightTheme: boolean
}

export const BrandHeader: React.FunctionComponent<BrandHeaderProps> = ({ isLightTheme }) => (
    <>
        <img
            className={classNames(styles.logo)}
            src={`https://sourcegraph.com/.assets/img/sourcegraph-logo-${isLightTheme ? 'light' : 'dark'}.svg`}
            alt="Sourcegraph logo"
        />
        <div data-testid="brand-header" className={classNames(styles.logoText)}>
            Search millions of open source repositories
        </div>
    </>
)
