import { type ApolloCache, type ApolloClient, gql } from '@apollo/client'
import { from, type Observable, of } from 'rxjs'
import { catchError, map, switchMap } from 'rxjs/operators'

import { isDefined } from '@sourcegraph/common'
import { fromObservableQuery } from '@sourcegraph/http-client'

import type {
    AddInsightViewToDashboardResult,
    DeleteDashboardResult,
    GetDashboardInsightsResult,
    GetFrozenInsightsCountResult,
    GetInsightsResult,
    RemoveInsightViewFromDashboardResult,
    RemoveInsightViewFromDashboardVariables,
} from '../../../../../graphql-operations'
import { type Insight, type InsightsDashboardOwner, isComputeInsight } from '../../types'
import type { CodeInsightsBackend } from '../code-insights-backend'
import type {
    AssignInsightsToDashboardInput,
    DashboardCreateInput,
    DashboardDeleteInput,
    DashboardUpdateInput,
    DashboardUpdateResult,
    InsightCreateInput,
    InsightUpdateInput,
    RemoveInsightFromDashboardInput,
    DashboardCreateResult,
} from '../code-insights-backend-types'

import { createInsightView } from './deserialization/create-insight-view'
import { GET_DASHBOARD_INSIGHTS_GQL } from './gql/GetDashboardInsights'
import { GET_INSIGHTS_GQL } from './gql/GetInsights'
import { REMOVE_INSIGHT_FROM_DASHBOARD_GQL } from './gql/RemoveInsightFromDashboard'
import { createDashboard } from './methods/create-dashboard/create-dashboard'
import { createInsight } from './methods/create-insight/create-insight'
import { getDashboardOwners } from './methods/get-dashboard-owners'
import { updateDashboard } from './methods/update-dashboard'
import { updateInsight } from './methods/update-insight/update-insight'

export class CodeInsightsGqlBackend implements CodeInsightsBackend {
    constructor(private apolloClient: ApolloClient<object>) {}

    // Insights
    public getInsights = (input: { dashboardId: string; withCompute: boolean }): Observable<Insight[]> => {
        const { dashboardId, withCompute } = input

        // Get all insights from the user-created dashboard
        return fromObservableQuery(
            this.apolloClient.watchQuery<GetDashboardInsightsResult>({
                query: GET_DASHBOARD_INSIGHTS_GQL,
                // Prevent unnecessary network request after mutation over dashboard or insights within
                // current dashboard
                nextFetchPolicy: 'cache-first',
                errorPolicy: 'all',
                variables: { id: dashboardId },
            })
        ).pipe(
            map(({ data }) => data.insightsDashboards.nodes[0]),
            map(dashboard => dashboard.views?.nodes.filter(isDefined).map(createInsightView) ?? []),
            map(insights => (withCompute ? insights : insights.filter(insight => !isComputeInsight(insight))))
        )
    }

    public getInsightById = (id: string): Observable<Insight | null> =>
        fromObservableQuery(
            this.apolloClient.watchQuery<GetInsightsResult>({
                query: GET_INSIGHTS_GQL,
                variables: { id },
                errorPolicy: 'all',
            })
        ).pipe(
            map(({ data }) => {
                const insightData = data.insightViews.nodes[0]

                if (!insightData) {
                    return null
                }

                return createInsightView(insightData)
            }),
            catchError(() => of(null))
        )

    public getActiveInsightsCount = (first: number): Observable<number> =>
        fromObservableQuery(
            this.apolloClient.watchQuery<GetFrozenInsightsCountResult>({
                query: gql`
                    query GetFrozenInsightsCount($first: Int!) {
                        insightViews(first: $first, isFrozen: false) {
                            nodes {
                                id
                            }
                        }
                    }
                `,
                variables: { first },
            })
        ).pipe(map(({ data }) => data.insightViews.nodes.length))

    public createInsight = (input: InsightCreateInput): Observable<unknown> => createInsight(this.apolloClient, input)

    public updateInsight = (input: InsightUpdateInput): Observable<unknown> => updateInsight(this.apolloClient, input)

    public deleteInsight = (insightId: string): Observable<unknown> =>
        from(
            this.apolloClient.mutate({
                mutation: gql`
                    mutation DeleteInsightView($id: ID!) {
                        deleteInsightView(id: $id) {
                            alwaysNil
                        }
                    }
                `,
                variables: { id: insightId },
                update(cache: ApolloCache<DeleteDashboardResult>) {
                    const deletedInsightReference = cache.identify({ __typename: 'InsightView', id: insightId })

                    // Remove deleted insights from the apollo cache
                    cache.evict({ id: deletedInsightReference })
                },
            })
        )

    public removeInsightFromDashboard = (input: RemoveInsightFromDashboardInput): Observable<unknown> => {
        const { insightId, dashboardId } = input

        return from(
            this.apolloClient.mutate<RemoveInsightViewFromDashboardResult, RemoveInsightViewFromDashboardVariables>({
                mutation: REMOVE_INSIGHT_FROM_DASHBOARD_GQL,
                variables: { insightId, dashboardId },
                update(cache: ApolloCache<RemoveInsightViewFromDashboardResult>) {
                    const deletedInsightReference = cache.identify({ __typename: 'InsightView', id: insightId })

                    // Remove deleted insights from the apollo cache
                    cache.evict({ id: deletedInsightReference })
                },
            })
        )
    }

    public getDashboardOwners = (): Observable<InsightsDashboardOwner[]> => getDashboardOwners(this.apolloClient)

    public createDashboard = (input: DashboardCreateInput): Observable<DashboardCreateResult> =>
        createDashboard(this.apolloClient, input)

    public deleteDashboard = ({ id }: DashboardDeleteInput): Observable<void> => {
        if (!id) {
            throw new Error('`id` is required to delete a dashboard')
        }

        return from(
            this.apolloClient.mutate<DeleteDashboardResult>({
                mutation: gql`
                    mutation DeleteDashboard($id: ID!) {
                        deleteInsightsDashboard(id: $id) {
                            alwaysNil
                        }
                    }
                `,
                variables: { id },
                update(cache: ApolloCache<DeleteDashboardResult>) {
                    const deletedDashboardReference = cache.identify({ __typename: 'InsightsDashboard', id })

                    // Remove deleted insights from the apollo cache
                    cache.evict({ id: deletedDashboardReference })
                },
            })
        ).pipe(map(() => undefined))
    }

    public updateDashboard = (input: DashboardUpdateInput): Observable<DashboardUpdateResult> =>
        updateDashboard(this.apolloClient, input)

    public assignInsightsToDashboard = ({
        id,
        prevInsightIds,
        nextInsightIds,
    }: AssignInsightsToDashboardInput): Observable<unknown> => {
        const addInsightViewToDashboard = (insightViewId: string, dashboardId: string): Promise<any> =>
            this.apolloClient.mutate<AddInsightViewToDashboardResult>({
                mutation: gql`
                    mutation AddInsightViewToDashboard($insightViewId: ID!, $dashboardId: ID!) {
                        addInsightViewToDashboard(input: { insightViewId: $insightViewId, dashboardId: $dashboardId }) {
                            dashboard {
                                id
                            }
                        }
                    }
                `,
                variables: { insightViewId, dashboardId },
            })

        const removeInsightViewFromDashboard = (insightId: string, dashboardId: string): Promise<any> =>
            this.apolloClient.mutate<RemoveInsightViewFromDashboardResult, RemoveInsightViewFromDashboardVariables>({
                mutation: REMOVE_INSIGHT_FROM_DASHBOARD_GQL,
                variables: { insightId, dashboardId },
            })

        const addedInsightIds = nextInsightIds.filter(insightId => !prevInsightIds.includes(insightId)) || []

        // Get array of removed insight view ids
        const removedInsightIds = prevInsightIds.filter(insightId => !nextInsightIds.includes(insightId)) || []

        return from(
            Promise.all([
                ...addedInsightIds.map(insightId => addInsightViewToDashboard(insightId, id || '')),
                ...removedInsightIds.map(insightId => removeInsightViewFromDashboard(insightId, id || '')),
            ])
        ).pipe(
            // Next query is needed to update local apollo cache and re-trigger getInsights query.
            // Usually Apollo does that under the hood by itself based on response from a mutation
            // but in this case since we don't have one single query to assign/unassigned insights
            // from dashboard we have to call query manually.
            switchMap(() =>
                this.apolloClient.query<GetDashboardInsightsResult>({
                    query: GET_DASHBOARD_INSIGHTS_GQL,
                    variables: { id: id ?? '' },
                })
            )
        )
    }
}
