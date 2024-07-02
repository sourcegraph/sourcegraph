import type { FC, ReactNode } from 'react'

import classNames from 'classnames'
import { noop } from 'rxjs'

import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { FilterType } from '@sourcegraph/shared/src/search/query/filters'
import { findFilter, FilterKind } from '@sourcegraph/shared/src/search/query/query'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import { Button, Card, Input, Code, useForm, useField, getDefaultInputProps } from '@sourcegraph/wildcard'

import { defaultPatternTypeFromSettings } from '../../../../../../../util/settings'
import { DEFAULT_DATA_SERIES_COLOR } from '../../../../../constants'
import { InsightQueryInput } from '../../../../form'
import type { EditableDataSeries } from '../../types'
import { FormColorInput } from '../form-color-input/FormColorInput'

import { getQueryPatternTypeFilter } from './get-pattern-type-filter'
import { SERIES_NAME_VALIDATORS, SERIES_QUERY_VALIDATORS } from './validators'

interface FormSeriesInputProps {
    series: EditableDataSeries

    /** Series index. */
    index: number

    /** Show all validation error of all fields within the form. */
    showValidationErrorsOnMount?: boolean

    /**
     * Code Insight repositories field string value - repo1, repo2, ...
     * This prop is used in order to generate a proper link for the query preview button.
     */
    repositories: string[]

    /**
     * Code Insight repoQuery field string value - repo:github.com/sourcegraph/*
     * This prop is used in order to generate a proper link for the query preview button.
     */
    repoQuery: string | null

    /**
     * This field is only needed for specifying a special compute-specific
     * query field description when this component is used on the compute-powered insight.
     * This prop should be removed when we will have a better form series management
     * solution, see https://github.com/sourcegraph/sourcegraph/issues/38236
     */
    queryFieldDescription?: ReactNode

    /** Enable autofocus behavior of the first input element of series form. */
    autofocus?: boolean

    /** Enable cancel button. */
    cancel?: boolean

    /** Custom class name for root element of form series. */
    className?: string

    /** Whenever a user clicks submit (done) button of the series form. */
    onSubmit?: (series: EditableDataSeries) => void

    /** Whenever a user clicks cancel button of the series form. */
    onCancel?: () => void

    /** Whenever a user types new values in any field of the series form. */
    onChange?: (formValues: EditableDataSeries, valid: boolean) => void
}

export const FormSeriesInput: FC<FormSeriesInputProps> = props => {
    const {
        index,
        series,
        showValidationErrorsOnMount = false,
        className,
        cancel = false,
        autofocus = true,
        repoQuery,
        repositories,
        queryFieldDescription,
        onCancel = noop,
        onSubmit = noop,
        onChange = noop,
    } = props

    const { name, query, stroke: color } = series

    const hasPatternType = (query: string): boolean => {
        const patternType = findFilter(query, FilterType.patterntype, FilterKind.Global)
        return patternType?.value !== undefined
    }

    const defaultPatternType: SearchPatternType = defaultPatternTypeFromSettings(useSettingsCascade())

    const { formAPI, handleSubmit, ref } = useForm({
        touched: showValidationErrorsOnMount,
        initialValues: {
            seriesName: name ?? '',
            seriesQuery: query ?? '',
            seriesColor: color ?? DEFAULT_DATA_SERIES_COLOR,
        },
        onSubmit: values =>
            onSubmit({
                ...series,
                name: values.seriesName,
                query: hasPatternType(values.seriesQuery)
                    ? values.seriesQuery
                    : `patterntype:${defaultPatternType} ${values.seriesQuery}`,
                stroke: values.seriesColor,
            }),
        onChange: event => {
            const { values } = event

            onChange(
                {
                    ...series,
                    name: values.seriesName,
                    query: values.seriesQuery,
                    stroke: values.seriesColor,
                },
                event.valid
            )
        },
    })

    const nameField = useField({
        name: 'seriesName',
        formApi: formAPI,
        validators: { sync: SERIES_NAME_VALIDATORS },
    })

    const queryField = useField({
        name: 'seriesQuery',
        formApi: formAPI,
        validators: { sync: SERIES_QUERY_VALIDATORS },
    })

    const colorField = useField({
        name: 'seriesColor',
        formApi: formAPI,
    })

    return (
        <Card data-testid="series-form" ref={ref} className={classNames('d-flex flex-column', className)}>
            <Input
                label="Name"
                required={true}
                autoFocus={autofocus}
                placeholder="Example: Function component"
                message="Name shown in the legend and tooltip"
                {...getDefaultInputProps(nameField)}
            />

            <Input
                label="Search query"
                required={true}
                as={InsightQueryInput}
                repoQuery={repoQuery}
                repositories={repositories}
                patternType={getQueryPatternTypeFilter(queryField.input.value, defaultPatternType)}
                placeholder="Example: patternType:regexp const\s\w+:\s(React\.)?FunctionComponent"
                message={
                    queryFieldDescription ?? (
                        <span>
                            Do not include <Code>context:</Code> <Code>repo:</Code> or <Code>rev:</Code> filters; if
                            needed, <Code>repo:</Code> will be added automatically.
                        </span>
                    )
                }
                className="mt-4"
                {...getDefaultInputProps(queryField)}
            />

            <FormColorInput
                name={`color group of ${index} series`}
                title="Color"
                className="mt-4"
                value={colorField.input.value}
                onChange={colorField.input.onChange}
            />

            <div className="mt-4">
                <Button
                    aria-label="Submit button for data series"
                    type="button"
                    variant="secondary"
                    onClick={handleSubmit}
                >
                    Done
                </Button>

                {cancel && (
                    <Button type="button" onClick={onCancel} variant="secondary" outline={true} className="ml-2">
                        Cancel
                    </Button>
                )}
            </div>
        </Card>
    )
}
