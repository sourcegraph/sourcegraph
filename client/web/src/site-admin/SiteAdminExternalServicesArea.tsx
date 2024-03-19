import type { FC } from 'react'

import { Routes, Route, Navigate } from 'react-router-dom'

import { useQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { LoadingSpinner, ErrorAlert } from '@sourcegraph/wildcard'

import { codeHostExternalServices, nonCodeHostExternalServices } from '../components/externalServices/externalServices'
import type { SiteExternalServiceConfigResult, SiteExternalServiceConfigVariables } from '../graphql-operations'

import { SITE_EXTERNAL_SERVICE_CONFIG } from './backend'

const ExternalServicesPage = lazyComponent(
    () => import('../components/externalServices/ExternalServicesPage'),
    'ExternalServicesPage'
)
const ExternalServiceEditPage = lazyComponent(
    () => import('../components/externalServices/ExternalServiceEditPage'),
    'ExternalServiceEditPage'
)
const ExternalServicePage = lazyComponent(
    () => import('../components/externalServices/ExternalServicePage'),
    'ExternalServicePage'
)
const AddExternalServicesPage = lazyComponent(
    () => import('../components/externalServices/AddExternalServicesPage'),
    'AddExternalServicesPage'
)

interface Props extends TelemetryProps, TelemetryV2Props, PlatformContextProps, SettingsCascadeProps {
    authenticatedUser: AuthenticatedUser
}

export const SiteAdminExternalServicesArea: FC<Props> = props => {
    const { data, error, loading } = useQuery<SiteExternalServiceConfigResult, SiteExternalServiceConfigVariables>(
        SITE_EXTERNAL_SERVICE_CONFIG,
        {}
    )

    if (error && !loading) {
        return <ErrorAlert error={error} />
    }

    if (loading && !error) {
        return <LoadingSpinner />
    }

    if (!data) {
        return null
    }

    return (
        <Routes>
            <Route
                index={true}
                element={
                    <ExternalServicesPage
                        {...props}
                        externalServicesFromFile={data?.site?.externalServicesFromFile}
                        allowEditExternalServicesWithFile={data?.site?.allowEditExternalServicesWithFile}
                    />
                }
            />

            <Route path="/add" element={<Navigate to="new" replace={true} />} />
            <Route
                path="new"
                element={
                    <AddExternalServicesPage
                        {...props}
                        codeHostExternalServices={codeHostExternalServices}
                        nonCodeHostExternalServices={nonCodeHostExternalServices}
                        externalServicesFromFile={data?.site?.externalServicesFromFile}
                        allowEditExternalServicesWithFile={data?.site?.allowEditExternalServicesWithFile}
                    />
                }
            />
            <Route
                path=":externalServiceID"
                element={
                    <ExternalServicePage
                        {...props}
                        afterDeleteRoute="/site-admin/external-services"
                        externalServicesFromFile={data?.site?.externalServicesFromFile}
                        allowEditExternalServicesWithFile={data?.site?.allowEditExternalServicesWithFile}
                    />
                }
            />
            <Route
                path=":externalServiceID/edit"
                element={
                    <ExternalServiceEditPage
                        {...props}
                        externalServicesFromFile={data?.site?.externalServicesFromFile}
                        allowEditExternalServicesWithFile={data?.site?.allowEditExternalServicesWithFile}
                    />
                }
            />
        </Routes>
    )
}
