import React, { useMemo, type FC } from 'react'

import { mdiProgressClock } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import { Navigate, Route, Routes, useParams } from 'react-router-dom'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { useQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Badge, ErrorMessage, Icon, LinkOrSpan, LoadingSpinner } from '@sourcegraph/wildcard'

import { withAuthenticatedUser } from '../../../../auth/withAuthenticatedUser'
import { HeroPage } from '../../../../components/HeroPage'
import { Duration } from '../../../../components/time/Duration'
import {
    BatchSpecSource,
    type BatchSpecExecutionByIDResult,
    type BatchSpecExecutionByIDVariables,
    type BatchSpecExecutionFields,
    type GetBatchChangeToEditResult,
    type GetBatchChangeToEditVariables,
} from '../../../../graphql-operations'
import type { NamespaceProps } from '../../../../namespaces'
import { GET_BATCH_CHANGE_TO_EDIT } from '../../create/backend'
import { ConfigurationForm } from '../../create/ConfigurationForm'
import { NewBatchChangePreviewPage } from '../../preview/BatchChangePreviewPage'
import { BatchSpecContextProvider, useBatchSpecContext, type BatchSpecContextState } from '../BatchSpecContext'
import { ActionButtons } from '../header/ActionButtons'
import { BatchChangeHeader } from '../header/BatchChangeHeader'
import { TabBar, type TabsConfig } from '../TabBar'

import { ActionsMenu } from './ActionsMenu'
import { FETCH_BATCH_SPEC_EXECUTION, type queryWorkspacesList as _queryWorkspacesList } from './backend'
import { BatchSpecStateBadge } from './BatchSpecStateBadge'
import { ExecutionStat, ExecutionStatsBar } from './ExecutionStatsBar'
import { ReadOnlyBatchSpecForm } from './ReadOnlyBatchSpecForm'
import { ExecutionWorkspaces } from './workspaces/ExecutionWorkspaces'

import layoutStyles from '../Layout.module.scss'
import styles from './ExecuteBatchSpecPage.module.scss'

export interface AuthenticatedExecuteBatchSpecPageProps extends TelemetryProps, TelemetryV2Props, NamespaceProps {
    authenticatedUser: AuthenticatedUser
    /** FOR TESTING ONLY */
    testContextState?: Partial<BatchSpecContextState<BatchSpecExecutionFields>>
    queryWorkspacesList?: typeof _queryWorkspacesList
}

export const AuthenticatedExecuteBatchSpecPage: FC<AuthenticatedExecuteBatchSpecPageProps> = ({
    testContextState,
    ...props
}) => {
    const { batchChangeName, batchSpecID } = useParams()
    const { id } = props.namespace

    const batchChange = useMemo(() => ({ name: batchChangeName!, namespace: id }), [batchChangeName, id])
    const {
        data: batchChangeData,
        error: batchChangeError,
        loading: batchChangeLoading,
    } = useQuery<GetBatchChangeToEditResult, GetBatchChangeToEditVariables>(GET_BATCH_CHANGE_TO_EDIT, {
        variables: batchChange,
        // Cache this data but always re-request it in the background when we revisit
        // this page to pick up newer changes.
        fetchPolicy: 'cache-and-network',
    })

    const { data, error, loading } = useQuery<BatchSpecExecutionByIDResult, BatchSpecExecutionByIDVariables>(
        FETCH_BATCH_SPEC_EXECUTION,
        {
            variables: { id: batchSpecID! },
            fetchPolicy: 'cache-and-network',
            pollInterval: 2500,
        }
    )

    if ((loading || batchChangeLoading) && (!data || !batchChangeData)) {
        return (
            <div className="w-100 text-center">
                <Icon aria-label="Loading" className="m-2" as={LoadingSpinner} />
            </div>
        )
    }

    if (data?.node === null) {
        return <HeroPage icon={MapSearchIcon} title="404: Not Found" />
    }

    if (!data?.node || data.node.__typename !== 'BatchSpec' || !batchChangeData?.batchChange) {
        if (error || batchChangeError) {
            return <HeroPage icon={AlertCircleIcon} title={String(error || batchChangeError)} />
        }
        return <HeroPage icon={AlertCircleIcon} title="Batch change not found" />
    }

    return (
        <BatchSpecContextProvider
            batchChange={batchChangeData.batchChange}
            batchSpec={data.node}
            testState={testContextState}
        >
            <ExecuteBatchSpecPageContent {...props} />
        </BatchSpecContextProvider>
    )
}

interface ExecuteBatchSpecPageContentProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser
    queryWorkspacesList?: typeof _queryWorkspacesList
}

const ExecuteBatchSpecPageContent: React.FunctionComponent<
    React.PropsWithChildren<ExecuteBatchSpecPageContentProps>
> = props => {
    const { batchChange, batchSpec, errors } = useBatchSpecContext<BatchSpecExecutionFields>()

    return (
        <MemoizedExecuteBatchSpecContent {...props} batchChange={batchChange} batchSpec={batchSpec} errors={errors} />
    )
}

type MemoizedExecuteBatchSpecContentProps = ExecuteBatchSpecPageContentProps &
    Pick<BatchSpecContextState, 'batchChange' | 'batchSpec' | 'errors'>

const MemoizedExecuteBatchSpecContent: FC<MemoizedExecuteBatchSpecContentProps> = React.memo(
    function MemoizedExecuteBatchSpecContent({
        telemetryService,
        telemetryRecorder,
        authenticatedUser,
        batchChange,
        batchSpec,
        errors,
        queryWorkspacesList,
    }) {
        const { executionURL, workspaceResolution, applyURL } = batchSpec

        const tabsConfig = useMemo<TabsConfig[]>(
            () => [
                { key: 'configuration', isEnabled: true, handler: { type: 'link' } },
                { key: 'spec', isEnabled: true, handler: { type: 'link' } },
                { key: 'execution', isEnabled: true, handler: { type: 'link' } },
                { key: 'preview', isEnabled: applyURL !== null, handler: { type: 'link' } },
            ],
            [applyURL]
        )

        return (
            <div className={layoutStyles.pageContainer}>
                <div className={layoutStyles.headerContainer}>
                    <BatchChangeHeader
                        className={styles.header}
                        namespace={{
                            to: `${batchChange.namespace.url}/batch-changes`,
                            text: batchChange.namespace.namespaceName,
                        }}
                        title={{ to: batchChange.url, text: batchChange.name }}
                        description={
                            <>
                                Created <Timestamp date={batchSpec.createdAt} /> by{' '}
                                <LinkOrSpan to={batchSpec.creator?.url}>
                                    {batchSpec.creator?.displayName || batchSpec.creator?.username || 'a deleted user'}
                                </LinkOrSpan>
                            </>
                        }
                    />
                    <div className={styles.statsBar}>
                        <div className={styles.stateBadge}>
                            {batchSpec.source === BatchSpecSource.REMOTE ? (
                                <BatchSpecStateBadge state={batchSpec.state} />
                            ) : (
                                <>
                                    <VisuallyHidden>This batch spec was executed with src-cli.</VisuallyHidden>
                                    <Badge
                                        variant="secondary"
                                        tooltip="This batch spec was executed with src-cli."
                                        aria-hidden={true}
                                    >
                                        LOCAL
                                    </Badge>
                                </>
                            )}
                        </div>
                        {batchSpec.startedAt && (
                            <ExecutionStat>
                                <Icon aria-hidden={true} className={styles.durationIcon} svgPath={mdiProgressClock} />
                                <Duration
                                    start={batchSpec.startedAt}
                                    end={batchSpec.finishedAt ?? undefined}
                                    labelPrefix={`The batch spec ${
                                        batchSpec.finishedAt ? 'finished executing in' : 'has been executing for'
                                    }`}
                                />
                            </ExecutionStat>
                        )}
                        {workspaceResolution && <ExecutionStatsBar {...workspaceResolution.workspaces.stats} />}
                    </div>

                    <ActionButtons className="flex-shrink-0">
                        <ActionsMenu telemetryRecorder={telemetryRecorder} />
                    </ActionButtons>
                </div>

                {errors.actions && <ErrorMessage error={errors.actions} key={String(errors.actions)} />}

                <Routes>
                    <Route element={<Navigate to="execution" replace={true} />} index={true} />
                    <Route
                        path="configuration"
                        element={
                            <>
                                <TabBar activeTabKey="configuration" tabsConfig={tabsConfig} matchURL={executionURL} />
                                <ConfigurationForm isReadOnly={true} batchChange={batchChange} />
                            </>
                        }
                    />
                    <Route
                        path="spec"
                        element={
                            <>
                                <TabBar activeTabKey="spec" tabsConfig={tabsConfig} matchURL={executionURL} />
                                <ReadOnlyBatchSpecForm telemetryRecorder={telemetryRecorder} />
                            </>
                        }
                    />
                    <Route
                        path="execution/workspaces/:workspaceID"
                        element={
                            <>
                                <TabBar activeTabKey="execution" tabsConfig={tabsConfig} matchURL={executionURL} />
                                <ExecutionWorkspaces
                                    queryWorkspacesList={queryWorkspacesList}
                                    telemetryRecorder={telemetryRecorder}
                                />
                            </>
                        }
                    />
                    <Route
                        path="execution"
                        element={
                            <>
                                <TabBar activeTabKey="execution" tabsConfig={tabsConfig} matchURL={executionURL} />
                                <ExecutionWorkspaces
                                    queryWorkspacesList={queryWorkspacesList}
                                    telemetryRecorder={telemetryRecorder}
                                />
                            </>
                        }
                    />

                    <Route
                        path="preview"
                        element={
                            batchSpec.applyURL ? (
                                <>
                                    <TabBar
                                        activeTabKey="preview"
                                        tabsConfig={tabsConfig}
                                        matchURL={executionURL}
                                        className="mb-4"
                                    />
                                    <NewBatchChangePreviewPage
                                        authenticatedUser={authenticatedUser}
                                        telemetryService={telemetryService}
                                        telemetryRecorder={telemetryRecorder}
                                    />
                                </>
                            ) : (
                                // If the batch spec is not ready to be previewed, redirect to the spec instead.
                                <Navigate to="spec" replace={true} />
                            )
                        }
                    />
                    <Route path="*" element={<HeroPage icon={MapSearchIcon} title="404: Not Found" />} />
                </Routes>
            </div>
        )
    }
)

export const ExecuteBatchSpecPage = withAuthenticatedUser(AuthenticatedExecuteBatchSpecPage)
