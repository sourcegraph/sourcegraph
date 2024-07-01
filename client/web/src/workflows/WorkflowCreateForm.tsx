import { useCallback, useEffect, useMemo, type FunctionComponent } from 'react'

import { useNavigate } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { screenReaderAnnounce } from '@sourcegraph/wildcard'

import type { CreateWorkflowResult, CreateWorkflowVariables } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'

import { createWorkflowMutation } from './backend'
import { WorkflowForm, type WorkflowFormValue } from './WorkflowForm'

export const WorkflowCreateForm: FunctionComponent<
    NamespaceProps & {
        telemetryRecorder: TelemetryRecorder
    }
> = ({ namespace, telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('workflows.new', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [namespace, telemetryRecorder])

    const navigate = useNavigate()

    const [createWorkflow, { loading, error }] = useMutation<CreateWorkflowResult, CreateWorkflowVariables>(
        createWorkflowMutation,
        {}
    )

    const onSubmit = useCallback(
        async (fields: WorkflowFormValue): Promise<void> => {
            try {
                await createWorkflow({
                    variables: {
                        input: {
                            name: fields.name,
                            description: fields.description,
                            templateText: fields.templateText,
                            draft: fields.draft,
                            owner: namespace.id,
                        },
                    },
                })
                telemetryRecorder.recordEvent('workflows', 'create', {
                    metadata: namespaceTelemetryMetadata(namespace),
                })
                screenReaderAnnounce(`Created new workflow: ${fields.description}`)
                navigate(`${namespace.url}/workflows`, {
                    state: { description: fields.description },
                })
            } catch {
                // Mutation error is read in useMutation call.
            }
        },
        [namespace, telemetryRecorder, navigate, createWorkflow]
    )

    const initialValue = useMemo<Partial<WorkflowFormValue>>(() => ({}), [])

    return (
        <WorkflowForm
            namespace={namespace}
            telemetryRecorder={telemetryRecorder}
            submitLabel="Create"
            title="New workflow"
            initialValue={initialValue}
            onSubmit={onSubmit}
            loading={loading}
            error={error}
        />
    )
}
