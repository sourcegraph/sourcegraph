import { type FC, useEffect, useMemo } from 'react'

import { mdiServer, mdiTimelineClockOutline } from '@mdi/js'
import classNames from 'classnames'
import { format, parseISO, subDays } from 'date-fns'
import indicator from 'ordinal/indicator'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    H3,
    PageHeader,
    Text,
    Link,
    Container,
    Badge,
    Alert,
    Tooltip,
    Icon,
    BarChart,
    Button,
} from '@sourcegraph/wildcard'

import { ExecutionLogEntry } from '../components/ExecutionLogEntry'
import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    ShowMoreButton,
    SummaryContainer,
} from '../components/FilteredConnection/ui'
import { PageTitle } from '../components/PageTitle'
import { Duration } from '../components/time/Duration'
import { RepositoryJobFields, RepositoryJobState } from '../graphql-operations'

import { ChartContainer } from './analytics/components/ChartContainer'
import { REPOSITORY_JOBS_PER_PAGE_COUNT, useGlobalRepositoryJobsConnection } from './backend'

import styles from './SiteAdminRepositoryJobsPage.module.scss'

export interface SiteAdminRepositoryJobsPageProps extends TelemetryProps {}

export const SiteAdminRepositoryJobsPage: FC<SiteAdminRepositoryJobsPageProps> = ({ telemetryService }) => {
    const now = useMemo(() => new Date(), [])

    useEffect(() => {
        telemetryService.logPageView('SiteAdminRepositoryJobsPage')
    }, [telemetryService])

    const { connection, hasNextPage, fetchMore, loading, error } = useGlobalRepositoryJobsConnection()

    return (
        <div>
            <PageTitle title="Repository Jobs" />
            <PageHeader
                path={[{ icon: mdiServer }, { text: 'Repository Jobs' }]}
                className="mb-3"
                headingElement="h2"
                description="Overview of all recent repository jobs that ran. Repository jobs include things like cloning, updating, and maintaining repositories."
            />

            <Container className="mb-4">
                <ChartContainer title="Repository fetches" labelY="Runtime" className="w-50 d-inline-block">
                    {width => (
                        <BarChart<RepositoryJobRuntimeDatum>
                            width={width}
                            height={150}
                            data={[
                                {
                                    date: subDays(now, 5).toISOString(),
                                    runtimeSeconds: 12312,
                                    success: true,
                                },
                                {
                                    date: subDays(now, 4).toISOString(),
                                    runtimeSeconds: 14022,
                                    success: true,
                                },
                                {
                                    date: subDays(now, 3).toISOString(),
                                    runtimeSeconds: 10293,
                                    success: true,
                                },
                                {
                                    date: subDays(now, 2).toISOString(),
                                    runtimeSeconds: 2923,
                                    success: true,
                                },
                                {
                                    date: subDays(now, 1).toISOString(),
                                    runtimeSeconds: 18393,
                                    success: false,
                                },
                                {
                                    date: now.toISOString(),
                                    runtimeSeconds: 14953,
                                    success: true,
                                },
                            ]}
                            getDatumColor={({ success }) => (success ? 'var(--green)' : 'var(--red)')}
                            getDatumName={({ date }) => format(parseISO(date), 'MM-dd')}
                            getDatumValue={({ runtimeSeconds }) => runtimeSeconds}
                        />
                    )}
                </ChartContainer>
                <ChartContainer title="Repository cleanup" labelY="Runtime" className="w-50 d-inline-block">
                    {width => (
                        <BarChart<RepositoryJobRuntimeDatum>
                            width={width}
                            height={150}
                            data={[
                                {
                                    date: subDays(now, 5).toISOString(),
                                    runtimeSeconds: 171,
                                    success: false,
                                },
                                {
                                    date: subDays(now, 4).toISOString(),
                                    runtimeSeconds: 182,
                                    success: false,
                                },
                                {
                                    date: subDays(now, 3).toISOString(),
                                    runtimeSeconds: 99,
                                    success: true,
                                },
                                {
                                    date: subDays(now, 2).toISOString(),
                                    runtimeSeconds: 923,
                                    success: true,
                                },
                                {
                                    date: subDays(now, 1).toISOString(),
                                    runtimeSeconds: 231,
                                    success: false,
                                },
                                {
                                    date: now.toISOString(),
                                    runtimeSeconds: 171,
                                    success: true,
                                },
                            ]}
                            getDatumColor={({ success }) => (success ? 'var(--green)' : 'var(--red)')}
                            getDatumName={({ date }) => format(parseISO(date), 'MM-dd')}
                            getDatumValue={({ runtimeSeconds }) => runtimeSeconds}
                        />
                    )}
                </ChartContainer>
            </Container>

            <Container className="mb-4">
                <ConnectionContainer>
                    {error && <ConnectionError errors={[error.message]} />}
                    <ConnectionList className={styles.grid} aria-label="repository jobs">
                        {connection?.nodes?.map(node => (
                            <RepositoryJobNode key={node.id} node={node} displayRepository={true} />
                        ))}
                    </ConnectionList>
                    {loading && <ConnectionLoading />}
                    {connection && (
                        <SummaryContainer centered={true}>
                            <ConnectionSummary
                                centered={true}
                                noSummaryIfAllNodesVisible={true}
                                first={REPOSITORY_JOBS_PER_PAGE_COUNT}
                                connection={connection}
                                noun="repository job"
                                pluralNoun="repository jobs"
                                hasNextPage={hasNextPage}
                                emptyElement={<ListEmptyElement />}
                            />
                            {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                        </SummaryContainer>
                    )}
                </ConnectionContainer>
            </Container>
        </div>
    )
}

interface RepositoryJobNodeProps {
    node: RepositoryJobFields
    displayRepository: boolean
}

export const RepositoryJobNode: React.FunctionComponent<React.PropsWithChildren<RepositoryJobNodeProps>> = ({
    node,
    displayRepository,
}) => {
    return (
        <li className={styles.node}>
            <span className={styles.nodeSeparator} />
            <StateBadge state={node.state} />
            <div className={styles.nodeContent}>
                <div className="m-0 d-md-flex d-block align-items-baseline">
                    <H3 className={classNames(styles.nodeTitle, 'm-0 d-md-inline-block d-block')}>
                        {displayRepository && (
                            <div className="d-md-inline-block d-block">
                                <Link className="text-muted" to={node.repository.url}>
                                    {node.repository.name}
                                </Link>
                                <span className="text-muted d-inline-block mx-1">/</span>
                            </div>
                        )}
                        <span className="mr-2">{node.type}</span>
                    </H3>
                    <small className="text-muted d-sm-block">
                        {node.finishedAt !== null && (
                            <>
                                finished <Timestamp date={node.finishedAt} />
                            </>
                        )}
                        {node.finishedAt === null && node.startedAt !== null && (
                            <>
                                started <Timestamp date={node.startedAt} />
                            </>
                        )}
                        {node.startedAt === null && (
                            <>
                                queued <Timestamp date={node.queuedAt} />
                            </>
                        )}{' '}
                        because {node.scheduleReason}.
                    </small>
                </div>
            </div>
            <div className="d-flex align-items-center justify-content-end">
                {typeof node.placeInQueue === 'number' && (
                    <Tooltip content={`This job is number ${node.placeInQueue} in the global queue`}>
                        <span className={classNames(styles.placeInQueue, 'd-flex align-items-center')}>
                            <Icon aria-hidden={true} svgPath={mdiTimelineClockOutline} />
                            <strong className="ml-1 mr-1">
                                <NumberInQueue number={node.placeInQueue} />
                            </strong>
                            in queue
                        </span>
                    </Tooltip>
                )}
                {node.state === RepositoryJobState.PROCESSING && (
                    <Button variant="secondary" size="sm" className="mr-2">
                        Cancel
                    </Button>
                )}
                {node.startedAt !== null && <Duration start={node.startedAt} end={node.finishedAt ?? undefined} />}
            </div>
            {node.executionLogs !== null && node.executionLogs.length > 0 && (
                <div className={styles.fullContent}>
                    {node.executionDeadlineSeconds !== null && (
                        <Alert variant="info">Execution deadline: {node.executionDeadlineSeconds} seconds</Alert>
                    )}
                    {node.executionLogs.map(logEntry => (
                        <ExecutionLogEntry key={logEntry.key} logEntry={logEntry} />
                    ))}
                </div>
            )}
            {node.failureMessage !== null && (
                <div className={classNames(styles.fullContent, 'w-100')}>
                    <Alert variant="danger" className="mb-0">
                        {node.failureMessage}
                    </Alert>
                </div>
            )}
        </li>
    )
}

const ListEmptyElement: React.FunctionComponent<React.PropsWithChildren<{}>> = () => {
    return (
        <div className="w-100 py-5 text-center">
            <Text>
                <strong>No repository jobs.</strong>
            </Text>
        </div>
    )
}

const StateBadge: React.FunctionComponent<React.PropsWithChildren<{ state: RepositoryJobState }>> = ({ state }) => {
    switch (state) {
        case RepositoryJobState.COMPLETED:
            return (
                <Badge variant="success" className={classNames('a11y-ignore', styles.nodeBadge, 'text-uppercase')}>
                    Completed
                </Badge>
            )
        case RepositoryJobState.FAILED:
            return (
                <Badge variant="danger" className={classNames(styles.nodeBadge, 'text-uppercase')}>
                    Failed
                </Badge>
            )
        case RepositoryJobState.QUEUED:
            return (
                <Badge variant="secondary" className={classNames(styles.nodeBadge, 'text-uppercase')}>
                    Queued
                </Badge>
            )
        case RepositoryJobState.QUEUED:
            return (
                <Badge variant="secondary" className={classNames(styles.nodeBadge, 'text-uppercase')}>
                    Queued
                </Badge>
            )
        case RepositoryJobState.CANCELING:
            return (
                <Badge variant="secondary" className={classNames(styles.nodeBadge, 'text-uppercase')}>
                    Canceling
                </Badge>
            )
        case RepositoryJobState.CANCELED:
            return (
                <Badge variant="secondary" className={classNames(styles.nodeBadge, 'text-uppercase')}>
                    Canceled
                </Badge>
            )
        case RepositoryJobState.PROCESSING:
            return (
                <Badge variant="secondary" className={classNames(styles.nodeBadge, 'text-uppercase')}>
                    Processing
                </Badge>
            )
    }
}

const NumberInQueue: React.FunctionComponent<React.PropsWithChildren<{ number: number }>> = ({ number }) => (
    <>
        {number}
        <sup>{indicator(number)}</sup>
    </>
)

interface RepositoryJobRuntimeDatum {
    date: string
    runtimeSeconds: number
    success: boolean
}
