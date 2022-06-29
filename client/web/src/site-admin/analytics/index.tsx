/* eslint-disable react/forbid-dom-props */
import { useMemo, useState } from 'react'

import { mdiChartLineVariant, mdiChartTimelineVariantShimmer } from '@mdi/js'
import classNames from 'classnames'
import { addDays, getDayOfYear, startOfDay, startOfWeek, sub } from 'date-fns'
import { upperFirst } from 'lodash'
import { RouteComponentProps } from 'react-router'

import { useQuery } from '@sourcegraph/http-client'
import {
    H2,
    Card,
    Select,
    Input,
    H3,
    Text,
    Icon,
    ButtonGroup,
    Button,
    LoadingSpinner,
    Tooltip,
} from '@sourcegraph/wildcard'

import { LineChart, ParentSize, Series } from '../../charts'
import { AnalyticsDateRange, SearchStatisticsResult, SearchStatisticsVariables } from '../../graphql-operations'

import { formatNumber } from './format-number'
import { SEARCH_STATISTICS } from './queries'

import styles from './index.module.scss'

interface CalculatorProps {
    color: string
    label: string
    value: number
    minPerItem: number
    description?: string
}

const TimeSavedCalculator: React.FunctionComponent<CalculatorProps> = ({
    color,
    value,
    description,
    minPerItem: minsPerCount,
    label,
}) => {
    const [minutesPerCount, setMinutesPerCount] = useState(minsPerCount)
    const hoursSaved = useMemo(() => (value * minutesPerCount) / 60, [value, minutesPerCount])
    return (
        <Card className="mb-3 p-4 d-flex justify-content-between flex-row" key={label}>
            <div className={styles.calculatorInnerLeft}>
                <Text as="span" style={{ color }} className={classNames(styles.count, 'text-center')}>
                    {formatNumber(value)}
                </Text>
                <Input
                    type="number"
                    value={minutesPerCount}
                    className={styles.calculatorInput}
                    onChange={event => setMinutesPerCount(Number(event.target.value))}
                />
                <Text as="span" className={styles.count}>
                    {formatNumber(hoursSaved)}
                </Text>
                <Text as="span">{label}</Text>
                <Text as="span">
                    minutes saved
                    <br />
                    per action
                </Text>
                <Text as="span">hours saved</Text>
            </div>
            <div className="m-0 flex-1 d-flex flex-column justify-content-between">
                <Text as="span" className="font-weight-bold">
                    About this statistics
                </Text>
                <Text as="span">{description}</Text>
            </div>
        </Card>
    )
}

interface StandardDatum {
    date: Date
    value: number
}

interface ChatLegendItemProps {
    color: string
    description: string
    value: number
}

const ChartLegendItem: React.FunctionComponent<ChatLegendItemProps> = ({ value, color, description }) => (
    <div className="d-flex flex-column align-items-center mr-3 justify-content-center">
        <span style={{ color }} className={styles.count}>
            {formatNumber(value)}
        </span>
        <span className={classNames('text-center', styles.textWrap)}>{description}</span>
    </div>
)

interface ChartLegendListProps {
    className?: string
    items: (ChatLegendItemProps & { position?: 'left' | 'right' })[]
}

const ChartLegendList: React.FunctionComponent<ChartLegendListProps> = ({ items, className }) => (
    <div className={classNames('d-flex justify-content-between', className)}>
        <div className="d-flex justify-content-left">
            {items
                .filter(item => item.position !== 'right')
                .map(item => (
                    <ChartLegendItem key={item.description} {...item} />
                ))}
        </div>
        <div className="d-flex justify-content-right">
            {items
                .filter(item => item.position === 'right')
                .map(item => (
                    <ChartLegendItem key={item.description} {...item} />
                ))}
        </div>
    </div>
)

interface DateRangeSelectorProps {
    onDateRangeChange: (dateRange: AnalyticsDateRange) => void
    dateRange: AnalyticsDateRange
    className?: string
}

const DateRangeSelector: React.FunctionComponent<DateRangeSelectorProps> = ({
    dateRange,
    onDateRangeChange,
    className,
}) => (
    <Select
        id="date-range"
        label="Date&nbsp;range"
        isCustomStyle={true}
        className={classNames('d-flex align-items-center m-0', className)}
        labelClassName="mb-0"
        selectClassName="ml-2"
        onChange={value => onDateRangeChange(value.target.value as AnalyticsDateRange)}
    >
        {[
            { value: AnalyticsDateRange.LAST_WEEK, label: 'Last week' },
            { value: AnalyticsDateRange.LAST_MONTH, label: 'Last month' },
            { value: AnalyticsDateRange.LAST_THREE_MONTHS, label: 'Last 3 months' },
            { value: AnalyticsDateRange.CUSTOM, label: 'Custom (coming soon)', disabled: true },
        ].map(({ value, label, disabled }) => (
            <option key={value} value={value} selected={dateRange === value} disabled={disabled}>
                {label}
            </option>
        ))}
    </Select>
)
interface ChartStatItem {
    name: string
    color: string
    totalCount: number
    series?: StandardDatum[]
    legendPosition?: 'left' | 'right'
}

interface ChartProps {
    stats: ChartStatItem[]
    labelY?: string
    labelX?: string
    className?: string
}

const Chart: React.FunctionComponent<ChartProps> = ({ stats, labelY, labelX, className }) => {
    const series: Series<StandardDatum>[] = useMemo(
        () =>
            stats
                .filter(item => item.series)
                .map(item => ({
                    id: item.name,
                    name: item.name,
                    data: item.series as StandardDatum[],
                    color: item.color,
                    getXValue: datum => datum.date,
                    getYValue: datum => datum.value,
                })),
        [stats]
    )
    const legendList = useMemo(
        () =>
            stats.map(item => ({
                value: item.totalCount,
                color: item.color,
                description: item.name,
                position: item.legendPosition,
            })),
        [stats]
    )
    return (
        <div className={className}>
            <ChartLegendList className="mb-3" items={legendList} />
            <div className="d-flex mr-1">
                {labelY && <span className={styles.chartYLabel}>{labelY}</span>}
                <ParentSize>{({ width }) => <LineChart width={width} height={400} series={series} />}</ParentSize>
            </div>
            {labelX && <div className={styles.chartXLabel}>{labelX}</div>}
        </div>
    )
}

interface ToggleGroupProps<T> {
    selected: T
    items: {
        tooltip: string
        label: string
        value: T
    }[]
    onChange: (value: T) => void
}

const ToggleSelect = <T extends any>({
    selected,
    items,
    onChange,
}: React.PropsWithChildren<ToggleGroupProps<T>>): JSX.Element => (
    <ButtonGroup>
        {items.map(({ tooltip, label, value }) => (
            <Tooltip key={label} content={tooltip} placement="top">
                <Button
                    onClick={() => onChange(value)}
                    outline={selected !== value}
                    variant={selected !== value ? 'secondary' : 'primary'}
                    display="inline"
                    size="sm"
                >
                    {label}
                </Button>
            </Tooltip>
        ))}
    </ButtonGroup>
)

function fillWithEmptyDatum(datums: StandardDatum[], dateRange: AnalyticsDateRange): StandardDatum[] {
    // Generates 0 value series for dates that don't exist in the original data
    const [to, daysOffset] =
        dateRange === AnalyticsDateRange.LAST_THREE_MONTHS
            ? [startOfWeek(new Date(), { weekStartsOn: 1 }), 7]
            : [startOfDay(new Date()), 1]
    const from =
        dateRange === AnalyticsDateRange.LAST_THREE_MONTHS
            ? sub(to, { months: 3 })
            : dateRange === AnalyticsDateRange.LAST_MONTH
            ? sub(to, { months: 1 })
            : sub(to, { weeks: 1 })
    const newDatums: StandardDatum[] = []
    let date = to
    while (date >= from) {
        const datum = datums?.find(datum => getDayOfYear(datum.date) === getDayOfYear(date))
        newDatums.push(datum ? { ...datum, date } : { date, value: 0 })
        date = addDays(date, -daysOffset)
    }

    return newDatums
}

export const AnalyticsSearchPage: React.FunctionComponent<RouteComponentProps<{}>> = () => {
    const [eventAggregation, setEventAggregation] = useState<'count' | 'uniqueUsers'>('count')
    const [dateRange, setDateRange] = useState<AnalyticsDateRange>(AnalyticsDateRange.LAST_WEEK)
    const { data, error, loading } = useQuery<SearchStatisticsResult, SearchStatisticsVariables>(SEARCH_STATISTICS, {
        variables: {
            dateRange,
        },
    })
    const [stats, timeSavedStats] = useMemo(() => {
        if (!data) {
            return []
        }
        const { searches, fileViews, fileOpens } = data.site.analytics.search
        const stats = [
            {
                ...searches.summary,
                totalCount: searches.summary[eventAggregation === 'count' ? 'totalCount' : 'totalUniqueUsers'],
                name: eventAggregation === 'count' ? 'Searches' : 'Users searched',
                color: 'var(--cyan)',
                series: fillWithEmptyDatum(
                    searches.nodes.map(node => ({
                        date: new Date(node.date),
                        value: node[eventAggregation],
                    })),
                    dateRange
                ),
            },
            {
                ...fileViews.summary,
                totalCount: fileViews.summary[eventAggregation === 'count' ? 'totalCount' : 'totalUniqueUsers'],
                name: eventAggregation === 'count' ? 'File views' : 'Users viewed files',
                color: 'var(--orange)',
                series: fillWithEmptyDatum(
                    fileViews.nodes.map(node => ({
                        date: new Date(node.date),
                        value: node[eventAggregation],
                    })),
                    dateRange
                ),
            },
            {
                ...fileOpens.summary,
                totalCount: fileOpens.summary[eventAggregation === 'count' ? 'totalCount' : 'totalUniqueUsers'],
                name: eventAggregation === 'count' ? 'File opens' : 'Users opened files',
                color: 'var(--body-color)',
                series: fillWithEmptyDatum(
                    fileOpens.nodes.map(node => ({
                        date: new Date(node.date),
                        value: node[eventAggregation],
                    })),
                    dateRange
                ),
            },
        ]
        const timeSavedStats = [
            {
                label: 'Searches,\nfile views\n& file opens',
                color: 'var(--purple)',
                minPerItem: 5,
                description:
                    'Each search or file view represents a developer solving a code use problem, getting information an active incident, or other use case. ',
                value: searches.summary.totalCount + fileViews.summary.totalCount + fileOpens.summary.totalCount,
            },
        ]
        return [stats, timeSavedStats]
    }, [data, eventAggregation, dateRange])

    if (error) {
        return <div>Something went wrong! :( Please, try again later. </div>
    }

    if (loading) {
        return <LoadingSpinner />
    }

    return (
        <>
            <H2 className="mb-4 d-flex align-items-center">
                <Icon
                    className="mr-1"
                    color="var(--link-color)"
                    svgPath={mdiChartLineVariant}
                    size="sm"
                    aria-label="Search Statistics"
                />
                Analytics / Search
            </H2>

            <Card className="p-2 position-relative">
                <div className="d-flex justify-content-end align-items-stretch">
                    <DateRangeSelector dateRange={dateRange} onDateRangeChange={setDateRange} className="mr-2" />
                    <ToggleSelect<typeof eventAggregation>
                        selected={eventAggregation}
                        onChange={setEventAggregation}
                        items={[
                            { tooltip: 'total # of actions triggered', label: 'Totals', value: 'count' },
                            {
                                tooltip: 'unique # of users triggered',
                                label: 'Uniques',
                                value: 'uniqueUsers',
                            },
                        ]}
                    />
                </div>
                {stats && <Chart className="ml-4" labelX="Time" stats={stats} />}
                <H3 className="my-3">Time saved</H3>
                {timeSavedStats?.map(timeSavedStatItem => (
                    <TimeSavedCalculator key={timeSavedStatItem.label} {...timeSavedStatItem} />
                ))}
            </Card>
        </>
    )
}

export const AnalyticsComingSoon: React.FunctionComponent<RouteComponentProps<{}>> = props => {
    const title = useMemo(() => {
        // eslint-disable-next-line unicorn/prefer-array-find
        const title = props.match.path.split('/').filter(Boolean)[2] ?? 'Overview'
        return upperFirst(title.replace('-', ' '))
    }, [props.match.path])
    return (
        <>
            <H2 className="mb-4 d-flex align-items-center">
                <Icon
                    className="mr-1"
                    color="var(--link-color)"
                    svgPath={mdiChartLineVariant}
                    size="sm"
                    aria-label="Search Statistics"
                />
                Analytics / {title}
            </H2>
            <div className="d-flex flex-column justify-content-center align-items-center p-5">
                <Icon
                    svgPath={mdiChartTimelineVariantShimmer}
                    aria-label="Home analytics icon"
                    className={classNames(styles.largeIcon, 'm-3')}
                />
                <H3>Coming soon</H3>
                <Text>We are working on making this live.</Text>
            </div>
        </>
    )
}
