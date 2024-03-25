/* eslint-disable ban/ban */
import type { ReactElement } from 'react'

import type { MockedResponse } from '@apollo/client/testing'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import sinon from 'sinon'
import { beforeAll, describe, expect, it, vi } from 'vitest'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { MockIntersectionObserver } from '@sourcegraph/shared/src/testing/MockIntersectionObserver'

import type { InsightsDashboardsResult } from '../../../graphql-operations'
import { type CodeInsightsBackend, CodeInsightsBackendContext, FakeDefaultCodeInsightsBackend } from '../core'
import { GET_INSIGHT_DASHBOARDS_GQL } from '../core/hooks/use-insight-dashboards'

import { CodeInsightsRootPage, CodeInsightsRootPageTab } from './CodeInsightsRootPage'

vi.mock('react-router-dom', async () => ({
    ...(await vi.importActual<typeof import('react-router-dom')>('react-router-dom')),
    useNavigate: () => ({
        push: vi.fn(),
    }),
}))

const mockTelemetryService = {
    log: sinon.spy(),
    logViewEvent: sinon.spy(),
    logPageView: sinon.spy(),
}

const fakeApi = new FakeDefaultCodeInsightsBackend()

const Wrapper: React.FunctionComponent<React.PropsWithChildren<{ api: Partial<CodeInsightsBackend> }>> = ({
    children,
    api = {},
}) => {
    const extendedApi: CodeInsightsBackend = {
        ...fakeApi,
        ...api,
    }
    return <CodeInsightsBackendContext.Provider value={extendedApi}>{children}</CodeInsightsBackendContext.Provider>
}

const mockedGQL: MockedResponse[] = [
    {
        request: {
            query: getDocumentNode(GET_INSIGHT_DASHBOARDS_GQL),
        },
        result: {
            data: {
                insightsDashboards: {
                    nodes: [
                        {
                            __typename: 'InsightsDashboard',
                            id: 'foo',
                            title: 'Global Dashboard',
                            grants: {
                                __typename: 'InsightsPermissionGrants',
                                users: [],
                                organizations: [],
                                global: true,
                            },
                        },
                    ],
                },
                currentUser: {
                    __typename: 'User',
                    id: '001',
                    organizations: {
                        nodes: [],
                    },
                },
            },
        },
    } satisfies MockedResponse<InsightsDashboardsResult>,
]

const renderWithBrandedContext = (component: ReactElement, { route = '/', path = '*', api = {} } = {}) => ({
    ...render(
        <MockedTestProvider mocks={mockedGQL}>
            <Wrapper api={api}>
                <MemoryRouter initialEntries={[route]}>
                    <Routes>
                        <Route path={path} element={component} />
                    </Routes>
                </MemoryRouter>
            </Wrapper>
        </MockedTestProvider>
    ),
})

describe('CodeInsightsRootPage', () => {
    beforeAll(() => {
        window.IntersectionObserver = MockIntersectionObserver
    })

    it('should render dashboard not found page when id is not found', () => {
        renderWithBrandedContext(
            <CodeInsightsRootPage
                activeTab={CodeInsightsRootPageTab.Dashboards}
                telemetryService={mockTelemetryService}
                telemetryRecorder={noOpTelemetryRecorder}
            />,
            {
                route: '/insights/dashboards/foo',
                path: '/insights/dashboards/:dashboardId',
            }
        )

        screen.findByText("Hmm, the dashboard wasn't found.")
    })

    it('should log events', () => {
        renderWithBrandedContext(
            <CodeInsightsRootPage
                activeTab={CodeInsightsRootPageTab.Dashboards}
                telemetryService={mockTelemetryService}
                telemetryRecorder={noOpTelemetryRecorder}
            />,
            {
                route: '/insights/dashboards/foo',
                path: '/insights/dashboards/:dashboardId',
            }
        )

        userEvent.click(screen.getByText('Create insight'))
        expect(mockTelemetryService.log.calledWith('InsightAddMoreClick')).toBe(true)
    })
})
