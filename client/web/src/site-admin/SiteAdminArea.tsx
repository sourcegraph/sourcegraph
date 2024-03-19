import React, { useMemo, useRef } from 'react'

import classNames from 'classnames'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import { Routes, Route } from 'react-router-dom'

import type { SiteSettingFields } from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { PageHeader, LoadingSpinner } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { withAuthenticatedUser } from '../auth/withAuthenticatedUser'
import type { BatchChangesProps } from '../batches'
import { RouteError } from '../components/ErrorBoundary'
import { HeroPage } from '../components/HeroPage'
import { Page } from '../components/Page'
import { useFeatureFlag } from '../featureFlags/useFeatureFlag'
import { useUserExternalAccounts } from '../hooks/useUserExternalAccounts'
import type { RouteV6Descriptor } from '../util/contributions'
import { getLicenseFeatures } from '../util/license'

import {
    maintenanceGroupHeaderLabel,
    maintenanceGroupInstrumentationItemLabel,
    maintenanceGroupMonitoringItemLabel,
    maintenanceGroupMigrationsItemLabel,
    maintenanceGroupUpdatesItemLabel,
    maintenanceGroupTracingItemLabel,
} from './sidebaritems'
import { SiteAdminSidebar, type SiteAdminSideBarGroups } from './SiteAdminSidebar'

import styles from './SiteAdminArea.module.scss'

const NotFoundPage: React.ComponentType<React.PropsWithChildren<{}>> = () => (
    <HeroPage
        icon={MapSearchIcon}
        title="404: Not Found"
        subtitle="Sorry, the requested site admin page was not found."
    />
)

const NotSiteAdminPage: React.ComponentType<React.PropsWithChildren<{}>> = () => (
    <HeroPage icon={MapSearchIcon} title="403: Forbidden" subtitle="Only site admins are allowed here." />
)

export interface SiteAdminAreaRouteContext
    extends PlatformContextProps,
        SettingsCascadeProps,
        BatchChangesProps,
        TelemetryProps,
        TelemetryV2Props {
    site: Pick<SiteSettingFields, '__typename' | 'id'>
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean

    /** This property is only used by {@link SiteAdminOverviewPage}. */
    overviewComponents: readonly React.ComponentType<React.PropsWithChildren<{}>>[]

    codeInsightsEnabled: boolean

    endUserOnboardingEnabled: boolean

    license: {
        isCodeSearchEnabled: boolean
        isCodyEnabled: boolean
    }
}

export interface SiteAdminAreaRoute extends RouteV6Descriptor<SiteAdminAreaRouteContext> {}

interface SiteAdminAreaProps
    extends PlatformContextProps,
        SettingsCascadeProps,
        BatchChangesProps,
        TelemetryProps,
        TelemetryV2Props {
    routes: readonly SiteAdminAreaRoute[]
    sideBarGroups: SiteAdminSideBarGroups
    overviewComponents: readonly React.ComponentType<React.PropsWithChildren<unknown>>[]
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean
    codeInsightsEnabled: boolean
}

const sourcegraphOperatorSiteAdminMaintenanceBlockItems = new Set([
    maintenanceGroupInstrumentationItemLabel,
    maintenanceGroupMonitoringItemLabel,
    maintenanceGroupMigrationsItemLabel,
    maintenanceGroupUpdatesItemLabel,
    maintenanceGroupTracingItemLabel,
])

const AuthenticatedSiteAdminArea: React.FunctionComponent<React.PropsWithChildren<SiteAdminAreaProps>> = props => {
    const reference = useRef<HTMLDivElement>(null)

    const { data: externalAccounts, loading: isExternalAccountsLoading } = useUserExternalAccounts(
        props.authenticatedUser.username
    )
    const [endUserOnboardingEnabled] = useFeatureFlag('end-user-onboarding')
    const [isSourcegraphOperatorSiteAdminHideMaintenance] = useFeatureFlag(
        'sourcegraph-operator-site-admin-hide-maintenance'
    )
    const adminSideBarGroups = useMemo(
        () =>
            props.sideBarGroups.map(group => {
                if (
                    !isSourcegraphOperatorSiteAdminHideMaintenance ||
                    group.header?.label !== maintenanceGroupHeaderLabel ||
                    (!isExternalAccountsLoading &&
                        externalAccounts.some(account => account.serviceType === 'sourcegraph-operator'))
                ) {
                    return group
                }

                return {
                    ...group,
                    items: group.items.filter(
                        item => !sourcegraphOperatorSiteAdminMaintenanceBlockItems.has(item.label)
                    ),
                }
            }),
        [
            props.sideBarGroups,
            isSourcegraphOperatorSiteAdminHideMaintenance,
            isExternalAccountsLoading,
            externalAccounts,
        ]
    )

    // If not site admin, redirect to sign in.
    if (!props.authenticatedUser.siteAdmin) {
        return <NotSiteAdminPage />
    }

    const context: SiteAdminAreaRouteContext = {
        authenticatedUser: props.authenticatedUser,
        platformContext: props.platformContext,
        settingsCascade: props.settingsCascade,
        isSourcegraphDotCom: props.isSourcegraphDotCom,
        batchChangesEnabled: props.batchChangesEnabled,
        batchChangesExecutionEnabled: props.batchChangesExecutionEnabled,
        batchChangesWebhookLogsEnabled: props.batchChangesWebhookLogsEnabled,
        site: { __typename: 'Site' as const, id: window.context.siteGQLID },
        overviewComponents: props.overviewComponents,
        telemetryService: props.telemetryService,
        telemetryRecorder: props.telemetryRecorder,
        codeInsightsEnabled: props.codeInsightsEnabled,
        endUserOnboardingEnabled,
        license: getLicenseFeatures(),
    }

    return (
        <Page>
            <PageHeader>
                <PageHeader.Heading as="h2" styleAs="h1">
                    <PageHeader.Breadcrumb>Admin</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <div className="d-flex my-3 flex-column flex-sm-row" ref={reference}>
                <SiteAdminSidebar
                    license={context.license}
                    className={classNames('flex-0 mr-3 mb-4', styles.sidebar)}
                    groups={adminSideBarGroups}
                    isSourcegraphDotCom={props.isSourcegraphDotCom}
                    batchChangesEnabled={props.batchChangesEnabled}
                    batchChangesExecutionEnabled={props.batchChangesExecutionEnabled}
                    batchChangesWebhookLogsEnabled={props.batchChangesWebhookLogsEnabled}
                    codeInsightsEnabled={props.codeInsightsEnabled}
                    endUserOnboardingEnabled={endUserOnboardingEnabled}
                />
                <div className="flex-bounded">
                    <React.Suspense fallback={<LoadingSpinner className="m-2" />}>
                        <Routes>
                            {props.routes.map(
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
 * Renders a layout of a sidebar and a content area to display site admin information.
 */
export const SiteAdminArea = withAuthenticatedUser(AuthenticatedSiteAdminArea)
