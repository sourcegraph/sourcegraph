import React from 'react'

import type { MockedResponse } from '@apollo/client/testing'
import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { H2 } from '@sourcegraph/wildcard'

import { WebStory } from '../../../../../../components/WebStory'
import { type GetInsightViewResult, SeriesSortDirection, SeriesSortMode } from '../../../../../../graphql-operations'
import {
    type SeriesChartContent,
    type SearchBasedInsight,
    type CaptureGroupInsight,
    InsightType,
} from '../../../../core'
import { GET_INSIGHT_VIEW_GQL } from '../../../../core/backend/gql-backend'
import { InsightInProcessError } from '../../../../core/backend/utils/errors'

import { BackendInsightView } from './BackendInsight'

const defaultStory: Meta = {
    title: 'web/insights/BackendInsight',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
    parameters: {
        chromatic: {
            disableSnapshot: false,
        },
    },
}

export default defaultStory

const INSIGHT_CONFIGURATION_MOCK: SearchBasedInsight = {
    id: 'searchInsights.insight.mock_backend_insight_id',
    title: 'Backend Insight Mock',
    repositories: [],
    repoQuery: '',
    series: [
        { id: 'series_001', query: '', name: 'A metric', stroke: 'var(--warning)' },
        { id: 'series_002', query: '', name: 'B metric', stroke: 'var(--warning)' },
    ],
    type: InsightType.SearchBased,
    step: { weeks: 2 },
    filters: {
        excludeRepoRegexp: '',
        includeRepoRegexp: '',
        context: '',
        seriesDisplayOptions: {
            numSamples: 12,
            limit: 20,
            sortOptions: {
                direction: SeriesSortDirection.DESC,
                mode: SeriesSortMode.RESULT_COUNT,
            },
        },
    },
    dashboardReferenceCount: 0,
    isFrozen: false,
    dashboards: [],
}

interface BackendInsightDatum {
    x: number
    value: number
    link?: string
}

const getXValue = (datum: BackendInsightDatum): Date => new Date(datum.x)
const getYValue = (datum: BackendInsightDatum): number => datum.value
const getLinkURL = (datum: BackendInsightDatum): string | undefined => datum.link

const LINE_CHART_CONTENT_MOCK: SeriesChartContent<BackendInsightDatum> = {
    series: [
        {
            id: 'series_001',
            data: [
                { x: 1588965700286 - 4 * 24 * 60 * 60 * 1000, value: 4000, link: '#A:1st_data_point' },
                { x: 1588965700286 - 3 * 24 * 60 * 60 * 1000, value: 4000, link: '#A:2st_data_point' },
                { x: 1588965700286 - 2 * 24 * 60 * 60 * 1000, value: 5600, link: '#A:3rd_data_point' },
                { x: 1588965700286 - 1 * 24 * 60 * 60 * 1000, value: 9800, link: '#A:4th_data_point' },
                { x: 1588965700286, value: 12300, link: '#A:5th_data_point' },
            ],
            name: 'A metric',
            color: 'var(--warning)',
            alerts: [],
            getXValue,
            getYValue,
            getLinkURL,
        },
        {
            id: 'series_002',
            data: [
                { x: 1588965700286 - 4 * 24 * 60 * 60 * 1000, value: 15000 },
                { x: 1588965700286 - 3 * 24 * 60 * 60 * 1000, value: 26000 },
                { x: 1588965700286 - 2 * 24 * 60 * 60 * 1000, value: 20000 },
                { x: 1588965700286 - 1 * 24 * 60 * 60 * 1000, value: 19000 },
                { x: 1588965700286, value: 17000 },
            ],
            name: 'B metric',
            color: 'var(--warning)',
            alerts: [],
            getXValue,
            getYValue,
            getLinkURL,
        },
    ],
}

const LINE_CHART_CONTENT_MOCK_EMPTY: SeriesChartContent<BackendInsightDatum> = {
    series: [
        {
            id: 'series_001',
            data: [],
            name: 'A metric',
            color: 'var(--warning)',
            alerts: [],
            getXValue,
            getYValue,
        },
        {
            id: 'series_002',
            data: [],
            name: 'B metric',
            color: 'var(--warning)',
            alerts: [],
            getXValue,
            getYValue,
        },
    ],
}

function generateSeries(chartContent: SeriesChartContent<BackendInsightDatum>, isFetchingHistoricalData: boolean) {
    return chartContent.series.map(series => ({
        seriesId: series.id,
        label: series.name,
        points: series.data.map(point => ({
            dateTime: new Date(point.x).toUTCString(),
            value: point.value,
            __typename: 'InsightDataPoint',
            diffQuery: 'type:diff',
        })),
        status: {
            isLoadingData: isFetchingHistoricalData,
            incompleteDatapoints: series.alerts
                ? [{ __typename: 'TimeoutDatapointAlert', time: '2022-04-21T01:13:43Z' }]
                : [],
            __typename: 'InsightSeriesStatus',
        },
        __typename: 'InsightsSeries',
    }))
}

const mockInsightAPIResponse = ({
    isFetchingHistoricalData = false,
    delayAmount = 0,
    throwProcessingError = false,
    hasData = true,
} = {}): MockedResponse[] => {
    if (throwProcessingError) {
        return [
            {
                request: {
                    query: GET_INSIGHT_VIEW_GQL,
                    variables: {
                        id: 'searchInsights.insight.mock_backend_insight_id',
                        filters: { includeRepoRegex: '', excludeRepoRegex: '', searchContexts: [''] },
                        seriesDisplayOptions: {
                            limit: 20,
                            numSamples: 12,
                            sortOptions: {
                                direction: SeriesSortDirection.DESC,
                                mode: SeriesSortMode.RESULT_COUNT,
                            },
                        },
                    },
                },
                error: new InsightInProcessError(),
            },
        ]
    }

    return [
        {
            request: {
                query: GET_INSIGHT_VIEW_GQL,
                variables: {
                    id: 'searchInsights.insight.mock_backend_insight_id',
                    filters: { includeRepoRegex: '', excludeRepoRegex: '', searchContexts: [''] },
                    seriesDisplayOptions: {
                        limit: 20,
                        numSamples: 12,
                        sortOptions: {
                            direction: SeriesSortDirection.DESC,
                            mode: SeriesSortMode.RESULT_COUNT,
                        },
                    },
                },
            },
            result: {
                data: {
                    insightViews: {
                        nodes: [
                            {
                                id: 'searchInsights.insight.mock_backend_insight_id',
                                appliedSeriesDisplayOptions: {
                                    limit: 20,
                                    sortOptions: {
                                        mode: 'RESULT_COUNT',
                                        direction: 'DESC',
                                        __typename: 'SeriesSortOptions',
                                    },
                                    __typename: 'SeriesDisplayOptions',
                                },
                                defaultSeriesDisplayOptions: {
                                    limit: null,
                                    sortOptions: {
                                        mode: null,
                                        direction: null,
                                        __typename: 'SeriesSortOptions',
                                    },
                                    __typename: 'SeriesDisplayOptions',
                                },
                                dataSeries: generateSeries(
                                    hasData ? LINE_CHART_CONTENT_MOCK : LINE_CHART_CONTENT_MOCK_EMPTY,
                                    isFetchingHistoricalData
                                ),
                                __typename: 'InsightView',
                            },
                        ],
                        __typename: 'InsightViewConnection',
                    },
                },
            },
            delay: delayAmount,
        },
    ]
}

const TestBackendInsight: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <BackendInsightView
        style={{ width: 400, height: 400 }}
        insight={INSIGHT_CONFIGURATION_MOCK}
        telemetryService={NOOP_TELEMETRY_SERVICE}
        telemetryRecorder={noOpTelemetryRecorder}
    />
)

const COMPONENT_MIGRATION_INSIGHT_CONFIGURATION: SearchBasedInsight = {
    type: InsightType.SearchBased,
    id: 'backend-mock',
    title: 'Backend Insight Mock',
    series: [
        { id: '001', name: 'wildcard', query: '', stroke: 'blue' },
        { id: '002', name: 'branded', query: '', stroke: 'orange' },
        { id: '003', name: 'shared', query: '', stroke: 'red' },
    ],
    step: { weeks: 2 },
    filters: {
        excludeRepoRegexp: '',
        includeRepoRegexp: '',
        context: '',
        seriesDisplayOptions: {
            limit: 20,
            numSamples: 12,
            sortOptions: {
                direction: SeriesSortDirection.DESC,
                mode: SeriesSortMode.RESULT_COUNT,
            },
        },
    },
    dashboardReferenceCount: 0,
    isFrozen: false,
    repositories: [],
    repoQuery: '',
    dashboards: [],
}

const DATA_FETCHING_INSIGHT_CONFIGURATION: SearchBasedInsight = {
    type: InsightType.SearchBased,
    id: 'backend-mock',
    title: 'Backend Insight Mock',
    series: [
        { id: '001', name: 'requestGraphql', query: '', stroke: 'blue' },
        { id: '002', name: 'queryGraphQL | mutateGraphQL', query: '', stroke: 'orange' },
        { id: '003', name: 'useMutation | useQuery | useConnection hooks', query: '', stroke: 'red' },
    ],
    step: { weeks: 2 },
    filters: {
        excludeRepoRegexp: '',
        includeRepoRegexp: '',
        context: '',
        seriesDisplayOptions: {
            limit: 20,
            numSamples: 12,
            sortOptions: {
                direction: SeriesSortDirection.DESC,
                mode: SeriesSortMode.RESULT_COUNT,
            },
        },
    },
    dashboardReferenceCount: 0,
    isFrozen: false,
    repositories: [],
    repoQuery: '',
    dashboards: [],
}

const TERRAFORM_INSIGHT_CONFIGURATION: CaptureGroupInsight = {
    type: InsightType.CaptureGroup,
    id: 'backend-mock',
    title: 'Backend Insight Mock',
    step: { weeks: 2 },
    repositories: [],
    repoQuery: '',
    query: '',
    filters: {
        excludeRepoRegexp: '',
        includeRepoRegexp: '',
        context: '',
        seriesDisplayOptions: {
            limit: 20,
            numSamples: 12,
            sortOptions: {
                direction: SeriesSortDirection.DESC,
                mode: SeriesSortMode.RESULT_COUNT,
            },
        },
    },
    dashboardReferenceCount: 0,
    isFrozen: false,
    dashboards: [],
}

const BACKEND_INSIGHT_COMPONENT_MIGRATION_MOCK: MockedResponse<GetInsightViewResult> = {
    request: {
        query: GET_INSIGHT_VIEW_GQL,
        variables: {
            id: 'backend-mock',
            filters: { includeRepoRegex: '', excludeRepoRegex: '', searchContexts: [''] },
            seriesDisplayOptions: {
                limit: 20,
                numSamples: 12,
                sortOptions: {
                    direction: SeriesSortDirection.DESC,
                    mode: SeriesSortMode.RESULT_COUNT,
                },
            },
        },
    },
    result: {
        data: {
            insightViews: {
                nodes: [
                    {
                        id: 'aW5zaWdodF92aWV3OiIyNU9aNFFpTERPMGRQVUZSQWNtYnBvZ1hhWnMi',
                        dataSeries: [
                            {
                                seriesId: '001',
                                label: 'Wildcard components',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:03:19Z',
                                        value: 1311,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:43Z',
                                        value: 586,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:25Z',
                                        value: 586,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 1212,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 1164,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 490,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 393,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 357,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 348,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 276,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 213,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 192,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 81,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    __typename: 'InsightSeriesStatus',
                                    incompleteDatapoints: [],
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '002',
                                label: 'Branded components',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:03:24Z',
                                        value: 538,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:44Z',
                                        value: 283,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:25Z',
                                        value: 283,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 516,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 513,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 275,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 267,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 261,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 252,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 243,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 216,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 213,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 213,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    __typename: 'InsightSeriesStatus',
                                    incompleteDatapoints: [],
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '003',
                                label: 'Shared components',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:03:37Z',
                                        value: 468,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:44Z',
                                        value: 328,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:25Z',
                                        value: 328,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 475,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 477,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 671,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 660,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 648,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 642,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 621,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 606,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 573,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 564,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    __typename: 'InsightSeriesStatus',
                                    incompleteDatapoints: [],
                                },
                                __typename: 'InsightsSeries',
                            },
                        ],
                        __typename: 'InsightView',
                    },
                ],
                __typename: 'InsightViewConnection',
            },
        },
    },
}

const BACKEND_INSIGHT_DATA_FETCHING_MOCK: MockedResponse<GetInsightViewResult> = {
    request: {
        query: GET_INSIGHT_VIEW_GQL,
        variables: {
            id: 'backend-mock',
            filters: { includeRepoRegex: '', excludeRepoRegex: '', searchContexts: [''] },
            seriesDisplayOptions: {
                limit: 20,
                numSamples: 12,
                sortOptions: {
                    direction: SeriesSortDirection.DESC,
                    mode: SeriesSortMode.RESULT_COUNT,
                },
            },
        },
    },
    result: {
        data: {
            insightViews: {
                nodes: [
                    {
                        id: 'aW5zaWdodF92aWV3OiIyNU9ZY1VQdThxeXpvcnR1WmJXZE9qdVh2Y2Yi',
                        dataSeries: [
                            {
                                seriesId: '001',
                                label: 'requestGraphQL',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:02:30Z',
                                        value: 235,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T00:13:43Z',
                                        value: 239,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T00:13:20Z',
                                        value: 228,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-20T00:00:00Z',
                                        value: 232,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-20T00:00:00Z',
                                        value: 226,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-20T00:00:00Z',
                                        value: 217,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-20T00:00:00Z',
                                        value: 214,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-20T00:00:00Z',
                                        value: 212,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-20T00:00:00Z',
                                        value: 227,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-20T00:00:00Z',
                                        value: 218,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-20T00:00:00Z',
                                        value: 211,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-20T00:00:00Z',
                                        value: 213,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-20T00:00:00Z',
                                        value: 200,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    __typename: 'InsightSeriesStatus',
                                    incompleteDatapoints: [],
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '002',
                                label: 'queryGraphQL | mutateGraphQL',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:02:31Z',
                                        value: 71,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T00:13:42Z',
                                        value: 71,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T00:13:20Z',
                                        value: 73,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-20T00:00:00Z',
                                        value: 73,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-20T00:00:00Z',
                                        value: 74,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-20T00:00:00Z',
                                        value: 73,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-20T00:00:00Z',
                                        value: 73,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-20T00:00:00Z',
                                        value: 75,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-20T00:00:00Z',
                                        value: 76,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-20T00:00:00Z',
                                        value: 79,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-20T00:00:00Z',
                                        value: 80,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-20T00:00:00Z',
                                        value: 80,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-20T00:00:00Z',
                                        value: 82,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    __typename: 'InsightSeriesStatus',
                                    incompleteDatapoints: [],
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '003',
                                label: 'useMutation | useQuery | useConnection hooks',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:02:53Z',
                                        value: 227,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T00:13:48Z',
                                        value: 219,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T00:13:21Z',
                                        value: 200,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-20T00:00:00Z',
                                        value: 156,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-20T00:00:00Z',
                                        value: 109,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-20T00:00:00Z',
                                        value: 102,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-20T00:00:00Z',
                                        value: 85,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-20T00:00:00Z',
                                        value: 62,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-20T00:00:00Z',
                                        value: 49,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-20T00:00:00Z',
                                        value: 24,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-20T00:00:00Z',
                                        value: 11,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-20T00:00:00Z',
                                        value: 5,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-20T00:00:00Z',
                                        value: 5,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    __typename: 'InsightSeriesStatus',
                                    incompleteDatapoints: [],
                                },
                                __typename: 'InsightsSeries',
                            },
                        ],
                        __typename: 'InsightView',
                    },
                ],
                __typename: 'InsightViewConnection',
            },
        },
    },
}

const BACKEND_INSIGHT_TERRAFORM_AWS_VERSIONS_MOCK: MockedResponse<GetInsightViewResult> = {
    request: {
        query: GET_INSIGHT_VIEW_GQL,
        variables: {
            id: 'backend-mock',
            filters: { includeRepoRegex: '', excludeRepoRegex: '', searchContexts: [''] },
            seriesDisplayOptions: {
                limit: 20,
                numSamples: 12,
                sortOptions: {
                    direction: SeriesSortDirection.DESC,
                    mode: SeriesSortMode.RESULT_COUNT,
                },
            },
        },
    },
    result: {
        data: {
            insightViews: {
                nodes: [
                    {
                        id: 'aW5zaWdodF92aWV3OiIyNU9lSm8xcTZub05nUkh3aG9MWEdCdUdtN3Yi',
                        dataSeries: [
                            {
                                seriesId: '25OeJqBD4dOacJDmOA1cXY97Iyb',
                                label: 'v 1.x',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:03:02Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:41Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:19Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    incompleteDatapoints: [],
                                    __typename: 'InsightSeriesStatus',
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '25OeJrXQQNK0fjqOqybnEMsHnyR',
                                label: 'v 2.x',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:03:05Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:41Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:20Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 12,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 0,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    incompleteDatapoints: [],
                                    __typename: 'InsightSeriesStatus',
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '25OeJlaDvpkh02abDBsPisvDSdi',
                                label: 'v 3.x',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:02:49Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:41Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:20Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 4,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    incompleteDatapoints: [],
                                    __typename: 'InsightSeriesStatus',
                                },
                                __typename: 'InsightsSeries',
                            },
                            {
                                seriesId: '25OeJmiafuCZe66UR7yQFV3663S',
                                label: 'v 4.x',
                                points: [
                                    {
                                        dateTime: '2022-04-26T00:02:59Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-04-21T01:13:43Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-03-21T01:13:20Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-02-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2022-01-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-12-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-11-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-10-21T00:00:00Z',
                                        value: 2,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-09-21T00:00:00Z',
                                        value: 0,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-08-21T00:00:00Z',
                                        value: 0,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-07-21T00:00:00Z',
                                        value: 0,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-06-21T00:00:00Z',
                                        value: 0,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                    {
                                        dateTime: '2021-05-21T00:00:00Z',
                                        value: 0,
                                        __typename: 'InsightDataPoint',
                                        diffQuery: 'type:diff',
                                    },
                                ],
                                status: {
                                    isLoadingData: false,
                                    incompleteDatapoints: [],
                                    __typename: 'InsightSeriesStatus',
                                },
                                __typename: 'InsightsSeries',
                            },
                        ],
                        __typename: 'InsightView',
                    },
                ],
                __typename: 'InsightViewConnection',
            },
        },
    },
}

export const BackendInsightDemoCasesShowcase: StoryFn = () => (
    <div>
        <MockedTestProvider mocks={[BACKEND_INSIGHT_COMPONENT_MIGRATION_MOCK]}>
            <BackendInsightView
                style={{ width: 400, height: 400 }}
                insight={COMPONENT_MIGRATION_INSIGHT_CONFIGURATION}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </MockedTestProvider>

        <MockedTestProvider mocks={[BACKEND_INSIGHT_DATA_FETCHING_MOCK]}>
            <BackendInsightView
                style={{ width: 400, height: 400 }}
                insight={DATA_FETCHING_INSIGHT_CONFIGURATION}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </MockedTestProvider>

        <MockedTestProvider mocks={[BACKEND_INSIGHT_TERRAFORM_AWS_VERSIONS_MOCK]}>
            <BackendInsightView
                style={{ width: 400, height: 400 }}
                insight={TERRAFORM_INSIGHT_CONFIGURATION}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        </MockedTestProvider>
    </div>
)

export const BackendInsightVitrine: StoryFn = () => (
    <section>
        <article>
            <H2>Card</H2>
            <MockedTestProvider addTypename={true} mocks={mockInsightAPIResponse()}>
                <TestBackendInsight />
            </MockedTestProvider>
        </article>
        <article className="mt-3">
            <H2>Card with delay API</H2>
            <MockedTestProvider mocks={mockInsightAPIResponse({ delayAmount: 2000 })}>
                <TestBackendInsight />
            </MockedTestProvider>
        </article>
        <article className="mt-3">
            <H2>Card backfilling data</H2>
            <MockedTestProvider addTypename={true} mocks={mockInsightAPIResponse({ isFetchingHistoricalData: true })}>
                <TestBackendInsight />
            </MockedTestProvider>
        </article>
        <article className="mt-3">
            <H2>Card no data</H2>
            <MockedTestProvider addTypename={true} mocks={mockInsightAPIResponse({ hasData: false })}>
                <TestBackendInsight />
            </MockedTestProvider>
        </article>
        <article className="mt-3">
            <H2>Card insight syncing</H2>
            <MockedTestProvider addTypename={true} mocks={mockInsightAPIResponse({ throwProcessingError: true })}>
                <TestBackendInsight />
            </MockedTestProvider>
        </article>
        <article className="mt-3">
            <H2>Locked Card insight</H2>
            <MockedTestProvider addTypename={true} mocks={mockInsightAPIResponse()}>
                <BackendInsightView
                    style={{ width: 400, height: 400 }}
                    insight={{ ...INSIGHT_CONFIGURATION_MOCK, isFrozen: true }}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    telemetryRecorder={noOpTelemetryRecorder}
                />
            </MockedTestProvider>
        </article>
    </section>
)
