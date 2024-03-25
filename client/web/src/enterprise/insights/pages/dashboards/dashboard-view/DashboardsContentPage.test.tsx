import React from 'react'

import { useApolloClient } from '@apollo/client'
import type { MockedResponse } from '@apollo/client/testing'
import { within } from '@testing-library/dom'
import { act, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import sinon from 'sinon'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { MockIntersectionObserver } from '@sourcegraph/shared/src/testing/MockIntersectionObserver'
import { type RenderWithBrandedContextResult, renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import type {
    FindInsightsBySearchTermResult,
    GetDashboardInsightsResult,
    GetInsightsResult,
    InsightsDashboardsResult,
    InsightSubjectsResult,
} from '../../../../../graphql-operations'
import { CodeInsightsBackendContext, CodeInsightsGqlBackend } from '../../../core'
import {
    GET_DASHBOARD_INSIGHTS_GQL,
    GET_INSIGHTS_GQL,
    GET_INSIGHTS_DASHBOARD_OWNERS_GQL,
} from '../../../core/backend/gql-backend'
import { GET_INSIGHT_DASHBOARDS_GQL } from '../../../core/hooks/use-insight-dashboards'
import { useCodeInsightsLicenseState } from '../../../stores'

import { GET_INSIGHTS_BY_SEARCH_TERM } from './components/add-insight-modal'
import { DashboardsView } from './DashboardsView'

type UserEvent = typeof userEvent

const mockCopyURL = sinon.spy()

vi.mock('../../../hooks/use-copy-url-handler', () => ({
    useCopyURLHandler: () => [mockCopyURL],
}))

const mockTelemetryService = {
    log: sinon.spy(),
    logViewEvent: sinon.spy(),
    logPageView: sinon.spy(),
}

const Wrapper: React.FunctionComponent<React.PropsWithChildren<unknown>> = ({ children }) => {
    const apolloClient = useApolloClient()
    const api = new CodeInsightsGqlBackend(apolloClient)
    useCodeInsightsLicenseState.setState({ licensed: true, insightsLimit: 2 })

    return <CodeInsightsBackendContext.Provider value={api}>{children}</CodeInsightsBackendContext.Provider>
}

const mockDashboard: InsightsDashboardsResult['insightsDashboards']['nodes'][0] = {
    __typename: 'InsightsDashboard',
    id: 'foo',
    title: 'Global Dashboard',
    grants: {
        __typename: 'InsightsPermissionGrants',
        users: [],
        organizations: [],
        global: true,
    },
}

const mockDashboard2: InsightsDashboardsResult['insightsDashboards']['nodes'][0] = {
    __typename: 'InsightsDashboard',
    id: 'bar',
    title: 'Global Dashboard 2',
    grants: {
        __typename: 'InsightsPermissionGrants',
        users: [],
        organizations: [],
        global: true,
    },
}

const userMock = {
    __typename: 'User',
    id: '001',
    organizations: {
        nodes: [],
    },
}

const mocks: MockedResponse[] = [
    {
        request: {
            query: GET_INSIGHT_DASHBOARDS_GQL,
        },
        result: {
            data: { insightsDashboards: { nodes: [mockDashboard, mockDashboard2] }, currentUser: userMock },
        },
    } as MockedResponse<InsightsDashboardsResult>,
    {
        request: {
            query: GET_INSIGHT_DASHBOARDS_GQL,
            variables: { id: 'foo' },
        },
        result: {
            data: { insightsDashboards: { nodes: [mockDashboard, mockDashboard2] }, currentUser: userMock },
        },
    } as MockedResponse<InsightsDashboardsResult>,
    {
        request: {
            query: getDocumentNode(GET_INSIGHTS_BY_SEARCH_TERM),
            variables: { search: '', first: 20, after: null, excludeIds: [] },
        },
        result: {
            data: {
                insightViews: {
                    nodes: [],
                    totalCount: 0,
                    pageInfo: {
                        endCursor: null,
                        hasNextPage: false,
                    },
                },
            },
        },
    } as MockedResponse<FindInsightsBySearchTermResult>,
    {
        request: {
            query: GET_INSIGHTS_DASHBOARD_OWNERS_GQL,
        },
        result: {
            data: {
                currentUser: userMock,
                site: { id: 'global_instance_id' },
            },
        },
    } as MockedResponse<InsightSubjectsResult>,
    {
        request: {
            query: GET_DASHBOARD_INSIGHTS_GQL,
            variables: { id: 'foo' },
        },
        result: {
            data: {
                insightsDashboards: {
                    nodes: [
                        {
                            id: 'foo',
                            views: null,
                        },
                    ],
                },
            },
        },
    } as MockedResponse<GetDashboardInsightsResult>,
    {
        request: {
            query: GET_INSIGHTS_GQL,
            variables: {},
        },
        result: {
            data: { insightViews: { nodes: [] } },
        },
    } as MockedResponse<GetInsightsResult>,
]

const renderDashboardsContent = (
    dashboardID: string = 'foo'
): RenderWithBrandedContextResult & { user: UserEvent } => ({
    user: userEvent,
    ...renderWithBrandedContext(
        <MockedTestProvider mocks={mocks}>
            <Wrapper>
                <DashboardsView
                    dashboardId={dashboardID}
                    telemetryService={mockTelemetryService}
                    telemetryRecorder={noOpTelemetryRecorder}
                />
            </Wrapper>
        </MockedTestProvider>
    ),
})

const triggerDashboardMenuItem = async (
    screen: RenderWithBrandedContextResult & { user: UserEvent },
    buttonText: string
) => {
    const { user } = screen
    const dashboardMenu = await waitFor(() => screen.getByRole('img', { name: 'dashboard options' }))
    user.click(dashboardMenu)

    const dialog = screen.getByRole('dialog', { hidden: true })
    const dashboardMenuItem = within(dialog).getByText(buttonText)

    act(() => {
        dashboardMenuItem.focus()
        user.click(dashboardMenuItem)
    })
}

beforeEach(() => {
    vi.clearAllMocks()
    window.IntersectionObserver = MockIntersectionObserver
})

describe('DashboardsContent', () => {
    it('renders a loading indicator', () => {
        const screen = renderDashboardsContent('baz')

        expect(screen.getByLabelText('Loading')).toBeInTheDocument()
    })

    it('renders dashboard not found', async () => {
        const screen = renderDashboardsContent('baz')

        await waitFor(() => expect(screen.getByText("Hmm, the dashboard wasn't found.")).toBeInTheDocument())
    })

    it('renders a dashboard', async () => {
        const screen = renderDashboardsContent()

        await waitFor(() => expect(screen.getByRole('button', { name: /Global Dashboard/ })).toBeInTheDocument())
    })

    it('redirect to new dashboard page on selection', async () => {
        const screen = renderDashboardsContent()
        const { locationRef, user } = screen

        const chooseDashboard = await waitFor(() => screen.getByRole('button', { name: /Choose a dashboard/ }))
        user.click(chooseDashboard)

        const coboboxPopover = screen.getByRole('dialog', { hidden: true })
        const dashboard2 = coboboxPopover.querySelector('[title="Global Dashboard 2"]')

        if (dashboard2) {
            user.click(dashboard2)
        }

        expect(locationRef.current?.pathname).toEqual('/insights/dashboards/bar')
    })

    it('redirects to dashboard edit page', async () => {
        const screen = renderDashboardsContent()

        await triggerDashboardMenuItem(screen, 'Configure dashboard')

        expect(screen.locationRef.current?.pathname).toEqual('/insights/dashboards/foo/edit')
    })

    it('opens add insight modal', async () => {
        const screen = renderDashboardsContent()
        const addInsightsButton = await waitFor(() => screen.getByRole('button', { name: /Add or remove insights/ }))

        userEvent.click(addInsightsButton)

        const addInsightHeader = await waitFor(() =>
            screen.getByRole('heading', { name: /Add insight to Global Dashboard/ })
        )
        expect(addInsightHeader).toBeInTheDocument()
    })

    it('opens delete dashboard modal', async () => {
        const screen = renderDashboardsContent()

        await triggerDashboardMenuItem(screen, 'Delete')

        const addInsightHeader = await waitFor(() => screen.getByRole('heading', { name: /Delete/ }))
        expect(addInsightHeader).toBeInTheDocument()
    })

    // copies dashboard url
    it('copies dashboard url', async () => {
        const screen = renderDashboardsContent()

        await triggerDashboardMenuItem(screen, 'Copy link')

        sinon.assert.calledOnce(mockCopyURL)
    })
})
