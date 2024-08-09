import { Navigate, useLocation } from 'react-router-dom'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { FeedbackBadge } from '@sourcegraph/wildcard'

import type { BatchSpecsPageProps } from '../enterprise/batches/BatchSpecsPage'
import { SHOW_BUSINESS_FEATURES } from '../enterprise/dotcom/productSubscriptions/features'
import { OwnAnalyticsPage } from '../enterprise/own/admin-ui/OwnAnalyticsPage'
import type { SiteAdminRolesPageProps } from '../enterprise/rbac/SiteAdminRolesPage'
import type { RoleAssignmentModalProps } from '../enterprise/site-admin/UserManagement/components/RoleAssignmentModal'
import { GitHubAppKind } from '../graphql-operations'
import { checkRequestAccessAllowed } from '../util/checkRequestAccessAllowed'

import { isPackagesEnabled } from './flags'
import { PermissionsSyncJobsTable } from './permissions-center/PermissionsSyncJobsTable'
import type { SiteAdminAreaRoute } from './SiteAdminArea'

const AnalyticsOverviewPage = lazyComponent(() => import('./analytics/AnalyticsOverviewPage'), 'AnalyticsOverviewPage')
const AnalyticsSearchPage = lazyComponent(() => import('./analytics/AnalyticsSearchPage'), 'AnalyticsSearchPage')
const AnalyticsCodeIntelPage = lazyComponent(
    () => import('./analytics/AnalyticsCodeIntelPage'),
    'AnalyticsCodeIntelPage'
)
const AnalyticsExtensionsPage = lazyComponent(
    () => import('./analytics/AnalyticsExtensionsPage'),
    'AnalyticsExtensionsPage'
)
const AnalyticsUsersPage = lazyComponent(() => import('./analytics/AnalyticsUsersPage'), 'AnalyticsUsersPage')
const AnalyticsCodyPage = lazyComponent(() => import('./analytics/AnalyticsCodyPage'), 'AnalyticsCodyPage')
const AnalyticsCodeInsightsPage = lazyComponent(
    () => import('./analytics/AnalyticsCodeInsightsPage'),
    'AnalyticsCodeInsightsPage'
)
const AnalyticsBatchChangesPage = lazyComponent(
    () => import('./analytics/AnalyticsBatchChangesPage'),
    'AnalyticsBatchChangesPage'
)
const AnalyticsNotebooksPage = lazyComponent(
    () => import('./analytics/AnalyticsNotebooksPage'),
    'AnalyticsNotebooksPage'
)
const SiteAdminConfigurationPage = lazyComponent(
    () => import('./SiteAdminConfigurationPage'),
    'SiteAdminConfigurationPage'
)
const SiteAdminSettingsPage = lazyComponent(() => import('./SiteAdminSettingsPage'), 'SiteAdminSettingsPage')
const SiteAdminOnboardingTourPage = lazyComponent(
    () => import('./SiteAdminOnboardingTourPage'),
    'SiteAdminOnboardingTourPage'
)
const SiteAdminExternalServicesArea = lazyComponent(
    () => import('./SiteAdminExternalServicesArea'),
    'SiteAdminExternalServicesArea'
)
const SiteAdminGitHubAppsArea = lazyComponent(() => import('./SiteAdminGitHubAppsArea'), 'SiteAdminGitHubAppsArea')
const SiteAdminRepositoriesPage = lazyComponent(
    () => import('./SiteAdminRepositoriesPage'),
    'SiteAdminRepositoriesPage'
)
const SiteAdminOrgsPage = lazyComponent(() => import('./SiteAdminOrgsPage'), 'SiteAdminOrgsPage')
export const UsersManagement = lazyComponent(() => import('./UserManagement'), 'UsersManagement')
const AccessRequestsPage = lazyComponent(() => import('./AccessRequestsPage'), 'AccessRequestsPage')

const SiteAdminCreateUserPage = lazyComponent(() => import('./SiteAdminCreateUserPage'), 'SiteAdminCreateUserPage')
const SiteAdminTokensPage = lazyComponent(() => import('./SiteAdminTokensPage'), 'SiteAdminTokensPage')
const SiteAdminUpdatesPage = lazyComponent(() => import('./SiteAdminUpdatesPage'), 'SiteAdminUpdatesPage')
const SiteAdminPingsPage = lazyComponent(() => import('./SiteAdminPingsPage'), 'SiteAdminPingsPage')
const SiteAdminReportBugPage = lazyComponent(() => import('./SiteAdminReportBugPage'), 'SiteAdminReportBugPage')
const SiteAdminSurveyResponsesPage = lazyComponent(
    () => import('./SiteAdminSurveyResponsesPage'),
    'SiteAdminSurveyResponsesPage'
)
const SiteAdminMigrationsPage = lazyComponent(() => import('./SiteAdminMigrationsPage'), 'SiteAdminMigrationsPage')
const SiteAdminOutboundRequestsPage = lazyComponent(
    () => import('./SiteAdminOutboundRequestsPage'),
    'SiteAdminOutboundRequestsPage'
)
const SiteAdminBackgroundJobsPage = lazyComponent(
    () => import('./SiteAdminBackgroundJobsPage'),
    'SiteAdminBackgroundJobsPage'
)
const SiteAdminFeatureFlagsPage = lazyComponent(
    () => import('./SiteAdminFeatureFlagsPage'),
    'SiteAdminFeatureFlagsPage'
)
const SiteAdminFeatureFlagConfigurationPage = lazyComponent(
    () => import('./SiteAdminFeatureFlagConfigurationPage'),
    'SiteAdminFeatureFlagConfigurationPage'
)
const OutboundWebhooksPage = lazyComponent(
    () => import('./outbound-webhooks/OutboundWebhooksPage'),
    'OutboundWebhooksPage'
)
const OutgoingWebhookCreatePage = lazyComponent(() => import('./outbound-webhooks/CreatePage'), 'CreatePage')
const OutgoingWebhookEditPage = lazyComponent(() => import('./outbound-webhooks/EditPage'), 'EditPage')
const SiteAdminWebhooksPage = lazyComponent(() => import('./SiteAdminWebhooksPage'), 'SiteAdminWebhooksPage')
const SiteAdminWebhookCreatePage = lazyComponent(
    () => import('./SiteAdminWebhookCreatePage'),
    'SiteAdminWebhookCreatePage'
)
const SiteAdminWebhookPage = lazyComponent(() => import('./SiteAdminWebhookPage'), 'SiteAdminWebhookPage')
const SiteAdminSlowRequestsPage = lazyComponent(
    () => import('./SiteAdminSlowRequestsPage'),
    'SiteAdminSlowRequestsPage'
)
const SiteAdminWebhookUpdatePage = lazyComponent(
    () => import('./SiteAdminWebhookUpdatePage'),
    'SiteAdminWebhookUpdatePage'
)
const SiteAdminPackagesPage = lazyComponent(() => import('./SiteAdminPackagesPage'), 'SiteAdminPackagesPage')
const GitserversPageProps = lazyComponent(() => import('./SiteAdminGitserversPage'), 'SiteAdminGitserversPage')

const SiteAdminProductSubscriptionPage = lazyComponent(
    () => import('../enterprise/site-admin/productSubscription/SiteAdminProductSubscriptionPage'),
    'SiteAdminProductSubscriptionPage'
)
const SiteAdminCreateProductSubscriptionPage = lazyComponent(
    () => import('../enterprise/site-admin/dotcom/productSubscriptions/SiteAdminCreateProductSubscriptionPage'),
    'SiteAdminCreateProductSubscriptionPage'
)
const DotComSiteAdminProductSubscriptionPage = lazyComponent(
    () => import('../enterprise/site-admin/dotcom/productSubscriptions/SiteAdminProductSubscriptionPage'),
    'SiteAdminProductSubscriptionPage'
)
const SiteAdminProductSubscriptionsPage = lazyComponent(
    () => import('../enterprise/site-admin/dotcom/productSubscriptions/SiteAdminProductSubscriptionsPage'),
    'SiteAdminProductSubscriptionsPage'
)
const SiteAdminLicenseKeyLookupPage = lazyComponent(
    () => import('../enterprise/site-admin/dotcom/productSubscriptions/SiteAdminLicenseKeyLookupPage'),
    'SiteAdminLicenseKeyLookupPage'
)
const SiteAdminAuthenticationProvidersPage = lazyComponent(
    () => import('../enterprise/site-admin/SiteAdminAuthenticationProvidersPage'),
    'SiteAdminAuthenticationProvidersPage'
)
const SiteAdminExternalAccountsPage = lazyComponent(
    () => import('../enterprise/site-admin/SiteAdminExternalAccountsPage'),
    'SiteAdminExternalAccountsPage'
)
const BatchChangesSiteConfigSettingsPage = lazyComponent(
    () => import('../enterprise/batches/settings/BatchChangesSiteConfigSettingsPage'),
    'BatchChangesSiteConfigSettingsPage'
)
const BatchChangesCreateGitHubAppPage = lazyComponent(
    () => import('../enterprise/batches/settings/BatchChangesCreateGitHubAppPage'),
    'BatchChangesCreateGitHubAppPage'
)
const GitHubAppPage = lazyComponent(() => import('../components/gitHubApps/GitHubAppPage'), 'GitHubAppPage')
const BatchSpecsPage = lazyComponent<BatchSpecsPageProps, 'BatchSpecsPage'>(
    () => import('../enterprise/batches/BatchSpecsPage'),
    'BatchSpecsPage'
)
const AdminCodeIntelArea = lazyComponent(
    () => import('../enterprise/codeintel/admin/AdminCodeIntelArea'),
    'AdminCodeIntelArea'
)
const SiteAdminPreciseIndexPage = lazyComponent(
    () => import('../enterprise/site-admin/SiteAdminPreciseIndexPage'),
    'SiteAdminPreciseIndexPage'
)
const ExecutorsSiteAdminArea = lazyComponent(
    () => import('../enterprise/executors/ExecutorsSiteAdminArea'),
    'ExecutorsSiteAdminArea'
)

const SiteAdminRolesPage = lazyComponent<SiteAdminRolesPageProps, 'SiteAdminRolesPage'>(
    () => import('../enterprise/rbac/SiteAdminRolesPage'),
    'SiteAdminRolesPage'
)

const RoleAssignmentModal = lazyComponent<RoleAssignmentModalProps, 'RoleAssignmentModal'>(
    () => import('../enterprise/site-admin/UserManagement/components/RoleAssignmentModal'),
    'RoleAssignmentModal'
)

const CodeInsightsJobsPage = lazyComponent(
    () => import('../enterprise/insights/admin-ui/CodeInsightsJobs'),
    'CodeInsightsJobs'
)
const OwnStatusPage = lazyComponent(() => import('../enterprise/own/admin-ui/OwnStatusPage'), 'OwnStatusPage')

export const otherSiteAdminRoutes: readonly SiteAdminAreaRoute[] = [
    {
        path: '/',
        render: props => <AnalyticsOverviewPage {...props} />,
    },
    {
        path: '/analytics/search',
        render: props => <AnalyticsSearchPage {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
    {
        path: '/analytics/code-intel',
        render: props => <AnalyticsCodeIntelPage {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
    {
        path: '/analytics/extensions',
        render: props => <AnalyticsExtensionsPage {...props} />,
    },
    {
        path: '/analytics/users',
        render: props => <AnalyticsUsersPage {...props} />,
    },
    {
        path: '/analytics/cody',
        render: props => <AnalyticsCodyPage {...props} />,
        condition: () => window.context?.codyEnabledOnInstance,
    },
    {
        path: '/analytics/code-insights',
        render: props => <AnalyticsCodeInsightsPage {...props} />,
        condition: ({ codeInsightsEnabled }) => codeInsightsEnabled,
    },
    {
        path: '/analytics/batch-changes',
        render: props => <AnalyticsBatchChangesPage {...props} />,
        condition: ({ batchChangesEnabled }) => batchChangesEnabled,
    },
    {
        path: '/analytics/notebooks',
        render: props => <AnalyticsNotebooksPage {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
    {
        path: '/configuration',
        render: props => <SiteAdminConfigurationPage {...props} />,
    },
    {
        path: '/global-settings',
        render: props => <SiteAdminSettingsPage {...props} />,
    },
    {
        path: '/end-user-onboarding',
        render: props => <SiteAdminOnboardingTourPage {...props} />,
        condition: ({ endUserOnboardingEnabled }) => endUserOnboardingEnabled,
    },
    {
        path: '/github-apps/*',
        render: props => <SiteAdminGitHubAppsArea {...props} />,
    },
    {
        path: '/external-services/*',
        render: props => <SiteAdminExternalServicesArea {...props} />,
    },
    {
        path: '/repositories',
        render: props => <SiteAdminRepositoriesPage {...props} />,
    },
    {
        path: '/organizations',
        render: props => <SiteAdminOrgsPage {...props} />,
    },
    {
        path: '/account-requests',
        render: props => <AccessRequestsPage {...props} />,
        condition: () => checkRequestAccessAllowed(window.context),
    },
    {
        path: '/users/new',
        render: props => <SiteAdminCreateUserPage {...props} />,
    },
    {
        path: '/tokens',
        render: props => <SiteAdminTokensPage {...props} />,
    },
    {
        path: '/updates',
        render: props => <SiteAdminUpdatesPage {...props} />,
    },
    {
        path: '/pings',
        render: props => <SiteAdminPingsPage {...props} />,
    },
    {
        path: '/report-bug',
        render: props => <SiteAdminReportBugPage {...props} />,
    },
    {
        path: '/surveys',
        render: props => <SiteAdminSurveyResponsesPage {...props} />,
    },
    {
        path: '/migrations',
        render: props => <SiteAdminMigrationsPage {...props} />,
    },
    {
        path: '/outbound-requests',
        render: props => <SiteAdminOutboundRequestsPage {...props} />,
    },
    {
        path: '/background-jobs',
        render: props => <SiteAdminBackgroundJobsPage {...props} />,
    },
    {
        path: '/feature-flags',
        render: props => <SiteAdminFeatureFlagsPage {...props} />,
    },
    {
        path: '/feature-flags/configuration/:name',
        render: props => <SiteAdminFeatureFlagConfigurationPage {...props} />,
    },
    {
        path: '/webhooks/outgoing',
        render: props => <OutboundWebhooksPage {...props} />,
    },
    {
        path: '/webhooks/outgoing/create',
        render: props => <OutgoingWebhookCreatePage {...props} />,
    },
    {
        path: '/webhooks/outgoing/:id',
        render: props => <OutgoingWebhookEditPage {...props} />,
    },
    {
        path: '/webhooks/incoming',
        render: props => <SiteAdminWebhooksPage {...props} />,
    },
    {
        path: '/webhooks/incoming/create',
        render: props => <SiteAdminWebhookCreatePage {...props} />,
    },
    {
        path: '/webhooks/incoming/:id',
        render: props => <SiteAdminWebhookPage {...props} />,
    },
    {
        path: '/webhooks/incoming/:id/edit',
        render: props => <SiteAdminWebhookUpdatePage {...props} />,
    },
    {
        path: '/slow-requests',
        render: props => <SiteAdminSlowRequestsPage {...props} />,
    },
    {
        path: '/packages',
        render: props => <SiteAdminPackagesPage {...props} />,
        condition: isPackagesEnabled,
    },
    {
        path: '/permissions-syncs',
        render: props => <PermissionsSyncJobsTable {...props} />,
    },
    {
        path: '/gitservers',
        render: props => <GitserversPageProps {...props} />,
    },
    {
        path: '/users',
        render: props => (
            <UsersManagement
                renderAssignmentModal={(onCancel, onSuccess, user) => (
                    <RoleAssignmentModal onCancel={onCancel} onSuccess={onSuccess} user={user} />
                )}
                {...props}
            />
        ),
    },
    {
        path: '/license',
        render: props => (
            <SiteAdminProductSubscriptionPage telemetryRecorder={props.platformContext.telemetryRecorder} />
        ),
    },
    {
        path: '/dotcom/product/subscriptions/new',
        render: props => <SiteAdminCreateProductSubscriptionPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/dotcom/product/subscriptions/:subscriptionUUID',
        render: props => <DotComSiteAdminProductSubscriptionPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/dotcom/product/subscriptions',
        render: props => (
            <SiteAdminProductSubscriptionsPage {...props} telemetryRecorder={props.platformContext.telemetryRecorder} />
        ),
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/dotcom/product/licenses',
        render: props => <SiteAdminLicenseKeyLookupPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/auth/providers',
        render: () => <SiteAdminAuthenticationProvidersPage />,
    },
    {
        path: '/auth/external-accounts',
        render: () => <SiteAdminExternalAccountsPage />,
    },
    {
        path: '/batch-changes',
        render: props => <BatchChangesSiteConfigSettingsPage {...props} />,
        condition: ({ batchChangesEnabled }) => batchChangesEnabled,
    },
    {
        path: '/batch-changes/github-apps/new',
        render: ({ authenticatedUser }) => (
            <BatchChangesCreateGitHubAppPage
                authenticatedUser={authenticatedUser}
                kind={GitHubAppKind.COMMIT_SIGNING}
            />
        ),
        condition: ({ batchChangesEnabled }) => batchChangesEnabled,
    },
    {
        path: '/batch-changes/github-apps/:appID',
        render: props => (
            <GitHubAppPage
                headerParentBreadcrumb={{ to: '/site-admin/batch-changes', text: 'Batch Changes settings' }}
                headerAnnotation={<FeedbackBadge status="beta" feedback={{ mailto: 'support@sourcegraph.com' }} />}
                telemetryService={props.telemetryService}
                telemetryRecorder={props.platformContext.telemetryRecorder}
            />
        ),
        condition: ({ batchChangesEnabled }) => batchChangesEnabled,
    },
    {
        path: '/batch-changes/specs',
        render: props => <BatchSpecsPage {...props} />,
        condition: ({ batchChangesEnabled, batchChangesExecutionEnabled }) =>
            batchChangesEnabled && batchChangesExecutionEnabled,
    },
    // Old batch changes webhooks logs page redirects to new incoming webhooks page.
    // The old page components and documentation are still available in the codebase
    // but should be fully removed in the next release.
    {
        path: '/batch-changes/webhook-logs',
        render: () => <Navigate to="/site-admin/webhooks/incoming" replace={true} />,
        condition: ({ batchChangesEnabled, batchChangesWebhookLogsEnabled }) =>
            batchChangesEnabled && batchChangesWebhookLogsEnabled,
    },

    // Enterprise maintenance area

    {
        exact: true,
        path: '/code-insights-jobs',
        render: props => <CodeInsightsJobsPage {...props} />,
        condition: ({ codeInsightsEnabled }) => codeInsightsEnabled,
    },
    {
        exact: true,
        path: '/own-signal-page',
        render: props => <OwnStatusPage telemetryRecorder={props.platformContext.telemetryRecorder} />,
    },

    // Code intelligence redirect
    {
        path: '/code-intelligence/*',
        render: () => <NavigateToCodeGraph />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
    // Code graph routes
    {
        path: '/code-graph/*',
        render: props => <AdminCodeIntelArea {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
    {
        path: '/lsif-uploads/:id',
        render: props => <SiteAdminPreciseIndexPage {...props} />,
    },

    // Executor routes
    {
        path: '/executors/*',
        render: props => <ExecutorsSiteAdminArea telemetryRecorder={props.platformContext.telemetryRecorder} />,
        condition: () => Boolean(window.context?.executorsEnabled),
    },

    // rbac-related routes
    {
        path: '/roles',
        exact: true,
        render: props => <SiteAdminRolesPage telemetryRecorder={props.platformContext.telemetryRecorder} />,
    },

    // Own analytics
    {
        exact: true,
        path: '/analytics/own',
        render: props => <OwnAnalyticsPage telemetryRecorder={props.platformContext.telemetryRecorder} />,
    },
]

function NavigateToCodeGraph(): JSX.Element {
    const location = useLocation()
    return <Navigate to={location.pathname.replace('/code-intelligence', '/code-graph')} />
}

const siteAdminUserManagementRoute: SiteAdminAreaRoute = {
    path: '/users',
    render: props => <UsersManagement renderAssignmentModal={() => null} {...props} />,
}

export const siteAdminAreaRoutes: readonly SiteAdminAreaRoute[] = [
    ...otherSiteAdminRoutes,
    siteAdminUserManagementRoute,
]
