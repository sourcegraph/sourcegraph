import React from 'react'
import classNames from 'classnames'
import { kebabCase, omit } from 'lodash'
import { NavLink, NavLinkProps } from 'react-router-dom'
import { LinkProps, Link } from '../../../shared/src/components/Link'

interface Props extends LinkProps, Pick<NavLinkProps, 'activeClassName'> {
    text: string
}

interface PropsWithIcon extends Props {
    icon: React.ComponentType<{ className?: string }>
}

interface PropsWithIconPlaceholder extends Props {
    hasIconPlaceholder: true
}

/**
 * A link displaying an icon along with text.
 *
 */
export const LinkWithIcon: React.FunctionComponent<PropsWithIcon | PropsWithIconPlaceholder> = props => {
    const { to, text, className = '', activeClassName, ...restProps } = props
    const LinkComponent = activeClassName ? NavLink : Link

    // use `svg` element as a placeholder when `hasIconPlaceholder` is true
    const Icon = 'hasIconPlaceholder' in props ? 'svg' : props.icon

    const linkProps = {
        to,
        className: classNames('d-flex', 'align-items-center', className),
        ...(activeClassName && { activeClassName }),
        ...omit(restProps, ['hasIconPlaceholder', 'icon']),
    }

    return (
        <LinkComponent {...linkProps} data-testid={kebabCase(text)}>
            <Icon className="icon-inline mr-1" />
            <span className="inline-block">{text}</span>
        </LinkComponent>
    )
}
