import { type FC, useEffect, useState } from 'react'

import { gql, useLazyQuery } from '@apollo/client'
import { Route, Routes, Navigate } from 'react-router-dom'

import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../../auth'
import { withAuthenticatedUser } from '../../auth/withAuthenticatedUser'
import { NotFoundPage } from '../../components/HeroPage'
import { RedirectRoute } from '../../components/RedirectRoute'
import type { GetFirstAvailableDashboardResult, GetFirstAvailableDashboardVariables } from '../../graphql-operations'

import { CodeInsightsBackendContext } from './core'
import { useApi } from './hooks'
import { useLicense } from './hooks/use-license'
import { GaConfirmationModal } from './modals/GaConfirmationModal'
import { CodeInsightsRootPage, CodeInsightsRootPageTab } from './pages/CodeInsightsRootPage'
import { InsightsDashboardCreationPage } from './pages/dashboards/creation/InsightsDashboardCreationPage'
import { EditDashboardPage } from './pages/dashboards/edit-dashboard/EditDashobardPage'
import { CreationRoutes } from './pages/insights/creation/CreationRoutes'
import { CodeInsightIndependentPage } from './pages/insights/insight/CodeInsightIndependentPage'

const EditInsightLazyPage = lazyComponent(
    () => import('./pages/insights/edit-insight/EditInsightPage'),
    'EditInsightPage'
)

export interface CodeInsightsAppRouter extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser
}

const rootPagePathsToTab = {
    'dashboards/:dashboardId?': CodeInsightsRootPageTab.Dashboards,
    all: CodeInsightsRootPageTab.AllInsights,
    about: CodeInsightsRootPageTab.GettingStarted,
}

export const CodeInsightsAppRouter = withAuthenticatedUser<CodeInsightsAppRouter>(props => {
    const { telemetryService, telemetryRecorder } = props

    const fetched = useLicense()
    const api = useApi()

    if (!fetched) {
        return null
    }

    return (
        <CodeInsightsBackendContext.Provider value={api}>
            <GaConfirmationModal />

            <Routes>
                <Route index={true} element={<CodeInsightsSmartRoutingRedirect />} />

                <Route
                    path="create/*"
                    element={
                        <CreationRoutes telemetryService={telemetryService} telemetryRecorder={telemetryRecorder} />
                    }
                />

                <Route path="dashboards/:dashboardId/edit" element={<EditDashboardPage />} />

                <Route
                    path="add-dashboard"
                    element={
                        <InsightsDashboardCreationPage
                            telemetryService={telemetryService}
                            telemetryRecorder={telemetryRecorder}
                        />
                    }
                />

                {Object.entries(rootPagePathsToTab).map(([path, activeTab]) => (
                    <Route
                        key="hardcoded-key"
                        path={path}
                        element={
                            <CodeInsightsRootPage
                                activeTab={activeTab}
                                telemetryService={telemetryService}
                                telemetryRecorder={telemetryRecorder}
                            />
                        }
                    />
                ))}

                <Route
                    // Deprecated URL, delete this in the 4.10
                    path="edit/:insightId"
                    element={<RedirectRoute getRedirectURL={({ params }) => `/insights/${params.insightId}/edit`} />}
                />
                <Route path=":insightId/edit" element={<EditInsightLazyPage telemetryRecorder={telemetryRecorder} />} />

                <Route
                    // Deprecated URL, delete this in the 4.10
                    path="insight/:insightId"
                    element={<RedirectRoute getRedirectURL={({ params }) => `/insights/${params.insightId}`} />}
                />
                <Route
                    path=":insightId"
                    element={
                        <CodeInsightIndependentPage
                            telemetryService={telemetryService}
                            telemetryRecorder={telemetryRecorder}
                        />
                    }
                />

                <Route path="*" element={<NotFoundPage pageType="code insights" />} />
            </Routes>
        </CodeInsightsBackendContext.Provider>
    )
})

const CodeInsightsSmartRoutingRedirect: FC = () => {
    const state = useDashboardExistence()

    if (state.status === 'loading') {
        return null
    }

    // No dashboards status means that there are no insights either, so redirect
    // to the getting started page in this case
    if (state.status === 'noDashboards') {
        return <Navigate to="about" replace={true} />
    }

    // There are some dashboards, but we didn't find any particular dashboard in the user
    // temporal settings so redirect to the dashboard tab and select first private dashboard
    if (state.status === 'availableDashboard') {
        return <Navigate to="dashboards" replace={true} />
    }

    // We found a recently viewed dashboard id in the temporal settings, so redirect to this
    // dashboard.
    return <Navigate to={`dashboards/${state.dashboardId}`} replace={true} />
}

type DashboardExistence =
    | { status: 'availableDashboard' }
    | { status: 'lastVisitedDashboard'; dashboardId: string }
    | { status: 'noDashboards' }
    | { status: 'loading' }

function useDashboardExistence(): DashboardExistence {
    const [state, setState] = useState<DashboardExistence>({ status: 'loading' })
    const [lastVisitedDashboardId, , temporalSettingStatus] = useTemporarySetting(
        'insights.lastVisitedDashboardId',
        null
    )

    const [fetchFirstAvailableDashboard] = useLazyQuery<
        GetFirstAvailableDashboardResult,
        GetFirstAvailableDashboardVariables
    >(gql`
        query GetFirstAvailableDashboard($lastVisitedDashboardId: ID) {
            lastVisitedDashboard: insightsDashboards(id: $lastVisitedDashboardId) {
                nodes {
                    id
                }
            }
            firstAvailableDashboard: insightsDashboards(first: 1) {
                nodes {
                    id
                }
            }
        }
    `)

    useEffect(() => {
        // We're still loading temporal settings
        if (temporalSettingStatus === 'initial') {
            return
        }

        const normalizedLastVisitedDashboardId = lastVisitedDashboardId ?? null

        // Check asynchronously about dashboard existence on the backend
        fetchFirstAvailableDashboard({ variables: { lastVisitedDashboardId: normalizedLastVisitedDashboardId } })
            .then(result => {
                const {
                    data = {
                        lastVisitedDashboard: { nodes: [] },
                        firstAvailableDashboard: { nodes: [] },
                    },
                } = result

                const [lastVisitedDashboard] = data.lastVisitedDashboard.nodes
                const [firstAvailableDashboard] = data.firstAvailableDashboard.nodes

                // We resolved dashboard by lastVisitedDashboardId in the temporal settings
                if (lastVisitedDashboard) {
                    setState({ status: 'lastVisitedDashboard', dashboardId: lastVisitedDashboard.id })
                    return
                }

                // If it's just another dashboard and not undefined (this mean we have at least one dashboard
                // on the backend redirect to the dashboard page without id
                if (firstAvailableDashboard) {
                    setState({ status: 'availableDashboard' })
                } else {
                    // Otherwise, redirect to the standard no dashboard view (which is getting started tab)
                    setState({ status: 'noDashboards' })
                }
            })
            .catch(() => {
                setState({ status: 'noDashboards' })
            })
    }, [fetchFirstAvailableDashboard, lastVisitedDashboardId, temporalSettingStatus])

    return state
}
