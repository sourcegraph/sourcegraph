import classNames from 'classnames'
import React, { FormEventHandler, RefObject, useContext } from 'react'

import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'
import { Form } from '@sourcegraph/branded/src/components/Form'
import { Button } from '@sourcegraph/wildcard'

import { LoaderButton } from '../../../../../../../../components/LoaderButton'
import { CodeInsightDashboardsVisibility, VisibilityPicker } from '../../../../../../components/creation-ui-kit'
import { FormInput } from '../../../../../../components/form/form-input/FormInput'
import { useFieldAPI } from '../../../../../../components/form/hooks/useField'
import { FORM_ERROR, SubmissionErrors } from '../../../../../../components/form/hooks/useForm'
import { RepositoryField } from '../../../../../../components/form/repositories-field/RepositoryField'
import { CodeInsightsBackendContext } from '../../../../../../core/backend/code-insights-backend-context'
import { CodeInsightsGqlBackend } from '../../../../../../core/backend/gql-api/code-insights-gql-backend'
import { SupportedInsightSubject } from '../../../../../../core/types/subjects'
import { LangStatsCreationFormFields } from '../../types'

import styles from './LangStatsInsightCreationForm.module.scss'

export interface LangStatsInsightCreationFormProps {
    mode?: 'creation' | 'edit'
    innerRef: RefObject<any>
    handleSubmit: FormEventHandler
    submitErrors: SubmissionErrors
    submitting: boolean
    className?: string
    isFormClearActive?: boolean
    dashboardReferenceCount?: number

    title: useFieldAPI<LangStatsCreationFormFields['title']>
    repository: useFieldAPI<LangStatsCreationFormFields['repository']>
    threshold: useFieldAPI<LangStatsCreationFormFields['threshold']>
    visibility: useFieldAPI<LangStatsCreationFormFields['visibility']>
    subjects: SupportedInsightSubject[]

    onCancel: () => void
    onFormReset: () => void
}

export const LangStatsInsightCreationForm: React.FunctionComponent<LangStatsInsightCreationFormProps> = props => {
    const {
        mode = 'creation',
        innerRef,
        handleSubmit,
        submitErrors,
        submitting,
        className,
        title,
        repository,
        threshold,
        visibility,
        subjects,
        isFormClearActive,
        dashboardReferenceCount,
        onCancel,
        onFormReset,
    } = props

    const isEditMode = mode === 'edit'
    const api = useContext(CodeInsightsBackendContext)

    // We have to know about what exactly api we use to be able switch our UI properly.
    // In the creation UI case we should hide visibility section since we don't use that
    // concept anymore with new GQL backend.
    // TODO [VK]: Remove this condition rendering when we deprecate setting-based api
    const isGqlBackend = api instanceof CodeInsightsGqlBackend

    return (
        <Form
            ref={innerRef}
            noValidate={true}
            className={classNames(className, 'd-flex flex-column')}
            onSubmit={handleSubmit}
            onReset={onFormReset}
        >
            <FormInput
                as={RepositoryField}
                required={true}
                autoFocus={true}
                title="Repository"
                description="This insight is limited to one repository. You can set up multiple language usage charts for analyzing other repositories."
                placeholder="Example: github.com/sourcegraph/sourcegraph"
                loading={repository.meta.validState === 'CHECKING'}
                valid={repository.meta.touched && repository.meta.validState === 'VALID'}
                error={repository.meta.touched && repository.meta.error}
                {...repository.input}
                className="mb-0"
            />

            <FormInput
                required={true}
                title="Title"
                description="Shown as the title for your insight."
                placeholder="Example: Language Usage in RepositoryName"
                valid={title.meta.touched && title.meta.validState === 'VALID'}
                error={title.meta.touched && title.meta.error}
                {...title.input}
                className="mb-0 mt-4"
            />

            <FormInput
                required={true}
                min={1}
                max={100}
                type="number"
                title="Threshold of ‘Other’ category"
                description="Languages with usage lower than the threshold are grouped into an 'other' category."
                valid={threshold.meta.touched && threshold.meta.validState === 'VALID'}
                error={threshold.meta.touched && threshold.meta.error}
                {...threshold.input}
                className="mb-0 mt-4"
                inputClassName={styles.formThresholdInput}
                inputSymbol={<span className={styles.formThresholdInputSymbol}>%</span>}
            />

            {!isGqlBackend && (
                <VisibilityPicker
                    subjects={subjects}
                    value={visibility.input.value}
                    onChange={visibility.input.onChange}
                />
            )}

            {!!dashboardReferenceCount && dashboardReferenceCount > 1 && (
                <CodeInsightDashboardsVisibility className="mt-5 mb-n1" dashboardCount={dashboardReferenceCount} />
            )}

            <hr className={styles.formSeparator} />

            <div className="d-flex flex-wrap align-items-center">
                {submitErrors?.[FORM_ERROR] && <ErrorAlert className="w-100" error={submitErrors[FORM_ERROR]} />}

                <LoaderButton
                    alwaysShowLabel={true}
                    data-testid="insight-save-button"
                    loading={submitting}
                    label={submitting ? 'Submitting' : isEditMode ? 'Save insight' : 'Create code insight'}
                    type="submit"
                    disabled={submitting}
                    className="mr-2 mb-2"
                    variant="primary"
                />

                <Button type="button" variant="secondary" outline={true} className="mb-2 mr-auto" onClick={onCancel}>
                    Cancel
                </Button>

                <Button
                    type="reset"
                    variant="secondary"
                    outline={true}
                    disabled={!isFormClearActive}
                    className="border-0"
                >
                    Clear all fields
                </Button>
            </div>
        </Form>
    )
}
