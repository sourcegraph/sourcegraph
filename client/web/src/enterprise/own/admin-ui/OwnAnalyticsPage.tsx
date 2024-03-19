import { useEffect, type FC } from 'react'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { useQuery } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Alert, BarChart, Card, ErrorAlert, Link, LoadingSpinner, Text } from '@sourcegraph/wildcard'

import type {
    GetInstanceOwnStatsResult,
    GetOwnSignalConfigurationsResult,
    OwnSignalConfig,
} from '../../../graphql-operations'
import { AnalyticsPageTitle } from '../../../site-admin/analytics/components/AnalyticsPageTitle'
import { ChartContainer } from '../../../site-admin/analytics/components/ChartContainer'

import { GET_INSTANCE_OWN_STATS, GET_OWN_JOB_CONFIGURATIONS } from './query'

interface OwnCoverageDatum {
    name: string
    count: number
    fill: string
    tooltip: string
}

interface OwnAnalyticsPageProps extends TelemetryV2Props {}

export const OwnAnalyticsPage: FC<OwnAnalyticsPageProps> = ({ telemetryRecorder }) => {
    const { data, loading, error } = useQuery<GetOwnSignalConfigurationsResult>(GET_OWN_JOB_CONFIGURATIONS, {})
    const enabled =
        data?.ownSignalConfigurations.some(
            (config: OwnSignalConfig) => config.name === 'analytics' && config.isEnabled
        ) || false

    useEffect(() => {
        telemetryRecorder.recordEvent('admin.analytics.own', 'view')
    }, [telemetryRecorder])

    return (
        <>
            {loading && <LoadingSpinner />}
            {error && <ErrorAlert prefix="Error finding out if own analytics are enabled" error={error} />}
            <AnalyticsPageTitle>Code ownership</AnalyticsPageTitle>
            {enabled ? <OwnAnalyticsPanel /> : <OwnEnableAnalytics />}
        </>
    )
}

const OwnAnalyticsPanel: FC = () => {
    const { data, loading, error } = useQuery<GetInstanceOwnStatsResult>(GET_INSTANCE_OWN_STATS, {})

    const totalFiles = data?.instanceOwnershipStats.totalFiles || 0
    const totalCodeownedFiles = data?.instanceOwnershipStats.totalCodeownedFiles || 0
    const totalAssignedOwnershipFiles = data?.instanceOwnershipStats.totalAssignedOwnershipFiles || 0
    const totalOwnedFiles = data?.instanceOwnershipStats.totalOwnedFiles || 0

    // Use Math.max(totalFiles, 1) to make sure we do not divide by 0.
    const totalCodeownedFilesPercent = Math.round((totalCodeownedFiles / Math.max(totalFiles, 1)) * 100)
    const totalAssignedOwnershipFilesPercent = Math.round((totalAssignedOwnershipFiles / Math.max(totalFiles, 1)) * 100)
    const totalOwnedFilesPercent = Math.round((totalOwnedFiles / Math.max(totalFiles, 1)) * 100)

    const ownSignalsData: OwnCoverageDatum[] = [
        {
            name: 'CODEOWNERS',
            count: totalCodeownedFilesPercent,
            fill: 'var(--info-2)',
            tooltip: `Files owned through CODEOWNERS: ${totalCodeownedFiles}/${totalFiles}`,
        },
        {
            name: 'Assigned ownership',
            count: totalAssignedOwnershipFilesPercent,
            fill: 'var(--info)',
            tooltip: `Files with assigned owners: ${totalAssignedOwnershipFiles}/${totalFiles}`,
        },
        {
            name: 'All owned files',
            count: totalOwnedFilesPercent,
            fill: 'var(--info-3)',
            tooltip: `Owned files: ${totalOwnedFiles}/${totalFiles}`,
        },
    ]

    const lastUpdatedAt = data?.instanceOwnershipStats.updatedAt && (
        <>
            Last generated: <Timestamp date={data.instanceOwnershipStats.updatedAt} />
        </>
    )

    return (
        <>
            {loading && <LoadingSpinner />}
            {error && <ErrorAlert prefix="Error getting code ownership analytics" error={error} />}
            {!loading && !error && (
                <>
                    {/* TODO(#52826): If only partial data is available - make that clear to the user. */}
                    <Card className="p-3 position-relative">
                        {ownSignalsData && (
                            <div>
                                <ChartContainer
                                    title="Ownership coverage in %"
                                    labelX="Ownership type"
                                    labelY="Files percentage"
                                >
                                    {width => (
                                        <BarChart
                                            width={width}
                                            height={300}
                                            data={ownSignalsData}
                                            maxValueLowerBound={100}
                                            getDatumName={datum => datum.name}
                                            getDatumValue={datum => datum.count}
                                            getDatumColor={datum => datum.fill}
                                            getDatumFadeColor={() => 'var(--gray-04)'}
                                            getDatumHover={datum => datum.tooltip}
                                            getDatumHoverValueLabel={datum => `${datum.count}%`}
                                        />
                                    )}
                                </ChartContainer>
                            </div>
                        )}
                    </Card>
                    <Text className="font-italic text-center mt-2">
                        Data is generated periodically from CODEOWNERS files and repository contents.{' '}
                        {lastUpdatedAt && lastUpdatedAt}
                    </Text>
                </>
            )}
        </>
    )
}

const OwnEnableAnalytics: FC = () => (
    <Alert variant="info">
        Analytics is not enabled, please <Link to="/site-admin/own-signal-page">enable code ownership analytics</Link>{' '}
        job first to see code ownership stats.
    </Alert>
)
