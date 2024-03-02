import React, { useRef } from 'react'

import classNames from 'classnames'
import BriefcaseIcon from 'mdi-react/BriefcaseIcon'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import { Routes, Route, Navigate } from 'react-router-dom'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { PageHeader, LoadingSpinner } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { withAuthenticatedUser } from '../auth/withAuthenticatedUser'
import { RouteError } from '../components/ErrorBoundary'
import { HeroPage } from '../components/HeroPage'
import { Page } from '../components/Page'
import { SHOW_BUSINESS_FEATURES } from '../enterprise/dotcom/productSubscriptions/features'
import { canReadLicenseManagement } from '../rbac/check'
import { SiteAdminSidebar } from '../site-admin/SiteAdminSidebar'
import type { RouteV6Descriptor } from '../util/contributions'

import styles from './LicenseManagementArea.module.scss'

const SiteAdminProductCustomersPage = lazyComponent(
    () => import('../enterprise/site-admin/dotcom/customers/SiteAdminCustomersPage'),
    'SiteAdminProductCustomersPage'
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

const NotFoundPage: React.ComponentType<React.PropsWithChildren<{}>> = () => (
    <HeroPage icon={MapSearchIcon} title="404: Not Found" subtitle="Sorry, the requested page was not found." />
)

const NotAllowedPage: React.ComponentType<React.PropsWithChildren<{}>> = () => (
    <HeroPage icon={MapSearchIcon} title="403: Forbidden" subtitle="Only license managers are allowed here." />
)

const routes: LicenseManagementAreaRoute[] = [
    {
        path: '/customers',
        render: props => <SiteAdminProductCustomersPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/product/subscriptions/new',
        render: props => <SiteAdminCreateProductSubscriptionPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/product/subscriptions/:subscriptionUUID',
        render: props => <DotComSiteAdminProductSubscriptionPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/product/subscriptions',
        render: props => <SiteAdminProductSubscriptionsPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
    {
        path: '/product/licenses',
        render: props => <SiteAdminLicenseKeyLookupPage {...props} />,
        condition: () => SHOW_BUSINESS_FEATURES,
    },
]

interface LicenseManagementAreaRouteContext {
    authenticatedUser: AuthenticatedUser
}

export interface LicenseManagementAreaRoute extends RouteV6Descriptor<LicenseManagementAreaRouteContext> {}

interface LicenseManagementAreaProps {
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean
}

const AuthenticatedLicenseManagementArea: React.FunctionComponent<
    React.PropsWithChildren<LicenseManagementAreaProps>
> = props => {
    const reference = useRef<HTMLDivElement>(null)

    // If business features are disabled, this Area is not accessible.
    if (!SHOW_BUSINESS_FEATURES) {
        return <Navigate to="/search" replace={true} />
    }

    // If not license manager, bail out.
    if (!canReadLicenseManagement(props.authenticatedUser)) {
        return <NotAllowedPage />
    }

    const context: LicenseManagementAreaRouteContext = {
        authenticatedUser: props.authenticatedUser,
    }

    return (
        <Page>
            <PageHeader>
                <PageHeader.Heading as="h2" styleAs="h1">
                    <PageHeader.Breadcrumb>License Management</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <div className="d-flex my-3 flex-column flex-sm-row" ref={reference}>
                <SiteAdminSidebar
                    className={classNames('flex-0 mr-3 mb-4', styles.sidebar)}
                    groups={[
                        {
                            header: { label: 'License Management', icon: BriefcaseIcon },
                            items: [
                                {
                                    label: 'Customers',
                                    to: '/license-admin/customers',
                                    condition: () => SHOW_BUSINESS_FEATURES,
                                },
                                {
                                    label: 'Subscriptions',
                                    to: '/license-admin/product/subscriptions',
                                    condition: () => SHOW_BUSINESS_FEATURES,
                                },
                                {
                                    label: 'License key lookup',
                                    to: '/license-admin/product/licenses',
                                    condition: () => SHOW_BUSINESS_FEATURES,
                                },
                            ],
                        },
                    ]}
                    isSourcegraphDotCom={props.isSourcegraphDotCom}
                    batchChangesEnabled={false}
                    batchChangesExecutionEnabled={false}
                    batchChangesWebhookLogsEnabled={false}
                    codeInsightsEnabled={false}
                    endUserOnboardingEnabled={false}
                />
                <div className="flex-bounded">
                    <React.Suspense fallback={<LoadingSpinner className="m-2" />}>
                        <Routes>
                            {routes.map(
                                ({ render, path, condition = () => true }) =>
                                    condition(context) && (
                                        <Route
                                            // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                                            key="hardcoded-key"
                                            errorElement={<RouteError />}
                                            path={path}
                                            element={render(context)}
                                        />
                                    )
                            )}
                            <Route path="*" element={<NotFoundPage />} />
                        </Routes>
                    </React.Suspense>
                </div>
            </div>
        </Page>
    )
}

/**
 * Renders a layout of a sidebar and a content area to display the license management UIs.
 */
export const LicenseManagementArea = withAuthenticatedUser(AuthenticatedLicenseManagementArea)
