import type { FC } from 'react'

import { Routes, Route, Navigate, useParams } from 'react-router-dom'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { SiteAdminAreaRouteContext } from '../../../site-admin/SiteAdminArea'
import type { RouteV6Descriptor } from '../../../util/contributions'
import type { CodeIntelConfigurationPageProps } from '../configuration/pages/CodeIntelConfigurationPage'
import type { CodeIntelConfigurationPolicyPageProps } from '../configuration/pages/CodeIntelConfigurationPolicyPage'
import type { CodeIntelInferenceConfigurationPageProps } from '../configuration/pages/CodeIntelInferenceConfigurationPage'
import type { GlobalDashboardPageProps } from '../dashboard/pages/GlobalDashboardPage'
import type { CodeIntelPreciseIndexesPageProps } from '../indexes/pages/CodeIntelPreciseIndexesPage'
import type { CodeIntelPreciseIndexPageProps } from '../indexes/pages/CodeIntelPreciseIndexPage'
import type { CodeIntelRankingPageProps } from '../ranking/pages/CodeIntelRankingPage'

export interface AdminCodeIntelAreaRouteContext extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
}

export interface AdminCodeIntelAreaRoute extends RouteV6Descriptor<AdminCodeIntelAreaRouteContext> {}

const CodeIntelPreciseIndexesPage = lazyComponent<CodeIntelPreciseIndexesPageProps, 'CodeIntelPreciseIndexesPage'>(
    () => import('../indexes/pages/CodeIntelPreciseIndexesPage'),
    'CodeIntelPreciseIndexesPage'
)
const CodeIntelPreciseIndexPage = lazyComponent<CodeIntelPreciseIndexPageProps, 'CodeIntelPreciseIndexPage'>(
    () => import('../indexes/pages/CodeIntelPreciseIndexPage'),
    'CodeIntelPreciseIndexPage'
)

const CodeIntelConfigurationPage = lazyComponent<CodeIntelConfigurationPageProps, 'CodeIntelConfigurationPage'>(
    () => import('../configuration/pages/CodeIntelConfigurationPage'),
    'CodeIntelConfigurationPage'
)

const CodeIntelInferenceConfigurationPage = lazyComponent<
    CodeIntelInferenceConfigurationPageProps,
    'CodeIntelInferenceConfigurationPage'
>(() => import('../configuration/pages/CodeIntelInferenceConfigurationPage'), 'CodeIntelInferenceConfigurationPage')

const CodeIntelRankingPage = lazyComponent<CodeIntelRankingPageProps, 'CodeIntelRankingPage'>(
    () => import('../ranking/pages/CodeIntelRankingPage'),
    'CodeIntelRankingPage'
)

const CodeIntelConfigurationPolicyPage = lazyComponent<
    CodeIntelConfigurationPolicyPageProps,
    'CodeIntelConfigurationPolicyPage'
>(() => import('../configuration/pages/CodeIntelConfigurationPolicyPage'), 'CodeIntelConfigurationPolicyPage')

const GlobalDashboardPage = lazyComponent<GlobalDashboardPageProps, 'GlobalDashboardPage'>(
    () => import('../dashboard/pages/GlobalDashboardPage'),
    'GlobalDashboardPage'
)

export const codeIntelAreaRoutes: readonly AdminCodeIntelAreaRoute[] = (
    [
        // Code intelligence dashboard routes
        {
            path: '/',
            render: () => <Navigate to="./code-graph/dashboard" replace={true} />,
        },
        {
            path: '/dashboard',
            render: props => <GlobalDashboardPage {...props} />,
        },

        // Precise index routes
        {
            path: '/indexes',
            render: props => <CodeIntelPreciseIndexesPage {...props} />,
        },
        {
            path: '/indexes/:id',
            render: props => <CodeIntelPreciseIndexPage {...props} />,
        },

        // Code graph configuration
        {
            path: '/configuration',
            render: props => <CodeIntelConfigurationPage {...props} />,
        },
        {
            path: '/configuration/:id',
            render: props => <CodeIntelConfigurationPolicyPage {...props} />,
        },
        {
            path: '/inference-configuration',
            render: props => <CodeIntelInferenceConfigurationPage {...props} />,
            condition: () => window.context?.codeIntelAutoIndexingEnabled,
        },

        // Ranking
        {
            path: '/ranking',
            render: props => <CodeIntelRankingPage {...props} />,
        },

        // Legacy routes
        {
            path: '/uploads/:id',
            render: () => <NavigateToLegacyUploadPage />,
        },
    ] as readonly (AdminCodeIntelAreaRoute | undefined)[]
).filter(Boolean) as readonly AdminCodeIntelAreaRoute[]

/**
 * Properties passed to all page components in the repository code navigation area.
 */
export interface AdminCodeIntelAreaPageProps extends SiteAdminAreaRouteContext {}

/**
 * Renders pages related to repository code graph.
 */
export const AdminCodeIntelArea: FC<AdminCodeIntelAreaPageProps> = props => (
    <Routes>
        {codeIntelAreaRoutes.map(
            ({ path, render, condition = () => true }) =>
                condition(props) && (
                    <Route
                        key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                        path={path}
                        element={render(props)}
                    />
                )
        )}
    </Routes>
)

function NavigateToLegacyUploadPage(): JSX.Element {
    const { id = '' } = useParams<{ id: string }>()
    return (
        <Navigate
            to={`/site-admin/code-graph/indexes/${btoa(`PreciseIndex:"U:${(atob(id).match(/(\d+)/) ?? [''])[0]}"`)}`}
        />
    )
}
