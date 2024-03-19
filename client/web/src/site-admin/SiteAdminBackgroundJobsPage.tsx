import React, { useCallback, useEffect, useMemo, useState } from 'react'

import {
    mdiAccountHardHat,
    mdiAlert,
    mdiCached,
    mdiCheck,
    mdiClose,
    mdiDatabase,
    mdiHelp,
    mdiNumeric,
    mdiShape,
} from '@mdi/js'
import { format } from 'date-fns'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { pluralize } from '@sourcegraph/common'
import { useQuery } from '@sourcegraph/http-client'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Container,
    ErrorAlert,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    Select,
    Text,
    Tooltip,
    useSessionStorage,
} from '@sourcegraph/wildcard'

import { PageTitle } from '../components/PageTitle'
import { type BackgroundJobsResult, type BackgroundJobsVariables, BackgroundRoutineType } from '../graphql-operations'
import { formatDurationLong } from '../util/time'

import { ValueLegendList } from './analytics/components/ValueLegendList'
import { BACKGROUND_JOBS, BACKGROUND_JOBS_PAGE_POLL_INTERVAL_MS } from './backend'

import styles from './SiteAdminBackgroundJobsPage.module.scss'

export interface SiteAdminBackgroundJobsPageProps extends TelemetryProps {}

export type BackgroundJob = BackgroundJobsResult['backgroundJobs']['nodes'][0]
export type BackgroundRoutine = BackgroundJob['routines'][0]

// "short" runs are displayed with a “success” style.
// “long” runs are displayed with a “warning” style to make sure they stand out somewhat.
// “dangerous” runs are displayed with a “danger” style to make sure they stand out even more.
type RunLengthCategory = 'short' | 'long' | 'dangerous'

// The maximum number of recent runs to fetch for each routine.
const recentRunCount = 5

// A map of the routine icons by type
const routineTypeToIcon: Record<BackgroundRoutineType, string> = {
    [BackgroundRoutineType.PERIODIC]: mdiCached,
    [BackgroundRoutineType.PERIODIC_WITH_METRICS]: mdiNumeric,
    [BackgroundRoutineType.DB_BACKED]: mdiDatabase,
    [BackgroundRoutineType.CUSTOM]: mdiShape,
}

export const SiteAdminBackgroundJobsPage: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminBackgroundJobsPageProps>
> = ({ telemetryService }) => {
    // Log page view
    useEffect(() => {
        telemetryService.logPageView('SiteAdminBackgroundJobs')
    }, [telemetryService])

    // Data query and polling setting
    const { data, loading, error, stopPolling, startPolling } = useQuery<BackgroundJobsResult, BackgroundJobsVariables>(
        BACKGROUND_JOBS,
        {
            variables: { recentRunCount },
            pollInterval: BACKGROUND_JOBS_PAGE_POLL_INTERVAL_MS,
        }
    )
    const [polling, setPolling] = useState(true)
    const togglePolling = useCallback(() => {
        if (polling) {
            stopPolling()
        } else {
            startPolling(BACKGROUND_JOBS_PAGE_POLL_INTERVAL_MS)
        }
        setPolling(!polling)
    }, [polling, startPolling, stopPolling])

    return (
        <div>
            <PageTitle title="Background jobs - Admin" />
            <Button variant="secondary" onClick={togglePolling} className="float-right">
                {polling ? 'Pause polling' : 'Resume polling'}
            </Button>
            <PageHeader
                path={[{ text: 'Background jobs' }]}
                headingElement="h2"
                description={
                    <>
                        This page lists{' '}
                        <Link to="/help/admin/workers" target="_blank" rel="noopener noreferrer">
                            all running jobs
                        </Link>
                        , their routines, recent runs, any errors, timings, and stats.
                    </>
                }
                className="mb-3"
            />
            <Text>Terminology:</Text>
            <ul>
                <li>
                    <strong>Job</strong>: a bag of routines, started when the Cody app is launched
                </li>
                <li>
                    <strong>Routine</strong>: a background process that repeatedly executes its task indefinitely, using
                    an interval passed at start
                </li>
                <li>
                    <strong>Run</strong>: a single execution of a routine's task
                </li>
                <li>
                    <strong>Host</strong>: a Sourcegraph instance that starts some jobs when launched
                </li>
                <li>
                    <strong>Instance</strong>: a job ran on a host
                </li>
            </ul>
            <Container className="mb-3">
                {error && !loading && <ErrorAlert error={error} />}
                {loading && !error && <LoadingSpinner />}
                {!loading && !error && data?.backgroundJobs.nodes && <JobList jobs={data.backgroundJobs.nodes} />}
            </Container>
        </div>
    )
}

const JobList: React.FunctionComponent<{
    jobs: BackgroundJob[]
}> = ({ jobs }) => {
    const [onlyShowProblematic, setOnlyShowProblematic] = useSessionStorage(
        'site-admin.background-jobs.only-show-problematic-routines',
        false
    )

    const hostNames = useMemo(
        () =>
            jobs
                .map(job => job.routines[0]?.instances[0]?.hostName) // get the host name of the first routine
                .filter((host, index, hosts) => hosts.indexOf(host) === index) // deduplicate
                .filter(host => !!host), // remove undefined
        [jobs]
    )

    const problematicJobs = useMemo(
        () => jobs.filter(job => job.routines.some(routine => isRoutineProblematic(routine))),
        [jobs]
    )
    const jobsToDisplay = onlyShowProblematic ? problematicJobs : jobs

    return (
        <>
            <LegendList jobs={jobs} hostNameCount={hostNames.length} />
            {jobsToDisplay ? (
                <>
                    <div className={styles.tableHeader}>
                        <div>
                            <Select
                                aria-label="Filter for problematic routines"
                                onChange={value => setOnlyShowProblematic(value.target.value === 'problematic')}
                                selectClassName={styles.filterSelect}
                                defaultValue={onlyShowProblematic ? 'problematic' : 'all'}
                            >
                                <option value="all">Show all routines</option>
                                <option value="problematic">Only show problematic routines</option>
                            </Select>
                        </div>
                        <div className="text-center">Fastest / avg / slowest run (ms)</div>
                    </div>
                    <ul className="list-group list-group-flush">
                        {jobsToDisplay.map(job => (
                            <JobItem
                                key={job.name}
                                job={job}
                                hostNames={hostNames}
                                onlyShowProblematic={onlyShowProblematic}
                            />
                        ))}
                    </ul>
                </>
            ) : (
                'No jobs to display.'
            )}
        </>
    )
}

const JobItem: React.FunctionComponent<{ job: BackgroundJob; hostNames: string[]; onlyShowProblematic: boolean }> =
    React.memo(function JobItem({ job, hostNames, onlyShowProblematic }) {
        const jobHostNames = [
            ...new Set(job.routines.map(routine => routine.instances.map(instance => instance.hostName)).flat()),
        ].sort()

        return (
            <li key={job.name} className="list-group-item px-0 py-2">
                <div className="d-flex align-items-center justify-content-between mb-2">
                    <div className="d-flex flex-row align-items-center mb-0">
                        <Icon aria-hidden={true} svgPath={mdiAccountHardHat} />{' '}
                        <Text className="mb-0 ml-2">
                            <strong>{job.name}</strong>{' '}
                            <span className="text-muted">
                                (starts {job.routines.length} {pluralize('routine', job.routines.length)}
                                {hostNames.length > 1
                                    ? ` on ${jobHostNames.length} ${pluralize('instance', jobHostNames.length)}`
                                    : ''}
                                )
                            </span>
                        </Text>
                    </div>
                </div>
                {job.routines
                    .filter(routine => (onlyShowProblematic ? isRoutineProblematic(routine) : true))
                    .map(routine => (
                        <RoutineItem routine={routine} key={routine.name} />
                    ))}
            </li>
        )
    })

const LegendList: React.FunctionComponent<{ jobs: BackgroundJob[]; hostNameCount: number }> = React.memo(
    ({ jobs, hostNameCount }) => {
        const routineCount = jobs.reduce((acc, job) => acc + job.routines.length, 0)
        const routineInstanceCount = jobs.reduce(
            (acc, job) => acc + job.routines.reduce((acc, routine) => acc + routine.instances.length, 0),
            0
        )
        const recentRunErrors = jobs.reduce(
            (acc, job) =>
                acc +
                job.routines.reduce(
                    (acc, routine) => acc + routine.recentRuns.filter(run => run.errorMessage).length,
                    0
                ),
            0
        )

        const legends = [
            {
                value: jobs.length,
                description: pluralize('Job', jobs.length),
                tooltip: 'The number of known background jobs in the system.',
            },
            {
                value: routineCount,
                description: pluralize('Routine', routineCount),
                tooltip: 'The total number of routines across all jobs.',
            },
            {
                value: hostNameCount,
                description: pluralize('Host', hostNameCount),
                tooltip: 'The total number of known hosts where jobs run.',
            },
            {
                value: routineInstanceCount,
                description: pluralize('Instance', routineInstanceCount),
                tooltip: 'The total number of routine instances across all jobs and hosts.',
            },
            {
                value: recentRunErrors,
                description: pluralize('Recent error', recentRunErrors),
                color: recentRunErrors > 0 ? 'var(--red)' : undefined,
                tooltip: 'The total number of errors across all runs across all routine instances.',
            },
        ]

        return <ValueLegendList className="mb-3" items={legends} />
    }
)

const RoutineItem: React.FunctionComponent<{ routine: BackgroundRoutine }> = ({ routine }) => {
    const allHostNames = routine.recentRuns
        .map(run => run.hostName) // get host name
        .filter((host, index, hosts) => hosts.indexOf(host) === index) // deduplicate
    const commonHostName = allHostNames.length === 1 ? allHostNames[0] : undefined

    const routineTypeDisplayableName = routine.type.toLowerCase().replaceAll('_', ' ')

    const recentRunsTooltipContent = (
        <div>
            {commonHostName ? <Text className="mb-0">All on “{commonHostName}”:</Text> : ''}
            <ul className="pl-4">
                {routine.recentRuns.map(run => (
                    <li key={run.at}>
                        <Text className="mb-0">
                            {run.errorMessage ? (
                                <Icon aria-hidden={true} svgPath={mdiAlert} className="text-danger" />
                            ) : (
                                ''
                            )}{' '}
                            <Timestamp date={new Date(run.at)} noAbout={true} />
                            {commonHostName ? '' : ` on the host called “${run.hostName}”,`} for{' '}
                            <span className={getRunDurationTextClass(run.durationMs, routine.intervalMs)}>
                                {run.durationMs}ms
                            </span>
                            .{run.errorMessage ? ` Error: ${run.errorMessage}` : ''}
                        </Text>
                    </li>
                ))}
            </ul>
        </div>
    )
    const recentRunsWithErrors = routine.recentRuns.filter(run => run.errorMessage)

    return (
        <div className={styles.routine}>
            <div className={styles.nameAndDescription}>
                <Text className="mb-1 ml-4">
                    <span className="mr-2">
                        <StartedStoppedIndicator routine={routine} />
                    </span>
                    <Tooltip content={routineTypeDisplayableName} placement="top">
                        <Icon
                            aria-label={routineTypeDisplayableName}
                            svgPath={routineTypeToIcon[routine.type] ?? mdiHelp}
                        />
                    </Tooltip>
                    <span className="ml-2">
                        <strong>{routine.name}</strong>
                    </span>
                    <span className="ml-2 text-muted">{routine.description}</span>
                </Text>
                <Text className="mb-0 ml-4 text-muted">
                    {routine.intervalMs ? (
                        <>
                            {routine.type === 'DB_BACKED' ? 'Checks queue ' : 'Runs '}every{' '}
                            <strong>{formatDurationLong(routine.intervalMs)}</strong>.{' '}
                        </>
                    ) : null}
                    {routine.recentRuns.length > 0 ? (
                        <Tooltip content={recentRunsTooltipContent}>
                            <span>
                                <strong>
                                    <span className={recentRunsWithErrors.length ? 'text-danger' : 'text-success'}>{`${
                                        recentRunsWithErrors.length
                                    } ${pluralize('error', recentRunsWithErrors.length)}`}</span>
                                </strong>
                                <span className={styles.linkColor}>*</span> in the last{' '}
                                {`${routine.recentRuns.length} ${pluralize('run', routine.recentRuns.length)}`}.{' '}
                            </span>
                        </Tooltip>
                    ) : null}
                    {routine.stats.runCount ? (
                        <>
                            <span className={routine.stats.errorCount ? 'text-danger' : 'text-success'}>
                                <strong>
                                    {routine.stats.errorCount} {pluralize('error', routine.stats.errorCount)}
                                </strong>
                            </span>{' '}
                            in <strong>{routine.stats.runCount}</strong> {pluralize('run', routine.stats.runCount)}
                            {routine.stats.since ? (
                                <>
                                    {' '}
                                    in the last{' '}
                                    <Timestamp date={new Date(routine.stats.since)} noAbout={true} noAgo={true} />.
                                </>
                            ) : null}
                        </>
                    ) : null}
                </Text>
            </div>
            <div className="text-center">
                {routine.stats.runCount ? (
                    <Tooltip content="Fastest / avg / slowest run in milliseconds">
                        <div>
                            <span className={getRunDurationTextClass(routine.stats.minDurationMs, routine.intervalMs)}>
                                {routine.stats.minDurationMs}
                            </span>{' '}
                            /{' '}
                            <span className={getRunDurationTextClass(routine.stats.avgDurationMs, routine.intervalMs)}>
                                {routine.stats.avgDurationMs}
                            </span>{' '}
                            /{' '}
                            <span className={getRunDurationTextClass(routine.stats.maxDurationMs, routine.intervalMs)}>
                                {routine.stats.maxDurationMs}
                            </span>
                        </div>
                    </Tooltip>
                ) : (
                    <span className="text-muted">No stats yet.</span>
                )}
            </div>
        </div>
    )
}

const StartedStoppedIndicator: React.FunctionComponent<{ routine: BackgroundRoutine }> = ({ routine }) => {
    // The last time this job was started
    const latestStartDateString = routine.instances.reduce(
        (mostRecent, instance) =>
            instance.lastStartedAt && (!mostRecent || instance.lastStartedAt > mostRecent)
                ? instance.lastStartedAt
                : mostRecent,
        ''
    )

    // The earliest time this job was stopped
    const earliestStopDateString = routine.instances.reduce(
        (earliest, instance) =>
            instance.lastStoppedAt && (!earliest || instance.lastStoppedAt < earliest)
                ? instance.lastStoppedAt
                : earliest,
        ''
    )

    // The date of the most recent run
    const mostRecentRunDate = routine.recentRuns.length ? new Date(routine.recentRuns[0].at) : null

    // See if this routine is stopped or not seen recently
    const isStopped =
        latestStartDateString && earliestStopDateString ? earliestStopDateString >= latestStartDateString : false
    const isUnseenInAWhile = !!(
        routine.intervalMs &&
        routine.type !== BackgroundRoutineType.DB_BACKED &&
        (!mostRecentRunDate ||
            mostRecentRunDate.getTime() +
                routine.intervalMs +
                routine.stats.maxDurationMs +
                BACKGROUND_JOBS_PAGE_POLL_INTERVAL_MS <=
                Date.now())
    )

    const tooltip = isStopped
        ? `This routine is currently stopped.
Started at ${format(new Date(latestStartDateString), 'yyyy-MM-dd HH:mm:ss')},
Stopped at: ${format(new Date(earliestStopDateString), 'yyyy-MM-dd HH:mm:ss')}`
        : isUnseenInAWhile
        ? mostRecentRunDate
            ? `This routine has not been seen in a while. It should've run at ${format(
                  new Date(mostRecentRunDate.getTime() + (routine.intervalMs || 0)),
                  'yyyy-MM-dd HH:mm:ss'
              )}.`
            : 'This routine was started but it has never been seen running.'
        : `This routine is currently started.${
              mostRecentRunDate ? `\nLast seen running at ${format(mostRecentRunDate, 'yyyy-MM-dd HH:mm:ss')}.` : ''
          }`

    return isStopped || isUnseenInAWhile ? (
        <Tooltip content={tooltip}>
            <Icon aria-label="stopped or unseen" svgPath={mdiClose} className="text-danger" />
        </Tooltip>
    ) : (
        <Tooltip content={tooltip}>
            <Icon aria-label="started" svgPath={mdiCheck} className="text-success" />
        </Tooltip>
    )
}

function isRoutineProblematic(routine: BackgroundRoutine): boolean {
    return (
        routine.stats.errorCount > 0 ||
        routine.recentRuns.some(
            run => run.errorMessage || categorizeRunDuration(run.durationMs, routine.intervalMs) !== 'short'
        ) ||
        categorizeRunDuration(routine.stats.minDurationMs, routine.intervalMs) !== 'short' ||
        categorizeRunDuration(routine.stats.avgDurationMs, routine.intervalMs) !== 'short' ||
        categorizeRunDuration(routine.stats.maxDurationMs, routine.intervalMs) !== 'short'
    )
}

// Contains some magic numbers
function categorizeRunDuration(durationMs: number, routineIntervalMs: number | null): RunLengthCategory {
    // Recognize dangerously long runs
    const dangerouslyLongRunRelativeCutoff = 0.7
    if (routineIntervalMs && durationMs > routineIntervalMs * dangerouslyLongRunRelativeCutoff) {
        return 'dangerous'
    }

    // Recognize long runs
    const longRunCutoffMs = 5000
    if (durationMs > longRunCutoffMs) {
        return 'long'
    }

    // Shorter runs of non-periodic routines are always “short”
    if (!routineIntervalMs) {
        return 'short'
    }

    // If the run is more than 10% longer than the interval, it's long. (the cutoff is 50% for very short intervals)
    const veryShortIntervalCutoffMs = 1000
    const relativeLongRunCutoffMs = routineIntervalMs * (routineIntervalMs <= veryShortIntervalCutoffMs ? 0.5 : 0.1)
    return durationMs > relativeLongRunCutoffMs ? 'long' : 'short'
}

function getRunDurationTextClass(durationMs: number, routineIntervalMs: number | null): string {
    const category = categorizeRunDuration(durationMs, routineIntervalMs)
    switch (category) {
        case 'dangerous': {
            return 'text-danger'
        }
        case 'long': {
            return 'text-warning'
        }
        default: {
            return 'text-success'
        }
    }
}
