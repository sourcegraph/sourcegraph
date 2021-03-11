import MenuDownIcon from 'mdi-react/MenuDownIcon'
import MenuIcon from 'mdi-react/MenuIcon'
import MenuUpIcon from 'mdi-react/MenuUpIcon'
import React, { useCallback, useState } from 'react'
import { ButtonDropdown, DropdownItem, DropdownMenu, DropdownToggle } from 'reactstrap'

interface MenuNavItemProps {
    children: React.ReactNode
    openByDefault?: boolean
}

/**
 * Displays a dropdown menu in the navbar
 * displaiyng navigation links as menu items
 *
 */

export const MenuNavItem: React.FunctionComponent<MenuNavItemProps> = props => {
    const { children, openByDefault } = props
    const [isOpen, setIsOpen] = useState(() => !!openByDefault)
    const toggleIsOpen = useCallback(() => setIsOpen(open => !open), [])

    return (
        <ButtonDropdown direction="down" isOpen={isOpen} toggle={toggleIsOpen}>
            <DropdownToggle className="bg-transparent" nav={true}>
                <MenuIcon className="icon-inline" />
                {isOpen ? <MenuUpIcon className="icon-inline" /> : <MenuDownIcon className="icon-inline" />}
            </DropdownToggle>
            <DropdownMenu>
                {React.Children.map(children, child => child && <DropdownItem>{child}</DropdownItem>)}
            </DropdownMenu>
        </ButtonDropdown>
    )
}
