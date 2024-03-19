import type { FC } from 'react'

import { Routes, Route, Navigate } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../../../auth'
import type { BreadcrumbSetters } from '../../../components/Breadcrumbs'
import { NotFoundPage } from '../../../components/HeroPage'
import { RedirectRoute } from '../../../components/RedirectRoute'
import type { RepositoryFields } from '../../../graphql-operations'
import type { RouteV6Descriptor } from '../../../util/contributions'
import type { CodeIntelConfigurationPageProps } from '../configuration/pages/CodeIntelConfigurationPage'
import type { CodeIntelConfigurationPolicyPageProps } from '../configuration/pages/CodeIntelConfigurationPolicyPage'
import type { CodeIntelRepositoryIndexConfigurationPageProps } from '../configuration/pages/CodeIntelRepositoryIndexConfigurationPage'
import type { RepoDashboardPageProps } from '../dashboard/pages/RepoDashboardPage'
import type { CodeIntelPreciseIndexesPageProps } from '../indexes/pages/CodeIntelPreciseIndexesPage'
import type { CodeIntelPreciseIndexPageProps } from '../indexes/pages/CodeIntelPreciseIndexPage'

import { CodeIntelSidebar, type CodeIntelSideBarGroups } from './CodeIntelSidebar'

export interface CodeIntelAreaRouteContext extends TelemetryProps, TelemetryV2Props {
    repo: { id: string; name: string }
    authenticatedUser: AuthenticatedUser | null
}

export interface CodeIntelAreaRoute extends RouteV6Descriptor<CodeIntelAreaRouteContext> {}

const RepoDashboardPage = lazyComponent<RepoDashboardPageProps, 'RepoDashboardPage'>(
    () => import('../dashboard/pages/RepoDashboardPage'),
    'RepoDashboardPage'
)

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

const RepositoryIndexConfigurationPage = lazyComponent<
    CodeIntelRepositoryIndexConfigurationPageProps,
    'CodeIntelRepositoryIndexConfigurationPage'
>(
    () => import('../configuration/pages/CodeIntelRepositoryIndexConfigurationPage'),
    'CodeIntelRepositoryIndexConfigurationPage'
)

const CodeIntelConfigurationPolicyPage = lazyComponent<
    CodeIntelConfigurationPolicyPageProps,
    'CodeIntelConfigurationPolicyPage'
>(() => import('../configuration/pages/CodeIntelConfigurationPolicyPage'), 'CodeIntelConfigurationPolicyPage')

export const codeIntelAreaRoutes: readonly CodeIntelAreaRoute[] = [
    // Code intelligence dashboard routes
    {
        path: '/',
        render: () => <Navigate to="./dashboard" replace={true} />,
    },
    {
        path: '/dashboard',
        render: props => <RepoDashboardPage {...props} />,
        condition: context => Boolean(context.authenticatedUser?.siteAdmin),
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
        path: '/index-configuration',
        render: props => <RepositoryIndexConfigurationPage {...props} />,
        condition: () => window.context?.codeIntelAutoIndexingEnabled,
    },

    // Legacy routes
    {
        path: '/uploads/:id',
        render: () => (
            <RedirectRoute
                getRedirectURL={({ params }) =>
                    `../indexes/${btoa(`PreciseIndex:"U:${(atob(params.id!).match(/(\d+)/) ?? [''])[0]}"`)}`
                }
            />
        ),
    },
]

/**
 * Properties passed to all page components in the repository code navigation area.
 */
export interface RepositoryCodeIntelAreaPageProps extends BreadcrumbSetters, TelemetryProps, TelemetryV2Props {
    /** The active repository. */
    repo: RepositoryFields
    authenticatedUser: AuthenticatedUser | null
}

const sidebarRoutes: CodeIntelSideBarGroups = [
    {
        header: { label: 'Code graph data' },
        items: [
            {
                to: '/dashboard',
                label: 'Dashboard',
            },
            {
                to: '/indexes',
                label: 'Precise indexes',
            },
            {
                to: '/configuration',
                label: 'Configuration policies',
            },

            ...(window.context?.codeIntelAutoIndexingEnabled
                ? [
                      {
                          to: '/index-configuration',
                          label: 'Auto-index configuration',
                      },
                  ]
                : []),
        ],
    },
]

const BREADCRUMB = { key: 'code-intelligence', element: 'Code graph data' }

/**
 * Renders pages related to repository code graph.
 */
export const RepositoryCodeIntelArea: FC<RepositoryCodeIntelAreaPageProps> = props => {
    const { useBreadcrumb, repo } = props

    useBreadcrumb(BREADCRUMB)

    return (
        <div className="container d-flex mt-3">
            <CodeIntelSidebar className="flex-0 mr-3" codeIntelSidebarGroups={sidebarRoutes} repo={repo} />

            <div className="flex-bounded">
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

                    <Route path="*" element={<NotFoundPage pageType="repository" />} />
                </Routes>
            </div>
        </div>
    )
}
