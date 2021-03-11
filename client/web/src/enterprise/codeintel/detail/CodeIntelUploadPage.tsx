import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import DeleteIcon from 'mdi-react/DeleteIcon'
import InformationOutlineIcon from 'mdi-react/InformationOutlineIcon'
import React, { FunctionComponent, useCallback, useEffect, useMemo, useState } from 'react'
import { Redirect, RouteComponentProps } from 'react-router'
import { SchedulerLike, timer } from 'rxjs'
import { catchError, concatMap, delay, repeatWhen, takeWhile } from 'rxjs/operators'
import { LSIFUploadState } from '../../../../../shared/src/graphql-operations'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { ErrorAlert } from '../../../components/alerts'
import { PageHeader } from '../../../components/PageHeader'
import { PageTitle } from '../../../components/PageTitle'
import { LsifUploadFields } from '../../../graphql-operations'
import { CodeIntelStateBanner } from '../shared/CodeIntelStateBanner'
import { deleteLsifUpload, fetchLsifUpload as defaultFetchUpload } from './backend'
import { CodeIntelUploadMeta } from './CodeIntelUploadMeta'
import { CodeIntelUploadTimeline } from './CodeIntelUploadTimeline'

export interface CodeIntelUploadPageProps extends RouteComponentProps<{ id: string }>, TelemetryProps {
    fetchLsifUpload?: typeof defaultFetchUpload
    now?: () => Date
    /** Scheduler for the refresh timer */
    scheduler?: SchedulerLike
}

const REFRESH_INTERVAL_MS = 5000

const classNamesByState = new Map([
    [LSIFUploadState.COMPLETED, 'alert-success'],
    [LSIFUploadState.ERRORED, 'alert-danger'],
])

export const CodeIntelUploadPage: FunctionComponent<CodeIntelUploadPageProps> = ({
    scheduler,
    match: {
        params: { id },
    },
    fetchLsifUpload = defaultFetchUpload,
    telemetryService,
    now,
}) => {
    useEffect(() => telemetryService.logViewEvent('CodeIntelUpload'), [telemetryService])

    const [deletionOrError, setDeletionOrError] = useState<'loading' | 'deleted' | ErrorLike>()

    const uploadOrError = useObservable(
        useMemo(
            () =>
                timer(0, REFRESH_INTERVAL_MS, scheduler).pipe(
                    concatMap(() =>
                        fetchLsifUpload({ id }).pipe(
                            catchError((error): [ErrorLike] => [asError(error)]),
                            repeatWhen(observable => observable.pipe(delay(REFRESH_INTERVAL_MS)))
                        )
                    ),
                    takeWhile(shouldReload, true)
                ),
            [id, scheduler, fetchLsifUpload]
        )
    )

    const deleteUpload = useCallback(async (): Promise<void> => {
        if (!uploadOrError || isErrorLike(uploadOrError)) {
            return
        }

        let description = `${uploadOrError.inputCommit.slice(0, 7)}`
        if (uploadOrError.inputRoot) {
            description += ` rooted at ${uploadOrError.inputRoot}`
        }

        if (!window.confirm(`Delete upload for commit ${description}?`)) {
            return
        }

        setDeletionOrError('loading')

        try {
            await deleteLsifUpload({ id }).toPromise()
            setDeletionOrError('deleted')
        } catch (error) {
            setDeletionOrError(error)
        }
    }, [id, uploadOrError])

    return deletionOrError === 'deleted' ? (
        <Redirect to="." />
    ) : isErrorLike(deletionOrError) ? (
        <ErrorAlert prefix="Error deleting LSIF upload" error={deletionOrError} />
    ) : (
        <div className="site-admin-lsif-upload-page w-100 web-content">
            <PageTitle title="Code intelligence - uploads" />
            {isErrorLike(uploadOrError) ? (
                <ErrorAlert prefix="Error loading LSIF upload" error={uploadOrError} />
            ) : !uploadOrError ? (
                <LoadingSpinner className="icon-inline" />
            ) : (
                <>
                    <PageHeader
                        path={[
                            {
                                text: (
                                    <>
                                        <span className="text-muted">Upload for commit</span>
                                        <span className="ml-2">
                                            {uploadOrError.projectRoot
                                                ? uploadOrError.projectRoot.commit.abbreviatedOID
                                                : uploadOrError.inputCommit.slice(0, 7)}
                                        </span>
                                        <span className="ml-2 text-muted">indexed by</span>
                                        <span className="ml-2">{uploadOrError.inputIndexer}</span>
                                        <span className="ml-2 text-muted">rooted at</span>
                                        <span className="ml-2">
                                            {(uploadOrError.projectRoot
                                                ? uploadOrError.projectRoot.path
                                                : uploadOrError.inputRoot) || '/'}
                                        </span>
                                    </>
                                ),
                            },
                        ]}
                        actions={
                            <CodeIntelDeleteUpload deleteUpload={deleteUpload} deletionOrError={deletionOrError} />
                        }
                        className="mb-3"
                    />
                    <CodeIntelStateBanner
                        state={uploadOrError.state}
                        placeInQueue={uploadOrError.placeInQueue}
                        failure={uploadOrError.failure}
                        typeName="upload"
                        pluralTypeName="uploads"
                        className={classNamesByState.get(uploadOrError.state)}
                    />
                    {uploadOrError.isLatestForRepo && (
                        <div className="mb-3">
                            <InformationOutlineIcon className="icon-inline" /> This upload can answer queries for the
                            tip of the default branch and are targets of cross-repository find reference operations.
                        </div>
                    )}
                    <div className="card mb-3">
                        <div className="card-body">
                            <CodeIntelUploadMeta node={uploadOrError} now={now} />
                        </div>
                    </div>
                    <CodeIntelUploadTimeline now={now} upload={uploadOrError} className="mb-3" />
                </>
            )}
        </div>
    )
}

const terminalStates = new Set([LSIFUploadState.COMPLETED, LSIFUploadState.ERRORED])

function shouldReload(upload: LsifUploadFields | ErrorLike | null | undefined): boolean {
    return !isErrorLike(upload) && !(upload && terminalStates.has(upload.state))
}

interface CodeIntelDeleteUploadProps {
    deleteUpload: () => Promise<void>
    deletionOrError?: 'loading' | 'deleted' | ErrorLike
}

const CodeIntelDeleteUpload: FunctionComponent<CodeIntelDeleteUploadProps> = ({ deleteUpload, deletionOrError }) => (
    <button
        type="button"
        className="btn btn-outline-danger"
        onClick={deleteUpload}
        disabled={deletionOrError === 'loading'}
        aria-describedby="upload-delete-button-help"
        data-tooltip="Deleting this upload makes it immediately unavailable to answer code intelligence queries."
    >
        <DeleteIcon className="icon-inline" /> Delete upload
    </button>
)
