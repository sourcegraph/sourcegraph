import classNames from 'classnames'
import React, { ChangeEvent, FocusEventHandler, forwardRef, ReactNode } from 'react'

import { InsightStep } from '../../../pages/insights/creation/search-insight/types'
import { FormGroup } from '../../form/form-group/FormGroup'
import { FormInput } from '../../form/form-input/FormInput'
import { FormRadioInput } from '../../form/form-radio-input/FormRadioInput'

import styles from './CodeInsightTimeStepPicker.module.scss'

interface CodeInsightTimeStepPickerProps {
    value: string | number
    numberOfPoints: number
    name?: string
    valid?: boolean
    disabled?: boolean
    error?: string
    errorInputState?: boolean

    onChange: (event: ChangeEvent<HTMLInputElement>) => void
    onFocus?: FocusEventHandler<HTMLInputElement>
    onBlur: FocusEventHandler<HTMLInputElement>

    stepType: InsightStep
    onStepTypeChange: (event: ChangeEvent<HTMLInputElement>) => void
}

export const CodeInsightTimeStepPicker = forwardRef<HTMLInputElement, CodeInsightTimeStepPickerProps>(
    (props, reference) => {
        const {
            error,
            errorInputState,
            valid,
            disabled,
            name,
            value,
            stepType,
            numberOfPoints,
            onChange,
            onStepTypeChange,
            onBlur,
            onFocus,
        } = props

        return (
            <FormGroup
                name="insight step group"
                title="Granularity: distance between data points"
                description={getDescriptionText({ stepValue: +value, stepType, numberOfPoints })}
                error={error}
                className="mt-4"
                labelClassName={styles.groupLabel}
                contentClassName="d-flex flex-wrap mb-n2"
            >
                <FormInput
                    placeholder="ex. 2"
                    required={true}
                    type="number"
                    min={1}
                    disabled={disabled}
                    valid={valid}
                    ref={reference}
                    name={name}
                    value={value}
                    errorInputState={errorInputState}
                    onChange={onChange}
                    onBlur={onBlur}
                    onFocus={onFocus}
                    className={classNames(styles.stepInput)}
                />

                <FormRadioInput
                    title="Hours"
                    name="step"
                    value="hours"
                    checked={stepType === 'hours'}
                    onChange={onStepTypeChange}
                    disabled={disabled}
                    className="mr-3"
                />
                <FormRadioInput
                    title="Days"
                    name="step"
                    value="days"
                    checked={stepType === 'days'}
                    onChange={onStepTypeChange}
                    disabled={disabled}
                    className="mr-3"
                />
                <FormRadioInput
                    title="Weeks"
                    name="step"
                    value="weeks"
                    checked={stepType === 'weeks'}
                    onChange={onStepTypeChange}
                    disabled={disabled}
                    className="mr-3"
                />
                <FormRadioInput
                    title="Months"
                    name="step"
                    value="months"
                    checked={stepType === 'months'}
                    onChange={onStepTypeChange}
                    disabled={disabled}
                    className="mr-3"
                />
                <FormRadioInput
                    title="Years"
                    name="step"
                    value="years"
                    checked={stepType === 'years'}
                    onChange={onStepTypeChange}
                    disabled={disabled}
                    className="mr-3"
                />
            </FormGroup>
        )
    }
)

interface DescriptionTextOptions {
    numberOfPoints: number
    stepType: InsightStep
    stepValue: number
}

function getDescriptionText(options: DescriptionTextOptions): ReactNode {
    const { stepType, stepValue, numberOfPoints } = options
    // Remove s at the end of stepType value, in the singular. We need to do this
    // because Intl accepts only singular value of units.
    const unit = stepType.slice(0, -1)

    const pastUnits = (stepValue * numberOfPoints - 1).toLocaleString('en-GB', {
        unit,
        style: 'unit',
        unitDisplay: 'long',
    })

    const everyUnit = stepValue.toLocaleString('en-GB', {
        unit,
        style: 'unit',
        unitDisplay: 'long',
    })

    const everyUnitText = stepValue < 2 ? everyUnit.slice(2) : everyUnit

    return (
        <span>
            The prototype supports timeframe up to {numberOfPoints} datapoints: past <b>{pastUnits} of data</b>, one
            datapoint every {everyUnitText}.
        </span>
    )
}
