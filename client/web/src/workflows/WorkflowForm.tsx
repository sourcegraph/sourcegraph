import React, { useCallback, useState, type ReactNode } from 'react'

import classNames from 'classnames'
import { noop } from 'lodash'

import {
    Alert,
    Button,
    Code,
    Container,
    ErrorAlert,
    Form,
    Input,
    InputDescription,
    Label,
    PageHeader,
    TextArea,
} from '@sourcegraph/wildcard'

import { PageTitle } from '../components/PageTitle'
import { NamespaceSelector } from '../enterprise/batches/create/NamespaceSelector'
import type { WorkflowInput, WorkflowUpdateInput } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'

import { generateSuggestedPrompt } from './promptAssistance'

import styles from './WorkflowForm.module.scss'

export interface WorkflowFormValue
    extends Pick<WorkflowInput | WorkflowUpdateInput, 'name' | 'description' | 'templateText' | 'draft'> {}

export interface WorkflowFormProps extends NamespaceProps {
    initialValue?: Partial<WorkflowFormValue>
    title?: string
    submitLabel: string
    onSubmit: (fields: WorkflowFormValue) => void
    loading: boolean
    error?: any
    flash?: ReactNode
}

const workflowNamePattern = '^[0-9A-Za-z](?:[0-9A-Za-z]|[.-](?=[0-9A-Za-z]))*-?$'

export const WorkflowForm: React.FunctionComponent<React.PropsWithChildren<WorkflowFormProps>> = ({
    initialValue,
    namespace,
    title,
    submitLabel,
    onSubmit,
    loading,
    error,
    flash,
}) => {
    const [value, setValue] = useState<WorkflowFormValue>(() => ({
        name: initialValue?.name || '',
        description: initialValue?.description || '',
        templateText: initialValue?.templateText || '',
        draft: initialValue?.draft || false,
    }))

    /**
     * Returns an input change handler that updates the SavedQueryFields in the component's state
     * @param key The key of saved query fields that a change of this input should update
     */
    const createInputChangeHandler =
        (key: keyof WorkflowFormValue): React.FormEventHandler<HTMLInputElement | HTMLTextAreaElement> =>
        event => {
            const { value, type } = event.currentTarget
            const checked = 'checked' in event.currentTarget ? event.currentTarget.checked : undefined
            setValue(values => ({
                ...values,
                [key]: type === 'checkbox' ? checked : value,
            }))
        }

    const onTemplateTextFocus = useCallback(async (): Promise<void> => {
        if (value.templateText === '') {
            const suggestion = await generateSuggestedPrompt(value)
            setValue(values =>
                values.templateText === ''
                    ? {
                          ...values,
                          templateText: suggestion,
                      }
                    : values
            )
        }
    }, [value])

    return (
        <div className="workflow-form" data-testid="workflow-form">
            <PageHeader className="mb-3">
                <PageTitle title={title} />
                <PageHeader.Heading as="h3" styleAs="h2">
                    <PageHeader.Breadcrumb>{title}</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <Form
                onSubmit={event => {
                    event.preventDefault()
                    onSubmit(value)
                }}
            >
                <Container className="mb-3">
                    <div className="d-flex flex-gap-4">
                        <NamespaceSelector
                            namespaces={[namespace]}
                            selectedNamespace={namespace.id}
                            label="Owner"
                            onSelect={noop}
                            disabled={true}
                            className={classNames(
                                'd-flex flex-column form-group flex-grow-0',
                                styles.namespaceSelector
                            )}
                        />
                        <div className="form-group">
                            <Label className="d-block" aria-hidden={true}>
                                &nbsp;
                            </Label>
                            <span className={styles.namespaceSlash}>/</span>
                        </div>
                        <div className="form-group">
                            <Input
                                name="description"
                                required={true}
                                value={value.name}
                                pattern={workflowNamePattern.toString()}
                                onChange={createInputChangeHandler('name')}
                                label="Workflow name"
                                autoComplete="off"
                                autoCapitalize="off"
                            />
                            <InputDescription className="mt-n1">
                                Only letters, numbers, _, and - are allowed. Example:{' '}
                                <Code>generate-typescript-e2e-tests</Code>
                            </InputDescription>
                        </div>
                    </div>
                    <Input
                        name="description"
                        value={value.description}
                        onChange={createInputChangeHandler('description')}
                        className="form-group"
                        autoComplete="off"
                        label="Description (optional)"
                    />
                    <div className="form-group">
                        <TextArea
                            name="templateText"
                            value={value.templateText}
                            onChange={createInputChangeHandler('templateText')}
                            label="Prompt template"
                            rows={10}
                            resizeable={true}
                            onFocus={onTemplateTextFocus}
                        />
                        <InputDescription>Tell Cody your overall goal and specific requirements.</InputDescription>
                    </div>
                    <div className="d-flex flex-gap-4 mt-1">
                        <Button
                            type="submit"
                            disabled={loading}
                            className="test-workflow-form-submit-button"
                            variant="primary"
                        >
                            {submitLabel}
                        </Button>
                        {flash && !loading && (
                            <Alert variant="success" className="mb-0" withIcon={false}>
                                {flash}
                            </Alert>
                        )}
                    </div>
                    {error && !loading && <ErrorAlert className="mt-3 mb-0" error={error} />}
                </Container>
            </Form>
        </div>
    )
}
