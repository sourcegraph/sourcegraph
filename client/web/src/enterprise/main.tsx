// This is the entry point for the enterprise web app

// Order is important here
// Don't remove the empty lines between these imports

import '../../../shared/src/polyfills'

import '../sentry'

import React from 'react'
import { render } from 'react-dom'
import '../enterprise.scss'
import { SourcegraphWebApp } from '../SourcegraphWebApp'
import { enterpriseExtensionAreaHeaderNavItems } from './extensions/extension/extensionAreaHeaderNavItems'
import { enterpriseExtensionAreaRoutes } from './extensions/extension/routes'
import { enterpriseExtensionsAreaHeaderActionButtons } from './extensions/extensionsAreaHeaderActionButtons'
import { enterpriseExtensionsAreaRoutes } from './extensions/routes'
import { enterpriseOrgAreaHeaderNavItems } from './organizations/navitems'
import { enterpriseOrganizationAreaRoutes } from './organizations/routes'
import { enterpriseRepoHeaderActionButtons } from './repo/repoHeaderActionButtons'
import { enterpriseRepoContainerRoutes, enterpriseRepoRevisionContainerRoutes } from './repo/routes'
import { enterpriseRoutes } from './routes'
import { enterpriseSiteAdminOverviewComponents } from './site-admin/overview/overviewComponents'
import { enterpriseSiteAdminAreaRoutes } from './site-admin/routes'
import { siteAdminSidebarGroups } from '../site-admin/sidebaritems'
import { enterpriseUserAreaHeaderNavItems } from './user/navitems'
import { enterpriseUserAreaRoutes } from './user/routes'
import { enterpriseUserSettingsAreaRoutes } from './user/settings/routes'
import { enterpriseUserSettingsSideBarItems } from './user/settings/sidebaritems'
import { KEYBOARD_SHORTCUTS } from '../keyboardShortcuts/keyboardShortcuts'
import { enterpriseRepoSettingsAreaRoutes } from './repo/settings/routes'
import { enterpriseRepoSettingsSidebarGroups } from './repo/settings/sidebaritems'

window.addEventListener('DOMContentLoaded', () => {
    render(
        <SourcegraphWebApp
            extensionAreaRoutes={enterpriseExtensionAreaRoutes}
            extensionAreaHeaderNavItems={enterpriseExtensionAreaHeaderNavItems}
            extensionsAreaRoutes={enterpriseExtensionsAreaRoutes}
            extensionsAreaHeaderActionButtons={enterpriseExtensionsAreaHeaderActionButtons}
            siteAdminAreaRoutes={enterpriseSiteAdminAreaRoutes}
            siteAdminSideBarGroups={siteAdminSidebarGroups}
            siteAdminOverviewComponents={enterpriseSiteAdminOverviewComponents}
            userAreaHeaderNavItems={enterpriseUserAreaHeaderNavItems}
            userAreaRoutes={enterpriseUserAreaRoutes}
            userSettingsSideBarItems={enterpriseUserSettingsSideBarItems}
            userSettingsAreaRoutes={enterpriseUserSettingsAreaRoutes}
            orgAreaRoutes={enterpriseOrganizationAreaRoutes}
            orgAreaHeaderNavItems={enterpriseOrgAreaHeaderNavItems}
            repoContainerRoutes={enterpriseRepoContainerRoutes}
            repoRevisionContainerRoutes={enterpriseRepoRevisionContainerRoutes}
            repoHeaderActionButtons={enterpriseRepoHeaderActionButtons}
            repoSettingsAreaRoutes={enterpriseRepoSettingsAreaRoutes}
            repoSettingsSidebarGroups={enterpriseRepoSettingsSidebarGroups}
            routes={enterpriseRoutes}
            keyboardShortcuts={KEYBOARD_SHORTCUTS}
            showBatchChanges={window.context.batchChangesEnabled}
        />,
        document.querySelector('#root')
    )
})
