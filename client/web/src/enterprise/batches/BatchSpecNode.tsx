import React, { useCallback, useEffect, useState } from 'react'

import {
    mdiCheckCircle,
    mdiTimerSand,
    mdiCancel,
    mdiAlertCircle,
    mdiChevronDown,
    mdiChevronUp,
    mdiStar,
    mdiPencil,
    mdiFileDownload,
    mdiFileDocumentOutline,
} from '@mdi/js'
import classNames from 'classnames'
import { upperFirst } from 'lodash'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { useQuery } from '@sourcegraph/http-client'
import { BatchSpecSource, BatchSpecState } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Code,
    Link,
    Icon,
    H3,
    H4,
    Tooltip,
    Text,
    Button,
    LoadingSpinner,
    Alert,
    AnchorLink,
} from '@sourcegraph/wildcard'

import { Duration } from '../../components/time/Duration'
import type {
    BatchSpecListFields,
    Scalars,
    PartialBatchSpecWorkspaceFileFields,
    BatchSpecWorkspaceFileResult,
    BatchSpecWorkspaceFileVariables,
} from '../../graphql-operations'
import { humanizeSize } from '../../util/size'

import { BATCH_SPEC_WORKSPACE_FILE, generateFileDownloadLink } from './backend'
import { BatchSpec } from './BatchSpec'

import styles from './BatchSpecNode.module.scss'

export interface BatchSpecNodeProps extends TelemetryV2Props {
    node: BatchSpecListFields
    currentSpecID?: Scalars['ID']
    /** Used for testing purposes. Sets the current date */
    now?: () => Date
}

export const BatchSpecNode: React.FunctionComponent<React.PropsWithChildren<BatchSpecNodeProps>> = ({
    node,
    currentSpecID,
    now = () => new Date(),
    telemetryRecorder,
}) => {
    const [isExpanded, setIsExpanded] = useState(currentSpecID === node.id)
    const toggleIsExpanded = useCallback<React.MouseEventHandler<HTMLButtonElement>>(() => {
        setIsExpanded(!isExpanded)
    }, [isExpanded])

    return (
        <li className={styles.node}>
            <span className={styles.nodeSeparator} />
            <Button
                variant="icon"
                aria-label={isExpanded ? 'Collapse section' : 'Expand section'}
                onClick={toggleIsExpanded}
            >
                <Icon aria-hidden={true} svgPath={isExpanded ? mdiChevronUp : mdiChevronDown} />
            </Button>
            <div className="d-flex flex-column justify-content-center align-items-center px-2 pb-1">
                <StateIcon source={node.source} state={node.state} />
                <span className="text-muted">
                    {node.source === BatchSpecSource.LOCAL && 'Uploaded'}
                    {node.source !== BatchSpecSource.LOCAL && upperFirst(node.state.toLowerCase())}
                </span>
            </div>
            <div className="px-2 pb-1">
                <H3 className="pr-2">
                    {currentSpecID && (
                        <Link to={`${node.namespace.url}/batch-changes/${node.description.name}/executions/${node.id}`}>
                            {currentSpecID === node.id && (
                                <>
                                    <Tooltip content="Currently applied spec">
                                        <Icon
                                            aria-label="Currently applied spec"
                                            className="text-warning"
                                            svgPath={mdiStar}
                                        />
                                    </Tooltip>{' '}
                                </>
                            )}
                            Created by <strong>{node.creator?.username}</strong>{' '}
                            <Timestamp date={node.createdAt} now={now} />
                        </Link>
                    )}
                    {!currentSpecID && (
                        <>
                            <Link className="text-muted" to={`${node.namespace.url}/batch-changes`}>
                                {node.namespace.namespaceName}
                            </Link>
                            <span className="text-muted d-inline-block mx-1">/</span>
                            <Link
                                to={`${node.namespace.url}/batch-changes/${node.description.name}/executions/${node.id}`}
                            >
                                {node.description.name || '-'}
                            </Link>
                        </>
                    )}
                </H3>
                {!currentSpecID && (
                    <small className="text-muted d-block">
                        Created by <strong>{node.creator?.username}</strong>{' '}
                        <Timestamp date={node.createdAt} now={now} />
                    </small>
                )}
            </div>
            <div className="text-center pb-1">
                {node.startedAt && <Duration start={node.startedAt} end={node.finishedAt ?? undefined} />}
            </div>
            {isExpanded && (
                <div className={styles.nodeExpandedSection}>
                    <BatchSpecInfo spec={node} telemetryRecorder={telemetryRecorder} />
                </div>
            )}
        </li>
    )
}

interface BatchSpecInfoProps extends TelemetryV2Props {
    spec: Pick<BatchSpecListFields, 'originalInput' | 'id' | 'files' | 'description'>
}

type BatchWorkspaceFile = {
    isSpecFile: boolean
} & Omit<PartialBatchSpecWorkspaceFileFields, '__typename'>

export const BatchSpecInfo: React.FunctionComponent<BatchSpecInfoProps> = ({ spec, telemetryRecorder }) => {
    const specFile: BatchWorkspaceFile = {
        binary: false,
        isSpecFile: true,
        name: 'spec_file.yaml',
        id: spec.id,
        byteSize: spec.originalInput.length,
        url: '',
    }
    const [selectedFile, setSelectedFile] = useState<BatchWorkspaceFile>(specFile)

    if (spec.files && spec.files.totalCount > 0) {
        const mountedFiles: BatchWorkspaceFile[] = spec.files.nodes.map(file => ({
            isSpecFile: false,
            ...file,
        }))
        const allFiles = [specFile, ...mountedFiles]

        return (
            <div className={styles.specFilesContainer}>
                <ul className={styles.specFilesList}>
                    {allFiles.map(file => (
                        <li
                            key={file.id}
                            className={classNames(styles.specFilesListNode, {
                                [styles.specFilesListActiveNode]: file.id === selectedFile.id,
                            })}
                        >
                            <Button
                                title={file.name}
                                className={styles.specFilesListNodeButton}
                                onClick={() => setSelectedFile(file)}
                            >
                                {file.name}
                            </Button>
                        </li>
                    ))}
                </ul>

                {selectedFile.isSpecFile ? (
                    <BatchSpec
                        name={spec.description.name}
                        originalInput={spec.originalInput}
                        className={classNames(styles.batchSpec, 'mb-0')}
                        telemetryRecorder={telemetryRecorder}
                    />
                ) : (
                    <BatchWorkspaceFileContent file={selectedFile} specId={spec.id} />
                )}
            </div>
        )
    }

    return (
        <>
            <H4>Input spec</H4>
            <BatchSpec
                name={spec.description.name}
                originalInput={spec.originalInput}
                className={classNames(styles.batchSpec, 'mb-0')}
                telemetryRecorder={telemetryRecorder}
            />
        </>
    )
}

interface BatchWorkspaceFileContentProps {
    specId: string
    file: BatchWorkspaceFile
}

const BatchWorkspaceFileContent: React.FunctionComponent<BatchWorkspaceFileContentProps> = ({ file, specId }) => {
    if (file.binary) {
        return <BinaryBatchWorkspaceFile file={file} specId={specId} />
    }

    return <NonBinaryBatchWorkspaceFile id={file.id} />
}

const BinaryBatchWorkspaceFile: React.FunctionComponent<BatchWorkspaceFileContentProps> = ({ file }) => {
    const [loading, setIsLoading] = useState<boolean>(true)
    const [downloadUrl, setDownloadUrl] = useState<string>('')
    const [downloadError, setDownloadError] = useState<Error | null>(null)

    useEffect(() => {
        generateFileDownloadLink(file.url)
            .then(fileUrl => setDownloadUrl(fileUrl))
            .catch(error => setDownloadError(error))
            .finally(() => setIsLoading(false))
    }, [file.url])

    if (loading) {
        return <LoadingSpinner />
    }

    if (downloadError) {
        return (
            <Alert variant="danger" className={styles.fileError}>
                <Text>Error fetching file content: {downloadError?.message}</Text>
            </Alert>
        )
    }

    return (
        <div className={styles.specFileBinary}>
            <Icon aria-hidden={true} svgPath={mdiFileDocumentOutline} className={styles.specFileBinaryIcon} />
            <Text className={styles.specFileBinaryName}>
                {file.name} <span className={styles.specFileBinarySize}>{humanizeSize(file.byteSize)}</span>
            </Text>
            <Button
                outline={true}
                variant="secondary"
                size="sm"
                to={downloadUrl}
                download={file.name}
                className="mt-1"
                as={AnchorLink}
            >
                <Icon aria-hidden={true} svgPath={mdiFileDownload} className="mr-1" />
                {'  '}
                Download file
            </Button>
        </div>
    )
}

const NonBinaryBatchWorkspaceFile: React.FunctionComponent<Pick<BatchWorkspaceFile, 'id'>> = ({ id }) => {
    const { data, loading, error } = useQuery<BatchSpecWorkspaceFileResult, BatchSpecWorkspaceFileVariables>(
        BATCH_SPEC_WORKSPACE_FILE,
        {
            variables: { id },
            fetchPolicy: 'cache-first',
        }
    )

    if (loading) {
        return <LoadingSpinner />
    }

    if (error) {
        return (
            <Alert variant="danger" className={styles.fileError}>
                <Text>Error fetching file content: {error?.message}</Text>
            </Alert>
        )
    }

    if (!data || data.node?.__typename !== 'BatchSpecWorkspaceFile') {
        return (
            <Alert variant="danger" className={styles.fileError}>
                <Text>Not a valid BatchSpecWorkspaceFile</Text>
            </Alert>
        )
    }

    const { html } = data.node.highlight

    return (
        <pre className={styles.blobWrapper}>
            <Code
                className={styles.blobCode}
                dangerouslySetInnerHTML={{
                    __html: html,
                }}
            />
        </pre>
    )
}

const StateIcon: React.FunctionComponent<
    React.PropsWithChildren<{ state: BatchSpecState; source: BatchSpecSource }>
> = ({ state, source }) => {
    if (source === BatchSpecSource.LOCAL) {
        return (
            <Icon
                aria-hidden={true}
                className={classNames(styles.nodeStateIcon, 'text-success mb-1')}
                svgPath={mdiCheckCircle}
            />
        )
    }
    switch (state) {
        case BatchSpecState.COMPLETED: {
            return (
                <Icon
                    aria-hidden={true}
                    className={classNames(styles.nodeStateIcon, 'text-success mb-1')}
                    svgPath={mdiCheckCircle}
                />
            )
        }

        case BatchSpecState.PROCESSING:
        case BatchSpecState.QUEUED: {
            return (
                <Icon
                    aria-hidden={true}
                    className={classNames(styles.nodeStateIcon, 'text-muted mb-1')}
                    svgPath={mdiTimerSand}
                />
            )
        }

        case BatchSpecState.CANCELED:
        case BatchSpecState.CANCELING: {
            return (
                <Icon
                    aria-hidden={true}
                    className={classNames(styles.nodeStateIcon, 'text-muted mb-1')}
                    svgPath={mdiCancel}
                />
            )
        }

        case BatchSpecState.FAILED: {
            return (
                <Icon
                    aria-hidden={true}
                    className={classNames(styles.nodeStateIcon, 'text-danger mb-1')}
                    svgPath={mdiAlertCircle}
                />
            )
        }
        case BatchSpecState.PENDING: {
            return (
                <Icon
                    aria-hidden={true}
                    className={classNames(styles.nodeStateIcon, 'text-muted mb-1')}
                    svgPath={mdiPencil}
                />
            )
        }
    }
}
