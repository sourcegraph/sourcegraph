import { UserSettingsSidebarItems } from './UserSettingsSidebar'
import { showAccountSecurityPage, showPasswordsPage, allowUserExternalServicePublic } from './cloud-ga'

export const userSettingsSideBarItems: UserSettingsSidebarItems = {
    account: [
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
            label: 'Password',
            to: '/password',
            exact: true,
            // Only the builtin auth provider has a password.
            condition: showPasswordsPage,
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
        //  future GA Cloud nav items
        {
            label: 'Account security',
            to: '/security',
            exact: true,
            condition: showAccountSecurityPage,
        },
        {
            label: 'Code host connections',
            to: '/code-hosts',
            condition: allowUserExternalServicePublic,
        },
        {
            label: 'Repositories',
            to: '/repositories',
            condition: allowUserExternalServicePublic,
        },
        {
            label: 'Product research',
            to: '/product-research',
            condition: () => window.context.productResearchPageEnabled,
            status: 'new',
        },
    ],
}
