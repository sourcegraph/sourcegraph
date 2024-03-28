import React from 'react'

import { type Observable, of, throwError } from 'rxjs'

import type { CodeInsightsBackend } from './code-insights-backend'

const errorMockMethod = (methodName: string) => () =>
    throwError(() => new Error(`Implement ${methodName} method first`))

/**
 * Default context api class. Provides mock methods only.
 */
export class FakeDefaultCodeInsightsBackend implements CodeInsightsBackend {
    // Insights
    public getInsights = errorMockMethod('getInsights')
    public getInsightById = errorMockMethod('getInsightById')
    public getActiveInsightsCount = (number: number): Observable<number> => of(number - 1)
    public createInsight = errorMockMethod('createInsight')
    public updateInsight = errorMockMethod('updateInsight')
    public deleteInsight = errorMockMethod('deleteInsight')
    public removeInsightFromDashboard = errorMockMethod('removeInsightFromDashboard')

    // Dashboards
    public getDashboardOwners = errorMockMethod('getDashboardSubjects')
    public createDashboard = errorMockMethod('createDashboard')
    public deleteDashboard = errorMockMethod('deleteDashboard')
    public updateDashboard = errorMockMethod('updateDashboard')
    public assignInsightsToDashboard = errorMockMethod('assignInsightsToDashboard')
}

export const CodeInsightsBackendContext = React.createContext<CodeInsightsBackend>(new FakeDefaultCodeInsightsBackend())
