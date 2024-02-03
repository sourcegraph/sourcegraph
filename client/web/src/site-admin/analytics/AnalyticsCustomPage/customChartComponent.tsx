import React, { useMemo } from 'react'

import { startCase } from 'lodash'

import { useQuery } from '@sourcegraph/http-client'
import { H2, LineChart, LoadingSpinner, Series } from '@sourcegraph/wildcard'

import { CustomStatisticsResult, CustomStatisticsVariables } from '../../../graphql-operations'
import { ChartContainer } from '../components/ChartContainer'
import { HorizontalSelect } from '../components/HorizontalSelect'
import { ToggleSelect } from '../components/ToggleSelect'
import { IResult } from '../useChartFilters'
import { StandardDatum } from '../utils'

import { CUSTOM_STATISTICS } from './queries'

interface Props extends Pick<IResult, 'dateRange' | 'aggregation' | 'grouping'> {
    debouncedSearchText: string[]
}

export const AnalyticsCustomChartComponent: React.FunctionComponent<Props> = (props: Props) => {
    const { dateRange, aggregation, debouncedSearchText, grouping } = props
    const queryVariables = {
        dateRange: dateRange.value,
        grouping: grouping.value,
        events: debouncedSearchText,
    }
    const {
        data: chartData,
        error: chartError,
        loading: chartLoading,
    } = useQuery<CustomStatisticsResult, CustomStatisticsVariables>(CUSTOM_STATISTICS, { variables: queryVariables })
    const activities = useMemo(() => {
        if (!chartData) {
            return []
        }
        const { users } = chartData.site.analytics.custom
        const activities: Series<StandardDatum>[] = [
            {
                id: 'custom-actions',
                name: aggregation.selected === 'count' ? 'Total actions' : 'Unique users doing the actions',
                color: 'var(--orange)',
                data: users.nodes.map(
                    node => ({
                        date: new Date(node.date),
                        value: node[aggregation.selected],
                    }),
                    dateRange.value
                ),
                getXValue: ({ date }) => date,
                getYValue: ({ value }) => value,
            },
        ]

        return activities
    }, [chartData, aggregation.selected, dateRange.value])

    if (chartError) {
        throw chartError
    }

    if (chartLoading) {
        return <LoadingSpinner />
    }

    const groupingLabel = startCase(grouping.value.toLowerCase())

    return (
        <>
            <H2 className="my-3">Statistics</H2>
            {activities && (
                <div>
                    <ChartContainer
                        title={
                            aggregation.selected === 'count'
                                ? `${groupingLabel} activity`
                                : `${groupingLabel} unique users`
                        }
                        labelX="Time"
                        labelY={aggregation.selected === 'count' ? 'Activity' : 'Unique users'}
                    >
                        {width => <LineChart width={width} height={300} series={activities} />}
                    </ChartContainer>
                    <div className="d-flex justify-content-end align-items-stretch mb-4 text-nowrap">
                        <HorizontalSelect<typeof grouping.value> {...grouping} className="mr-4" />
                        <ToggleSelect<typeof aggregation.selected> {...aggregation} />
                    </div>
                </div>
            )}
        </>
    )
}
