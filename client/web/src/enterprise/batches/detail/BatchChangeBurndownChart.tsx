import * as H from 'history'
import React, { useMemo } from 'react'
import {
    Area,
    ComposedChart,
    LabelFormatter,
    Legend,
    ResponsiveContainer,
    Tooltip,
    XAxis,
    YAxis,
    TooltipPayload,
} from 'recharts'
import { ChangesetCountsOverTimeFields, Scalars } from '../../../graphql-operations'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { queryChangesetCountsOverTime as _queryChangesetCountsOverTime } from './backend'

interface Props {
    batchChangeID: Scalars['ID']
    history: H.History
    width?: string | number

    /** For testing only. */
    queryChangesetCountsOverTime?: typeof _queryChangesetCountsOverTime
}

const dateTickFormat = new Intl.DateTimeFormat(undefined, { month: 'long', day: 'numeric' })
const dateTickFormatter = (timestamp: number): string => dateTickFormat.format(timestamp)

// const tooltipLabelFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const tooltipLabelFormat = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
const tooltipLabelFormatter = (date: number): string => tooltipLabelFormat.format(date)

const toLocaleString = (value: number): string => value.toLocaleString()

const tooltipStyle: React.CSSProperties = {
    color: 'var(--body-color)',
    border: 'none',
    background: 'var(--body-bg)',
}

const commonAreaProps = {
    isAnimationActive: false,
    strokeWidth: 0,
    stackId: 'stack',
    type: 'stepBefore',
} as const

interface StateDefinition {
    fill: string
    label: string
    sortOrder: number
}

type DisplayableChangesetCounts = Pick<
    ChangesetCountsOverTimeFields,
    'openPending' | 'openChangesRequested' | 'openApproved' | 'closed' | 'merged' | 'draft'
>

const states: Record<keyof DisplayableChangesetCounts, StateDefinition> = {
    draft: { fill: 'var(--text-muted)', label: 'Draft', sortOrder: 5 },
    openPending: { fill: 'var(--warning)', label: 'Open & awaiting review', sortOrder: 4 },
    openChangesRequested: { fill: 'var(--danger)', label: 'Open & changes requested', sortOrder: 3 },
    openApproved: { fill: 'var(--success)', label: 'Open & approved', sortOrder: 2 },
    closed: { fill: 'var(--secondary)', label: 'Closed', sortOrder: 1 },
    merged: { fill: 'var(--merged)', label: 'Merged', sortOrder: 0 },
}

const tooltipItemSorter = ({ dataKey }: TooltipPayload): number =>
    states[dataKey as keyof DisplayableChangesetCounts].sortOrder

/**
 * A burndown chart showing progress of the batch change's changesets.
 */
export const BatchChangeBurndownChart: React.FunctionComponent<Props> = ({
    batchChangeID,
    queryChangesetCountsOverTime = _queryChangesetCountsOverTime,
    width = '100%',
}) => {
    const changesetCountsOverTime: ChangesetCountsOverTimeFields[] | undefined = useObservable(
        useMemo(() => queryChangesetCountsOverTime({ batchChange: batchChangeID }), [
            batchChangeID,
            queryChangesetCountsOverTime,
        ])
    )

    // Is loading.
    if (changesetCountsOverTime === undefined) {
        return (
            <div className="text-center">
                <LoadingSpinner className="icon-inline mx-auto my-4" />
            </div>
        )
    }

    if (changesetCountsOverTime.length <= 1) {
        return (
            <div className="col-md-8 offset-md-2 col-sm-12 card mt-5">
                <div className="card-body p-5">
                    <h2 className="text-center mb-4">The burndown chart requires 2 days of data</h2>
                    <p>
                        Come back in a few days and we'll be able to show you data on how your batch change is
                        progressing!
                    </p>
                </div>
            </div>
        )
    }
    const hasEntries = changesetCountsOverTime.some(counts => counts.total > 0)
    if (!hasEntries) {
        return (
            <div className="col-md-8 offset-md-2 col-sm-12 card mt-5">
                <div className="card-body p-5">
                    <h2 className="text-center mb-4">Burndown chart is not available</h2>
                    <p>The burndown chart will be shown when data is available.</p>
                </div>
            </div>
        )
    }
    return (
        <ResponsiveContainer width={width} height={300} className="test-batches-chart">
            <ComposedChart
                data={changesetCountsOverTime.map(snapshot => ({ ...snapshot, date: Date.parse(snapshot.date) }))}
            >
                <Legend verticalAlign="bottom" iconType="square" />
                <XAxis
                    dataKey="date"
                    domain={[
                        changesetCountsOverTime[0].date,
                        changesetCountsOverTime[changesetCountsOverTime.length - 1].date,
                    ]}
                    name="Time"
                    tickFormatter={dateTickFormatter}
                    type="number"
                    stroke="var(--text-muted)"
                    scale="time"
                />
                <YAxis
                    tickFormatter={toLocaleString}
                    stroke="var(--text-muted)"
                    type="number"
                    allowDecimals={false}
                    domain={[0, 'dataMax']}
                />
                <Tooltip
                    labelFormatter={tooltipLabelFormatter as LabelFormatter}
                    isAnimationActive={false}
                    wrapperStyle={{ border: '1px solid var(--border-color)' }}
                    contentStyle={tooltipStyle}
                    labelStyle={{ fontWeight: 'bold' }}
                    itemStyle={tooltipStyle}
                    itemSorter={tooltipItemSorter}
                />

                {Object.entries(states)
                    .sort(([, a], [, b]) => b.sortOrder - a.sortOrder)
                    .map(([dataKey, state]) => (
                        <Area
                            key={state.sortOrder}
                            dataKey={dataKey}
                            name={state.label}
                            fill={state.fill}
                            // The stroke is used to color the legend, which we
                            // want to match the fill color for each area.
                            stroke={state.fill}
                            {...commonAreaProps}
                        />
                    ))}
            </ComposedChart>
        </ResponsiveContainer>
    )
}
