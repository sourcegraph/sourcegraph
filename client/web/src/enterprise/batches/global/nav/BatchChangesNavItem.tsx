import React from 'react'
import { LinkWithIcon } from '../../../../components/LinkWithIcon'
import { BatchChangesIconNav } from '../../icons'
import classNames from 'classnames'

interface Props {
    className?: string
}

/**
 * An item in {@link GlobalNavbar} that links to the batch changes area.
 */
export const BatchChangesNavItem: React.FunctionComponent<Props> = ({ className }) => (
    <LinkWithIcon
        to="/batch-changes"
        text="Batch Changes"
        icon={BatchChangesIconNav}
        className={classNames('nav-link btn btn-link text-decoration-none', className)}
        activeClassName="active"
    />
)
