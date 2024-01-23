import { type FC, useEffect } from 'react'

import { mdiClockOutline, mdiServer, mdiTimerSand } from '@mdi/js'
import classNames from 'classnames'
import { parseISO } from 'date-fns'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { H3, PageHeader, Text, ErrorAlert, LoadingSpinner, Icon, Tooltip, Link } from '@sourcegraph/wildcard'

import { PageTitle } from '../components/PageTitle'
import { GitserverFields } from '../graphql-operations'
import { humanizeSize } from '../util/size'

import { useGitserversConnection } from './backend'

import styles from './SiteAdminGitserversPage.module.scss'

export interface GitserversPageProps extends TelemetryProps {}

export const SiteAdminGitserversPage: FC<GitserversPageProps> = ({ telemetryService }) => {
    useEffect(() => {
        telemetryService.logPageView('SiteAdminGitserversPage')
    }, [telemetryService])

    const { data, loading, error } = useGitserversConnection()

    return (
        <div>
            <PageTitle title="Gitservers" />
            <PageHeader
                path={[{ icon: mdiServer }, { text: 'Gitservers' }]}
                className="mb-3"
                headingElement="h2"
                description="Overview of all gitserver instances, on which this Sourcegraph instance stores source code."
            />

            {error && <ErrorAlert error={error} />}
            {loading && <LoadingSpinner />}
            {!loading && data && (
                <ul className={styles.wrapper}>
                    {data.gitservers.nodes.map((gitserverInfo, index) => {
                        const serverName = `Server ${index + 1}`
                        return (
                            <GitserverInstanceNode
                                key={gitserverInfo.id}
                                node={gitserverInfo}
                                serverName={serverName}
                                gitserverDiskUsageWarningThreshold={data.site.gitserverDiskUsageWarningThreshold}
                            />
                        )
                    })}
                </ul>
            )}
        </div>
    )
}

const GitserverInstanceNode: React.FunctionComponent<{
    node: GitserverFields
    serverName: string
    gitserverDiskUsageWarningThreshold: number
}> = ({ node, serverName, gitserverDiskUsageWarningThreshold }) => {
    const usedDiskSpace = BigInt(node.totalDiskSpaceBytes) - BigInt(node.freeDiskSpaceBytes)
    const hasExceededThreshold =
        Number(usedDiskSpace) / Number(BigInt(node.totalDiskSpaceBytes)) >= gitserverDiskUsageWarningThreshold / 100.0

    return (
        <li className={classNames('border', styles.node)}>
            <div className="d-flex justify-content-between">
                <H3>{serverName}</H3>
                <div className="d-flex">
                    <div className="mr-2">
                        <Tooltip content={`${node.repositoryJobs.stats.queued} repository jobs in queue`}>
                            <Icon
                                svgPath={mdiTimerSand}
                                aria-label={`${node.repositoryJobs.stats.queued} repository jobs in queue`}
                            />
                        </Tooltip>{' '}
                        {node.repositoryJobs.stats.queued}
                    </div>
                    <div>
                        <Tooltip content={`${node.repositoryJobs.stats.processing} repository jobs processing`}>
                            <Icon
                                svgPath={mdiClockOutline}
                                aria-label={`${node.repositoryJobs.stats.processing} repository jobs processing`}
                            />
                        </Tooltip>{' '}
                        {node.repositoryJobs.stats.processing}
                    </div>
                </div>
            </div>
            <hr className="mb-3" />
            <div>
                <Text className="d-flex justify-content-between">
                    <span className="font-weight-medium">Address</span>
                    <span className="text-muted">{node.address}</span>
                </Text>
                <Text className="d-flex justify-content-between">
                    <span className="font-weight-medium">Free disk space</span>
                    <span
                        className={classNames(
                            !hasExceededThreshold && 'text-muted',
                            hasExceededThreshold && 'text-warning'
                        )}
                    >
                        {humanizeSize(Number(node.freeDiskSpaceBytes))}
                    </span>
                </Text>
                <Text className="d-flex justify-content-between">
                    <span className="font-weight-medium">Total disk space</span>
                    <span className="text-muted">{humanizeSize(Number(node.totalDiskSpaceBytes))}</span>
                </Text>
                <Text className="d-flex justify-content-between">
                    <span className="font-weight-medium">Longest time in queue</span>
                    <span className="text-muted">
                        {node.repositoryJobs.stats.longestQueuedTime === null && <>No jobs in queue</>}
                        {node.repositoryJobs.stats.longestQueuedTime !== null && (
                            <Timestamp date={parseISO(node.repositoryJobs.stats.longestQueuedTime)} noAgo={true} />
                        )}
                    </span>
                </Text>
                <Text className="mb-0">
                    <Link to={'/site-admin/gitservers/' + node.id}>View recent jobs</Link>
                </Text>
            </div>
        </li>
    )
}
