import { canWriteBatchChanges } from '../../batches/utils'
import { SHOW_BUSINESS_FEATURES } from '../../enterprise/dotcom/productSubscriptions/features'

import type { UserSettingsSidebarItems } from './UserSettingsSidebar'

export const userSettingsSideBarItems: UserSettingsSidebarItems = [
    {
        label: 'Settings',
        to: '',
        exact: true,
    },
    {
        label: 'Profile',
        to: '/profile',
        exact: true,
    },
    {
        to: '/batch-changes',
        label: 'Batch Changes',
        condition: ({ batchChangesEnabled, user: { viewerCanAdminister }, authenticatedUser }) =>
            batchChangesEnabled && viewerCanAdminister && canWriteBatchChanges(authenticatedUser),
    },
    {
        to: '/executors/secrets',
        label: 'Executor secrets',
        condition: ({ batchChangesEnabled, user: { viewerCanAdminister }, authenticatedUser }) =>
            batchChangesEnabled && viewerCanAdminister && canWriteBatchChanges(authenticatedUser),
    },
    {
        label: 'Emails',
        to: '/emails',
        exact: true,
    },
    {
        label: 'Access tokens',
        to: '/tokens',
        condition: () => window.context.accessTokensAllow !== 'none',
    },
    {
        label: 'Account security',
        to: '/security',
        exact: true,
    },
    {
        label: 'Enterprise subscriptions',
        to: '/subscriptions',
        condition: ({ user }) => SHOW_BUSINESS_FEATURES && user.viewerCanAdminister,
    },
    {
        label: 'Quotas',
        to: '/quota',
        exact: true,
        condition: ({ authenticatedUser }) => authenticatedUser.siteAdmin,
    },
    {
        label: 'Product research',
        to: '/product-research',
        condition: () => window.context.productResearchPageEnabled,
    },
    {
        label: 'Permissions',
        to: '/permissions',
        exact: true,
    },
    {
        to: '/event-log',
        label: 'Event log',
        condition: ({ user: { viewerCanAdminister } }) => viewerCanAdminister,
    },
]
