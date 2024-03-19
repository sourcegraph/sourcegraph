import { type FunctionComponent, useCallback, useState } from 'react'

import type { Subject } from 'rxjs'

import { Alert, Button, ErrorAlert, Input, Label, Link } from '@sourcegraph/wildcard'

import { useEnqueueIndexJob as defaultUseEnqueueIndexJob } from '../hooks/useEnqueueIndexJob'

export interface EnqueueFormProps {
    repoId: string
    querySubject: Subject<string>
    useEnqueueIndexJob?: typeof defaultUseEnqueueIndexJob
}

enum State {
    Idle,
    Queueing,
    Queued,
}

export const EnqueueForm: FunctionComponent<EnqueueFormProps> = ({
    repoId,
    querySubject,
    useEnqueueIndexJob = defaultUseEnqueueIndexJob,
}) => {
    const [revlike, setRevlike] = useState('HEAD')
    const [state, setState] = useState(() => State.Idle)
    const [queueResult, setQueueResult] = useState<number>()
    const [enqueueError, setEnqueueError] = useState<Error>()
    const { handleEnqueueIndexJob } = useEnqueueIndexJob()

    const enqueue = useCallback(async () => {
        setState(State.Queueing)
        setEnqueueError(undefined)
        setQueueResult(undefined)

        try {
            const indexes = await handleEnqueueIndexJob({
                variables: { id: repoId, rev: revlike },
            }).then(({ data }) => data)

            const queueResultLength = indexes?.queueAutoIndexJobsForRepo.length || 0
            setQueueResult(queueResultLength)
            if (queueResultLength > 0 && indexes?.queueAutoIndexJobsForRepo[0].inputCommit !== undefined) {
                querySubject.next(indexes.queueAutoIndexJobsForRepo[0].inputCommit)
            }
        } catch (error) {
            setEnqueueError(error)
            setQueueResult(undefined)
        } finally {
            setState(State.Queued)
        }
    }, [repoId, revlike, querySubject, handleEnqueueIndexJob])

    return (
        <>
            {enqueueError && <ErrorAlert prefix="Error enqueueing index job" error={enqueueError} />}
            <div className="mb-3">
                Provide a{' '}
                <Link
                    to="https://git-scm.com/docs/git-rev-parse.html#_specifying_revisions"
                    rel="noopener noreferrer"
                    target="_blank"
                >
                    Git revspec
                </Link>{' '}
                to enqueue a new auto-indexing job.
            </div>
            <div className="form-inline">
                <Label htmlFor="revlike">Git revspec</Label>

                <Input
                    id="revlike"
                    className="ml-2"
                    value={revlike}
                    onChange={event => setRevlike(event.target.value)}
                />

                <Button
                    type="button"
                    title="Enqueue thing"
                    disabled={state === State.Queueing}
                    className="ml-2"
                    variant="primary"
                    onClick={enqueue}
                >
                    Enqueue
                </Button>
            </div>

            {state === State.Queued &&
                queueResult !== undefined &&
                (queueResult > 0 ? (
                    <Alert className="mt-3 mb-0" variant="success">
                        {queueResult} auto-indexing jobs enqueued.
                    </Alert>
                ) : (
                    <Alert className="mt-3 mb-0" variant="info">
                        Failed to enqueue any auto-indexing jobs.
                        <br />
                        Check if the auto-index configuration is up-to-date.
                    </Alert>
                ))}
        </>
    )
}
