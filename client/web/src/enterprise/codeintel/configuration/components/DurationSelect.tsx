import { type FunctionComponent, useState } from 'react'

import classNames from 'classnames'
import { intervalToDuration, formatDuration } from 'date-fns'

import { Select, Input } from '@sourcegraph/wildcard'

import { defaultDurationValues } from '../shared'

export interface DurationSelectProps {
    id: string
    value: string | null
    disabled?: boolean
    onChange?: (value: number | null) => void
    durationValues?: { value: number; displayText: string }[]
    className?: string
}

const toInt = (value: string): number | null => Math.floor(parseInt(value, 10)) || null

export const maxDuration = 87600

const MS_IN_HOURS = 1000 * 60 * 60

export const DurationSelect: FunctionComponent<DurationSelectProps> = ({
    id,
    value,
    disabled,
    onChange,
    durationValues = defaultDurationValues,
    className,
}) => {
    const customValueInHours = toInt(value || '') || 0
    const customValueMilliseconds = customValueInHours * MS_IN_HOURS
    const durationHint = formatDuration(intervalToDuration({ start: 0, end: customValueMilliseconds }))
    const [isCustom, setIsCustom] = useState(!durationValues.map(({ value }) => value).includes(customValueInHours))

    return (
        <>
            <div className="input-group">
                <Select
                    aria-label=""
                    id={id}
                    className={classNames('flex-1 mb-0', className)}
                    value={isCustom ? 'custom' : value || undefined}
                    disabled={disabled}
                    onChange={event => {
                        if (event.target.value === 'custom') {
                            setIsCustom(true)
                        } else {
                            setIsCustom(false)

                            onChange?.(toInt(event.target.value))
                        }
                    }}
                >
                    {durationValues.map(({ value, displayText }) => (
                        <option key={value} value={value || undefined}>
                            {displayText}
                        </option>
                    ))}

                    <option value="custom">Custom</option>
                </Select>
                {isCustom && (
                    <>
                        <Input
                            type="number"
                            className="ml-2"
                            value={customValueInHours}
                            min="1"
                            max={maxDuration}
                            disabled={disabled}
                            onChange={event => onChange?.(toInt(event.target.value) ?? 0)}
                        />

                        <div className="input-group-append">
                            <span className="input-group-text">hour{customValueInHours !== 1 && <>s</>}</span>
                        </div>
                    </>
                )}
            </div>
            {isCustom && (
                <div className="text-right">
                    &nbsp;
                    {customValueInHours === null ? (
                        <small className="text-danger">Please supply a value.</small>
                    ) : durationHint.match(`^${customValueInHours} hours?$`) ? (
                        <></>
                    ) : customValueInHours <= 0 ? (
                        <small className="text-danger">Please supply a positive value.</small>
                    ) : customValueInHours > maxDuration ? (
                        <small className="text-danger">Please supply a value no greater than {maxDuration}.</small>
                    ) : (
                        <small className="text-muted">{durationHint}</small>
                    )}
                </div>
            )}
        </>
    )
}
