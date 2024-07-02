import type { InsightDataNode } from '../../../../../../../graphql-operations'
import { SearchPatternType } from '../../../../../../../graphql-operations'
import { type BackendInsight, isComputeInsight, isCaptureGroupInsight } from '../../../../types'
import { InsightContentType } from '../../../../types/insight/common'
import type { BackendInsightData } from '../../../code-insights-backend-types'
import { createComputeCategoricalChart } from '../../../utils/create-categorical-content'
import { createLineChartContent } from '../../../utils/create-line-chart-content'

export const createBackendInsightData = (
    insight: BackendInsight,
    response: InsightDataNode,
    defaultPatternType: SearchPatternType
): BackendInsightData => {
    const seriesData = response.dataSeries
    const isFetchingHistoricalData = seriesData.some(({ status: { isLoadingData } }) => isLoadingData)
    const isAllSeriesErrored =
        seriesData.length > 0 && seriesData.every(series => series.status.incompleteDatapoints.length > 0)
    const topLevelIncompleteAlert = isAllSeriesErrored ? seriesData[0].status.incompleteDatapoints[0] : null

    if (isComputeInsight(insight)) {
        return {
            incompleteAlert: topLevelIncompleteAlert,
            isFetchingHistoricalData,
            data: {
                type: InsightContentType.Categorical,
                content: createComputeCategoricalChart(insight, seriesData),
            },
        }
    }

    if (isCaptureGroupInsight(insight)) {
        return {
            incompleteAlert: topLevelIncompleteAlert,
            isFetchingHistoricalData,
            data: {
                type: InsightContentType.Series,
                series: createLineChartContent({ insight, seriesData, showError: false, defaultPatternType }),
            },
        }
    }

    return {
        // Search based insight doesn't support top-level incomplete alerts, they will be generated on
        // series level, see createLineChartContent show error logic.
        incompleteAlert: null,
        isFetchingHistoricalData,
        data: {
            type: InsightContentType.Series,
            series: createLineChartContent({ insight, seriesData, showError: true, defaultPatternType }),
        },
    }
}
