import { QueryState } from '@sourcegraph/search'

import {
    createDefaultEditSeries,
    EditableDataSeries,
    Form,
    FormChangeEvent,
    insightSeriesValidator,
    insightStepValueValidator,
    insightTitleValidator,
    SubmissionErrors,
    useField,
    useFieldAPI,
    useForm,
} from '../../../../../components'
import { CreateInsightFormFields, InsightStep, RepoMode } from '../types'

import { useRepoFields } from './insight-repo-section/use-repo-fields'

export const INITIAL_INSIGHT_VALUES: CreateInsightFormFields = {
    // If user opens the creation form to create insight
    // we want to show the series form as soon as possible
    // and do not force the user to click the 'add another series' button
    series: [createDefaultEditSeries({ edit: true })],
    step: 'months',
    stepValue: '2',
    title: '',
    repositories: '',
    repoMode: 'urls-list',
    repoQuery: { query: '' },
    allRepos: false,
    dashboardReferenceCount: 0,
}

export interface UseInsightCreationFormProps {
    touched: boolean
    initialValue?: Partial<CreateInsightFormFields>
    onSubmit: (values: CreateInsightFormFields) => SubmissionErrors | Promise<SubmissionErrors> | void
    onChange?: (event: FormChangeEvent<CreateInsightFormFields>) => void
}

export interface InsightCreationForm {
    form: Form<CreateInsightFormFields>
    title: useFieldAPI<string>
    repositories: useFieldAPI<string>
    repoQuery: useFieldAPI<QueryState>
    repoMode: useFieldAPI<RepoMode>
    series: useFieldAPI<EditableDataSeries[]>
    step: useFieldAPI<InsightStep>
    stepValue: useFieldAPI<string>
}

/**
 * Hooks absorbs all insight creation form logic (field state managements,
 * validations, fields dependencies)
 */
export function useInsightCreationForm(props: UseInsightCreationFormProps): InsightCreationForm {
    const { touched, initialValue = {}, onSubmit, onChange } = props

    const form = useForm<CreateInsightFormFields>({
        initialValues: { ...INITIAL_INSIGHT_VALUES, ...initialValue },
        onSubmit,
        onChange,
        touched,
    })

    const { repoMode, repoQuery, repositories } = useRepoFields({ formApi: form.formAPI })

    const title = useField({
        name: 'title',
        formApi: form.formAPI,
        validators: { sync: insightTitleValidator },
    })

    const series = useField({
        name: 'series',
        formApi: form.formAPI,
        validators: { sync: insightSeriesValidator },
    })

    const step = useField({
        name: 'step',
        formApi: form.formAPI,
    })

    const stepValue = useField({
        name: 'stepValue',
        formApi: form.formAPI,
        validators: { sync: insightStepValueValidator },
    })

    return {
        form,
        title,
        repositories,
        repoQuery,
        repoMode,
        series,
        step,
        stepValue,
    }
}
