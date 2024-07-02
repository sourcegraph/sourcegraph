import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'

import { type InsightDataSeries, SearchPatternType } from '../../../../../graphql-operations'
import { PageRoutes } from '../../../../../routes.constants'
import type { BackendInsight, SearchBasedInsightSeries } from '../../types'
import type { BackendInsightDatum, BackendInsightSeries } from '../code-insights-backend-types'

import { getParsedSeriesMetadata } from './parse-series-metadata'

type SeriesDefinition = Record<string, SearchBasedInsightSeries>

interface LineChartContentInput {
    insight: BackendInsight
    seriesData: InsightDataSeries[]
    showError: boolean
    defaultPatternType: SearchPatternType
}

/**
 * Generates line chart content for visx chart. Note that this function relies on the fact that
 * all series are indexed.
 */
export function createLineChartContent(input: LineChartContentInput): BackendInsightSeries<BackendInsightDatum>[] {
    const { insight, seriesData, showError } = input
    const seriesDefinition = getParsedSeriesMetadata(insight, seriesData)
    const seriesDefinitionMap: SeriesDefinition = Object.fromEntries<SearchBasedInsightSeries>(
        seriesDefinition.map(definition => [definition.id, definition])
    )

    return seriesData.map<BackendInsightSeries<BackendInsightDatum>>(line => ({
        id: line.seriesId,
        alerts: showError ? line.status.incompleteDatapoints : [],
        data: line.points.map(point => ({
            dateTime: new Date(point.dateTime),
            value: point.value,
            link: generateLinkURL(point.pointInTimeQuery, input.defaultPatternType),
        })),
        name: seriesDefinitionMap[line.seriesId]?.name ?? line.label,
        color: seriesDefinitionMap[line.seriesId]?.stroke,
        getYValue: datum => datum.value,
        getXValue: datum => datum.dateTime,
        getLinkURL: datum => datum.link,
    }))
}

/**
 * Minimal input type model for {@link createLineChartContent} function
 */
export type InsightDataSeriesData = Pick<InsightDataSeries, 'seriesId' | 'label' | 'points'>

export function generateLinkURL(query: string | null, defaultPatternType: SearchPatternType): string | undefined {
    if (query) {
        const searchQueryParameter = buildSearchURLQuery(query, defaultPatternType, false)
        return `${window.location.origin}${PageRoutes.Search}?${searchQueryParameter}`
    }

    return undefined
}
