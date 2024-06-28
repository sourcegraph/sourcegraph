import AccountIcon from 'mdi-react/AccountIcon'
import CogOutlineIcon from 'mdi-react/CogOutlineIcon'

import { namespaceAreaHeaderNavItems } from '../../namespaces/navitems'
import { SavedSearchIcon } from '../../savedSearches/SavedSearchIcon'

import type { UserAreaHeaderNavItem } from './UserAreaHeader'

export const userAreaHeaderNavItems: readonly UserAreaHeaderNavItem[] = [
    {
        to: '/profile',
        label: 'Profile',
        icon: AccountIcon,
    },
    {
        to: '/settings',
        label: 'Settings',
        icon: CogOutlineIcon,
        condition: ({ user: { viewerCanAdminister } }) => viewerCanAdminister,
    },
    {
        to: '/searches',
        label: 'Saved Searches',
        icon: SavedSearchIcon,
        condition: ({ user: { viewerCanAdminister } }) =>
            viewerCanAdminister && window.context?.codeSearchEnabledOnInstance,
    },
    ...namespaceAreaHeaderNavItems,
]
