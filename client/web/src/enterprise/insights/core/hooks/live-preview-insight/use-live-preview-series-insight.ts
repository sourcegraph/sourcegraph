import { useEffect, useMemo, useState } from 'react'

import { type ApolloError, gql, useApolloClient } from '@apollo/client'
import type { Duration } from 'date-fns'
import { noop } from 'lodash'

import { HTTPStatusError } from '@sourcegraph/http-client'
import type { RepositoryScopeInput } from '@sourcegraph/shared/src/graphql-operations'
import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import type { Series } from '@sourcegraph/wildcard'

import type {
    GetInsightPreviewResult,
    GetInsightPreviewVariables,
    SearchSeriesPreviewInput,
} from '../../../../../graphql-operations'
import { defaultPatternTypeFromSettings } from '../../../../../util/settings'
import { DATA_SERIES_COLORS_LIST, MAX_NUMBER_OF_SERIES } from '../../../constants'
import { getStepInterval } from '../../backend/gql-backend/utils/get-step-interval'
import { generateLinkURL, type InsightDataSeriesData } from '../../backend/utils/create-line-chart-content'

import { LivePreviewStatus, type State } from './types'

export const GET_INSIGHT_PREVIEW_GQL = gql`
    query GetInsightPreview($input: SearchInsightPreviewInput!) {
        searchInsightPreview(input: $input) {
            points {
                dateTime
                value
                pointInTimeQuery
            }
            label
        }
    }
`

export interface SeriesWithStroke extends SearchSeriesPreviewInput {
    stroke?: string
}

interface Props {
    skip: boolean
    step: Duration
    repoScope: RepositoryScopeInput
    series: SeriesWithStroke[]
}

interface Result<R> {
    state: State<R>
    refetch: () => unknown
}

interface QueryResult {
    loading: boolean
    data?: GetInsightPreviewResult
    error?: ApolloError
    refetch: () => unknown
}

/**
 * Series insight (search based and capture group insights) live preview hook.
 * It's used primarily for presenting insight live preview data in the creation UI pages.
 *
 * All data for insight live preview isn't stored in the code insights DB tables
 * instead, it's calculated on the fly in query time on the backend.
 */
export function useLivePreviewSeriesInsight(props: Props): Result<Series<Datum>[]> {
    const { skip, repoScope, step, series } = props
    const [unit, value] = getStepInterval(step)

    const client = useApolloClient()
    // Apollo refetch doesn't work properly with watchQuery when stream gets query error
    // in order to recreate refetch we have here synthetic state which we update on every
    // refetch request and this triggers watchQuery re-subscribtion, see use effect below.
    const [counter, fourceUpdate] = useState(0)
    const [{ data, loading, error, refetch }, setResult] = useState<QueryResult>({
        data: undefined,
        loading: true,
        error: undefined,
        refetch: noop,
    })

    useEffect(() => {
        // Reset internal query result state if we run query again
        setResult({
            loading: !skip,
            data: undefined,
            error: undefined,
            refetch: noop,
        })

        if (skip) {
            return
        }

        // We have to work with apollo client directly since use query hook doesn't
        // cancel request automatically, there is a long conversation about it here
        // https://github.com/apollographql/apollo-client/issues/8858
        //
        // In the future we could write our own link to work with useQuery but cancel
        // all request from previously calls, for now since watchQuery supports unsubscribe
        // we use it instead of generic solution.
        const query = client.watchQuery<GetInsightPreviewResult, GetInsightPreviewVariables>({
            query: GET_INSIGHT_PREVIEW_GQL,
            variables: {
                input: {
                    series: series.map(srs => ({
                        query: srs.query,
                        label: srs.label,
                        generatedFromCaptureGroups: srs.generatedFromCaptureGroups,
                        groupBy: srs.groupBy,
                    })),
                    repositoryScope: repoScope,
                    timeScope: { stepInterval: { unit, value: +value } },
                },
            },
        })

        const refetch = (): void => {
            fourceUpdate(state => state + 1)
        }

        const subscription = query.subscribe(
            event => setResult({ ...event, refetch }),
            error => setResult({ loading: false, data: undefined, error, refetch })
        )

        return () => subscription.unsubscribe()
    }, [client, repoScope, series, skip, unit, value, counter])

    const defaultPatternType = defaultPatternTypeFromSettings(useSettingsCascade())

    const parsedSeries = useMemo(() => {
        if (data) {
            return createPreviewSeriesContent({
                response: data,
                originalSeries: series,
                repositories: repoScope.repositories,
                defaultPatternType,
            })
        }

        return null
    }, [data, repoScope, series, defaultPatternType])

    if (loading) {
        return { state: { status: LivePreviewStatus.Loading }, refetch }
    }

    if (error) {
        if (isGatewayTimeoutError(error)) {
            return {
                state: {
                    status: LivePreviewStatus.Error,
                    error: new Error(
                        'Live preview is not available for this chart as it did not complete in the allowed time'
                    ),
                },
                refetch,
            }
        }

        return { state: { status: LivePreviewStatus.Error, error }, refetch }
    }

    if (parsedSeries) {
        return { state: { status: LivePreviewStatus.Data, data: parsedSeries }, refetch }
    }

    return { state: { status: LivePreviewStatus.Intact }, refetch }
}

export interface Datum {
    dateTime: Date
    value: number
    link?: string
}

interface PreviewProps {
    response: GetInsightPreviewResult
    originalSeries: SeriesWithStroke[]
    repositories: string[]
    defaultPatternType: SearchPatternType
}

function createPreviewSeriesContent(props: PreviewProps): Series<Datum>[] {
    const { response, originalSeries, defaultPatternType } = props
    const { searchInsightPreview: series } = response

    // inputMetadata creates a lookup so that the correct color can be later applied to the preview series
    const inputMetadata = Object.fromEntries(
        originalSeries.map((previewSeries, index) => [`${previewSeries.label}-${index}`, previewSeries])
    )

    // Extend series with synthetic index based series id
    const indexedSeries = series.slice(0, MAX_NUMBER_OF_SERIES).map<InsightDataSeriesData>((series, index) => ({
        seriesId: `${index}`,
        ...series,
    }))

    // TODO(insights): inputMetadata and this function need to be re-evaluated in the future if/when support for
    // mixing series types in a single insight is possible
    function getColorForSeries(label: string, index: number): string {
        return (
            inputMetadata[`${label}-${index}`]?.stroke ||
            DATA_SERIES_COLORS_LIST[index % DATA_SERIES_COLORS_LIST.length]
        )
    }

    return indexedSeries.map((line, index) => ({
        id: line.seriesId,
        data: line.points.map(point => ({
            value: point.value,
            dateTime: new Date(point.dateTime),
            link: generateLinkURL(point.pointInTimeQuery, defaultPatternType),
        })),
        name: line.label,
        color: getColorForSeries(line.label, index),
        getLinkURL: datum => datum.link,
        getYValue: datum => datum.value,
        getXValue: datum => datum.dateTime,
    }))
}

function isGatewayTimeoutError(error: ApolloError): boolean {
    return error.networkError instanceof HTTPStatusError && error.networkError.status === 504
}
