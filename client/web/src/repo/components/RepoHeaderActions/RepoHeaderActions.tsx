import classNames from 'classnames'
import React from 'react'
import { DropdownToggle, DropdownToggleProps } from 'reactstrap'

import { Link, LinkProps, ButtonLink, ButtonLinkProps } from '@sourcegraph/wildcard'

import styles from './RepoHeaderActions.module.scss'

interface RepoHeaderButtonLinkProps extends ButtonLinkProps {
    /**
     * to determine if this button is for file or not
     */
    file?: boolean
}

interface RepoHeaderActionAnchorProps extends LinkProps {
    /**
     * to determine if this anchor is for file or not
     */
    file?: boolean
}

export const RepoHeaderActionButtonLink: React.FunctionComponent<RepoHeaderButtonLinkProps> = ({
    children,
    className,
    file,
    ...rest
}) => (
    <ButtonLink className={classNames(file ? styles.fileAction : styles.action, className)} {...rest}>
        {children}
    </ButtonLink>
)

export const RepoHeaderActionDropdownToggle: React.FunctionComponent<DropdownToggleProps> = ({
    children,
    className,
    ...rest
}) => (
    <DropdownToggle className={classNames(styles.action, className)} {...rest}>
        {children}
    </DropdownToggle>
)

export const RepoHeaderActionAnchor: React.FunctionComponent<RepoHeaderActionAnchorProps> = ({
    children,
    className,
    file,
    ...rest
}) => (
    <Link className={classNames(file ? styles.fileAction : styles.action, className)} {...rest}>
        {children}
    </Link>
)
