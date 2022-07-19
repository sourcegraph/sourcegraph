import React, { useMemo, useState, useEffect } from 'react'

import classNames from 'classnames'
import { RouteComponentProps } from 'react-router'

import { useQuery } from '@sourcegraph/http-client'
import { AlertType } from '@sourcegraph/shared/src/graphql-operations'
import { Card, LoadingSpinner, useMatchMedia, Text } from '@sourcegraph/wildcard'

import { LineChart, Series } from '../../../charts'
import { BarChart } from '../../../charts/components/bar-chart/BarChart'
import { GlobalAlert } from '../../../global/GlobalAlert'
import { AnalyticsDateRange, UsersStatisticsResult, UsersStatisticsVariables } from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'
import { AnalyticsPageTitle } from '../components/AnalyticsPageTitle'
import { ChartContainer } from '../components/ChartContainer'
import { HorizontalSelect } from '../components/HorizontalSelect'
import { ToggleSelect } from '../components/ToggleSelect'
import { ValueLegendList, ValueLegendListProps } from '../components/ValueLegendList'
import { StandardDatum, FrequencyDatum, buildFrequencyDatum } from '../utils'

import { USERS_STATISTICS } from './queries'

export const AnalyticsUsersPage: React.FunctionComponent<RouteComponentProps<{}>> = () => {
    const [eventAggregation, setEventAggregation] = useState<'count' | 'uniqueUsers'>('uniqueUsers')
    const [dateRange, setDateRange] = useState<AnalyticsDateRange>(AnalyticsDateRange.LAST_MONTH)
    const { data, error, loading } = useQuery<UsersStatisticsResult, UsersStatisticsVariables>(USERS_STATISTICS, {
        variables: {
            dateRange,
        },
    })
    useEffect(() => {
        eventLogger.logPageView('AdminAnalyticsUsers')
    }, [])
    const [frequencies, legends] = useMemo(() => {
        if (!data) {
            return []
        }
        const { users } = data.site.analytics
        const legends: ValueLegendListProps['items'] = [
            {
                value: users.activity.summary.totalUniqueUsers,
                description: 'Active users',
                color: 'var(--purple)',
            },
            {
                value: data.users.totalCount,
                description: 'Registered Users',
                color: 'var(--body-color)',
                position: 'right',
            },
            {
                value: data.site.productSubscription.license?.userCount ?? 0,
                description: 'User licenses',
                color: 'var(--body-color)',
                position: 'right',
            },
        ]

        const frequencies: FrequencyDatum[] = buildFrequencyDatum(users.frequencies, 1, 30)

        return [frequencies, legends]
    }, [data])

    const activities = useMemo(() => {
        if (!data) {
            return []
        }
        const { users } = data.site.analytics
        const activities: Series<StandardDatum>[] = [
            {
                id: 'activity',
                name: eventAggregation === 'count' ? 'Activities' : 'Active users',
                color: eventAggregation === 'count' ? 'var(--cyan)' : 'var(--purple)',
                data: users.activity.nodes.map(
                    node => ({
                        date: new Date(node.date),
                        value: node[eventAggregation],
                    }),
                    dateRange
                ),
                getXValue: ({ date }) => date,
                getYValue: ({ value }) => value,
            },
        ]

        return activities
    }, [data, eventAggregation, dateRange])

    const summary = useMemo(() => {
        if (!data) {
            return []
        }
        const { avgDAU, avgWAU, avgMAU } = data.site.analytics.users.summary
        return [
            {
                value: avgDAU,
                label: 'DAU',
            },
            {
                value: avgWAU,
                label: 'WAU',
            },
            {
                value: avgMAU,
                label: 'MAU',
            },
        ]
    }, [data])

    const isWideScreen = useMatchMedia('(min-width: 992px)', false)

    if (error) {
        throw error
    }

    if (loading) {
        return <LoadingSpinner />
    }

    return (
        <>
            <AnalyticsPageTitle>Analytics / Users</AnalyticsPageTitle>
            <Card className="p-3">
                <div className="d-flex justify-content-end align-items-stretch mb-2">
                    <HorizontalSelect<AnalyticsDateRange>
                        label="Date&nbsp;range"
                        value={dateRange}
                        onChange={value => {
                            setDateRange(value)
                            eventLogger.log(`AdminAnalyticsUsersDateRange${value}Selected`)
                        }}
                        items={[
                            { value: AnalyticsDateRange.LAST_WEEK, label: 'Last week' },
                            { value: AnalyticsDateRange.LAST_MONTH, label: 'Last month' },
                            { value: AnalyticsDateRange.LAST_THREE_MONTHS, label: 'Last 3 months' },
                            { value: AnalyticsDateRange.CUSTOM, label: 'Custom (coming soon)', disabled: true },
                        ]}
                    />
                </div>
                <GlobalAlert
                    alert={{
                        message:
                            'Note these charts are experimental. For billing information, use [usage stats](/site-admin/usage-statistics).',
                        type: AlertType.INFO,
                        isDismissibleWithKey: '',
                    }}
                    className="my-3"
                />
                {legends && <ValueLegendList className="mb-3" items={legends} />}
                {activities && (
                    <div>
                        <ChartContainer
                            title={eventAggregation === 'count' ? 'Activity by day' : 'Unique users by day'}
                            labelX="Time"
                            labelY={eventAggregation === 'count' ? 'Activity' : 'Unique users'}
                        >
                            {width => <LineChart width={width} height={300} series={activities} />}
                        </ChartContainer>
                        <div className="d-flex justify-content-end align-items-stretch mb-2">
                            <ToggleSelect<typeof eventAggregation>
                                selected={eventAggregation}
                                onChange={value => {
                                    setEventAggregation(value)
                                    eventLogger.log(
                                        `AdminAnalyticsUsersAgg${value === 'count' ? 'Totals' : 'Uniques'}Clicked`
                                    )
                                }}
                                items={[
                                    {
                                        tooltip: 'total # of actions triggered',
                                        label: 'Totals',
                                        value: 'count',
                                    },
                                    {
                                        tooltip: 'unique # of users triggered',
                                        label: 'Uniques',
                                        value: 'uniqueUsers',
                                    },
                                ]}
                            />
                        </div>
                    </div>
                )}
                <div className={classNames(isWideScreen && 'd-flex')}>
                    {summary && (
                        <ChartContainer
                            title="Average user activity by period"
                            className="mb-5"
                            labelX="Average DAU/WAU/MAU"
                            labelY="Unique users"
                        >
                            {width => (
                                <BarChart
                                    width={isWideScreen ? 280 : width}
                                    height={300}
                                    data={summary}
                                    getDatumName={datum => datum.label}
                                    getDatumValue={datum => datum.value}
                                    getDatumColor={() => 'var(--oc-blue-2)'}
                                />
                            )}
                        </ChartContainer>
                    )}
                    {frequencies && (
                        <ChartContainer
                            className="mb-5"
                            title="Frequency of use"
                            labelX="Days used"
                            labelY="Unique users"
                        >
                            {width => (
                                <BarChart
                                    width={isWideScreen ? 540 : width}
                                    height={300}
                                    data={frequencies}
                                    getDatumName={datum => datum.label}
                                    getDatumValue={datum => datum.value}
                                    getDatumColor={() => 'var(--oc-blue-2)'}
                                />
                            )}
                        </ChartContainer>
                    )}
                </div>
            </Card>
            <Text className="font-italic text-center mt-2">
                All events are generated from entries in the event logs table and are updated every 24 hours..
            </Text>
        </>
    )
}
