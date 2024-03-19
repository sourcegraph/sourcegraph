import React, { useCallback, useEffect, useRef } from 'react'

import { mdiClose } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import { useNavigate, useParams } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Card, CardBody, H3, H1, Icon, Text, Code, ErrorAlert } from '@sourcegraph/wildcard'

import { type BatchSpecExecutionFields, BatchSpecSource } from '../../../../../graphql-operations'
import type { queryChangesetSpecFileDiffs as _queryChangesetSpecFileDiffs } from '../../../preview/list/backend'
import { type BatchSpecContextState, useBatchSpecContext } from '../../BatchSpecContext'
import type {
    queryBatchSpecWorkspaceStepFileDiffs as _queryBatchSpecWorkspaceStepFileDiffs,
    queryWorkspacesList as _queryWorkspacesList,
} from '../backend'

import { WorkspaceDetails } from './WorkspaceDetails'
import { WorkspacesPanel } from './WorkspacesPanel'

import styles from './ExecutionWorkspaces.module.scss'

interface ExecutionWorkspacesProps extends TelemetryV2Props {
    /** For testing purposes only */
    queryBatchSpecWorkspaceStepFileDiffs?: typeof _queryBatchSpecWorkspaceStepFileDiffs
    queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
    queryWorkspacesList?: typeof _queryWorkspacesList
}

export const ExecutionWorkspaces: React.FunctionComponent<
    React.PropsWithChildren<ExecutionWorkspacesProps>
> = props => {
    const { batchSpec, errors } = useBatchSpecContext<BatchSpecExecutionFields>()

    if (batchSpec.source === BatchSpecSource.LOCAL) {
        return (
            <>
                <H1 className="text-center text-muted mt-5">
                    <Icon role="img" aria-hidden={true} svgPath={mdiClose} />
                    <VisuallyHidden>No Execution</VisuallyHidden>
                </H1>
                <Text alignment="center">
                    This batch spec was executed locally with <Code>src-cli</Code>.
                </Text>
            </>
        )
    }

    return <MemoizedExecutionWorkspaces {...props} batchSpec={batchSpec} errors={errors} />
}

type MemoizedExecutionWorkspacesProps = ExecutionWorkspacesProps & Pick<BatchSpecContextState, 'batchSpec' | 'errors'>

const MemoizedExecutionWorkspaces: React.FunctionComponent<React.PropsWithChildren<MemoizedExecutionWorkspacesProps>> =
    React.memo(function MemoizedExecutionWorkspaces({
        batchSpec,
        errors,
        queryBatchSpecWorkspaceStepFileDiffs,
        queryChangesetSpecFileDiffs,
        queryWorkspacesList,
        telemetryRecorder,
    }) {
        const navigate = useNavigate()
        const isLightTheme = useIsLightTheme()
        const { workspaceID: selectedWorkspaceID } = useParams()

        const deselectWorkspace = useCallback(() => {
            navigate(`${batchSpec.executionURL}/execution`)
        }, [batchSpec.executionURL, navigate])

        const videoRef = useRef<HTMLVideoElement | null>(null)
        // Pause the execution animation loop when the batch spec stops executing.
        useEffect(() => {
            if (!batchSpec.isExecuting) {
                videoRef.current?.pause()
            }
        }, [batchSpec.isExecuting])

        return (
            <div className={styles.container}>
                {errors.execute && <ErrorAlert error={errors.execute} className={styles.errors} />}
                <div className={styles.inner}>
                    <WorkspacesPanel
                        batchSpecID={batchSpec.id}
                        selectedNode={selectedWorkspaceID}
                        executionURL={batchSpec.executionURL}
                        queryWorkspacesList={queryWorkspacesList}
                    />
                    <Card className="w-100 overflow-auto flex-grow-1">
                        {/* This is necessary to prevent the margin collapse on `Card` */}
                        <div className="w-100">
                            <CardBody>
                                {selectedWorkspaceID ? (
                                    <WorkspaceDetails
                                        id={selectedWorkspaceID}
                                        deselectWorkspace={deselectWorkspace}
                                        queryBatchSpecWorkspaceStepFileDiffs={queryBatchSpecWorkspaceStepFileDiffs}
                                        queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                                        telemetryRecorder={telemetryRecorder}
                                    />
                                ) : (
                                    <>
                                        <div className={styles.videoContainer}>
                                            <video
                                                className="w-100 percy-hide"
                                                autoPlay={true}
                                                muted={true}
                                                loop={true}
                                                playsInline={true}
                                                controls={false}
                                                ref={videoRef}
                                            >
                                                <source
                                                    type="video/webm"
                                                    src={`https://storage.googleapis.com/sourcegraph-assets/batch-changes/execution-animation${
                                                        isLightTheme ? '' : '-dark'
                                                    }.webm`}
                                                />
                                                <source
                                                    type="video/mp4"
                                                    src={`https://storage.googleapis.com/sourcegraph-assets/batch-changes/execution-animation${
                                                        isLightTheme ? '' : '-dark'
                                                    }.mp4`}
                                                />
                                            </video>
                                        </div>
                                        <H3 className="text-center my-3">Select a workspace to view details.</H3>
                                    </>
                                )}
                            </CardBody>
                        </div>
                    </Card>
                </div>
            </div>
        )
    })
