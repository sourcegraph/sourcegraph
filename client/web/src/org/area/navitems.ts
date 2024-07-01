import CogOutlineIcon from 'mdi-react/CogOutlineIcon'

import { namespaceAreaHeaderNavItems } from '../../namespaces/navitems'
import { SavedSearchIcon } from '../../savedSearches/SavedSearchIcon'
import { WorkflowIcon } from '../../workflows/WorkflowIcon'

import type { OrgAreaHeaderNavItem } from './OrgHeader'

export const orgAreaHeaderNavItems: readonly OrgAreaHeaderNavItem[] = [
    {
        to: '/settings',
        label: 'Settings',
        icon: CogOutlineIcon,
        condition: ({ org: { viewerCanAdminister } }) => viewerCanAdminister,
    },
    {
        to: '/searches',
        label: 'Saved Searches',
        icon: SavedSearchIcon,
        condition: ({ org: { viewerCanAdminister } }) =>
            viewerCanAdminister && window.context?.codeSearchEnabledOnInstance,
    },
    {
        to: '/workflows',
        label: 'Workflows',
        icon: WorkflowIcon,
        condition: ({ org: { viewerCanAdminister } }) =>
            viewerCanAdminister && window.context?.codyEnabledForCurrentUser,
    },
    ...namespaceAreaHeaderNavItems,
]
