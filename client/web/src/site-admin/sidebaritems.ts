import EarthIcon from 'mdi-react/EarthIcon'
import UsersIcon from 'mdi-react/UsersIcon'
import { SiteAdminSideBarGroup, SiteAdminSideBarGroups } from './SiteAdminSidebar'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'
import CogsIcon from 'mdi-react/CogsIcon'
import MonitorStarIcon from 'mdi-react/MonitorStarIcon'
import ConsoleIcon from 'mdi-react/ConsoleIcon'
import PuzzleOutlineIcon from 'mdi-react/PuzzleOutlineIcon'
import BrainIcon from 'mdi-react/BrainIcon'
import BriefcaseIcon from 'mdi-react/BriefcaseIcon'
import { SHOW_BUSINESS_FEATURES } from '../enterprise/dotcom/productSubscriptions/features'

export const overviewGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Statistics',
        icon: EarthIcon,
    },
    items: [
        {
            label: 'Overview',
            to: '/site-admin',
            exact: true,
        },
        {
            label: 'Usage stats',
            to: '/site-admin/usage-statistics',
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
            label: 'License',
            to: '/site-admin/license',
        },
    ],
}

export const repositoriesGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Repositories',
        icon: SourceRepositoryIcon,
    },
    items: [
        {
            label: 'Manage code hosts',
            to: '/site-admin/external-services',
        },
        {
            label: 'Repository status',
            to: '/site-admin/repositories',
        },
    ],
}

export const usersGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Users & auth',
        icon: UsersIcon,
    },
    items: [
        {
            label: 'Users',
            to: '/site-admin/users',
        },
        {
            label: 'Organizations',
            to: '/site-admin/organizations',
        },
        {
            label: 'Access tokens',
            to: '/site-admin/tokens',
        },
    ],
}

export const maintenanceGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Maintenance',
        icon: MonitorStarIcon,
    },
    items: [
        {
            label: 'Updates',
            to: '/site-admin/updates',
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
            label: 'Instrumentation',
            to: '/-/debug/',
        },
        {
            label: 'Monitoring',
            to: '/-/debug/grafana',
        },
        {
            label: 'Tracing',
            to: '/-/jaeger',
        },
    ],
}

export const extensionsGroup: SiteAdminSideBarGroup = {
    header: {
        label: 'Extensions',
        icon: PuzzleOutlineIcon,
    },
    items: [
        {
            label: 'Extensions',
            to: '/site-admin/registry/extensions',
        },
    ],
}

export const apiConsoleGroup: SiteAdminSideBarGroup = {
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

export const businessGroup: SiteAdminSideBarGroup = {
    header: { label: 'Business', icon: BriefcaseIcon },
    items: [
        {
            label: 'Customers',
            to: '/site-admin/dotcom/customers',
            condition: () => SHOW_BUSINESS_FEATURES,
        },
        {
            label: 'Subscriptions',
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

export const codeIntelGroup: SiteAdminSideBarGroup = {
    header: { label: 'Code intelligence', icon: BrainIcon },
    items: [
        {
            to: '/site-admin/code-intelligence/uploads',
            label: 'Uploads',
        },
        {
            to: '/site-admin/code-intelligence/indexes',
            label: 'Auto indexing',
            condition: () => Boolean(window.context?.codeIntelAutoIndexingEnabled),
        },
    ],
}

export const siteAdminSidebarGroups: SiteAdminSideBarGroups = [
    overviewGroup,
    configurationGroup,
    repositoriesGroup,
    codeIntelGroup,
    usersGroup,
    maintenanceGroup,
    extensionsGroup,
    businessGroup,
    apiConsoleGroup,
]
