import { AnalyticsDateRange, AnalyticsGrouping } from '../../graphql-operations'
import { useURLSyncedState } from '../../hooks'
import { eventLogger } from '../../tracking/eventLogger'

type IAggregation = 'count' | 'uniqueUsers'

interface IProps {
    name: string
    aggregation?: IAggregation
    dateRange?: AnalyticsDateRange
    grouping?: AnalyticsGrouping
    events?: string
}

export interface IResult {
    dateRange: {
        value: AnalyticsDateRange
        label: string
        onChange: (value: AnalyticsDateRange) => void
        items: { value: AnalyticsDateRange; label: string }[]
    }
    aggregation: {
        selected: IAggregation
        onChange: (value: IAggregation) => void
        items: { value: IAggregation; label: string; tooltip: string }[]
    }
    grouping: {
        value: AnalyticsGrouping
        label: string
        onChange: (value: AnalyticsGrouping) => void
        items: { value: AnalyticsGrouping; label: string }[]
    }
    events: {
        value: string
        label: string
        onChange: (value: string) => void
    }
}

export function useChartFilters(props: IProps): IResult {
    const [data, setData] = useURLSyncedState({
        dateRange: props.dateRange || AnalyticsDateRange.LAST_THREE_MONTHS,
        aggregation: props.aggregation || 'count',
        grouping: props.grouping || AnalyticsGrouping.WEEKLY,
        events: props.events || '',
    })

    return {
        dateRange: {
            value: data.dateRange,
            label: 'Date range',
            onChange: value => {
                setData({
                    dateRange: value,
                    grouping:
                        value === AnalyticsDateRange.LAST_WEEK ? AnalyticsGrouping.DAILY : AnalyticsGrouping.WEEKLY,
                })
                eventLogger.log(`AdminAnalytics${props.name}DateRange${value}`)
            },
            items: [
                { value: AnalyticsDateRange.LAST_WEEK, label: 'Last week' },
                { value: AnalyticsDateRange.LAST_MONTH, label: 'Last month' },
                { value: AnalyticsDateRange.LAST_THREE_MONTHS, label: 'Last 3 months' },
            ],
        },
        aggregation: {
            selected: data.aggregation,
            onChange: value => {
                setData({ aggregation: value })
                eventLogger.log(`AdminAnalytics${props.name}Aggregate${value === 'count' ? 'Totals' : 'Uniques'}`)
            },
            items: [
                {
                    tooltip: 'total # of actions triggered',
                    label: 'Total events',
                    value: 'count',
                },
                {
                    tooltip: 'unique # of users triggered',
                    label: 'Unique users',
                    value: 'uniqueUsers',
                },
            ],
        },
        grouping: {
            value: data.grouping,
            label: 'Display as',
            onChange: value => {
                setData({ grouping: value })
                eventLogger.log(`AdminAnalytics${props.name}DisplayAs${value}`)
            },
            items: [
                { value: AnalyticsGrouping.DAILY, label: 'Daily' },
                { value: AnalyticsGrouping.WEEKLY, label: 'Weekly' },
            ],
        },
        events: {
            value: data.events,
            label: 'Event names to query',
            onChange: value => {
                setData({ events: value })
            },
        },
    }
}
