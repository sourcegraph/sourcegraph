import AccountMultipleIcon from 'mdi-react/AccountMultipleIcon'
import BrainIcon from 'mdi-react/BrainIcon'
import BriefcaseIcon from 'mdi-react/BriefcaseIcon'
import ChartLineVariantIcon from 'mdi-react/ChartLineVariantIcon'
import CogsIcon from 'mdi-react/CogsIcon'
import ConsoleIcon from 'mdi-react/ConsoleIcon'
import MonitorStarIcon from 'mdi-react/MonitorStarIcon'
import PackageVariantIcon from 'mdi-react/PackageVariantIcon'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'

import { BatchChangesIcon } from '../batches/icons'
import { SHOW_BUSINESS_FEATURES } from '../enterprise/dotcom/productSubscriptions/features'
import { checkRequestAccessAllowed } from '../util/checkRequestAccessAllowed'

import { isPackagesEnabled } from './flags'
import type { SiteAdminSideBarGroup, SiteAdminSideBarGroups } from './SiteAdminSidebar'

const analyticsGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Analytics',
        icon: ChartLineVariantIcon,
    },
    items: [
        {
            label: 'Overview',
            to: '/site-admin/',
            exact: true,
        },
        {
            label: 'Search',
            to: '/site-admin/analytics/search',
            condition: () => window.context?.codeSearchEnabledOnInstance,
        },
        {
            label: 'Cody',
            to: '/site-admin/analytics/cody',
            condition: () => window.context?.codyEnabledOnInstance,
        },
        {
            label: 'Code navigation',
            to: '/site-admin/analytics/code-intel',
            condition: () => window.context?.codeSearchEnabledOnInstance,
        },
        {
            label: 'Users',
            to: '/site-admin/analytics/users',
        },
        {
            label: 'Insights',
            to: '/site-admin/analytics/code-insights',
            condition: ({ codeInsightsEnabled }) => codeInsightsEnabled,
        },
        {
            label: 'Batch changes',
            to: '/site-admin/analytics/batch-changes',
            condition: ({ batchChangesEnabled }) => batchChangesEnabled,
        },
        {
            label: 'Notebooks',
            to: '/site-admin/analytics/notebooks',
            condition: () => window.context?.codeSearchEnabledOnInstance,
        },
        {
            label: 'Search extensions',
            to: '/site-admin/analytics/extensions',
        },
        {
            label: 'Code ownership',
            to: '/site-admin/analytics/own',
            condition: () => window.context?.codeSearchEnabledOnInstance,
        },
        {
            label: 'Feedback survey',
            to: '/site-admin/surveys',
        },
    ],
}

const configurationGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Configuration',
        icon: CogsIcon,
    },
    items: [
        {
            label: 'Site configuration',
            to: '/site-admin/configuration',
        },
        {
            label: 'Global settings',
            to: '/site-admin/global-settings',
        },
        {
            label: 'End user onboarding',
            to: '/site-admin/end-user-onboarding',
            condition: ({ endUserOnboardingEnabled }) => endUserOnboardingEnabled,
        },
        {
            label: 'Feature flags',
            to: '/site-admin/feature-flags',
        },
        {
            label: 'License',
            to: '/site-admin/license',
        },
        {
            label: 'Incoming webhooks',
            to: '/site-admin/webhooks/incoming',
        },
        {
            label: 'Outgoing webhooks',
            to: '/site-admin/webhooks/outgoing',
        },
    ],
}

export const maintenanceGroupHeaderLabel = 'Maintenance'

export const maintenanceGroupMonitoringItemLabel = 'Monitoring'

export const maintenanceGroupInstrumentationItemLabel = 'Instrumentation'

export const maintenanceGroupUpdatesItemLabel = 'Updates'

export const maintenanceGroupMigrationsItemLabel = 'Migrations'

export const maintenanceGroupTracingItemLabel = 'Tracing'

const maintenanceGroup: SiteAdminSideBarGroup = {
    header: {
        label: maintenanceGroupHeaderLabel,
        icon: MonitorStarIcon,
    },
    items: [
        {
            label: maintenanceGroupUpdatesItemLabel,
            to: '/site-admin/updates',
            condition: ({ applianceUpdateTarget }) => applianceUpdateTarget === '',
        },
        {
            label: maintenanceGroupUpdatesItemLabel,
            to: window.context.applianceUpdateTarget,
            condition: ({ applianceUpdateTarget }) => applianceUpdateTarget !== '',
        },
        {
            label: 'Documentation',
            to: '/help',
        },
        {
            label: 'Pings',
            to: '/site-admin/pings',
        },
        {
            label: 'Report a bug',
            to: '/site-admin/report-bug',
        },
        {
            label: maintenanceGroupMigrationsItemLabel,
            to: '/site-admin/migrations',
        },
        {
            label: maintenanceGroupInstrumentationItemLabel,
            to: '/-/debug/',
            source: 'server',
        },
        {
            label: maintenanceGroupMonitoringItemLabel,
            to: '/-/debug/grafana',
            source: 'server',
        },
        {
            label: maintenanceGroupTracingItemLabel,
            to: '/-/debug/jaeger',
            source: 'server',
        },
        {
            label: 'Outbound requests',
            to: '/site-admin/outbound-requests',
        },
        {
            label: 'Slow requests',
            to: '/site-admin/slow-requests',
        },
        {
            label: 'Background jobs',
            to: '/site-admin/background-jobs',
        },
        {
            label: 'Code Insights jobs',
            to: '/site-admin/code-insights-jobs',
            condition: ({ codeInsightsEnabled }) => codeInsightsEnabled,
        },
    ],
}

const executorsGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Executors',
        icon: PackageVariantIcon,
    },
    condition: () => Boolean(window.context?.executorsEnabled),
    items: [
        {
            to: '/site-admin/executors',
            label: 'Instances',
            exact: true,
        },
        {
            to: '/site-admin/executors/secrets',
            label: 'Secrets',
        },
    ],
}

export const batchChangesGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Batch Changes',
        icon: BatchChangesIcon,
    },
    items: [
        {
            label: 'Settings',
            to: '/site-admin/batch-changes',
        },
        {
            label: 'Batch specs',
            to: '/site-admin/batch-changes/specs',
            condition: props => props.batchChangesExecutionEnabled,
        },
    ],
    condition: ({ batchChangesEnabled }) => batchChangesEnabled,
}

const businessGroup: SiteAdminSideBarGroup = {
    header: { label: 'Business', icon: BriefcaseIcon },
    items: [
        {
            label: 'Enterprise subscriptions',
            to: '/site-admin/dotcom/product/subscriptions',
            condition: () => SHOW_BUSINESS_FEATURES,
        },
        {
            label: 'License key lookup',
            to: '/site-admin/dotcom/product/licenses',
            condition: () => SHOW_BUSINESS_FEATURES,
        },
    ],
    condition: () => SHOW_BUSINESS_FEATURES,
}

const codeIntelGroup: SiteAdminSideBarGroup = {
    header: { label: 'Code graph', icon: BrainIcon },
    items: [
        {
            to: '/site-admin/code-graph/dashboard',
            label: 'Dashboard',
        },
        {
            to: '/site-admin/code-graph/indexes',
            label: 'Precise indexes',
        },
        {
            to: '/site-admin/code-graph/configuration',
            label: 'Configuration',
        },
        {
            to: '/site-admin/code-graph/inference-configuration',
            label: 'Inference',
            condition: () => window.context?.codeIntelAutoIndexingEnabled,
        },
        {
            to: '/site-admin/code-graph/ranking',
            label: 'Ranking',
            condition: () => window.context?.codeIntelRankingDocumentReferenceCountsEnabled,
        },
        {
            label: 'Ownership signals',
            to: '/site-admin/own-signal-page',
        },
    ],
    condition: () => window.context?.codeSearchEnabledOnInstance,
}

const usersGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Users & auth',
        icon: AccountMultipleIcon,
    },
    items: [
        {
            label: 'Users',
            to: '/site-admin/users',
        },
        {
            label: 'Account requests',
            to: '/site-admin/account-requests',
            condition: () => checkRequestAccessAllowed(window.context),
        },
        {
            label: 'Organizations',
            to: '/site-admin/organizations',
        },
        {
            label: 'Access tokens',
            to: '/site-admin/tokens',
        },
        {
            label: 'Roles',
            to: '/site-admin/roles',
        },
        {
            label: 'Permissions',
            to: '/site-admin/permissions-syncs',
        },
    ],
}

const repositoriesGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Repositories',
        icon: SourceRepositoryIcon,
    },
    items: [
        {
            label: 'Code host connections',
            to: '/site-admin/external-services',
        },
        {
            label: 'Repositories',
            to: '/site-admin/repositories',
        },
        {
            label: 'GitHub Apps',
            to: '/site-admin/github-apps',
        },
        {
            label: 'Packages',
            to: '/site-admin/packages',
            condition: isPackagesEnabled,
        },
        {
            label: 'Gitservers',
            to: '/site-admin/gitservers',
        },
    ],
}

const apiConsoleGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'API Console',
        icon: ConsoleIcon,
    },
    items: [
        {
            label: 'API Console',
            to: '/api/console',
        },
    ],
}

export const siteAdminSidebarGroups: SiteAdminSideBarGroups = [
    analyticsGroup,
    configurationGroup,
    repositoriesGroup,
    codeIntelGroup,
    usersGroup,
    executorsGroup,
    maintenanceGroup,
    batchChangesGroup,
    businessGroup,
    apiConsoleGroup,
].filter(Boolean) as SiteAdminSideBarGroups
