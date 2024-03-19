import React, { useCallback, useMemo, useState } from 'react'

import {
    mdiClose,
    mdiTimelineClockOutline,
    mdiSourceBranch,
    mdiEyeOffOutline,
    mdiSync,
    mdiLinkVariantRemove,
    mdiChevronDown,
    mdiChevronUp,
    mdiOpenInNew,
} from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import indicator from 'ordinal/indicator'

import { dataOrThrowErrors } from '@sourcegraph/http-client'
import type { Maybe } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Badge,
    LoadingSpinner,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Button,
    Link,
    CardBody,
    Card,
    Icon,
    Code,
    H1,
    H3,
    H4,
    Text,
    Alert,
    CollapsePanel,
    CollapseHeader,
    Collapse,
    Heading,
    Tooltip,
    ErrorAlert,
} from '@sourcegraph/wildcard'

import { DiffStat } from '../../../../../components/diff/DiffStat'
import { FileDiffNode, type FileDiffNodeProps } from '../../../../../components/diff/FileDiffNode'
import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../../../../components/FilteredConnection'
import { useShowMorePagination } from '../../../../../components/FilteredConnection/hooks/useShowMorePagination'
import { HeroPage } from '../../../../../components/HeroPage'
import { LogOutput } from '../../../../../components/LogOutput'
import { Duration } from '../../../../../components/time/Duration'
import {
    type BatchSpecWorkspaceChangesetSpecFields,
    BatchSpecWorkspaceState,
    type BatchSpecWorkspaceStepFields,
    type HiddenBatchSpecWorkspaceFields,
    type Scalars,
    type VisibleBatchSpecWorkspaceFields,
    type FileDiffFields,
    type BatchSpecWorkspaceStepResult,
    type BatchSpecWorkspaceStepVariables,
} from '../../../../../graphql-operations'
import { eventLogger } from '../../../../../tracking/eventLogger'
import { queryChangesetSpecFileDiffs as _queryChangesetSpecFileDiffs } from '../../../preview/list/backend'
import { ChangesetSpecFileDiffConnection } from '../../../preview/list/ChangesetSpecFileDiffConnection'
import {
    useBatchSpecWorkspace,
    useRetryWorkspaceExecution,
    queryBatchSpecWorkspaceStepFileDiffs as _queryBatchSpecWorkspaceStepFileDiffs,
    BATCH_SPEC_WORKSPACE_STEP,
} from '../backend'
import { DiagnosticsModal } from '../DiagnosticsModal'

import { StepStateIcon } from './StepStateIcon'
import { WorkspaceStateIcon } from './WorkspaceStateIcon'

import styles from './WorkspaceDetails.module.scss'

export interface WorkspaceDetailsProps extends TelemetryV2Props {
    id: Scalars['ID']
    /** Handler to deselect the current workspace, i.e. close the details panel. */
    deselectWorkspace?: () => void
    /** For testing purposes only */
    queryBatchSpecWorkspaceStepFileDiffs?: typeof _queryBatchSpecWorkspaceStepFileDiffs
    queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
}

export const WorkspaceDetails: React.FunctionComponent<React.PropsWithChildren<WorkspaceDetailsProps>> = ({
    id,
    ...props
}) => {
    // Fetch and poll latest workspace information.
    const { loading, error, data } = useBatchSpecWorkspace(id)

    // If we're loading and haven't received any data yet
    if (loading && !data) {
        return <LoadingSpinner />
    }
    // If we received an error before we had received any data
    if (error && !data) {
        return <ErrorAlert error={error} />
    }
    // If there weren't any errors and we just didn't receive any data
    if (!data) {
        return <HeroPage icon={MapSearchIcon} title="404: Not Found" />
    }

    const workspace = data

    if (workspace.__typename === 'HiddenBatchSpecWorkspace') {
        return <HiddenWorkspaceDetails {...props} workspace={workspace} />
    }
    return <VisibleWorkspaceDetails {...props} workspace={workspace} />
}

interface WorkspaceHeaderProps extends Pick<WorkspaceDetailsProps, 'deselectWorkspace' | 'telemetryRecorder'> {
    workspace: HiddenBatchSpecWorkspaceFields | VisibleBatchSpecWorkspaceFields
    toggleShowDiagnostics?: () => void
}

const WorkspaceHeader: React.FunctionComponent<React.PropsWithChildren<WorkspaceHeaderProps>> = ({
    workspace,
    deselectWorkspace,
    toggleShowDiagnostics,
    telemetryRecorder,
}) => (
    <>
        <div className="d-flex align-items-center justify-content-between mb-2">
            <H3 className={styles.workspaceName}>
                <WorkspaceStateIcon
                    cachedResultFound={workspace.cachedResultFound}
                    state={workspace.state}
                    className="flex-shrink-0"
                />{' '}
                {workspace.__typename === 'VisibleBatchSpecWorkspace'
                    ? workspace.repository.name
                    : 'Workspace in hidden repository'}
                {workspace.__typename === 'VisibleBatchSpecWorkspace' && (
                    <Link to={workspace.repository.url} target="_blank" rel="noopener noreferrer">
                        <VisuallyHidden>Go to repository</VisuallyHidden>
                        <Icon aria-hidden={true} svgPath={mdiOpenInNew} />
                    </Link>
                )}
            </H3>
            <Button className="p-0 ml-2" onClick={deselectWorkspace} variant="icon">
                <VisuallyHidden>Deselect Workspace</VisuallyHidden>
                <Icon aria-hidden={true} svgPath={mdiClose} />
            </Button>
        </div>
        <div className="d-flex align-items-center">
            {typeof workspace.placeInQueue === 'number' && (
                <Tooltip content={`This workspace is number ${workspace.placeInGlobalQueue} in the global queue`}>
                    <span className={classNames(styles.workspaceDetail, 'd-flex align-items-center')}>
                        <Icon aria-hidden={true} svgPath={mdiTimelineClockOutline} />
                        <strong className="ml-1 mr-1">
                            <NumberInQueue number={workspace.placeInQueue} />
                        </strong>
                        in queue
                    </span>
                </Tooltip>
            )}
            {workspace.__typename === 'VisibleBatchSpecWorkspace' && workspace.path && (
                <span aria-label="Batch spec executed at path:" className={styles.workspaceDetail}>
                    {workspace.path}
                </span>
            )}
            {workspace.__typename === 'VisibleBatchSpecWorkspace' && (
                <span
                    aria-label="Batch spec executed on branch:"
                    className={classNames(styles.workspaceDetail, 'text-monospace')}
                >
                    <Icon aria-hidden={true} svgPath={mdiSourceBranch} /> {workspace.branch.displayName}
                </span>
            )}
            {workspace.startedAt && (
                <span className={classNames(styles.workspaceDetail, 'd-flex align-items-center')}>
                    Total time:
                    <strong className="pl-1">
                        <Duration
                            start={workspace.startedAt}
                            end={workspace.finishedAt ?? undefined}
                            labelPrefix={`Workspace ${
                                workspace.finishedAt ? 'finished executing in' : 'has been executing for'
                            }`}
                        />
                    </strong>
                </span>
            )}
            {toggleShowDiagnostics &&
                !workspace.cachedResultFound &&
                workspace.state !== BatchSpecWorkspaceState.SKIPPED && (
                    <Button
                        className={styles.workspaceDetail}
                        onClick={() => {
                            toggleShowDiagnostics()
                            eventLogger.log('batch_change_execution:workspace_timeline:clicked')
                            telemetryRecorder.recordEvent('batchChange.execution.workspaceTimeline', 'click')
                        }}
                        variant="link"
                    >
                        Diagnostics
                    </Button>
                )}
        </div>
        <hr className="mb-3" aria-hidden={true} />
    </>
)

interface HiddenWorkspaceDetailsProps extends Pick<WorkspaceDetailsProps, 'deselectWorkspace' | 'telemetryRecorder'> {
    workspace: HiddenBatchSpecWorkspaceFields
}

const HiddenWorkspaceDetails: React.FunctionComponent<React.PropsWithChildren<HiddenWorkspaceDetailsProps>> = ({
    workspace,
    deselectWorkspace,
    telemetryRecorder,
}) => (
    <div role="region" aria-label="workspace details">
        <WorkspaceHeader
            deselectWorkspace={deselectWorkspace}
            workspace={workspace}
            telemetryRecorder={telemetryRecorder}
        />
        <H1 className="text-center text-muted mt-5">
            <Icon aria-hidden={true} svgPath={mdiEyeOffOutline} />
            <VisuallyHidden>Hidden Workspace</VisuallyHidden>
        </H1>
        <Text alignment="center">This workspace is hidden due to permissions.</Text>
        <Text alignment="center">Contact the owner of this batch change for more information.</Text>
    </div>
)

interface VisibleWorkspaceDetailsProps extends Omit<WorkspaceDetailsProps, 'id'> {
    workspace: VisibleBatchSpecWorkspaceFields
}

const VisibleWorkspaceDetails: React.FunctionComponent<React.PropsWithChildren<VisibleWorkspaceDetailsProps>> = ({
    workspace,
    deselectWorkspace,
    queryBatchSpecWorkspaceStepFileDiffs,
    queryChangesetSpecFileDiffs,
    telemetryRecorder,
}) => {
    const [retryWorkspaceExecution, { loading: retryLoading, error: retryError }] = useRetryWorkspaceExecution(
        workspace.id
    )

    const [showDiagnostics, setShowDiagnostics] = useState<boolean>(false)
    const toggleShowDiagnostics = useCallback(() => {
        setShowDiagnostics(true)
    }, [])
    const onDismissDiagnostics = useCallback(() => {
        setShowDiagnostics(false)
    }, [])

    if (workspace.state === BatchSpecWorkspaceState.SKIPPED && workspace.ignored) {
        return (
            <IgnoredWorkspaceDetails
                workspace={workspace}
                deselectWorkspace={deselectWorkspace}
                telemetryRecorder={telemetryRecorder}
            />
        )
    }

    if (workspace.state === BatchSpecWorkspaceState.SKIPPED && workspace.unsupported) {
        return (
            <UnsupportedWorkspaceDetails
                workspace={workspace}
                deselectWorkspace={deselectWorkspace}
                telemetryRecorder={telemetryRecorder}
            />
        )
    }

    return (
        <div role="region" aria-label="workspace details">
            {showDiagnostics && <DiagnosticsModal node={workspace} onCancel={onDismissDiagnostics} />}
            <WorkspaceHeader
                deselectWorkspace={deselectWorkspace}
                toggleShowDiagnostics={toggleShowDiagnostics}
                workspace={workspace}
                telemetryRecorder={telemetryRecorder}
            />
            {workspace.state === BatchSpecWorkspaceState.CANCELED && (
                <Alert variant="warning">Execution of this workspace has been canceled.</Alert>
            )}
            {workspace.state === BatchSpecWorkspaceState.FAILED && workspace.failureMessage && (
                <>
                    <div className="d-flex my-3 w-100">
                        <ErrorAlert error={workspace.failureMessage} className="flex-grow-1 mb-0" />
                        <Button
                            className="ml-2"
                            onClick={() => retryWorkspaceExecution()}
                            disabled={retryLoading}
                            outline={true}
                            variant="danger"
                        >
                            <Icon aria-hidden={true} svgPath={mdiSync} /> Retry
                        </Button>
                    </div>
                    {retryError && <ErrorAlert error={retryError} />}
                </>
            )}

            {workspace.changesetSpecs && workspace.state === BatchSpecWorkspaceState.COMPLETED && (
                <div className="mb-3">
                    {workspace.changesetSpecs.length === 0 && (
                        <Text className="mb-0 text-muted">This workspace generated no changeset specs.</Text>
                    )}
                    {workspace.changesetSpecs.map((changesetSpec, index) => (
                        <React.Fragment key={changesetSpec.id}>
                            <ChangesetSpecNode
                                node={changesetSpec}
                                queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                            />
                            {index !== workspace.changesetSpecs!.length - 1 && <hr className="m-0" />}
                        </React.Fragment>
                    ))}
                </div>
            )}

            {workspace.steps.map((step, index) => (
                <React.Fragment key={step.number}>
                    <WorkspaceStep
                        step={step}
                        cachedResultFound={workspace.cachedResultFound}
                        workspaceID={workspace.id}
                        queryBatchSpecWorkspaceStepFileDiffs={queryBatchSpecWorkspaceStepFileDiffs}
                        telemetryRecorder={telemetryRecorder}
                    />
                    {index !== workspace.steps.length - 1 && <hr className="my-2" />}
                </React.Fragment>
            ))}
        </div>
    )
}

interface IgnoredWorkspaceDetailsProps extends Pick<WorkspaceDetailsProps, 'deselectWorkspace' | 'telemetryRecorder'> {
    workspace: VisibleBatchSpecWorkspaceFields
}

const IgnoredWorkspaceDetails: React.FunctionComponent<React.PropsWithChildren<IgnoredWorkspaceDetailsProps>> = ({
    workspace,
    deselectWorkspace,
    telemetryRecorder,
}) => (
    <>
        <WorkspaceHeader
            deselectWorkspace={deselectWorkspace}
            workspace={workspace}
            telemetryRecorder={telemetryRecorder}
        />
        <H1 className="text-center text-muted mt-5">
            <Icon aria-hidden={true} svgPath={mdiLinkVariantRemove} />
            <VisuallyHidden>Ignored Workspace</VisuallyHidden>
        </H1>
        <Text alignment="center">
            This workspace has been skipped because a <Code>.batchignore</Code> file is present in the workspace
            repository.
        </Text>
        <Text alignment="center">Enable the execution option ignored" to override.</Text>
    </>
)

interface UnsupportedWorkspaceDetailsProps
    extends Pick<WorkspaceDetailsProps, 'deselectWorkspace' | 'telemetryRecorder'> {
    workspace: VisibleBatchSpecWorkspaceFields
}

const UnsupportedWorkspaceDetails: React.FunctionComponent<
    React.PropsWithChildren<UnsupportedWorkspaceDetailsProps>
> = ({ workspace, deselectWorkspace, telemetryRecorder }) => (
    <>
        <WorkspaceHeader
            deselectWorkspace={deselectWorkspace}
            workspace={workspace}
            telemetryRecorder={telemetryRecorder}
        />
        <H1 className="text-center text-muted mt-5">
            <Icon aria-hidden={true} svgPath={mdiLinkVariantRemove} />
            <VisuallyHidden>Unsupported Workspace</VisuallyHidden>
        </H1>
        <Text alignment="center">This workspace has been skipped because it is from an unsupported codehost.</Text>
        <Text alignment="center">Enable the execution option "allow unsupported" to override.</Text>
    </>
)

const NumberInQueue: React.FunctionComponent<React.PropsWithChildren<{ number: number }>> = ({ number }) => (
    <>
        {number}
        <sup>{indicator(number)}</sup>
    </>
)

interface ChangesetSpecNodeProps {
    node: BatchSpecWorkspaceChangesetSpecFields
    queryChangesetSpecFileDiffs?: typeof _queryChangesetSpecFileDiffs
}

const ChangesetSpecNode: React.FunctionComponent<React.PropsWithChildren<ChangesetSpecNodeProps>> = ({
    node,
    queryChangesetSpecFileDiffs = _queryChangesetSpecFileDiffs,
}) => {
    // TODO: Under what conditions should this be auto-expanded?
    const [isExpanded, setIsExpanded] = useState(true)
    const [areChangesExpanded, setAreChangesExpanded] = useState(true)

    // TODO: This should not happen. When the workspace is visible, the changeset spec should be visible as well.
    if (node.__typename === 'HiddenChangesetSpec') {
        return (
            <Card>
                <CardBody>
                    <H4>Changeset in a hidden repo</H4>
                </CardBody>
            </Card>
        )
    }

    // This should not happen.
    if (node.description.__typename === 'ExistingChangesetReference') {
        return null
    }

    return (
        <Collapse isOpen={isExpanded} onOpenChange={setIsExpanded} openByDefault={true}>
            <CollapseHeader
                as={Button}
                className="w-100 p-0 m-0 border-0 d-flex align-items-center justify-content-between"
            >
                <Icon aria-hidden={true} svgPath={isExpanded ? mdiChevronUp : mdiChevronDown} className="mr-1" />
                <div className={styles.collapseHeader}>
                    <Heading as="h4" styleAs="h3" className="mb-0 d-inline-block mr-2">
                        <VisuallyHidden>Execution</VisuallyHidden>
                        <span className={styles.result}>Result</span>
                        {node.description.published !== null && (
                            <Badge className="text-uppercase ml-2">
                                {publishBadgeLabel(node.description.published)}
                            </Badge>
                        )}
                    </Heading>
                    <Icon aria-hidden={true} className="text-muted mr-1 flex-shrink-0" svgPath={mdiSourceBranch} />
                    <VisuallyHidden>on branch</VisuallyHidden>
                    <span className={classNames('text-monospace text-muted', styles.changesetSpecBranch)}>
                        {node.description.headRef}
                    </span>
                </div>
                <VisuallyHidden>, generated changeset with</VisuallyHidden>
                <DiffStat
                    {...node.description.diffStat}
                    expandedCounts={true}
                    className={classNames(styles.stepDiffStat, 'ml-3')}
                />
            </CollapseHeader>
            <CollapsePanel>
                <Card className={classNames('mt-2', styles.resultCard)}>
                    <CardBody>
                        <Heading as="h5" styleAs="h3" className={styles.changesetTemplateHeader}>
                            Changeset template
                        </Heading>
                        <Heading as="h6" styleAs="h4">
                            {node.description.title}
                        </Heading>
                        <Text className="mb-0">{node.description.body}</Text>
                        {node.description.published && (
                            <Text>
                                <strong>Published:</strong> {String(node.description.published)}
                            </Text>
                        )}
                        <Collapse isOpen={areChangesExpanded} onOpenChange={setAreChangesExpanded} openByDefault={true}>
                            <CollapseHeader as={Button} className="w-100 p-0 m-0 border-0 d-flex align-items-center">
                                <Icon
                                    aria-hidden={true}
                                    svgPath={areChangesExpanded ? mdiChevronUp : mdiChevronDown}
                                    className="mr-1"
                                />
                                <Heading className="mb-0" as="h4" styleAs="h3">
                                    Changes
                                </Heading>
                            </CollapseHeader>
                            <CollapsePanel>
                                <ChangesetSpecFileDiffConnection
                                    spec={node.id}
                                    queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                                />
                            </CollapsePanel>
                        </Collapse>
                    </CardBody>
                </Card>
            </CollapsePanel>
        </Collapse>
    )
}

function publishBadgeLabel(state: Scalars['PublishedValue']): string {
    switch (state) {
        case 'draft': {
            return 'will publish as draft'
        }
        case false: {
            return 'will not publish'
        }
        case true: {
            return 'will publish'
        }
    }
}

interface WorkspaceStepProps extends TelemetryV2Props {
    cachedResultFound: boolean
    step: BatchSpecWorkspaceStepFields
    workspaceID: Scalars['ID']
    /** For testing purposes only */
    queryBatchSpecWorkspaceStepFileDiffs?: typeof _queryBatchSpecWorkspaceStepFileDiffs
}

export const OUTPUT_LINES_PER_PAGE = 500

export const WorkspaceStepOutputLines: React.FunctionComponent<
    React.PropsWithChildren<Pick<WorkspaceStepProps, 'step' | 'workspaceID'>>
> = ({ step, workspaceID }) => {
    const { connection, error, loading, fetchMore, hasNextPage } = useShowMorePagination<
        BatchSpecWorkspaceStepResult,
        BatchSpecWorkspaceStepVariables,
        string
    >({
        query: BATCH_SPEC_WORKSPACE_STEP,
        variables: {
            workspaceID,
            stepIndex: step.number,
            first: OUTPUT_LINES_PER_PAGE,
            after: null,
        },
        options: {
            useURL: false,
            fetchPolicy: 'cache-and-network',
        },
        getConnection: result => {
            const data = dataOrThrowErrors(result)
            if (data.node?.__typename !== 'VisibleBatchSpecWorkspace' || data.node.step === null) {
                throw new Error('unable to fetch workspace step')
            }

            return data.node.step.outputLines
        },
    })

    const additionalOutputLines = useMemo(() => {
        const lines = []

        if (connection) {
            if (connection.nodes.length === 0) {
                lines.push('stdout: This command did not produce any output')
            }

            if (step.exitCode !== null && step.exitCode !== 0) {
                lines.push(`stderr: Command failed with status ${step.exitCode}`)
            }

            if (step.exitCode === 0) {
                lines.push(`stdout: \nstdout: Command exited successfully with status ${step.exitCode}`)
            }
        }

        return lines
    }, [connection, step.exitCode])

    if (loading && !connection) {
        return (
            <div className="d-flex justify-content-center mt-4">
                <LoadingSpinner />
            </div>
        )
    }

    if (error || !connection || connection.error) {
        return (
            <Text className="text-muted">
                <span className="text-muted">Unable to fetch output logs for step ${step.number}.</span>
            </Text>
        )
    }

    return (
        <div className={styles.stepOutputContainer}>
            {connection.nodes.length > 0 && <LogOutput text={connection.nodes.join('\n')} />}
            {hasNextPage && (
                <>
                    {loading ? (
                        <LoadingSpinner className="bg-transparent ml-3" />
                    ) : (
                        <Button size="sm" className={styles.stepOutputShowMoreBtn} onClick={fetchMore}>
                            Load more ...
                        </Button>
                    )}
                </>
            )}
            <LogOutput text={additionalOutputLines.join('\n')} />
        </div>
    )
}

const WorkspaceStep: React.FunctionComponent<React.PropsWithChildren<WorkspaceStepProps>> = ({
    step,
    workspaceID,
    cachedResultFound,
    queryBatchSpecWorkspaceStepFileDiffs,
    telemetryRecorder,
}) => {
    const [isExpanded, setIsExpanded] = useState(false)
    const tabsNames = ['logs', 'output', 'diff', 'files_env', 'cmd_container']
    return (
        <Collapse isOpen={isExpanded} onOpenChange={setIsExpanded}>
            <CollapseHeader
                as={Button}
                className="w-100 p-0 m-0 border-0 d-flex align-items-center justify-content-between"
            >
                <Icon aria-hidden={true} svgPath={isExpanded ? mdiChevronUp : mdiChevronDown} className="mr-1" />
                <div className={classNames(styles.collapseHeader, step.skipped && 'text-muted')}>
                    <StepStateIcon step={step} />
                    <H3 className={styles.stepNumber}>Step {step.number}</H3>
                    <Code className={classNames('text-muted', styles.stepCommand)}>{step.run}</Code>
                </div>
                {step.diffStat && <DiffStat className={styles.stepDiffStat} {...step.diffStat} expandedCounts={true} />}
                {step.startedAt && (
                    <span className={classNames('text-monospace text-muted', styles.stepTime)}>
                        <StepTimer startedAt={step.startedAt} finishedAt={step.finishedAt} />
                    </span>
                )}
            </CollapseHeader>
            <CollapsePanel>
                <Card className={classNames('mt-2', styles.stepCard)}>
                    <CardBody>
                        {!step.skipped && (
                            <Tabs
                                size="medium"
                                behavior="forceRender"
                                onChange={index => {
                                    eventLogger.log(`batch_change_execution:workspace_tab_${tabsNames[index]}:clicked`)
                                    telemetryRecorder.recordEvent('batchChange.execution.tab', 'click', {
                                        metadata: { tab: index },
                                    })
                                }}
                            >
                                <TabList>
                                    <Tab key="logs">
                                        <span className="text-content" data-tab-content="Logs">
                                            Logs
                                        </span>
                                    </Tab>
                                    <Tab key="output-variables">
                                        <span className="text-content" data-tab-content="Output variables">
                                            Output variables
                                        </span>
                                    </Tab>
                                    <Tab key="diff">
                                        <span className="text-content" data-tab-content="Diff">
                                            Diff
                                        </span>
                                    </Tab>
                                    <Tab key="files-env">
                                        <span className="text-content" data-tab-content="Files / Env">
                                            Files / Env
                                        </span>
                                    </Tab>
                                    <Tab key="command-container">
                                        <span className="text-content" data-tab-content="Commands / Container">
                                            Commands / Container
                                        </span>
                                    </Tab>
                                </TabList>
                                <TabPanels>
                                    <TabPanel className="pt-2" key="logs">
                                        {step.startedAt ? (
                                            <WorkspaceStepOutputLines step={step} workspaceID={workspaceID} />
                                        ) : (
                                            <Text className="text-muted mb-0">Step not started yet</Text>
                                        )}
                                    </TabPanel>
                                    <TabPanel className="pt-2" key="output-variables">
                                        {!step.startedAt && (
                                            <Text className="text-muted mb-0">Step not started yet</Text>
                                        )}
                                        {step.outputVariables?.length === 0 && (
                                            <Text className="text-muted mb-0">No output variables specified</Text>
                                        )}
                                        <ul className="mb-0">
                                            {step.outputVariables?.map(variable => (
                                                <li key={variable.name}>
                                                    {variable.name}: {JSON.stringify(variable.value)}
                                                </li>
                                            ))}
                                        </ul>
                                    </TabPanel>
                                    <TabPanel className="pt-2" key="diff">
                                        {!step.startedAt && (
                                            <Text className="text-muted mb-0">Step not started yet</Text>
                                        )}
                                        {step.startedAt && (
                                            <WorkspaceStepFileDiffConnection
                                                step={step}
                                                workspaceID={workspaceID}
                                                queryBatchSpecWorkspaceStepFileDiffs={
                                                    queryBatchSpecWorkspaceStepFileDiffs
                                                }
                                            />
                                        )}
                                    </TabPanel>
                                    <TabPanel className="pt-2" key="files-env">
                                        {step.environment.length === 0 && (
                                            <Text className="text-muted mb-0">No environment variables specified</Text>
                                        )}
                                        <ul className="mb-0">
                                            {step.environment.map(variable => (
                                                <li key={variable.name}>
                                                    {variable.name}: {variable.value !== null && <>{variable.value}</>}
                                                    {variable.value === null && <i>Set from secret</i>}
                                                </li>
                                            ))}
                                        </ul>
                                    </TabPanel>
                                    <TabPanel className="pt-2" key="command-container">
                                        {step.ifCondition !== null && (
                                            <>
                                                <H4>If condition</H4>
                                                <LogOutput text={step.ifCondition} className="mb-2" />
                                            </>
                                        )}
                                        <H4>Command</H4>
                                        <LogOutput text={step.run} className="mb-2" />
                                        <H4>Container</H4>
                                        <Text className="text-monospace mb-0">{step.container}</Text>
                                    </TabPanel>
                                </TabPanels>
                            </Tabs>
                        )}
                        {step.skipped && (
                            <Text className="mb-0">
                                <strong>
                                    Step has been skipped
                                    {cachedResultFound && <> because a cached result was found for this workspace</>}
                                    {!cachedResultFound && step.cachedResultFound && (
                                        <> because a cached result was found for this step</>
                                    )}
                                    .
                                </strong>
                            </Text>
                        )}
                    </CardBody>
                </Card>
            </CollapsePanel>
        </Collapse>
    )
}

const StepTimer: React.FunctionComponent<React.PropsWithChildren<{ startedAt: string; finishedAt: Maybe<string> }>> = ({
    startedAt,
    finishedAt,
}) => <Duration start={startedAt} end={finishedAt ?? undefined} />

interface WorkspaceStepFileDiffConnectionProps {
    workspaceID: Scalars['ID']
    // Require the entire step instead of just the spec number to ensure the query gets called as the step changes.
    step: BatchSpecWorkspaceStepFields
    queryBatchSpecWorkspaceStepFileDiffs?: typeof _queryBatchSpecWorkspaceStepFileDiffs
}

const WorkspaceStepFileDiffConnection: React.FunctionComponent<
    React.PropsWithChildren<WorkspaceStepFileDiffConnectionProps>
> = ({ workspaceID, step, queryBatchSpecWorkspaceStepFileDiffs = _queryBatchSpecWorkspaceStepFileDiffs }) => {
    const queryFileDiffs = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryBatchSpecWorkspaceStepFileDiffs({
                after: args.after ?? null,
                first: args.first ?? null,
                node: workspaceID,
                step: step.number,
            }),
        [workspaceID, step, queryBatchSpecWorkspaceStepFileDiffs]
    )
    return (
        <FilteredConnection<FileDiffFields, Omit<FileDiffNodeProps, 'node'>>
            listClassName="list-group list-group-flush"
            noun="changed file"
            pluralNoun="changed files"
            queryConnection={queryFileDiffs}
            nodeComponent={FileDiffNode}
            nodeComponentProps={{
                persistLines: true,
                lineNumbers: true,
            }}
            defaultFirst={15}
            hideSearch={true}
            noSummaryIfAllNodesVisible={true}
            withCenteredSummary={true}
            useURLQuery={false}
            cursorPaging={true}
        />
    )
}
