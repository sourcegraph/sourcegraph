import type { FC } from 'react'

import { Route, Routes } from 'react-router-dom'

import { useQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { ErrorAlert, LoadingSpinner } from '@sourcegraph/wildcard'

import { type SiteExternalServiceConfigResult, type SiteExternalServiceConfigVariables } from '../../graphql-operations'
import { SITE_EXTERNAL_SERVICE_CONFIG } from '../../site-admin/backend'

const GitHubAppPage = lazyComponent(() => import('../../components/gitHubApps/GitHubAppPage'), 'GitHubAppPage')
const GitHubAppsPage = lazyComponent(() => import('../../components/gitHubApps/GitHubAppsPage'), 'GitHubAppsPage')

interface Props extends TelemetryProps, PlatformContextProps {
    authenticatedUser: AuthenticatedUser
    batchChangesEnabled: boolean
}

export const UserGitHubAppsArea: FC<Props> = props => {
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
                    <GitHubAppsPage
                        batchChangesEnabled={props.batchChangesEnabled}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                        userOwned={true}
                    />
                }
            />

            <Route
                path=":appID"
                element={
                    <GitHubAppPage
                        headerParentBreadcrumb={{ to: '/user/github-apps', text: 'GitHub Apps' }}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                        userOwned={true}
                        {...props}
                    />
                }
            />
        </Routes>
    )
}
