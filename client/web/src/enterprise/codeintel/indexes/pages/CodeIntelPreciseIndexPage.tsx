import { type FunctionComponent, useCallback, useEffect, useMemo, useState } from 'react'

import { useApolloClient } from '@apollo/client'
import { mdiDelete, mdiGraph, mdiHistory, mdiRecycle, mdiRedo, mdiTimerSand } from '@mdi/js'
import classNames from 'classnames'
import { Navigate, useLocation, useParams, useNavigate } from 'react-router-dom'
import { takeWhile } from 'rxjs/operators'

import { type ErrorLike, isErrorLike } from '@sourcegraph/common'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Alert,
    type AlertProps,
    Button,
    Card,
    CardBody,
    CardText,
    CardTitle,
    Container,
    ErrorAlert,
    ErrorMessage,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Tooltip,
    useObservable,
} from '@sourcegraph/wildcard'

import { type PreciseIndexFields, PreciseIndexState } from '../../../../graphql-operations'
import { FlashMessage } from '../../configuration/components/FlashMessage'
import { AuditLogPanel } from '../components/AuditLog'
import { PreciseIndexLastUpdated } from '../components/CodeIntelLastUpdated'
import { DependenciesList, DependentsList } from '../components/Dependencies'
import { IndexTimeline } from '../components/IndexTimeline'
import { ProjectDescription } from '../components/ProjectDescription'
import { RetentionList } from '../components/RetentionList'
import type { queryDependencyGraph as defaultQueryDependencyGraph } from '../hooks/queryDependencyGraph'
import { queryPreciseIndex as defaultQueryPreciseIndex } from '../hooks/queryPreciseIndex'
import { useDeletePreciseIndex as defaultUseDeletePreciseIndex } from '../hooks/useDeletePreciseIndex'
import { useReindexPreciseIndex as defaultUseReindexPreciseIndex } from '../hooks/useReindexPreciseIndex'

import styles from './CodeIntelPreciseIndexPage.module.scss'

export interface CodeIntelPreciseIndexPageProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    now?: () => Date
    queryDependencyGraph?: typeof defaultQueryDependencyGraph
    queryPreciseIndex?: typeof defaultQueryPreciseIndex
    useDeletePreciseIndex?: typeof defaultUseDeletePreciseIndex
    useReindexPreciseIndex?: typeof defaultUseReindexPreciseIndex
    indexingEnabled?: boolean
}

const variantByState = new Map<PreciseIndexState, AlertProps['variant']>([
    [PreciseIndexState.COMPLETED, 'success'],
    [PreciseIndexState.INDEXING_ERRORED, 'danger'],
    [PreciseIndexState.PROCESSING_ERRORED, 'danger'],
])

export const CodeIntelPreciseIndexPage: FunctionComponent<CodeIntelPreciseIndexPageProps> = ({
    authenticatedUser,
    queryPreciseIndex = defaultQueryPreciseIndex,
    useDeletePreciseIndex = defaultUseDeletePreciseIndex,
    useReindexPreciseIndex = defaultUseReindexPreciseIndex,
    indexingEnabled = window.context?.codeIntelAutoIndexingEnabled,
    telemetryService,
    telemetryRecorder,
}) => {
    const { id } = useParams<{ id: string }>()
    const navigate = useNavigate()
    const location = useLocation()

    useEffect(() => {
        telemetryService.logViewEvent('CodeIntelPreciseIndexPage')
        telemetryRecorder.recordEvent('repo.codeIntel.preciseIndex', 'view')
    }, [telemetryService, telemetryRecorder])

    const apolloClient = useApolloClient()
    const { handleReindexPreciseIndex, reindexError } = useReindexPreciseIndex()
    const { handleDeletePreciseIndex, deleteError } = useDeletePreciseIndex()

    // State to track reindex/delete operations
    const [reindexOrError, setReindexOrError] = useState<'loading' | 'reindexed' | ErrorLike>()
    const [deletionOrError, setDeletionOrError] = useState<'loading' | 'deleted' | ErrorLike>()

    // Seed initial state
    useEffect(() => setDeletionOrError(deleteError), [deleteError])
    useEffect(() => setReindexOrError(reindexError), [reindexError])

    const indexOrError = useObservable(
        useMemo(
            // Continuously re-fetch state while it's in a non-terminal state
            () => queryPreciseIndex(id!, apolloClient).pipe(takeWhile(shouldReload, true)),
            [id, queryPreciseIndex, apolloClient]
        )
    )

    const deleteUpload = useCallback(async (): Promise<void> => {
        if (!indexOrError || isErrorLike(indexOrError) || !window.confirm('Delete index?')) {
            return
        }

        setDeletionOrError('loading')

        try {
            await handleDeletePreciseIndex({
                variables: { id: id! },
                update: cache => cache.modify({ fields: { node: () => {} } }),
            })
            setDeletionOrError('deleted')
            navigate(
                {},
                {
                    state: {
                        modal: 'SUCCESS',
                        message: 'Index deleted.',
                    },
                }
            )
        } catch (error) {
            setDeletionOrError(error)
            navigate(
                {},
                {
                    state: {
                        modal: 'ERROR',
                        message: 'There was an error while deleting an index.',
                    },
                }
            )
        }
    }, [id, indexOrError, handleDeletePreciseIndex, navigate])

    const reindexUpload = useCallback(async (): Promise<void> => {
        if (!indexOrError || isErrorLike(indexOrError)) {
            return
        }

        setReindexOrError('loading')

        try {
            await handleReindexPreciseIndex({
                variables: { id: id! },
                update: cache => cache.modify({ fields: { node: () => {} } }),
            })
            setReindexOrError('reindexed')
            navigate(
                {},
                {
                    state: {
                        modal: 'SUCCESS',
                        message: 'Marked index as replaceable by auto-indexing.',
                    },
                }
            )
        } catch (error) {
            setReindexOrError(error)
            navigate(
                {},
                {
                    state: {
                        modal: 'ERROR',
                        message: 'There was an error while marking index as replaceable by auto-indexing.',
                    },
                }
            )
        }
    }, [id, indexOrError, handleReindexPreciseIndex, navigate])

    return deletionOrError === 'deleted' ? (
        <Navigate to="../indexes" replace={true} />
    ) : isErrorLike(deletionOrError) ? (
        <ErrorAlert prefix="Error deleting precise index" error={deletionOrError} />
    ) : isErrorLike(reindexOrError) ? (
        <ErrorAlert prefix="Error marking precise index as replaceable by auto-indexing" error={reindexOrError} />
    ) : isErrorLike(indexOrError) ? (
        <ErrorAlert prefix="Error fetching precise index" error={indexOrError} />
    ) : !indexOrError ? (
        <LoadingSpinner />
    ) : (
        <>
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: (
                            <>
                                Precise index of{' '}
                                {indexOrError.projectRoot
                                    ? `${indexOrError.projectRoot.repository.name}@${indexOrError.projectRoot.commit.abbreviatedOID}`
                                    : 'an unknown commit'}
                            </>
                        ),
                    },
                ]}
                className="mb-3"
            />

            {!!location.state && <FlashMessage state={location.state.modal} message={location.state.message} />}

            <Container>
                <IndexDescription index={indexOrError} />

                <div className="mt-2">
                    <Alert variant={variantByState.get(indexOrError.state) ?? 'primary'}>
                        <span>
                            {indexOrError.state === PreciseIndexState.UPLOADING_INDEX ? (
                                <span>Still uploading...</span>
                            ) : indexOrError.state === PreciseIndexState.DELETING ? (
                                <span>Upload is queued for deletion.</span>
                            ) : indexOrError.state === PreciseIndexState.QUEUED_FOR_INDEXING ? (
                                <span>
                                    {indexOrError.placeInQueue === 1 ? (
                                        <>This index is next up for indexing.</>
                                    ) : (
                                        <>
                                            {indexOrError.placeInQueue
                                                ? `There are ${
                                                      indexOrError.placeInQueue - 1
                                                  } indexes ahead of this one in the indexing queue.`
                                                : ''}
                                        </>
                                    )}
                                </span>
                            ) : indexOrError.state === PreciseIndexState.QUEUED_FOR_PROCESSING ? (
                                <span>
                                    {indexOrError.placeInQueue === 1 ? (
                                        <>This index is next up for processing.</>
                                    ) : (
                                        <>
                                            {indexOrError.placeInQueue
                                                ? `There are ${
                                                      indexOrError.placeInQueue - 1
                                                  } indexes ahead of this one in the processing queue.`
                                                : ''}
                                        </>
                                    )}
                                </span>
                            ) : indexOrError.state === PreciseIndexState.INDEXING ? (
                                <span>Indexing in progress...</span>
                            ) : indexOrError.state === PreciseIndexState.PROCESSING ? (
                                <span>Index is currently being processed...</span>
                            ) : indexOrError.state === PreciseIndexState.COMPLETED ? (
                                <span>Index processed successfully.</span>
                            ) : indexOrError.state === PreciseIndexState.INDEXING_ERRORED ? (
                                <span>
                                    Index failed to index: <ErrorMessage error={indexOrError.failure} />
                                </span>
                            ) : indexOrError.state === PreciseIndexState.PROCESSING_ERRORED ? (
                                <span>
                                    Index failed to process: <ErrorMessage error={indexOrError.failure} />
                                </span>
                            ) : (
                                <></>
                            )}
                        </span>
                    </Alert>

                    {indexOrError.isLatestForRepo && (
                        <Alert variant="secondary">
                            <span>
                                This upload can answer queries for the tip of the default branch and are targets of
                                cross-repository find reference operations.
                            </span>
                        </Alert>
                    )}
                </div>

                {authenticatedUser?.siteAdmin && (
                    <div className="my-4">
                        <CodeIntelDeleteUpload
                            state={indexOrError.state}
                            deleteUpload={deleteUpload}
                            deletionOrError={deletionOrError}
                        />

                        {indexingEnabled && (
                            <CodeIntelReindexUpload reindexUpload={reindexUpload} reindexOrError={reindexOrError} />
                        )}
                    </div>
                )}

                <Tabs size="medium" className={classNames('mt-2', styles.tabs)}>
                    <TabList>
                        <Tab>
                            <span>
                                <Icon aria-hidden={true} className="text-muted mr-2" svgPath={mdiTimerSand} />
                                <span className="text-content" data-tab-content="Timeline">
                                    Timeline
                                </span>
                            </span>
                        </Tab>

                        {(indexOrError.state === PreciseIndexState.COMPLETED ||
                            indexOrError.state === PreciseIndexState.DELETING) && (
                            <>
                                <Tab>
                                    <span>
                                        <Icon aria-hidden={true} className="text-muted mr-2" svgPath={mdiGraph} />
                                        <span className="text-content" data-tab-content="Dependencies">
                                            Dependencies
                                        </span>
                                    </span>
                                </Tab>

                                <Tab>
                                    <span>
                                        <Icon aria-hidden={true} className="text-muted mr-2" svgPath={mdiGraph} />
                                        <span className="text-content" data-tab-content="Dependents">
                                            Dependents
                                        </span>
                                    </span>
                                </Tab>

                                <Tab>
                                    <span>
                                        <Icon aria-hidden={true} className="text-muted mr-2" svgPath={mdiRecycle} />
                                        <span className="text-content" data-tab-content="Retention">
                                            Retention
                                        </span>
                                    </span>
                                </Tab>

                                {(indexOrError.auditLogs?.length ?? 0) > 0 && (
                                    <Tab>
                                        <span>
                                            <Icon aria-hidden={true} className="text-muted mr-2" svgPath={mdiHistory} />
                                            <span className="text-content" data-tab-content="Audit logs">
                                                Audit logs
                                            </span>
                                        </span>
                                    </Tab>
                                )}
                            </>
                        )}
                    </TabList>

                    <TabPanels>
                        <TabPanel>
                            <div className="mt-2">
                                <IndexTimeline index={indexOrError} />
                            </div>
                        </TabPanel>

                        {(indexOrError.state === PreciseIndexState.COMPLETED ||
                            indexOrError.state === PreciseIndexState.DELETING) && (
                            <>
                                <TabPanel>
                                    <div className="mt-2">
                                        <DependenciesList index={indexOrError} />
                                    </div>
                                </TabPanel>

                                <TabPanel>
                                    <div className="mt-2">
                                        <DependentsList index={indexOrError} />
                                    </div>
                                </TabPanel>

                                <TabPanel>
                                    <div className="mt-2">
                                        <RetentionList index={indexOrError} />
                                    </div>
                                </TabPanel>

                                {(indexOrError.auditLogs?.length ?? 0) > 0 && (
                                    <TabPanel>
                                        <div className="mt-2">
                                            <AuditLogPanel logs={indexOrError.auditLogs || []} />
                                        </div>
                                    </TabPanel>
                                )}
                            </>
                        )}
                    </TabPanels>
                </Tabs>
            </Container>
        </>
    )
}

interface IndexDescriptionProps {
    index: PreciseIndexFields
}

const IndexDescription: FunctionComponent<IndexDescriptionProps> = ({ index }) => (
    <Card>
        <CardBody>
            <CardTitle>
                {index.projectRoot ? (
                    <Link to={index.projectRoot.repository.url}>{index.projectRoot.repository.name}</Link>
                ) : (
                    <span>Unknown repository</span>
                )}
            </CardTitle>

            <CardText>
                <span className="d-block">
                    <ProjectDescription index={index} />
                </span>

                <small className="text-mute">
                    <PreciseIndexLastUpdated index={index} />{' '}
                    {index.shouldReindex && (
                        <Tooltip content="This index has been marked as replaceable by auto-indexing.">
                            <span className={classNames(styles.tag, 'ml-1 rounded')}>
                                (replaceable by auto-indexing)
                            </span>
                        </Tooltip>
                    )}
                </small>
            </CardText>
        </CardBody>
    </Card>
)

interface CodeIntelReindexUploadProps {
    reindexUpload: () => Promise<void>
    reindexOrError?: 'loading' | 'reindexed' | ErrorLike
}

const CodeIntelReindexUpload: FunctionComponent<CodeIntelReindexUploadProps> = ({ reindexUpload, reindexOrError }) => (
    <Tooltip content="Allow Sourcegraph to re-index this commit in the future and replace this data.">
        <Button type="button" variant="secondary" onClick={reindexUpload} disabled={reindexOrError === 'loading'}>
            <Icon aria-hidden={true} svgPath={mdiRedo} /> Mark index as replaceable by autoindexing
        </Button>
    </Tooltip>
)

interface CodeIntelDeleteUploadProps {
    state: PreciseIndexState
    deleteUpload: () => Promise<void>
    deletionOrError?: 'loading' | 'deleted' | ErrorLike
}

const CodeIntelDeleteUpload: FunctionComponent<CodeIntelDeleteUploadProps> = ({
    state,
    deleteUpload,
    deletionOrError,
}) =>
    state === PreciseIndexState.DELETING ? (
        <></>
    ) : (
        <Tooltip
            content={
                state === PreciseIndexState.COMPLETED
                    ? 'Deleting this index will make it unavailable to answer code navigation queries the next time the repository commit graph is refreshed.'
                    : 'Delete this index immediately'
            }
        >
            <Button
                type="button"
                className="float-right"
                variant="danger"
                onClick={deleteUpload}
                disabled={deletionOrError === 'loading'}
            >
                <Icon aria-hidden={true} svgPath={mdiDelete} /> Delete index
            </Button>
        </Tooltip>
    )

const terminalStates = new Set(['COMPLETED', 'INDEXING_ERRORED', 'PROCESSING_ERRORED'])

function shouldReload(index: PreciseIndexFields | ErrorLike | null | undefined): boolean {
    return !isErrorLike(index) && !(index && terminalStates.has(index.state))
}
