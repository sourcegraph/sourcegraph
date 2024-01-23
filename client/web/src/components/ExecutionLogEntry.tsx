import React from 'react'

import { mdiAlertCircle, mdiCheckCircle } from '@mdi/js'
import classNames from 'classnames'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { Card, CardBody, Icon, LoadingSpinner } from '@sourcegraph/wildcard'

import { formatDurationLong } from '../util/time'

import { Collapsible } from './Collapsible'
import { LogOutput } from './LogOutput'

interface ExecutionLogEntryProps extends React.PropsWithChildren<{}> {
    logEntry: {
        key: string
        command: string[]
        startTime: string
        exitCode: number | null
        out: string
        durationMilliseconds: number | null
    }
    className?: string
    now?: () => Date
}

export const ExecutionLogEntry: React.FunctionComponent<React.PropsWithChildren<ExecutionLogEntryProps>> = ({
    logEntry,
    children,
    className,
    now,
}) => (
    <Card className={classNames('mb-3', className)}>
        <CardBody>
            {logEntry.command.length > 0 ? (
                <LogOutput text={logEntry.command.join(' ')} className="mb-3" logDescription="Executed command:" />
            ) : (
                <div className="mb-3">
                    <span className="text-muted">Internal step {logEntry.key}.</span>
                </div>
            )}

            <div>
                {logEntry.exitCode === null && <LoadingSpinner className="mr-1" />}
                {logEntry.exitCode !== null && (
                    <>
                        {logEntry.exitCode === 0 ? (
                            <Icon
                                className="text-success mr-1"
                                svgPath={mdiCheckCircle}
                                inline={false}
                                aria-label="Success"
                            />
                        ) : (
                            <Icon
                                className="text-danger mr-1"
                                svgPath={mdiAlertCircle}
                                inline={false}
                                aria-label="Failed"
                            />
                        )}
                    </>
                )}
                <span className="text-muted">Started</span>{' '}
                <Timestamp date={logEntry.startTime} now={now} noAbout={true} />
                {logEntry.exitCode !== null && logEntry.durationMilliseconds !== null && (
                    <>
                        <span className="text-muted">, ran for</span>{' '}
                        {formatDurationLong(logEntry.durationMilliseconds)}
                    </>
                )}
            </div>
            {children}
        </CardBody>

        <div className="p-2">
            {logEntry.out ? (
                <Collapsible title="Log output" titleAtStart={true} buttonClassName="p-2">
                    <LogOutput text={logEntry.out} logDescription="Log output:" />
                </Collapsible>
            ) : (
                <div className="p-2">
                    <span className="text-muted">No log output available.</span>
                </div>
            )}
        </div>
    </Card>
)
