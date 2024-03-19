import type { FC } from 'react'

import { Navigate, Route, Routes } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../../../auth'
import type { BreadcrumbSetters } from '../../../components/Breadcrumbs'
import { NotFoundPage } from '../../../components/HeroPage'
import type { RepositoryFields } from '../../../graphql-operations'
import type { RouteV6Descriptor } from '../../../util/contributions'
import type { CodeIntelConfigurationPolicyPageProps } from '../../codeintel/configuration/pages/CodeIntelConfigurationPolicyPage'
import { CodyConfigurationPage } from '../configuration/pages/CodyConfigurationPage'

import { CodyRepoSidebar, type CodyRepoSidebarGroups } from './CodyRepoSidebar'

export interface CodyRepoAreaRouteContext extends TelemetryProps, TelemetryV2Props {
    repo: { id: string; name: string }
    authenticatedUser: AuthenticatedUser | null
}

export interface CodyRepoAreaRoute extends RouteV6Descriptor<CodyRepoAreaRouteContext> {}

const CodeIntelConfigurationPolicyPage = lazyComponent<
    CodeIntelConfigurationPolicyPageProps,
    'CodeIntelConfigurationPolicyPage'
>(
    () => import('../../codeintel/configuration/pages/CodeIntelConfigurationPolicyPage'),
    'CodeIntelConfigurationPolicyPage'
)

export const codyRepoAreaRoutes: readonly CodyRepoAreaRoute[] = [
    {
        path: '/',
        render: () => <Navigate to="./configuration" replace={true} />,
    },
    {
        path: '/configuration',
        render: props => <CodyConfigurationPage {...props} />,
    },
    {
        path: '/configuration/:id',
        render: props => <CodeIntelConfigurationPolicyPage {...props} domain="embeddings" />,
    },
]

export interface CodyRepoAreaProps extends BreadcrumbSetters, TelemetryProps, TelemetryV2Props {
    repo: RepositoryFields
    authenticatedUser: AuthenticatedUser | null
}

const sidebarRoutes: CodyRepoSidebarGroups = [
    {
        header: { label: 'Embeddings' },
        items: [
            {
                to: '/configuration',
                label: 'Configuration policies',
            },
        ],
    },
]

const BREADCRUMB = { key: 'embeddings', element: 'Embeddings' }

export const CodyRepoArea: FC<CodyRepoAreaProps> = props => {
    const { useBreadcrumb, repo } = props

    useBreadcrumb(BREADCRUMB)

    return (
        <div className="container d-flex mt-3">
            <CodyRepoSidebar className="flex-0 mr-3" codyRepoSidebarGroups={sidebarRoutes} repo={repo} />

            <div className="flex-bounded">
                <Routes>
                    {codyRepoAreaRoutes.map(
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
