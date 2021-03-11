import classNames from 'classnames'
import MenuDownIcon from 'mdi-react/MenuDownIcon'
import MenuUpIcon from 'mdi-react/MenuUpIcon'
import React, { useCallback, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { Collapse } from 'reactstrap'

export const SIDEBAR_BUTTON_CLASS = 'btn btn-secondary d-block w-100 my-2'

/**
 * Item of `SideBarGroupItems`.
 */
export const SidebarNavItem: React.FunctionComponent<{
    to: string
    icon?: React.ComponentType<{ className?: string }>
    className?: string
    exact?: boolean
}> = ({ icon: Icon, children, className, to, exact }) => (
    <NavLink to={to} exact={exact} className={classNames('list-group-item list-group-item-action py-2', className)}>
        {Icon && <Icon className="icon-inline mr-2" />}
        {children}
    </NavLink>
)

/**
 * Header of a `SideBarGroup`
 */
export const SidebarGroupHeader: React.FunctionComponent<{
    icon?: React.ComponentType<{ className?: string }>
    label: string
    children?: undefined
}> = ({ icon: Icon, label }) => (
    <div className="card-header">
        {Icon && <Icon className="icon-inline mr-1" />} {label}
    </div>
)

/**
 * Sidebar with collapsible items
 */
export const SidebarCollapseItems: React.FunctionComponent<{
    children: JSX.Element
    icon?: React.ComponentType<{ className?: string }>
    label?: string
    openByDefault?: boolean
}> = ({ children, label, icon: Icon, openByDefault = false }) => {
    const [isOpen, setOpen] = useState<boolean>(openByDefault)
    const handleOpen = useCallback(() => setOpen(!isOpen), [isOpen])
    return (
        <>
            <button
                aria-expanded={isOpen}
                aria-controls={label}
                type="button"
                onClick={handleOpen}
                className="bg-2 border-0 d-flex justify-content-between list-group-item-action py-2 w-100"
            >
                <span>
                    {Icon && <Icon className="icon-inline mr-1" />} {label}
                </span>
                {isOpen ? (
                    <MenuUpIcon className="sidebar__chevron icon-inline" />
                ) : (
                    <MenuDownIcon className="sidebar__chevron icon-inline" />
                )}
            </button>
            <Collapse id={label} isOpen={isOpen} className="border-top">
                {children}
            </Collapse>
        </>
    )
}

/**
 * A box of items in the side bar. Use `SideBarGroupHeader` and `SideBarGroupItems` as children.
 */
export const SidebarGroup: React.FunctionComponent<{}> = ({ children }) => (
    <div className="card mb-3 sidebar">{children}</div>
)

/**
 * Container for all `SideBarNavItem` in a `SideBarGroup`.
 */
export const SidebarGroupItems: React.FunctionComponent<{}> = ({ children }) => (
    <div className="list-group list-group-flush">{children}</div>
)
