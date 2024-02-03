import React, { useEffect } from 'react'

import { Card, useDebounce } from '@sourcegraph/wildcard'

import { eventLogger } from '../../../tracking/eventLogger'
import { AnalyticsPageTitle } from '../components/AnalyticsPageTitle'
import { HorizontalSelect } from '../components/HorizontalSelect'
import { useChartFilters } from '../useChartFilters'

import { AnalyticsCustomChartComponent } from './customChartComponent'
import { AnalyticsCustomConnectionComponent } from './customConnectionComponent'
import { EventNamesInputSection } from './eventNamesInputSection'

export const AnalyticsCustomPage: React.FunctionComponent<{}> = () => {
    const { dateRange, aggregation, events, grouping } = useChartFilters({ name: 'Custom', aggregation: 'uniqueUsers' })
    const debouncedSearchText = useDebounce(events.value, 300)
        .split(',')
        .map(eventNames => eventNames.trim())

    useEffect(() => {
        eventLogger.logPageView('AdminAnalyticsCustom')
    }, [])

    return (
        <>
            <AnalyticsPageTitle>Custom event tracking</AnalyticsPageTitle>
            <Card className="p-3">
                <div className="d-flex align-items-stretch mb-2 text-nowrap">
                    <EventNamesInputSection {...events} />
                </div>
                <div className="d-flex justify-content-end align-items-stretch mb-2 text-nowrap">
                    <HorizontalSelect<typeof dateRange.value> {...dateRange} />
                </div>

                <AnalyticsCustomChartComponent
                    dateRange={dateRange}
                    aggregation={aggregation}
                    debouncedSearchText={debouncedSearchText}
                    grouping={grouping}
                />
                <AnalyticsCustomConnectionComponent
                    dateRange={dateRange}
                    debouncedSearchText={debouncedSearchText}
                    grouping={grouping}
                />
            </Card>
        </>
    )
}
