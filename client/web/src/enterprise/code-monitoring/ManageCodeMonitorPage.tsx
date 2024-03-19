import React, { useEffect } from 'react'

import { VisuallyHidden } from '@reach/visually-hidden'
import { useParams } from 'react-router-dom'
import type { Observable } from 'rxjs'
import { startWith, catchError, tap } from 'rxjs/operators'

import { asError, isErrorLike } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { PageHeader, Link, LoadingSpinner, useObservable } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { withAuthenticatedUser } from '../../auth/withAuthenticatedUser'
import { CodeMonitoringLogo } from '../../code-monitoring/CodeMonitoringLogo'
import { PageTitle } from '../../components/PageTitle'
import type { CodeMonitorFields } from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'

import { convertActionsForUpdate } from './action-converters'
import {
    fetchCodeMonitor as _fetchCodeMonitor,
    updateCodeMonitor as _updateCodeMonitor,
    deleteCodeMonitor as _deleteCodeMonitor,
} from './backend'
import { CodeMonitorForm } from './components/CodeMonitorForm'

interface ManageCodeMonitorPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser

    fetchCodeMonitor?: typeof _fetchCodeMonitor
    updateCodeMonitor?: typeof _updateCodeMonitor
    deleteCodeMonitor?: typeof _deleteCodeMonitor

    isSourcegraphDotCom: boolean
}

const AuthenticatedManageCodeMonitorPage: React.FunctionComponent<
    React.PropsWithChildren<ManageCodeMonitorPageProps>
> = ({
    authenticatedUser,
    fetchCodeMonitor = _fetchCodeMonitor,
    updateCodeMonitor = _updateCodeMonitor,
    deleteCodeMonitor = _deleteCodeMonitor,
    isSourcegraphDotCom,
    telemetryRecorder,
}) => {
    const LOADING = 'loading' as const

    useEffect(() => {
        eventLogger.logPageView('ManageCodeMonitorPage')
        telemetryRecorder.recordEvent('codeMonitor.manage', 'view')
    }, [telemetryRecorder])

    const { id } = useParams()

    const [codeMonitorState, setCodeMonitorState] = React.useState<CodeMonitorFields>({
        id: '',
        description: '',
        enabled: true,
        trigger: { id: '', query: '' },
        actions: { nodes: [] },
        owner: {
            id: '',
            namespaceName: '',
            url: '',
        },
    })

    const codeMonitorOrError = useObservable(
        React.useMemo(
            () =>
                fetchCodeMonitor(id!).pipe(
                    tap(monitor => {
                        if (monitor.node !== null && monitor.node.__typename === 'Monitor') {
                            setCodeMonitorState(monitor.node)
                        }
                    }),
                    startWith(LOADING),
                    catchError(error => [asError(error)])
                ),
            [id, fetchCodeMonitor]
        )
    )

    const updateMonitorRequest = React.useCallback(
        (codeMonitor: CodeMonitorFields): Observable<Partial<CodeMonitorFields>> => {
            eventLogger.log('ManageCodeMonitorFormSubmitted')
            telemetryRecorder.recordEvent('codeMonitor.manage.update', 'submit')
            return updateCodeMonitor(
                {
                    id: id!,
                    update: {
                        namespace: codeMonitor.owner.id,
                        description: codeMonitor.description,
                        enabled: codeMonitor.enabled,
                    },
                },
                { id: codeMonitor.trigger.id, update: { query: codeMonitor.trigger.query } },
                convertActionsForUpdate(codeMonitor.actions.nodes, authenticatedUser.id)
            )
        },
        [authenticatedUser.id, id, updateCodeMonitor, telemetryRecorder]
    )

    const deleteMonitorRequest = React.useCallback(
        (id: string): Observable<void> => {
            eventLogger.log('ManageCodeMonitorDeleteSubmitted')
            telemetryRecorder.recordEvent('codeMonitor.manage.delete', 'submit')
            return deleteCodeMonitor(id)
        },
        [deleteCodeMonitor, telemetryRecorder]
    )

    return (
        <div className="container col-sm-8">
            <PageTitle title="Manage code monitor" />
            <PageHeader
                description={
                    <>
                        Code monitors watch your code for specific triggers and run actions in response.{' '}
                        <Link to="/help/code_monitoring" target="_blank" rel="noopener">
                            <VisuallyHidden>Learn more about code monitors</VisuallyHidden>
                            <span aria-hidden={true}>Learn more</span>
                        </Link>
                    </>
                }
            >
                <PageHeader.Heading as="h2" styleAs="h1">
                    <PageHeader.Breadcrumb
                        icon={CodeMonitoringLogo}
                        to="/code-monitoring"
                        aria-label="Code monitoring"
                    />
                    <PageHeader.Breadcrumb>Manage code monitor</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            {codeMonitorOrError === 'loading' && <LoadingSpinner />}
            {codeMonitorOrError && !isErrorLike(codeMonitorOrError) && codeMonitorOrError !== 'loading' && (
                <>
                    <CodeMonitorForm
                        authenticatedUser={authenticatedUser}
                        deleteCodeMonitor={deleteMonitorRequest}
                        onSubmit={updateMonitorRequest}
                        codeMonitor={codeMonitorState}
                        submitButtonLabel="Save"
                        showDeleteButton={true}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                    />
                </>
            )}
        </div>
    )
}

export const ManageCodeMonitorPage = withAuthenticatedUser(AuthenticatedManageCodeMonitorPage)
