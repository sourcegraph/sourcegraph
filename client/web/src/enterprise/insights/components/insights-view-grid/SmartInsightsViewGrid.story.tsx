import type { MockedResponse } from '@apollo/client/testing/core'
import type { Meta } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../../../../components/WebStory'
import {
    type GetInsightViewResult,
    type GetInsightViewVariables,
    SeriesSortDirection,
    SeriesSortMode,
} from '../../../../graphql-operations'
import { InsightType, type BackendInsight } from '../../core'
import { GET_INSIGHT_VIEW_GQL } from '../../core/backend/gql-backend'

import { SmartInsightsViewGrid } from './SmartInsightsViewGrid'

const defaultStory: Meta = {
    title: 'web/insights/SmartInsightsViewGridExample',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
    parameters: {
        chromatic: {
            viewports: [576, 1440],
            enableDarkMode: true,
        },
    },
}

export default defaultStory

const DEFAULT_FILTERS = {
    excludeRepoRegexp: '',
    includeRepoRegexp: '',
    context: '',
    seriesDisplayOptions: {
        limit: 20,
        numSamples: null,
        sortOptions: {
            direction: SeriesSortDirection.DESC,
            mode: SeriesSortMode.RESULT_COUNT,
        },
    },
}

const INSIGHT_CONFIGURATIONS: BackendInsight[] = [
    {
        id: 'searchInsights.insight.Backend_1',
        repositories: [],
        repoQuery: '',
        type: InsightType.SearchBased,
        title: 'Backend insight #1',
        series: [{ id: '001', query: 'test_query', stroke: 'blue', name: 'series A' }],
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
    {
        id: 'searchInsights.insight.Backend_2',
        repositories: [],
        repoQuery: '',
        type: InsightType.SearchBased,
        title: 'Backend insight #2',
        series: [
            { id: '001', query: 'test_query_1', stroke: 'blue', name: 'series A' },
            { id: '002', query: 'test_query_2', stroke: 'orange', name: 'series B' },
        ],
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
    {
        id: 'searchInsights.insight.Backend_3',
        repositories: [],
        repoQuery: '',
        type: InsightType.SearchBased,
        title: 'Backend insight #3',
        series: [
            { id: '001', query: 'test_query_1', stroke: 'blue', name: 'Series A' },
            { id: '002', query: 'test_query_2', stroke: 'green', name: 'Series B' },
            { id: '003', query: 'test_query_3', stroke: 'orange', name: 'Series C' },
        ],
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
    {
        id: 'searchInsights.insight.Backend_4',
        repositories: [],
        repoQuery: '',
        type: InsightType.SearchBased,
        title: 'Backend insight #4',
        series: [{ id: '001', query: 'test_query', stroke: 'blue', name: 'series A' }],
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
    {
        id: 'searchInsights.insight.Backend_5',
        repositories: [],
        repoQuery: '',
        type: InsightType.CaptureGroup,
        title: 'Backend insight #5',
        query: '',
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
    {
        id: 'searchInsights.insight.Backend_6',
        repositories: [],
        repoQuery: '',
        type: InsightType.SearchBased,
        title: 'Backend insight #6',
        series: [{ id: '001', query: 'test_query', stroke: 'blue', name: 'series A' }],
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
    {
        id: 'searchInsights.insight.Backend_7',
        repositories: [],
        repoQuery: '',
        type: InsightType.SearchBased,
        title: 'Backend insight #7',
        series: [{ id: '001', query: 'test_query', stroke: 'red', name: 'series A' }],
        step: { days: 1 },
        filters: DEFAULT_FILTERS,
        dashboardReferenceCount: 0,
        isFrozen: false,
        dashboards: [],
    },
]

const INSIGHT_DATA_MOCKS: MockedResponse<GetInsightViewResult>[] = [
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_1'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_1',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 8000,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 16000,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                                reason: 'Unable to process the current point',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                                reason: 'Exceeds error limit',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Tue May 05 2020 16:22:40 GMT-0300 (-03)',
                                                reason: 'Exceeds error limit',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Wed May 06 2020 16:22:40 GMT-0300 (-03)',
                                                reason: 'Unable to process the current point',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Tue May 05 2020 16:23:40 GMT-0300 (-03)',
                                                reason: 'Exceeds error limit',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Wed May 06 2020 16:23:40 GMT-0300 (-03)',
                                                reason: 'Exceeds error limit',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Tue May 05 2020 16:20:40 GMT-0300 (-03)',
                                                reason: 'Unable to process the current point',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Wed May 06 2020 16:20:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:19:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Wed May 06 2020 16:19:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:18:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Wed May 06 2020 16:18:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2021 16:21:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Wed May 06 2021 16:21:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2021 16:22:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Wed May 06 2021 16:22:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_2'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_2',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 12300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '002',
                                    label: 'Series B',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 10800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 14300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_3'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_3',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 12300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '002',
                                    label: 'Series B',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 10800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 14300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                            {
                                                __typename: 'GenericIncompleteDatapointAlert',
                                                time: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                                reason: 'Exceeds error limit, please rerun insight series',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '003',
                                    label: 'Series C',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 7500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 11800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 15300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_4'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_4',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_5'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_5',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 12300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '002',
                                    label: 'Series B',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 5000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 10800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 14300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '003',
                                    label: 'Series C',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 6000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 7500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 11800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 15300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '004',
                                    label: 'Series D',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 7000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 7000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 8500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 12800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 16300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '005',
                                    label: 'Series F',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 8000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 8000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 13800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 17300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '006',
                                    label: 'Series G',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 8000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 8000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 13800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 17300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '007',
                                    label: 'Series K',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 9000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 10500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 14800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 18300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '008',
                                    label: 'Series L',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 10000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 10000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 11500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 15800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 19300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '009',
                                    label: 'Series M',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 11000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 11000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 12500,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 16800,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 20300,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [
                                            {
                                                __typename: 'TimeoutDatapointAlert',
                                                time: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            },
                                        ],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_6'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_6',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
    {
        request: {
            query: GET_INSIGHT_VIEW_GQL,
            variables: generateDefaultRequestVariables('searchInsights.insight.Backend_7'),
        },
        result: {
            data: {
                insightViews: {
                    __typename: 'InsightViewConnection',
                    nodes: [
                        {
                            __typename: 'InsightView',
                            id: 'searchInsights.insight.Backend_7',
                            dataSeries: [
                                {
                                    __typename: 'InsightsSeries',
                                    seriesId: '001',
                                    label: 'Series A',
                                    points: [
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Mon May 04 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Tue May 05 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Wed May 06 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Thu May 07 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                        {
                                            __typename: 'InsightDataPoint',
                                            value: 4000,
                                            dateTime: 'Fri May 08 2020 16:21:40 GMT-0300 (-03)',
                                            diffQuery: 'type:diff',
                                        },
                                    ],
                                    status: {
                                        __typename: 'InsightSeriesStatus',
                                        isLoadingData: false,
                                        incompleteDatapoints: [],
                                    },
                                },
                            ],
                        },
                    ],
                },
            },
        },
    },
]

function generateDefaultRequestVariables(insightId: string): GetInsightViewVariables {
    return {
        id: insightId,
        filters: { includeRepoRegex: '', excludeRepoRegex: '', searchContexts: [''] },
        seriesDisplayOptions: {
            limit: 20,
            sortOptions: { direction: SeriesSortDirection.DESC, mode: SeriesSortMode.RESULT_COUNT },
        },
    }
}

export const SmartInsightsViewGridExample = (): JSX.Element => (
    <MockedTestProvider mocks={INSIGHT_DATA_MOCKS} addTypename={true}>
        <SmartInsightsViewGrid
            id="test-insight-id"
            persistSizeAndOrder={false}
            insights={INSIGHT_CONFIGURATIONS}
            telemetryService={NOOP_TELEMETRY_SERVICE}
            telemetryRecorder={noOpTelemetryRecorder}
        />
    </MockedTestProvider>
)
