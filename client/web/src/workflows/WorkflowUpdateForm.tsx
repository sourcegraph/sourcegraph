import { useCallback, useEffect, useState, type FunctionComponent } from 'react'

import { useNavigate, useParams } from 'react-router-dom'

import { logger } from '@sourcegraph/common'
import { useMutation, useQuery } from '@sourcegraph/http-client'
import { Alert, Button, Container, ErrorAlert, LoadingSpinner } from '@sourcegraph/wildcard'

import type {
    UpdateWorkflowResult,
    UpdateWorkflowVariables,
    WorkflowResult,
    WorkflowVariables,
} from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'

import { deleteWorkflowMutation, updateWorkflowMutation, workflowQuery } from './backend'
import { WorkflowForm, type WorkflowFormValue } from './WorkflowForm'

interface Props extends NamespaceProps {
    id: string
}

export const WorkflowUpdateForm: FunctionComponent<Omit<Props, 'id'>> = props => {
    const { id } = useParams<{ id: string }>()

    return <InnerWorkflowUpdateForm {...props} id={id!} />
}

const InnerWorkflowUpdateForm: FunctionComponent<Props> = ({ id, namespace, telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('workflows.update', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [telemetryRecorder, namespace])
    const navigate = useNavigate()

    const result = useQuery<WorkflowResult, WorkflowVariables>(workflowQuery, { variables: { id } })

    const [updateWorkflow, { loading: updateLoading, error: updateError }] = useMutation<
        UpdateWorkflowResult,
        UpdateWorkflowVariables
    >(updateWorkflowMutation, {})
    const [flashUpdated, setFlashUpdated] = useState(false)

    const onSubmit = useCallback(
        async (fields: WorkflowFormValue): Promise<void> => {
            try {
                await updateWorkflow({
                    variables: {
                        id,
                        input: {
                            name: fields.name,
                            description: fields.description,
                            templateText: fields.templateText,
                            draft: fields.draft,
                        },
                    },
                })
                telemetryRecorder.recordEvent('workflows', 'update', {
                    metadata: namespaceTelemetryMetadata(namespace),
                })
                setFlashUpdated(true)
                setTimeout(() => {
                    setFlashUpdated(false)
                }, 1000)
            } catch {
                // Mutation error is read in useMutation call.
            }
        },
        [namespace, telemetryRecorder, updateWorkflow, id]
    )

    const workflow = result.data?.node?.__typename === 'Workflow' ? result.data.node : null

    const [deleteWorkflow, { loading: deleteLoading, error: deleteError }] = useMutation(deleteWorkflowMutation)
    const onDeleteClick = useCallback(async (): Promise<void> => {
        if (!workflow) {
            return
        }
        if (!window.confirm(`Delete the workflow ${JSON.stringify(workflow.nameWithOwner)}?`)) {
            return
        }
        try {
            await deleteWorkflow({ variables: { id: workflow.id } })
            telemetryRecorder.recordEvent('workflows', 'delete', {
                metadata: namespaceTelemetryMetadata(namespace),
            })
            navigate(`${namespace.url}/workflows`)
        } catch (error) {
            logger.error(error)
        }
    }, [workflow, deleteWorkflow, telemetryRecorder, namespace, navigate])

    return (
        <div>
            {result.loading ? (
                <LoadingSpinner />
            ) : !workflow ? (
                <Alert variant="danger" as="p">
                    Workflow not found.
                </Alert>
            ) : (
                <WorkflowForm
                    namespace={namespace}
                    telemetryRecorder={telemetryRecorder}
                    submitLabel="Update workflow"
                    title="Edit workflow"
                    initialValue={workflow}
                    loading={updateLoading}
                    error={updateError}
                    flash={flashUpdated && 'Updated!'}
                    onSubmit={onSubmit}
                />
            )}
            {flashUpdated && (
                <Alert variant="success" as="p">
                    Updated!
                </Alert>
            )}
            {workflow && (
                <Container className="mt-3">
                    <Button
                        aria-label="Delete"
                        className="test-delete-workflow-button"
                        onClick={onDeleteClick}
                        disabled={updateLoading || deleteLoading}
                        variant="danger"
                        outline={true}
                    >
                        Delete
                    </Button>
                    {deleteError && !deleteLoading && <ErrorAlert className="mt-3 mb-0" error={deleteError} />}
                </Container>
            )}
        </div>
    )
}
